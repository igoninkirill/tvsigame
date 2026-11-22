# AGENTS.md — карта проекта для ИИ-агентов и разработчиков

Этот файл — точка входа для быстрого понимания кодовой базы. Читай его первым.

## Что это

«Своя Игра» в формате ТВ-шоу: телевизор = пассивное табло, смартфон ведущего = пульт, игроки отвечают устно. Полное ТЗ — в `TZ.md` (раздел 18 — обязательные ограничения, соблюдай их).

## Структура

```
├── docker-compose.yml      # db (postgres:16) + app (all-in-one образ), разработка
├── docker-compose.prod.yml # прод: + Caddy (HTTPS, DOMAIN из .env), Postgres не торчит наружу
├── deploy/                 # Caddyfile + DEPLOY.md (пошаговый прод-деплой)
├── scripts/deploy.sh       # rsync на сервер + пересборка (SERVER=user@host)
├── Dockerfile              # multi-stage: frontend build → go build → alpine runtime
├── .env.example            # все настраиваемые переменные
├── TZ.md                   # исходное техническое задание
├── backend/                # Go 1.23, модуль github.com/krutokir/svoya-igra/server
│   ├── cmd/server/main.go  # сборка приложения, восстановление после рестарта
│   ├── internal/
│   │   ├── config/         # env-конфигурация
│   │   ├── db/             # pgx pool + embed-миграции (migrations/*.sql по порядку)
│   │   ├── auth/           # токены: генерация, SHA-256 хэш, constant-time сравнение
│   │   ├── roomcode/       # коды комнат (алфавит без неоднозначных символов)
│   │   ├── model/          # нормализованная модель пака (Pack/Round/Theme/Question/Atom)
│   │   ├── siq/            # импортёр .siq: ZIP→content.xml→model + извлечение медиа
│   │   ├── game/           # ИГРОВОЙ ДВИЖОК (см. ниже)
│   │   ├── snapshot/       # сборка снапшота состояния для клиентов (host/display редакции)
│   │   ├── ws/             # WebSocket-хаб: auth первым сообщением, heartbeat, broadcast
│   │   └── httpapi/        # chi-роутер: REST + WS + статика + медиа
├── frontend/               # React 18 + TypeScript + Vite
│   ├── src/lib/            # типы протокола, REST-клиент, WS-клиент, session (localStorage), room context
│   ├── src/pages/          # HomePage, NewRoomPage, DisplayPage, HostPage
│   ├── src/tv/             # компоненты ТВ-экрана (без управления!)
│   ├── src/host/           # компоненты пульта ведущего
│   ├── src/components/Icon.tsx  # SVG-иконки (эмодзи в UI ЗАПРЕЩЕНЫ)
│   └── src/styles/         # base.css (токены), tv.css (НТВ-стилистика), host.css (мобильный пульт)
└── data/media/             # медиа пакетов (локальное хранилище; в будущем — S3)
```

## Ключевые инварианты (не нарушать)

1. **Сервер — единственный источник истины.** Баллы, стадии, таймеры считаются только в `backend/internal/game`. На клиенте — отображение.
2. **Каждая игровая команда идемпотентна**: `action_id` уникален в рамках комнаты (`game_actions`), повтор возвращает сохранённый результат без повторного применения.
3. **Все изменения атомарны**: команда = одна транзакция PostgreSQL (состояние JSONB + счета + события + запись команды).
4. **Каждая команда увеличивает `state_version`**; клиент шлёт `expected_state_version`, конфликт → 409.
5. **`game_events` — только INSERT** (неизменяемый журнал). Отмена действия (`UNDO_LAST_ACTION`) восстанавливает снапшот из `game_actions.prev_state/prev_scores` и пишет новое событие.
6. **Ответы не утекают на ТВ**: `snapshot.Build` выдаёт display-редакцию без `answer` (до показа) и без ставок финала (до раскрытия).
7. **Один ведущий на комнату**: `host_token_hash` + `host_client_id`, замена сессии только при отсутствии живого подключения (проверка через hub + БД-флаг, grace-период `HOST_GRACE_SEC`).
8. **Таймеры серверные**: `game.TimerManager` перезапускает `AfterFunc` после каждой команды; истечение = внутренняя команда `TIMER_EXPIRED` с `action_id="timer-{id}"` (обрабатывается ровно один раз). При рестарте backend таймеры перевзводятся из БД, активные игры уходят в паузу.

## Игровой движок (`backend/internal/game`)

Конечный автомат. Стадии (`types.go`):

```
lobby → round_intro → board → question → answer_window → answering → answer_shown → board …
        (BEGIN_ROUND)       ↘ auction ─────────────↗          ↘ (раунд кончился) round_end → round_intro …
                    ↘ cat_transfer → question …                    … → final_themes → final_stakes
                    ↘ for_all ──→ answer_shown                            → final_question → final_verdicts → results
```

