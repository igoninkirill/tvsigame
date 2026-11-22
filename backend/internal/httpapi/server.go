// Package httpapi — REST API, WebSocket-эндпоинты и раздача статики.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krutokir/svoya-igra/server/internal/config"
	"github.com/krutokir/svoya-igra/server/internal/auth"
	"github.com/krutokir/svoya-igra/server/internal/game"
	"github.com/krutokir/svoya-igra/server/internal/ws"
)

type Server struct {
	cfg     config.Config
	pool    *pgxpool.Pool
	engine  *game.Engine
	hub     *ws.Hub
	rl      *rateLimiter
	graces  *graceTimers
}

func NewServer(cfg config.Config, pool *pgxpool.Pool, engine *game.Engine, hub *ws.Hub) *Server {
	s := &Server{
		cfg: cfg, pool: pool, engine: engine, hub: hub,
		rl:     newRateLimiter(10, time.Minute),
		graces: newGraceTimers(),
	}
	hub.SetHostHandlers(s.onHostGone, s.onHostAlive)
	return s
}

// onHostGone — у комнаты пропало host-подключение. Ждём grace-период:
// если ведущий не вернулся, помечаем его отключённым и ставим игру на паузу.
func (s *Server) onHostGone(roomID, clientID string) {
	s.graces.Schedule(roomID, time.Duration(s.cfg.HostGraceSec)*time.Second, func() {
		if onlineID, online := s.hub.HostOnline(roomID); online && onlineID != clientID {
			return // подключился другой ведущий — не наша забота
		} else if online {
			return // тот же ведущий переподключился
		}
		ctx, cancel := contextWithTimeout()
		defer cancel()
		tag, err := s.pool.Exec(ctx, `
			UPDATE rooms SET host_connected = FALSE, updated_at = now()
			WHERE id = $1 AND host_client_id = $2`, roomID, clientID)
		if err != nil || tag.RowsAffected() == 0 {
			return
		}
		s.broadcastNow(roomID)
		s.engine.AutoPause(roomID)
	})
}

// onHostAlive — heartbeat от ведущего: фиксируем last_seen.
func (s *Server) onHostAlive(roomID, clientID string) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	_, _ = s.pool.Exec(ctx,
		`UPDATE rooms SET host_last_seen_at = now() WHERE id = $1 AND host_client_id = $2`,
		roomID, clientID)
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(s.recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/rooms", s.createRoom)
		r.Post("/rooms/join", s.joinByCode) // обмен кода на host_token (rate-limited)

		r.Route("/rooms/{roomID}", func(r chi.Router) {
			r.Use(s.roomAuth) // Bearer display_token или host_token
			r.Get("/", s.getRoom)
			r.Get("/state", s.getState)
			r.Get("/events", s.getEvents)

			r.With(s.requireHost).Post("/players", s.addPlayer)
			r.With(s.requireHost).Patch("/players/{playerID}", s.updatePlayer)
			r.With(s.requireHost).Delete("/players/{playerID}", s.deletePlayer)
			r.With(s.requireHost).Post("/actions", s.postAction)
			r.With(s.requireHost).Post("/signal", s.postSignal)
			r.With(s.requireHost).Post("/pack/import", s.importPack)
			r.With(s.requireHost).Get("/packs", s.listPacks)
			r.With(s.requireHost).Post("/pack/select", s.selectPack)
			r.With(s.requireHost).Delete("/packs/{packID}", s.deletePack)
		})
	})

	// WebSocket: аутентификация первым сообщением, токены не попадают в URL/логи.
	r.Get("/ws/room/{roomID}", s.serveWS)

	// Медиа пакетов.
	r.Get("/media/*", s.serveMedia)

	// Статика фронтенда (SPA fallback).
	if s.cfg.StaticDir != "" {
		s.serveStatic(r)
	}
	return r
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

