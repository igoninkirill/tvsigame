#!/usr/bin/env python3
"""Смоук-тест вертикального сценария MVP (см. TZ §16).

Прогоняет: создание комнаты → вход ведущего → импорт .siq → игроки →
старт игры → вопрос → ответы → спецвопросы (аукцион, кот) → финал →
идемпотентность → отмена действия.

Запуск: python3 scripts/smoke.py [base_url]   (по умолчанию http://localhost:8080)
"""

import io
import json
import sys
import urllib.request
import uuid
import zipfile

BASE = "http://localhost:8080"

CONTENT_XML = """<?xml version="1.0" encoding="utf-8"?>
<package name="Smoke Test Pack" version="5" date="2024-01-01" difficulty="3">
  <info><authors><author>Smoke</author></authors></info>
  <rounds>
    <round name="Первый раунд">
      <themes>
        <theme name="География">
          <questions>
            <question price="100">
              <scenario><atom type="text">Столица Франции?</atom></scenario>
              <right><answer>Париж</answer></right>
            </question>
            <question price="200">
              <type name="auction"/>
              <scenario><atom type="text">Сколько будет дважды два?</atom></scenario>
              <right><answer>Четыре</answer></right>
            </question>
          </questions>
        </theme>
        <theme name="Кино">
          <questions>
            <question price="100">
              <type name="cat"/>
              <scenario><atom type="text">Кот: лучший фильм по IMDb?</atom></scenario>
              <right><answer>Побег из Шоушенка</answer></right>
            </question>
            <question price="200">
              <scenario><atom type="text">Год выхода фильма «Бригада»?</atom></scenario>
              <right><answer>2002</answer></right>
            </question>
          </questions>
        </theme>
      </themes>
    </round>
    <round name="Финал" type="final">
      <themes>
        <theme name="Итоговая тема А">
          <questions>
            <question price="0">
              <scenario><atom type="text">Финальный вопрос А</atom></scenario>
              <right><answer>Ответ А</answer></right>
            </question>
          </questions>
        </theme>
        <theme name="Итоговая тема Б">
          <questions>
            <question price="0">
              <scenario><atom type="text">Финальный вопрос Б</atom></scenario>
              <right><answer>Ответ Б</answer></right>
            </question>
          </questions>
        </theme>
      </themes>
    </round>
  </rounds>
</package>
"""


def req(method, path, token=None, body=None, raw=None, ctype=None):
    url = BASE + path
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = None
    if raw is not None:
        data = raw
        headers["Content-Type"] = ctype
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"{}")


def multipart(filename, content):
    boundary = "----smoketest"
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        f"Content-Type: application/zip\r\n\r\n"
    ).encode() + content + f"\r\n--{boundary}--\r\n".encode()
    return body, f"multipart/form-data; boundary={boundary}"


def cmd(room, token, ctype, payload=None, expected=None, action_id=None):
    body = {
        "action_id": action_id or str(uuid.uuid4()),
        "command_type": ctype,
        "payload": payload or {},
        "expected_state_version": expected,
    }
    status, res = req("POST", f"/api/rooms/{room}/actions", token, body)
    assert status == 200, f"{ctype} failed: {status} {res}"
    return res


def state(room, token):
    status, res = req("GET", f"/api/rooms/{room}/state", token)
    assert status == 200, res
    return res


def check(name, cond, extra=""):
    mark = "OK " if cond else "FAIL"
    print(f"  [{mark}] {mark == 'OK ' and '' or ''}{extra or ''}{' ' if extra else ''}", end="")
    print(name)
    if not cond:
        sys.exit(1)


