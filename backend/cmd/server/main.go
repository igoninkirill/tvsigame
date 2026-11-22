// Точка входа backend «Своей Игры».
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krutokir/svoya-igra/server/internal/config"
	"github.com/krutokir/svoya-igra/server/internal/db"
	"github.com/krutokir/svoya-igra/server/internal/game"
	"github.com/krutokir/svoya-igra/server/internal/httpapi"
	"github.com/krutokir/svoya-igra/server/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	if err := os.MkdirAll(cfg.MediaDir, 0o755); err != nil {
		log.Fatalf("media dir: %v", err)
	}

	hub := ws.NewHub()
	engine := game.NewEngine(pool, hub)
	server := httpapi.NewServer(cfg, pool, engine, hub) // связывает hub ↔ grace-паузу

	// Восстановление после перезапуска backend:
	// 1) host-соединения не переживают рестарт — сбрасываем флаг подключения
	//    и ставим активные игры на паузу (ведущий переподключится и продолжит);
	// 2) перевзводим бегущие таймеры из сохранённого состояния.
	if err := recoverAfterRestart(ctx, pool, engine); err != nil {
		log.Printf("recovery: %v", err)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("shutdown complete")
}

func recoverAfterRestart(ctx context.Context, pool *pgxpool.Pool, engine *game.Engine) error {
	// Все активные игры без живого ведущего — на паузу.
	if _, err := pool.Exec(ctx, `
		UPDATE rooms SET host_connected = FALSE, host_client_id = NULL,
		       status = CASE WHEN status = 'active' THEN 'paused' ELSE status END,
		       updated_at = now()
		WHERE status IN ('active', 'paused', 'configuring', 'ready')`); err != nil {
		return fmt.Errorf("reset host flags: %w", err)
	}

	// Перевзводим таймеры, которые бежали на момент остановки.
	rows, err := pool.Query(ctx,
		`SELECT room_id, state->'timer' FROM game_states WHERE state->'timer' IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("load timers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var roomID string
		var raw []byte
		if err := rows.Scan(&roomID, &raw); err != nil {
			return err
		}
		var t game.Timer
		if json.Unmarshal(raw, &t) == nil && t.Running {
			engine.Timers().Arm(roomID, &t)
		}
	}
	return rows.Err()
}