- `engine.go` — `Execute()`: блокировка строки комнаты (`SELECT ... FOR UPDATE`), идемпотентность, проверка версии, транзакция, журнал, рассылка.
- `apply.go` — валидация и переходы для каждого типа команды (добавление команды = новая константа в `types.go` + ветка в `apply()` + кнопка на пульте).
- Типы вопросов пакета: `regular`, `auction`, `cat`, `noRisk`, `forAll`; неизвестные сохраняются как `unknown:<имя>` и проводятся вручную (движок не молча превращает их в обычные — выдаёт предупреждение в событии).

## Протокол клиент ↔ сервер

- REST: см. ТЗ §10. Все room-эндпоинты требуют `Authorization: Bearer <token>`; роль определяется токеном (host/display).
- WS `/ws/room/{id}`: первое сообщение `{"type":"auth","role","token","clientId"}`, далее сервер шлёт полный снапшот `{"type":"state", ...}` после каждой команды и при подключении. Heartbeat: клиент `{"type":"ping"}` раз в 5 с + серверные ping-фреймы.
- Снапшот: `{version, room, players, pack(скелет без ответов), game, events}` — клиент просто заменяет состояние целиком (`frontend/src/lib/connection.ts`, `room.tsx`).
- Эфемерные сигналы (пауза/плей медиа на ТВ): `POST /rooms/{id}/signal` → broadcast `{"type":"signal"}`; в журнал не пишутся.

## База данных

Миграции — `backend/internal/db/migrations/*.sql`, применяются автоматически при старте в порядке имён (новая миграция = следующий номер, никогда не редактируй применённые).

Таблицы: `packs`, `rooms` (код, хэши токенов, статус, `host_client_id`), `players`, `game_states` (JSONB + `state_version`), `game_events` (append-only), `game_actions` (идемпотентность + снапшоты для undo).

Статусы комнаты: `waiting_for_host → configuring → ready → active ⇄ paused → finished` (+ `expired`).

## Формат .siq (импортёр)

ZIP с `content.xml` (v5 приоритетно, v4 совместимо). Поддерживаются оба диалекта разметки:

- классический `<scenario><atom type="image">@file</atom>`;
- новый (SIBrowser/SIGame 8): `<params><param name="question" type="content"><item type="image" isRef="True">file.jpg</item>…`, плюс `param name="answer"` → `Question.AnswerScenario` (медиа, показываемые при раскрытии ответа; на ТВ скрыты до `answerShown`).

Тип вопроса берётся из атрибута `<question type="secret">` (новый) или `<type name="...">` (классика), маппинг — `canonicalQuestionType` (`stake/auction`, `secret/cat`, `noRisk/sponsored`, `forAll`). У `secret` с `numberSet` цена вопроса = минимум диапазона. Медиа-ссылки резолвятся по индексу архива (учитываются URL-кодированные имена записей, как у sibrowser), извлекаются в `MEDIA_DIR/{packId}/` с санитизацией имён, отдаются как `/media/{packId}/{file}`. Лимиты и защита от ZIP Slip — константы в `siq.go`.

## Рабочие команды

```bash
# backend (в контейнере, т.к. локального Go может не быть)
docker run --rm -v "$PWD/backend":/src -w /src -v gomodcache:/go/pkg/mod golang:1.23-alpine sh -c "go build ./... && go vet ./..."

# ручной драйвер полной партии (НЕ CI): start|round|next|final|status
python3 scripts/fullgame.py status <room_id> <host_token>

# frontend
cd frontend && npm install && npm run build   # tsc + vite
cd frontend && npm run dev                     # dev-сервер с прокси на :8080

# всё вместе
docker compose up --build
```

## Соглашения

- UI-тексты — по-русски. Без эмодзи: только SVG из `frontend/src/components/Icon.tsx` (новые иконки добавляй туда же, стиль — контурный 24×24, `currentColor`).
- ТВ (`src/tv/`) не содержит игровых кнопок и не вычисляет ничего, кроме отображения остатка таймера.
- Табло классическое: темы — строки слева, цены растут слева направо (`tv/GameBoard.tsx`, grid `repeat(qCount, 1fr)`).
- Звуки ТВ — аутентичные сэмплы SIGame (MIT, `frontend/public/sounds/*.mp3`, копирайт в `NOTICE.txt`): `src/tv/sounds.ts` (плеер), `src/tv/SoundManager.tsx` (триггеры по смене стадий/событий снапшота + кнопка вкл/выкл, настройка в `localStorage 'sigame:sound'`). Браузер разблокирует звук первым касанием экрана; музыку заставки/финала останавливай при уходе со стадии (`stopRoundIntro`/`stopFinalQuestion`).
- Пульт (`src/host/`) — mobile-first, кнопки ≥ 44px, опасные действия через `Confirm`.
- Новые поля состояния: добавляй в `game/types.go` + redact-правила в `snapshot/redactGame` (если секретно для ТВ) + типы в `frontend/src/lib/types.ts`.
- Не реализуй функции из §14 ТЗ (будущие этапы) до их явного запроса.
