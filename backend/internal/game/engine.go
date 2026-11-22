package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krutokir/svoya-igra/server/internal/model"
	"github.com/krutokir/svoya-igra/server/internal/snapshot"
)

// Error — ошибка валидации команды, возвращается клиенту с кодом.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func fail(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ErrVersionConflict — клиент прислал устаревшую expected_state_version.
var ErrVersionConflict = &Error{Code: "VERSION_CONFLICT", Message: "устаревшая версия состояния — обновите данные"}

// Command — игровая команда.
type Command struct {
	ActionID        string          `json:"action_id"`
	Type            string          `json:"command_type"`
	Payload         json.RawMessage `json:"payload"`
	ExpectedVersion *int64          `json:"expected_state_version"`
	Actor           string          `json:"-"` // host | system
	Internal        bool            `json:"-"` // внутренние команды движка (таймеры, автопауза)
}

// Outcome — результат выполнения команды.
type Outcome struct {
	Version    int64            `json:"version"`
	Duplicated bool             `json:"duplicated,omitempty"`
	Events     []map[string]any `json:"events,omitempty"`
}

// Broadcaster — отправка снапшотов клиентам комнаты (host/display редакции).
type Broadcaster interface {
	Broadcast(roomID string, hostMsg, displayMsg []byte)
}

type Engine struct {
	pool   *pgxpool.Pool
	hub    Broadcaster
	timers *TimerManager
}

func NewEngine(pool *pgxpool.Pool, hub Broadcaster) *Engine {
	e := &Engine{pool: pool, hub: hub}
	e.timers = NewTimerManager(e.onTimerExpire)
	return e
}

// Timers exposes the manager for startup recovery.
func (e *Engine) Timers() *TimerManager { return e.timers }

// onTimerExpire — обратный вызов TimerManager. Обрабатывается ровно один раз
// благодаря идемпотентному action_id вида "timer-{id}".
func (e *Engine) onTimerExpire(roomID string, timerID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]any{"timerId": timerID})
	_, err := e.Execute(ctx, roomID, Command{
		ActionID: fmt.Sprintf("timer-%d", timerID),
		Type:     CmdTimerExpired,
		Payload:  payload,
		Actor:    "system",
		Internal: true,
	})
	if err != nil {
		var ge *Error
		if !errors.As(err, &ge) || ge.Code != "DUPLICATE" {
			fmt.Printf("timer expire room=%s timer=%d: %v\n", roomID, timerID, err)
		}
	}
}

// AutoPause ставит игру на паузу при потере ведущего (вызывается WS-слоем).
func (e *Engine) AutoPause(roomID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = e.Execute(ctx, roomID, Command{
		ActionID: fmt.Sprintf("auto-pause-%d", time.Now().UnixNano()),
		Type:     CmdPauseGame,
		Payload:  json.RawMessage(`{"reason":"host_disconnected"}`),
		Actor:    "system",
		Internal: true,
	})
}