def main():
    if len(sys.argv) > 1:
        global BASE
        BASE = sys.argv[1].rstrip("/")

    print("== 1. Создание комнаты ==")
    status, room = req("POST", "/api/rooms")
    check("комната создана", status == 201 and room.get("code"), room.get("code", ""))
    room_id, code, display_token = room["roomId"], room["code"], room["displayToken"]

    print("== 2. Вход ведущего по коду ==")
    status, host = req("POST", "/api/rooms/join", body={"code": code.lower()})
    check("код принят (регистронезависимо)", status == 200, code.lower())
    host_token = host["hostToken"]
    st = state(room_id, display_token)
    check("статус configuring на ТВ", st["room"]["status"] == "configuring")

    print("== 3. Импорт .siq ==")
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr("content.xml", CONTENT_XML)
    raw, ctype = multipart("test.siq", buf.getvalue())
    status, res = req("POST", f"/api/rooms/{room_id}/pack/import", host_token, raw=raw, ctype=ctype)
    check("пакет импортирован", status == 201 and res.get("rounds") == 2, res.get("name", ""))

    print("== 4. Игроки ==")
    pids = []
    for name in ["Алиса", "Борис", "Вера"]:
        status, res = req("POST", f"/api/rooms/{room_id}/players", host_token, {"name": name})
        assert status == 201, res
        pids.append(res["id"])
    st = state(room_id, host_token)
    check("три игрока в снапшоте", len(st["players"]) == 3)

    print("== 5. Старт игры ==")
    res = cmd(room_id, host_token, "START_GAME", expected=st["version"])
    st = state(room_id, host_token)
    check("комната active, стадия round_intro", st["room"]["status"] == "active" and st["game"]["stage"] == "round_intro")
    check("темы раунда в заставке", st["game"]["intro"]["themeNames"] == ["География", "Кино"])
    cmd(room_id, host_token, "BEGIN_ROUND")
    st = state(room_id, host_token)
    check("после BEGIN_ROUND стадия board", st["game"]["stage"] == "board")

    print("== 6. Обычный вопрос: верный ответ ==")
    cmd(room_id, host_token, "SELECT_QUESTION", {"themeIndex": 0, "questionIndex": 0})
    cmd(room_id, host_token, "ALLOW_ANSWERS")
    cmd(room_id, host_token, "SELECT_RESPONDER", {"playerId": pids[0]})
    cmd(room_id, host_token, "SUBMIT_VERDICT", {"correct": True})
    st = state(room_id, host_token)
    check("+100 Алисе", st["players"][0]["score"] == 100)
    check("ответ показан", st["game"]["current"]["answerShown"] is True)
    check("стадия answer_shown", st["game"]["stage"] == "answer_shown")
    cmd(room_id, host_token, "CLOSE_QUESTION")
    st = state(room_id, host_token)
    check("ячейка использована", st["pack"]["rounds"][0]["themes"][0]["questions"][0]["used"] is True)

    print("== 7. Неверный ответ и повторная попытка ==")
    cmd(room_id, host_token, "SELECT_QUESTION", {"themeIndex": 1, "questionIndex": 1})
    cmd(room_id, host_token, "ALLOW_ANSWERS")
    cmd(room_id, host_token, "SELECT_RESPONDER", {"playerId": pids[1]})
    cmd(room_id, host_token, "SUBMIT_VERDICT", {"correct": False})
    st = state(room_id, host_token)
    check("-200 Борису", st["players"][1]["score"] == -200)
    check("ответ НЕ раскрыт после ошибки", st["game"]["current"]["answerShown"] is False)
    check("ответ на ТВ скрыт", "answer" not in state(room_id, display_token)["game"]["current"])
    cmd(room_id, host_token, "SELECT_RESPONDER", {"playerId": pids[2]})
    cmd(room_id, host_token, "SUBMIT_VERDICT", {"correct": True})
    st = state(room_id, host_token)
    check("+200 Вере", st["players"][2]["score"] == 200)
    cmd(room_id, host_token, "CLOSE_QUESTION")

    print("== 8. Аукцион ==")
    cmd(room_id, host_token, "SELECT_QUESTION", {"themeIndex": 0, "questionIndex": 1})
    st = state(room_id, host_token)
    check("стадия auction", st["game"]["stage"] == "auction")
    status, res = req("POST", f"/api/rooms/{room_id}/actions", host_token, {
        "action_id": str(uuid.uuid4()), "command_type": "PLACE_BID",
        "payload": {"playerId": pids[0], "amount": 50}, "expected_state_version": None})
    check("ставка ниже номинала отклонена", status == 400, res.get("error", {}).get("code", ""))
    status, res = req("POST", f"/api/rooms/{room_id}/actions", host_token, {
        "action_id": str(uuid.uuid4()), "command_type": "PLACE_BID",
        "payload": {"playerId": pids[0], "amount": 500}, "expected_state_version": None})
    check("ставка выше счёта отклонена", status == 400, res.get("error", {}).get("code", ""))
    cmd(room_id, host_token, "PLACE_BID", {"playerId": pids[2], "amount": 200})
    cmd(room_id, host_token, "PASS_BID", {"playerId": pids[1]})
    cmd(room_id, host_token, "PASS_BID", {"playerId": pids[0]})
    st = state(room_id, host_token)
    check("торги завершены автоматически, цена = ставке 200",
          st["game"]["stage"] == "question" and st["game"]["current"]["price"] == 200)
    cmd(room_id, host_token, "ALLOW_ANSWERS")
    status, res = req("POST", f"/api/rooms/{room_id}/actions", host_token, {
        "action_id": str(uuid.uuid4()), "command_type": "SELECT_RESPONDER",
        "payload": {"playerId": pids[1]}, "expected_state_version": None})
    check("чужой ответ на аукционе запрещён", status == 400)
    cmd(room_id, host_token, "SELECT_RESPONDER", {"playerId": pids[2]})
    cmd(room_id, host_token, "SUBMIT_VERDICT", {"correct": True})
    st = state(room_id, host_token)
    check("+200 Вере (ставка)", st["players"][2]["score"] == 400)
    cmd(room_id, host_token, "CLOSE_QUESTION")

    print("== 9. Кот в мешке ==")
    cmd(room_id, host_token, "SELECT_QUESTION", {"themeIndex": 1, "questionIndex": 0})
    st = state(room_id, host_token)
    check("стадия cat_transfer", st["game"]["stage"] == "cat_transfer")
    cmd(room_id, host_token, "TRANSFER_CAT", {"playerId": pids[0]})
    cmd(room_id, host_token, "ALLOW_ANSWERS")
    cmd(room_id, host_token, "SELECT_RESPONDER", {"playerId": pids[0]})
    cmd(room_id, host_token, "SUBMIT_VERDICT", {"correct": False})
    st = state(room_id, host_token)
    check("-100 Алисе (кот, единая попытка)", st["players"][0]["score"] == 0)
    check("вопрос завершён после ошибки", st["game"]["stage"] == "answer_shown")
    cmd(room_id, host_token, "CLOSE_QUESTION")
    st = state(room_id, host_token)
    check("раунд завершён (round_end)", st["game"]["stage"] == "round_end")

    print("== 10. Финал ==")
    cmd(room_id, host_token, "NEXT_ROUND")
    st = state(room_id, host_token)
    check("стадия final_themes", st["game"]["stage"] == "final_themes")
    finalists = st["game"]["final"]["finalistIds"]
    check("к финалу допущена только Вера (положительный счёт)", finalists == [pids[2]])
    cmd(room_id, host_token, "EXCLUDE_FINAL_THEME", {"themeIndex": 1})
    st = state(room_id, host_token)
    check("стадия final_stakes", st["game"]["stage"] == "final_stakes")
    status, res = req("POST", f"/api/rooms/{room_id}/actions", host_token, {
        "action_id": str(uuid.uuid4()), "command_type": "SET_FINAL_STAKE",
        "payload": {"playerId": pids[2], "amount": 999}, "expected_state_version": None})
    check("ставка выше счёта отклонена", status == 400)
    cmd(room_id, host_token, "SET_FINAL_STAKE", {"playerId": pids[2], "amount": 400})
    st = state(room_id, host_token)
    check("ставки скрыты от ТВ", "stakes" not in (state(room_id, display_token)["game"]["final"] or {}))
    cmd(room_id, host_token, "REVEAL_FINAL_STAKES")
    cmd(room_id, host_token, "OPEN_FINAL_QUESTION")
    st = state(room_id, host_token)
    check("финальный вопрос открыт, таймер идёт", st["game"]["stage"] == "final_question" and st["game"]["timer"]["running"])
    cmd(room_id, host_token, "SUBMIT_FINAL_ANSWER", {"playerId": pids[2], "correct": True})
    st = state(room_id, host_token)
    check("игра завершена", st["room"]["status"] == "finished" and st["game"]["stage"] == "results")
    check("Вера: +400 за верный финальный ответ", st["players"][2]["score"] == 800)
    check("победитель — Вера", st["game"]["winnerId"] == pids[2])

    print("== 11. Идемпотентность и отмена ==")
    # Комната уже finished — отмена финального вердикта вернёт игру в финал.
    res = cmd(room_id, host_token, "UNDO_LAST_ACTION")
    st = state(room_id, host_token)
    check("undo вернул игру в финал", st["room"]["status"] == "active")
    check("счёт Веры восстановлен", st["players"][2]["score"] == 400)
    # Повторная отправка той же команды с тем же action_id не применяется.
    aid = str(uuid.uuid4())
    cmd(room_id, host_token, "SET_SCORE", {"playerId": pids[0], "score": 42, "reason": "test"}, action_id=aid)
    res = cmd(room_id, host_token, "SET_SCORE", {"playerId": pids[0], "score": 42, "reason": "test"}, action_id=aid)
    check("повтор action_id — duplicated", res.get("duplicated") is True)
    st = state(room_id, host_token)
    check("счёт применён ровно один раз", st["players"][0]["score"] == 42)

    print("== 12. Конфликт версий ==")
    status, res = req("POST", f"/api/rooms/{room_id}/actions", host_token, {
        "action_id": str(uuid.uuid4()), "command_type": "SET_SCORE",
        "payload": {"playerId": pids[0], "score": 100, "reason": "stale"},
        "expected_state_version": 1})
    check("устаревшая версия → 409", status == 409)

    print("\nВсе проверки пройдены.")


if __name__ == "__main__":
    main()
