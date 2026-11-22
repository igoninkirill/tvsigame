package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/krutokir/svoya-igra/server/internal/siq"
)

// POST /api/rooms/{id}/pack/import — загрузка .siq (multipart, поле "file").
func (s *Server) importPack(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB<<20)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadMB << 20); err != nil {
		writeErr(w, 413, "TOO_LARGE", "файл слишком большой")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "BAD_REQUEST", "ожидается файл .siq в поле file")
		return
	}
	defer f.Close()

	packID := newUUID()
	tmpDir := filepath.Join(os.TempDir(), "siq-"+packID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		mapErr(w, err)
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "pack.siq")
	tmp, err := os.Create(tmpPath)
	if err != nil {
		mapErr(w, err)
		return
	}
	size, err := io.Copy(tmp, io.LimitReader(f, s.cfg.MaxUploadMB<<20))
	tmp.Close()
	if err != nil {
		mapErr(w, err)
		return
	}

	tmpFile, err := os.Open(tmpPath)
	if err != nil {
		mapErr(w, err)
		return
	}
	defer tmpFile.Close()

	res, err := siq.Import(tmpFile, size, packID, s.cfg.MediaDir)
	if err != nil {
		writeErr(w, 400, "IMPORT_FAILED", err.Error())
		return
	}

	parsed, err := json.Marshal(res.Pack)
	if err != nil {
		mapErr(w, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(), `
		INSERT INTO packs (id, name, author, version, parsed_data) VALUES ($1, $2, $3, $4, $5)`,
		packID, res.Pack.Name, res.Pack.Author, res.Pack.Version, parsed); err != nil {
		mapErr(w, err)
		return
	}
	// Привязываем пак к комнате.
	if _, err := s.pool.Exec(r.Context(),
		`UPDATE rooms SET pack_id = $2, updated_at = now() WHERE id = $1`, rc.ID, packID); err != nil {
		mapErr(w, err)
		return
	}
	s.broadcastNow(rc.ID)
	writeJSON(w, 201, map[string]any{
		"packId": packID, "name": res.Pack.Name, "author": res.Pack.Author,
		"version": res.Pack.Version, "rounds": len(res.Pack.Rounds),
		"warnings": res.Warnings,
	})
	_ = hdr
}

// GET /api/rooms/{id}/packs — библиотека пакетов.
func (s *Server) listPacks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, name, author, version, created_at,
		       COALESCE(parsed_data->>'logoUrl', '') AS logo_url,
		       jsonb_array_length(COALESCE(parsed_data->'rounds', '[]'::jsonb)) AS rounds
		FROM packs ORDER BY created_at DESC`)
	if err != nil {
		mapErr(w, err)
		return
	}
	defer rows.Close()
	packs := []map[string]any{}
	for rows.Next() {
		var id, name, author, version, logo string
		var created interface{}
		var rounds int
		if err := rows.Scan(&id, &name, &author, &version, &created, &logo, &rounds); err != nil {
			mapErr(w, err)
			return
		}
		packs = append(packs, map[string]any{
			"id": id, "name": name, "author": author, "version": version,
			"createdAt": created, "logoUrl": logo, "rounds": rounds,
		})
	}
	writeJSON(w, 200, map[string]any{"packs": packs})
}

// POST /api/rooms/{id}/pack/select — выбрать пак из библиотеки для комнаты.
func (s *Server) selectPack(w http.ResponseWriter, r *http.Request) {
	rc := roomOf(r)
	if rc.Status == "active" || rc.Status == "paused" {
		writeErr(w, 409, "GAME_ACTIVE", "нельзя менять пак во время игры")
		return
	}
	var body struct {
		PackID string `json:"packId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PackID == "" {
		writeErr(w, 400, "BAD_REQUEST", "укажите packId")
		return
	}
	tag, err := s.pool.Exec(r.Context(),
		`UPDATE rooms SET pack_id = $2, updated_at = now() WHERE id = $1`,
		rc.ID, body.PackID)
	if err != nil {
		mapErr(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "NOT_FOUND", "комната или пак не найдены")
		return
	}
	s.broadcastNow(rc.ID)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/rooms/{id}/packs/{packId} — удаление пакета из библиотеки.
func (s *Server) deletePack(w http.ResponseWriter, r *http.Request) {
	packID := chi.URLParam(r, "packID")
	// Нельзя удалить пак, который используется активной игрой.
	var inUse bool
	err := s.pool.QueryRow(r.Context(), `
		SELECT EXISTS (SELECT 1 FROM rooms WHERE pack_id = $1 AND status IN ('active','paused'))`,
		packID).Scan(&inUse)
	if err != nil {
		mapErr(w, err)
		return
	}
	if inUse {
		writeErr(w, 409, "PACK_IN_USE", "пакет используется активной игрой")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM packs WHERE id = $1`, packID); err != nil {
		mapErr(w, err)
		return
	}
	// Чистим медиа с диска.
	_ = os.RemoveAll(filepath.Join(s.cfg.MediaDir, packID))
	writeJSON(w, 200, map[string]any{"ok": true})
}

func newUUID() string {
	return uuid.NewString()
}