// mapErr переводит ошибки движка в HTTP-коды.
func mapErr(w http.ResponseWriter, err error) {
	var ge *game.Error
	if errors.As(err, &ge) {
		status := 400
		switch ge.Code {
		case "NOT_FOUND":
			status = 404
		case "VERSION_CONFLICT":
			status = 409
		case "DUPLICATE":
			status = 200
		}
		writeErr(w, status, ge.Code, ge.Message)
		return
	}
	log.Printf("internal error: %v", err)
	writeErr(w, 500, "INTERNAL", "внутренняя ошибка сервера")
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeErr(w, 500, "INTERNAL", "внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- контекст комнаты и аутентификация ---

type ctxKey string

const (
	ctxRoom ctxKey = "room"
	ctxRole ctxKey = "role"
)

type roomCtx struct {
	ID               string
	Code             string
	Status           string
	HostTokenHash    *string
	DisplayTokenHash *string
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// roomAuth загружает комнату и проверяет Bearer-токен (display или host).
func (s *Server) roomAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "roomID")
		var rc roomCtx
		var hostHash, displayHash *string
		err := s.pool.QueryRow(r.Context(),
			`SELECT id, room_code, status, host_token_hash, display_token_hash FROM rooms WHERE id = $1`,
			roomID,
		).Scan(&rc.ID, &rc.Code, &rc.Status, &hostHash, &displayHash)
		if err != nil {
			writeErr(w, 404, "NOT_FOUND", "комната не найдена")
			return
		}
		rc.HostTokenHash = hostHash
		rc.DisplayTokenHash = displayHash

		token := bearerToken(r)
		role := ""
		if token != "" && hostHash != nil && auth.Match(token, *hostHash) {
			role = "host"
		} else if token != "" && displayHash != nil && auth.Match(token, *displayHash) {
			role = "display"
		} else {
			writeErr(w, 401, "UNAUTHORIZED", "недействительный токен доступа")
			return
		}
		ctx := context.WithValue(r.Context(), ctxRoom, &rc)
		ctx = context.WithValue(ctx, ctxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxRole) != "host" {
			writeErr(w, 403, "FORBIDDEN", "требуются права ведущего")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func roomOf(r *http.Request) *roomCtx {
	rc, _ := r.Context().Value(ctxRoom).(*roomCtx)
	return rc
}

// --- rate limiter (per-IP) ---

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, entries: map[string][]time.Time{}}
}

func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	list := rl.entries[ip]
	kept := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.entries[ip] = kept
		return false
	}
	rl.entries[ip] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- grace timers для отключения ведущего ---

type graceTimers struct {
	mu sync.Mutex
	m  map[string]*time.Timer // roomID -> timer
}

func newGraceTimers() *graceTimers {
	return &graceTimers{m: map[string]*time.Timer{}}
}

func (g *graceTimers) Schedule(roomID string, d time.Duration, f func()) {
	g.Cancel(roomID)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.m[roomID] = time.AfterFunc(d, func() {
		g.mu.Lock()
		delete(g.m, roomID)
		g.mu.Unlock()
		f()
	})
}

func (g *graceTimers) Cancel(roomID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t, ok := g.m[roomID]; ok {
		t.Stop()
		delete(g.m, roomID)
	}
}

// --- статика и медиа ---

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/media/")
	clean := filepath.Clean("/" + rel) // нормализуем и отсекаем ../
	full := filepath.Join(s.cfg.MediaDir, strings.TrimPrefix(clean, "/"))
	if !strings.HasPrefix(full, filepath.Clean(s.cfg.MediaDir)+string(filepath.Separator)) {
		writeErr(w, 403, "FORBIDDEN", "недопустимый путь")
		return
	}
	http.ServeFile(w, r, full)
}

func (s *Server) serveStatic(r chi.Router) {
	fs := http.FileServer(http.Dir(s.cfg.StaticDir))
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		p := req.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/ws/") || strings.HasPrefix(p, "/media/") {
			writeErr(w, 404, "NOT_FOUND", "не найдено")
			return
		}
		// Файлы с расширением — статические ассеты; остальное — SPA-маршруты.
		if strings.Contains(p, ".") {
			fs.ServeHTTP(w, req)
			return
		}
		// SPA fallback: отдаём index.html для клиентских маршрутов.
		http.ServeFile(w, req, filepath.Join(s.cfg.StaticDir, "index.html"))
	})
}
