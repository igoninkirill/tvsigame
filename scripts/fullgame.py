#!/usr/bin/env python3
"""Драйвер полной партии на реальном паке (ручной прогон, НЕ часть CI).

Фазы (аргумент 1):
  start   — START_GAME (заставка раунда остаётся на экране)
  round   — доиграть текущий раунд до round_end (все ячейки)
  next    — NEXT_ROUND (заставка следующего раунда)
  final   — пройти финал до results
  status  — напечатать стадию и счета

Использование: python3 scripts/fullgame.py <phase> <room_id> <host_token>
Вердикты чередуются (верно/неверно), чтобы проверить оба пути начисления.
Медиа-вопросы логируются (тип атомов), чтобы сверить с ТВ.
"""

import json
import sys
import time
import urllib.request
import uuid

BASE = "http://localhost:8080"


def req(method, path, token, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(
        BASE + path, data=data, method=method,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(r) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"{}")


def cmd(room, token, ctype, payload=None):
    status, res = req("POST", f"/api/rooms/{room}/actions", token, {
        "action_id": str(uuid.uuid4()), "command_type": ctype, "payload": payload or {}})
    if status != 200:
        print(f"  !! {ctype} -> {status} {res}")
        raise SystemExit(1)
    return res


def state(room, token):
    status, res = req("GET", f"/api/rooms/{room}/state", token)
    assert status == 200, res
    return res


def atom_types(q):
    types = []
    for atom in q.get("scenario", []):
        t = atom.get("kind") or atom.get("type", "text")
        if t not in types:
            types.append(t)
    return types


def play_question(room, token, st, players, verdict_flip):
    """Играет один уже выбранный вопрос. Возвращает следующий verdict_flip."""
    game = st["game"]
    stage = game["stage"]
    cur = game.get("current") or {}
    qtype = cur.get("questionType", "standard")
    pids = [p["id"] for p in players if p.get("active", True)]

    if stage == "auction":
        # Торги: первый ставит номинал, остальные пасуют.
        print(f"    АУКЦИОН (цена {cur.get('price')})")
        nominal = cur.get("price", 100)
        cmd(room, token, "PLACE_BID", {"playerId": pids[0], "amount": nominal})
        for pid in pids[1:]:
            st2 = state(room, token)
            if st2["game"]["stage"] != "auction":
                break
            cmd(room, token, "PASS_BID", {"playerId": pid})
        st = state(room, token)
        cur = st["game"]["current"] or {}
        cmd(room, token, "ALLOW_ANSWERS")
        winner = (cur.get("answeringId") or (st["game"].get("current") or {}).get("answeringId"))
        st = state(room, token)
        winner = (st["game"]["current"] or {}).get("answeringId") or pids[0]
        cmd(room, token, "SELECT_RESPONDER", {"playerId": winner})
        cmd(room, token, "SUBMIT_VERDICT", {"correct": verdict_flip})
        verdict_flip = not verdict_flip

    elif stage == "cat_transfer":
        print(f"    КОТ В МЕШКЕ (цена {cur.get('price')})")
        target = pids[1] if len(pids) > 1 else pids[0]
        cmd(room, token, "TRANSFER_CAT", {"playerId": target})
        cmd(room, token, "ALLOW_ANSWERS")
        cmd(room, token, "SELECT_RESPONDER", {"playerId": target})
        cmd(room, token, "SUBMIT_VERDICT", {"correct": verdict_flip})
        verdict_flip = not verdict_flip

    elif stage == "for_all":
        print(f"    ДЛЯ ВСЕХ (цена {cur.get('price')})")
        cmd(room, token, "ALLOW_ANSWERS")
        for pid in pids:
            cmd(room, token, "SUBMIT_VERDICT", {"playerId": pid, "correct": verdict_flip})
            verdict_flip = not verdict_flip

    else:
        cmd(room, token, "ALLOW_ANSWERS")
        # Попытки по кругу: первая — по флагу верно/неверно, дальше добиваем верным.
        for attempt, pid in enumerate(pids):
            st2 = state(room, token)
            if st2["game"]["stage"] not in ("answer_window", "answering"):
                break
            cmd(room, token, "SELECT_RESPONDER", {"playerId": pid})
            correct = verdict_flip if attempt == 0 else True
            cmd(room, token, "SUBMIT_VERDICT", {"correct": correct})
            if correct:
                break
        verdict_flip = not verdict_flip

    # Добивка: ответ мог быть не раскрыт (все ошиблись / аукцион / кот).
    st = state(room, token)
    if st["game"]["stage"] != "answer_shown":
        cmd(room, token, "SHOW_ANSWER")
    cmd(room, token, "CLOSE_QUESTION")
    return verdict_flip


def play_round(room, token):
    st = state(room, token)
    game = st["game"]
    if game["stage"] == "round_intro":
        cmd(room, token, "BEGIN_ROUND")
        st = state(room, token)
    # Восстановление после прерванного прогона: закрываем болтающийся вопрос.
    if st["game"]["stage"] not in ("board", "round_end") and st["game"].get("current"):
        if st["game"]["stage"] != "answer_shown":
            cmd(room, token, "SHOW_ANSWER")
        cmd(room, token, "CLOSE_QUESTION")
        st = state(room, token)
    rnd = game["roundIndex"]
    themes = st["pack"]["rounds"][rnd]["themes"]
    print(f"  Раунд {rnd + 1}: {len(themes)} тем")
    players = st["players"]
    verdict_flip = True
    media_log = []

    # Порядок: сверху вниз по темам, как делает ведущий.
    for ti, theme in enumerate(themes):
        for qi, q in enumerate(theme["questions"]):
            st = state(room, token)
            if st["game"]["stage"] != "board":
                break
            cell = st["pack"]["rounds"][rnd]["themes"][ti]["questions"][qi]
            if cell.get("used"):
                continue
            cmd(room, token, "SELECT_QUESTION", {"themeIndex": ti, "questionIndex": qi})
            st = state(room, token)
            cur = st["game"].get("current") or {}
            types = atom_types(cur)
            tag = f"{theme['name']} / {cur.get('price')} [{','.join(types)}] type={cur.get('questionType','standard')}"
            if any(t != "text" for t in types):
                media_log.append(tag)
            verdict_flip = play_question(room, token, st, players, verdict_flip)
    st = state(room, token)
    print(f"  -> стадия после раунда: {st['game']['stage']}")
    if media_log:
        print("  Медиа-вопросы раунда:")
        for m in media_log:
            print(f"    {m}")
    return st


def main():
    phase, room, token = sys.argv[1], sys.argv[2], sys.argv[3]

    if phase == "status":
        st = state(room, token)
        print("stage:", st["game"]["stage"], "| round:", st["game"].get("roundIndex"))
        for p in st["players"]:
            print(f"  {p['name']}: {p['score']}")
        return

    if phase == "start":
        st = state(room, token)
        if st["room"]["status"] == "configuring":
            cmd(room, token, "START_GAME")
        st = state(room, token)
        print("stage:", st["game"]["stage"], "| темы:", st["game"]["intro"]["themeNames"])
        return

    if phase == "round":
        play_round(room, token)
        return

    if phase == "next":
        cmd(room, token, "NEXT_ROUND")
        st = state(room, token)
        print("stage:", st["game"]["stage"], "| темы:", (st["game"].get("intro") or {}).get("themeNames"))
        return

    if phase == "final":
        st = state(room, token)
        if st["game"]["stage"] == "round_intro":
            cmd(room, token, "BEGIN_ROUND")
            st = state(room, token)
        game = st["game"]
        print("  стадия финала:", game["stage"])
        finalists = game["final"]["finalistIds"]
        print("  финалисты:", [(p["name"], p["score"]) for p in st["players"] if p["id"] in finalists])
        # Исключаем темы (индексы оригинальные), пока не останется одна.
        while True:
            st = state(room, token)
            g = st["game"]
            if g["stage"] == "final_stakes":
                break
            excluded = set(g["final"]["excluded"])
            idx = next(i for i in range(len(g["final"]["themeNames"])) if i not in excluded)
            cmd(room, token, "EXCLUDE_FINAL_THEME", {"themeIndex": idx})
            print("  исключена тема:", g["final"]["themeNames"][idx])
        st = state(room, token)
        finalists = st["game"]["final"]["finalistIds"]
        for p in st["players"]:
            if p["id"] in finalists:
                stake = max(1, p["score"] // 2)
                cmd(room, token, "SET_FINAL_STAKE", {"playerId": p["id"], "amount": stake})
                print(f"  ставка {p['name']}: {stake}")
        cmd(room, token, "REVEAL_FINAL_STAKES")
        cmd(room, token, "OPEN_FINAL_QUESTION")
        st = state(room, token)
        cur = st["game"].get("current") or {}
        print("  финальный вопрос:", atom_types(cur))
        for p in st["players"]:
            if p["id"] in finalists:
                cmd(room, token, "SUBMIT_FINAL_ANSWER", {"playerId": p["id"], "correct": True})
        st = state(room, token)
        print("  стадия:", st["game"]["stage"], "| статус:", st["room"]["status"])
        for p in st["players"]:
            print(f"  {p['name']}: {p['score']}")
        w = st["game"].get("winnerId")
        print("  победитель:", next((p["name"] for p in st["players"] if p["id"] == w), None))
        return

    print("unknown phase", file=sys.stderr)
    sys.exit(2)


if __name__ == "__main__":
    main()
