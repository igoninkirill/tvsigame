-- 0001_init.sql — базовая схема данных «Своей Игры»

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS packs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    author      TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT '',
    parsed_data JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rooms (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_code         TEXT NOT NULL,
    display_token_hash TEXT NOT NULL,
    host_token_hash   TEXT,
    host_client_id    TEXT,
    host_connected    BOOLEAN NOT NULL DEFAULT FALSE,
    host_last_seen_at TIMESTAMPTZ,
    status            TEXT NOT NULL DEFAULT 'waiting_for_host',
    pack_id           UUID REFERENCES packs(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL
);

-- Уникальность кода среди активных (не завершённых и не истёкших) комнат.
CREATE UNIQUE INDEX IF NOT EXISTS rooms_active_code_uidx
    ON rooms (room_code)
    WHERE status NOT IN ('finished', 'expired');

CREATE TABLE IF NOT EXISTS players (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    score      BIGINT NOT NULL DEFAULT 0,
    position   INT NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS players_room_idx ON players (room_id, position);

CREATE TABLE IF NOT EXISTS game_states (
    room_id      UUID PRIMARY KEY REFERENCES rooms(id) ON DELETE CASCADE,
    state        JSONB NOT NULL,
    state_version BIGINT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS game_events (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sequence_number BIGINT NOT NULL,
    event_type      TEXT NOT NULL,
    actor           TEXT NOT NULL DEFAULT 'system',
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    state_version   BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, sequence_number)
);

CREATE INDEX IF NOT EXISTS game_events_room_idx ON game_events (room_id, sequence_number);

CREATE TABLE IF NOT EXISTS game_actions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    room_id     UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    action_id   TEXT NOT NULL,
    command_type TEXT NOT NULL,
    prev_state  JSONB,
    prev_scores JSONB,
    prev_version BIGINT,
    result      JSONB,
    undone      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, action_id)
);
