package httpapi

import (
	"encoding/json"
	"net/http"
)

// POST /api/rooms/{id}/signal — эфемерные сигналы ведущего клиентам комнаты
// (управление воспроизведением медиа на ТВ). В игровой журнал не пишется.
func (s *Server) postSignal(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	var body struct {
		Kind string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "некорректное тело запроса")
		return
	}
	switch body.Kind {
	case "media_pause", "media_play":
	default:
		writeErr(w, 400, "BAD_REQUEST", "неизвестный тип сигнала")
		return
	}
	msg, _ := json.Marshal(map[string]any{"type": "signal", "kind": body.Kind})
	s.hub.Broadcast(rc.ID, msg, msg)
	writeJSON(w, 200, map[string]any{"ok": true})
}
