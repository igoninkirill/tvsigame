package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// POST /api/rooms/{id}/players — добавление игрока (ведущий).
func (s *Server) addPlayer(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	var body struct {
		Name     string `json:"name"`
		Score    int64  `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len([]rune(body.Name)) == 0 {
		writeErr(w, 400, "BAD_REQUEST", "укажите имя игрока")
		return
	}
	if len([]rune(body.Name)) > 40 {
		writeErr(w, 400, "BAD_REQUEST", "имя слишком длинное (макс. 40 символов)")
		return
	}
	var id string
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO players (room_id, name, score, position)
		VALUES ($1, $2, $3, COALESCE((SELECT MAX(position) + 1 FROM players WHERE room_id = $1), 0))
		RETURNING id`, rc.ID, body.Name, body.Score,
	).Scan(&id)
	if err != nil {
		mapErr(w, err)
		return
	}
	s.broadcastNow(rc.ID)
	writeJSON(w, 201, map[string]any{"id": id})
}

// PATCH /api/rooms/{id}/players/{playerId} — имя, порядок, активность.
func (s *Server) updatePlayer(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	playerID := chi.URLParam(r, "playerID")
	var body struct {
		Name     *string `json:"name"`
		Position *int    `json:"position"`
		IsActive *bool   `json:"isActive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "некорректное тело запроса")
		return
	}
	if body.Name != nil {
		if len([]rune(*body.Name)) == 0 || len([]rune(*body.Name)) > 40 {
			writeErr(w, 400, "BAD_REQUEST", "некорректное имя (1–40 символов)")
			return
		}
		if _, err := s.pool.Exec(r.Context(),
			`UPDATE players SET name = $3 WHERE id = $1 AND room_id = $2`,
			playerID, rc.ID, *body.Name); err != nil {
			mapErr(w, err)
			return
		}
	}
	if body.Position != nil {
		if _, err := s.pool.Exec(r.Context(),
			`UPDATE players SET position = $3 WHERE id = $1 AND room_id = $2`,
			playerID, rc.ID, *body.Position); err != nil {
			mapErr(w, err)
			return
		}
	}
	if body.IsActive != nil {
		if _, err := s.pool.Exec(r.Context(),
			`UPDATE players SET is_active = $3 WHERE id = $1 AND room_id = $2`,
			playerID, rc.ID, *body.IsActive); err != nil {
			mapErr(w, err)
			return
		}
	}
	s.broadcastNow(rc.ID)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/rooms/{id}/players/{playerId}
func (s *Server) deletePlayer(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	if rc.Status == "active" || rc.Status == "paused" {
		writeErr(w, 409, "GAME_ACTIVE", "нельзя удалять игроков во время игры")
		return
	}
	tag, err := s.pool.Exec(r.Context(),
		`DELETE FROM players WHERE id = $1 AND room_id = $2`,
		chi.URLParam(r, "playerID"), rc.ID)
	if err != nil {
		mapErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "NOT_FOUND", "игрок не найден")
		return
	}
	s.broadcastNow(rc.ID)
	writeJSON(w, 200, map[string]any{"ok": true})
}
