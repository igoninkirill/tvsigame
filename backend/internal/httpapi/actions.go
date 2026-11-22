package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/krutokir/svoya-igra/server/internal/game"
)

// POST /api/rooms/{id}/actions — универсальный эндпоинт игровых команд.
//
// Тело: {action_id, command_type, payload, expected_state_version}.
// Команды валидируются игровым движком; повтор action_id не применяется.
func (s *Server) postAction(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	var cmd game.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeErr(w, 400, "BAD_REQUEST", "некорректное тело запроса")
		return
	}
	cmd.Actor = "host"
	outcome, err := s.engine.Execute(r.Context(), rc.ID, cmd)
	if err != nil {
		mapErr(w, err)
		return
	}
	writeJSON(w, 200, outcome)
}
