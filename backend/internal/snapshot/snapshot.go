// Package snapshot — сборка полного состояния комнаты для клиентов.
//
// Снимок собирается после каждой команды и при подключении клиента.
// Существует два варианта:
//   - host — полный (ведущему доступны ответы и скрытые ставки);
//   - display — редакция для ТВ: правильный ответ скрыт, пока он не показан,
//     ставки финала скрыты до одновременного раскрытия.
//
// Пакет намеренно не импортирует game (во избежание цикла) и работает
// с состоянием как с generic map.
package snapshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Build возвращает (hostJSON, displayJSON).
func Build(ctx context.Context, q Querier, roomID string, events []map[string]any) (hostJSON, displayJSON []byte, err error) {
	var (
		code, status   string
		hostConnected  bool
		packID         *string
		stateRaw       []byte
		version        int64
	)
	err = q.QueryRow(ctx, `
		SELECT r.room_code, r.status, r.host_connected, r.pack_id,
		       COALESCE(gs.state, '{}'::jsonb), COALESCE(gs.state_version, 0)
		FROM rooms r
		LEFT JOIN game_states gs ON gs.room_id = r.id
		WHERE r.id = $1`, roomID,
	).Scan(&code, &status, &hostConnected, &packID, &stateRaw, &version)
	if err != nil {
		return nil, nil, fmt.Errorf("room: %w", err)
	}

	players := []map[string]any{}
	rows, err := q.Query(ctx, `
		SELECT id, name, score, position, is_active
		FROM players WHERE room_id = $1 ORDER BY position, created_at`, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("players: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		var score int64
		var position int
		var active bool
		if err := rows.Scan(&id, &name, &score, &position, &active); err != nil {
			return nil, nil, err
		}
		players = append(players, map[string]any{
			"id": id, "name": name, "score": score, "position": position, "isActive": active,
		})
	}

	// Скелет пакета для табло: только цены/типы/использованность, без текстов и ответов.
	var pack any
	var stateMap map[string]any
	_ = json.Unmarshal(stateRaw, &stateMap)

	if packID != nil {
		var parsed []byte
		var name string
		err := q.QueryRow(ctx, `SELECT name, parsed_data FROM packs WHERE id = $1`, *packID).Scan(&name, &parsed)
		if err == nil {
			var p map[string]any
			if json.Unmarshal(parsed, &p) == nil {
				used := map[string]bool{}
				if u, ok := stateMap["usedCells"].(map[string]any); ok {
					for k, v := range u {
						if b, ok := v.(bool); ok {
							used[k] = b
						}
					}
				}
				pack = buildPackSkeleton(p, used)
			}
		}
	}

	var game any = stateMap

	base := map[string]any{
		"type":    "state",
		"version": version,
		"room": map[string]any{
			"id": roomID, "code": code, "status": status, "hostConnected": hostConnected,
		},
		"players": players,
		"pack":    pack,
		"game":    game,
		"events":  events,
	}

	hostJSON, err = json.Marshal(base)
	if err != nil {
		return nil, nil, err
	}

	// Редакция для ТВ.
	if g, ok := base["game"].(map[string]any); ok {
		redactGame(g)
	}
	displayJSON, err = json.Marshal(base)
	if err != nil {
		return nil, nil, err
	}
	return hostJSON, displayJSON, nil
}

// redactGame скрывает от ТВ нераскрытые ответы и ставки финала.
func redactGame(g map[string]any) {
	if cur, ok := g["current"].(map[string]any); ok {
		if shown, _ := cur["answerShown"].(bool); !shown {
			delete(cur, "answer")
			delete(cur, "answerScenario") // медиа ответа тоже могут спойлерить
		}
	}
	if fin, ok := g["final"].(map[string]any); ok {
		if revealed, _ := fin["stakesRevealed"].(bool); !revealed {
			delete(fin, "stakes")
		}
		if q, ok := fin["question"].(map[string]any); ok {
			if shown, _ := q["answerShown"].(bool); !shown {
				delete(q, "answer")
				delete(q, "answerScenario")
			}
		}
	}
}

// buildPackSkeleton строит облегчённое представление пакета для табло.
func buildPackSkeleton(p map[string]any, used map[string]bool) map[string]any {
	out := map[string]any{
		"name":    p["name"],
		"author":  p["author"],
		"logoUrl": p["logoUrl"],
	}
	rounds := []any{}
	if rs, ok := p["rounds"].([]any); ok {
		for _, rv := range rs {
			rm, ok := rv.(map[string]any)
			if !ok {
				continue
			}
			round := map[string]any{"name": rm["name"], "type": rm["type"], "themes": []any{}}
			if ts, ok := rm["themes"].([]any); ok {
				for _, tv := range ts {
					tm, ok := tv.(map[string]any)
					if !ok {
						continue
					}
					theme := map[string]any{"name": tm["name"], "questions": []any{}}
					if qs, ok := tm["questions"].([]any); ok {
						for _, qv := range qs {
							qm, ok := qv.(map[string]any)
							if !ok {
								continue
							}
							qid, _ := qm["id"].(string)
							theme["questions"] = append(theme["questions"].([]any), map[string]any{
								"price": qm["price"],
								"type":  qm["type"],
								"used":  used[qid],
							})
						}
					}
					round["themes"] = append(round["themes"].([]any), theme)
				}
			}
			rounds = append(rounds, round)
		}
	}
	out["rounds"] = rounds
	return out
}
