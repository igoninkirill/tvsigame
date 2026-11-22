package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"

	"github.com/krutokir/svoya-igra/server/internal/auth"
	"github.com/krutokir/svoya-igra/server/internal/roomcode"
	"github.com/krutokir/svoya-igra/server/internal/snapshot"
	"github.com/krutokir/svoya-igra/server/internal/ws"
)
// POST /api/rooms — создание комнаты с ТВ.
func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	displayToken, err := auth.NewToken()
	if err != nil {
		mapErr(w, err)
		return
	}
	var id string
	for attempt := 0; attempt < 5; attempt++ {
		code, err := roomcode.Generate(5)
		if err != nil {
			mapErr(w, err)
			return
		}
		expires := time.Now().Add(time.Duration(s.cfg.RoomTTLHours) * time.Hour)
		err = s.pool.QueryRow(r.Context(), `
			INSERT INTO rooms (room_code, display_token_hash, expires_at)
			VALUES ($1, $2, $3) RETURNING id`,
			code, auth.Hash(displayToken), expires,
		).Scan(&id)
		if err == nil {
			writeJSON(w, 201, map[string]any{
				"roomId": id, "code": code, "displayToken": displayToken,
				"displayUrl": "/room/" + id + "/display",
			})
			return
		}
		// Коллизия кода — пробуем ещё раз.
		if !isUniqueViolation(err) {
			mapErr(w, err)
			return
		}
	}
	writeErr(w, 500, "CODE_COLLISION", "не удалось сгенерировать уникальный код комнаты")
}

// POST /api/rooms/join — обмен кода комнаты на host_token (rate-limited).
func (s *Server) joinByCode(w http.ResponseWriter, r *http.Request) {
	if !s.rl.Allow(clientIP(r)) {
		writeErr(w, 429, "RATE_LIMIT", "слишком много попыток — подождите минуту")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeErr(w, 400, "BAD_REQUEST", "укажите код комнаты")
		return
	}
	var roomID, status string
	var hasHost bool
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, status, host_token_hash IS NOT NULL
		FROM rooms WHERE room_code = $1 AND status NOT IN ('expired')
		  AND expires_at > now()`,
		normalizeCode(body.Code),
	).Scan(&roomID, &status, &hasHost)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, 404, "ROOM_NOT_FOUND", "комната с таким кодом не найдена")
		return
	}
	if err != nil {
		mapErr(w, err)
		return
	}
	if status == "finished" {
		writeErr(w, 409, "ROOM_FINISHED", "игра в этой комнате уже завершена")
		return
	}
	// Код — первичный способ подключения; повторное получение доступа по коду
	// разрешено (механизм восстановления доступа), токен перевыпускается.
	hostToken, err := auth.NewToken()
	if err != nil {
		mapErr(w, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(), `
		UPDATE rooms SET host_token_hash = $2, updated_at = now(),
		       status = CASE WHEN status = 'waiting_for_host' THEN 'configuring' ELSE status END
		WHERE id = $1`, roomID, auth.Hash(hostToken)); err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"roomId": roomID, "hostToken": hostToken, "hostUrl": "/room/" + roomID + "/host",
	})
}

// GET /api/rooms/{id}
func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	writeJSON(w, 200, map[string]any{
		"id": rc.ID, "code": rc.Code, "status": rc.Status, "role": r.Context().Value(ctxRole),
	})
}

// GET /api/rooms/{id}/state — полный снапшот (для восстановления после реконнекта).
func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(ctxRole).(string)
	hostMsg, displayMsg, err := snapshot.Build(r.Context(), s.pool, roomOf(r).ID, nil)
	if err != nil {
		mapErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if role == "host" {
		_, _ = w.Write(hostMsg)
	} else {
		_, _ = w.Write(displayMsg)
	}
}

// GET /api/rooms/{id}/events — журнал событий (история начислений и действий).
func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT sequence_number, event_type, actor, payload, state_version, created_at
		FROM game_events WHERE room_id = $1 ORDER BY sequence_number DESC LIMIT 200`,
		roomOf(r).ID)
	if err != nil {
		mapErr(w, err)
		return
	}
	defer rows.Close()
	events := []map[string]any{}
	for rows.Next() {
		var seq, version int64
		var typ, actor string
		var payload []byte
		var created time.Time
		if err := rows.Scan(&seq, &typ, &actor, &payload, &version, &created); err != nil {
			mapErr(w, err)
			return
		}
		var p any
		_ = json.Unmarshal(payload, &p)
		events = append(events, map[string]any{
			"seq": seq, "type": typ, "actor": actor, "payload": p,
			"stateVersion": version, "at": created,
		})
	}
	writeJSON(w, 200, map[string]any{"events": events})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // LAN-сценарий; сужать на уровне прокси
}