// Execute — главная точка входа: валидация, идемпотентность, атомарное
// применение, журнал событий, рассылка снапшота и перевзвод таймера.
func (e *Engine) Execute(ctx context.Context, roomID string, cmd Command) (*Outcome, error) {
	if cmd.ActionID == "" {
		return nil, fail("BAD_REQUEST", "action_id обязателен")
	}
	if cmd.Actor == "" {
		cmd.Actor = "host"
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	// Блокируем строку комнаты — сериализуем команды внутри комнаты.
	var status string
	var packID *string
	err = tx.QueryRow(ctx,
		`SELECT status, pack_id FROM rooms WHERE id = $1 FOR UPDATE`, roomID,
	).Scan(&status, &packID)
	if err != nil {
		return nil, fail("NOT_FOUND", "комната не найдена")
	}

	// Идемпотентность: повтор команды не применяется повторно.
	var dupVersion int64
	err = tx.QueryRow(ctx,
		`SELECT prev_version FROM game_actions WHERE room_id = $1 AND action_id = $2`,
		roomID, cmd.ActionID,
	).Scan(&dupVersion)
	if err == nil {
		return &Outcome{Version: dupVersion + 1, Duplicated: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	// Текущее состояние.
	var stateRaw []byte
	var version int64
	err = tx.QueryRow(ctx,
		`SELECT state, state_version FROM game_states WHERE room_id = $1`, roomID,
	).Scan(&stateRaw, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		init := InitialState()
		stateRaw, _ = json.Marshal(init)
		version = 0
		if _, err := tx.Exec(ctx,
			`INSERT INTO game_states (room_id, state, state_version) VALUES ($1, $2, 0)`,
			roomID, stateRaw); err != nil {
			return nil, fmt.Errorf("init state: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	// Проверка версии состояния (защита от устаревших команд).
	if cmd.ExpectedVersion != nil && *cmd.ExpectedVersion != version {
		return nil, ErrVersionConflict
	}

	var st State
	if err := json.Unmarshal(stateRaw, &st); err != nil {
		return nil, fmt.Errorf("corrupt state: %w", err)
	}
	normalizeState(&st)

	// Пак вопросов (нужен большинству команд).
	var pack *model.Pack
	if packID != nil {
		var parsed []byte
		if err := tx.QueryRow(ctx, `SELECT parsed_data FROM packs WHERE id = $1`, *packID).Scan(&parsed); err == nil {
			var p model.Pack
			if json.Unmarshal(parsed, &p) == nil {
				pack = &p
			}
		}
	}

	// Текущие счета игроков (снапшот для отмены). Важно: маршалим копию
	// ДО применения команды — addScore мутирует эту мапу в процессе.
	prevScores, err := loadScores(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	prevScoresRaw, _ := json.Marshal(prevScores)

	// Контекст применения команды.
	ac := &applyCtx{
		ctx: ctx, tx: tx, roomID: roomID, cmd: cmd,
		state: &st, pack: pack, scores: prevScores, roomStatus: status,
		events: []map[string]any{},
	}

	if err := ac.apply(); err != nil {
		return nil, err
	}

	// Сохранение состояния.
	newRaw, err := json.Marshal(&st)
	if err != nil {
		return nil, err
	}
	newVersion := version + 1
	if _, err := tx.Exec(ctx,
		`UPDATE game_states SET state = $2, state_version = $3, updated_at = now() WHERE room_id = $1`,
		roomID, newRaw, newVersion); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	// Журнал событий (неизменяемый). sequence_number — монотонный в рамках
	// комнаты; строка комнаты уже заблокирована, гонок нет.
	var seq int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence_number), 0) FROM game_events WHERE room_id = $1`,
		roomID).Scan(&seq); err != nil {
		return nil, fmt.Errorf("event seq: %w", err)
	}
	for _, ev := range ac.events {
		seq++
		if _, err := tx.Exec(ctx, `
			INSERT INTO game_events (room_id, sequence_number, event_type, actor, payload, state_version)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			roomID, seq, ev["type"], cmd.Actor, ev["payload"], newVersion); err != nil {
			return nil, fmt.Errorf("save event: %w", err)
		}
	}

	// Запись о команде (идемпотентность + данные для отмены).
	resultRaw, _ := json.Marshal(map[string]any{"ok": true})
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_actions (room_id, action_id, command_type, prev_state, prev_scores, prev_version, result)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		roomID, cmd.ActionID, cmd.Type, stateRaw, prevScoresRaw, version, resultRaw); err != nil {
		return nil, fmt.Errorf("save action: %w", err)
	}

	// Статус комнаты.
	if ac.newRoomStatus != "" && ac.newRoomStatus != status {
		if _, err := tx.Exec(ctx,
			`UPDATE rooms SET status = $2, updated_at = now() WHERE id = $1`,
			roomID, ac.newRoomStatus); err != nil {
			return nil, fmt.Errorf("update room status: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// После коммита: рассылка и таймеры.
	hostMsg, displayMsg, err := snapshot.Build(ctx, e.pool, roomID, ac.events)
	if err == nil && e.hub != nil {
		e.hub.Broadcast(roomID, hostMsg, displayMsg)
	}
	e.timers.Arm(roomID, st.Timer)

	return &Outcome{Version: newVersion, Events: ac.events}, nil
}

// normalizeState приводит загруженное из JSON состояние к каноническому виду:
// пустые срезы/мапы не должны быть nil (иначе в JSON уходят null, а клиенты
// ожидают массивы/объекты).
func normalizeState(st *State) {
	if st.UsedCells == nil {
		st.UsedCells = map[string]bool{}
	}
	if cur := st.Current; cur != nil {
		if cur.AnsweredIDs == nil {
			cur.AnsweredIDs = []string{}
		}
		if cur.Scenario == nil {
			cur.Scenario = []model.Atom{}
		}
	}
	if auc := st.Auction; auc != nil && auc.PassedIDs == nil {
		auc.PassedIDs = []string{}
	}
	if intro := st.Intro; intro != nil && intro.ThemeNames == nil {
		intro.ThemeNames = []string{}
	}
	if fin := st.Final; fin != nil {
		if fin.ThemeNames == nil {
			fin.ThemeNames = []string{}
		}
		if fin.Excluded == nil {
			fin.Excluded = []int{}
		}
		if fin.FinalistIDs == nil {
			fin.FinalistIDs = []string{}
		}
		if fin.Stakes == nil {
			fin.Stakes = map[string]int64{}
		}
		if fin.Answers == nil {
			fin.Answers = map[string]bool{}
		}
		if fin.Question != nil {
			if fin.Question.AnsweredIDs == nil {
				fin.Question.AnsweredIDs = []string{}
			}
			if fin.Question.Scenario == nil {
				fin.Question.Scenario = []model.Atom{}
			}
		}
	}
}

// loadScores возвращает карту playerID -> score.
func loadScores(ctx context.Context, tx pgx.Tx, roomID string) (map[string]int64, error) {
	rows, err := tx.Query(ctx, `SELECT id, score FROM players WHERE room_id = $1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var score int64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, err
		}
		out[id] = score
	}
	return out, rows.Err()
}

// activePlayerIDs возвращает id активных игроков в порядке позиции.
func activePlayerIDs(ctx context.Context, tx pgx.Tx, roomID string) ([]string, error) {
	rows, err := tx.Query(ctx,
		`SELECT id FROM players WHERE room_id = $1 AND is_active ORDER BY position, created_at`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