// GET /ws/room/{id} — WebSocket для ТВ и ведущего.
// Аутентификация — первым сообщением {"type":"auth","role","token","clientId"}.
func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	authFn := func(msg []byte) (ws.Role, string, error) {
		var m struct {
			Type     string `json:"type"`
			Role     string `json:"role"`
			Token    string `json:"token"`
			ClientID string `json:"clientId"`
		}
		if err := json.Unmarshal(msg, &m); err != nil || m.Type != "auth" {
			return "", "", errors.New("первым сообщением ожидается auth")
		}
		if m.Token == "" {
			return "", "", errors.New("токен обязателен")
		}
		var hostHash, displayHash *string
		err := s.pool.QueryRow(r.Context(),
			`SELECT host_token_hash, display_token_hash FROM rooms WHERE id = $1`, roomID,
		).Scan(&hostHash, &displayHash)
		if err != nil {
			return "", "", errors.New("комната не найдена")
		}

		switch ws.Role(m.Role) {
		case ws.RoleDisplay:
			if displayHash == nil || !auth.Match(m.Token, *displayHash) {
				return "", "", errors.New("недействительный токен ТВ")
			}
			return ws.RoleDisplay, "", nil
		case ws.RoleHost:
			if hostHash == nil || !auth.Match(m.Token, *hostHash) {
				return "", "", errors.New("недействительный токен ведущего")
			}
			if m.ClientID == "" {
				return "", "", errors.New("clientId обязателен для ведущего")
			}
			// Один ведущий: другое устройство при живой сессии отклоняется.
			if onlineID, online := s.hub.HostOnline(roomID); online && onlineID != m.ClientID {
				return "", "", errors.New("ведущий уже подключён с другого устройства")
			}
			return ws.RoleHost, m.ClientID, nil
		default:
			return "", "", errors.New("неизвестная роль")
		}
	}

	hostMsg, displayMsg, err := snapshot.Build(r.Context(), s.pool, roomID, nil)
	if err != nil {
		conn.Close()
		return
	}

	onAuthed := func(role ws.Role, clientID string) {
		if role != ws.RoleHost {
			return
		}
		s.graces.Cancel(roomID) // ведущий вернулся в течение grace-периода
		ctx, cancel := contextWithTimeout()
		defer cancel()
		_, _ = s.pool.Exec(ctx, `
			UPDATE rooms SET host_connected = TRUE, host_client_id = $2, host_last_seen_at = now(),
			       status = CASE WHEN status = 'waiting_for_host' THEN 'configuring' ELSE status END
			WHERE id = $1`, roomID, clientID)
		s.broadcastNow(roomID)
	}

	ws.Serve(s.hub, conn, roomID, authFn, hostMsg, displayMsg, onAuthed)
}

func (s *Server) broadcastNow(roomID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hostMsg, displayMsg, err := snapshot.Build(ctx, s.pool, roomID, nil)
	if err != nil {
		return
	}
	s.hub.Broadcast(roomID, hostMsg, displayMsg)
}

func isUniqueViolation(err error) bool {
	type sqlState interface {
		SQLState() string
	}
	var se sqlState
	return errors.As(err, &se) && se.SQLState() == "23505"
}

func normalizeCode(code string) string {
	out := make([]byte, 0, len(code))
	for _, c := range []byte(code) {
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		}
	}
	return string(out)
}
