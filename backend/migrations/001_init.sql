-- +goose Up
CREATE TABLE IF NOT EXISTS teams (
    id          BIGSERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    name_key    TEXT NOT NULL,
    display_name TEXT NOT NULL,
    timezone    TEXT NOT NULL,
    leader_id   TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (guild_id, name_key)
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id     BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL,
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE IF NOT EXISTS schedule_settings (
    team_id             BIGINT PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
    cadence_days        SMALLINT NOT NULL CHECK (cadence_days IN (7, 14)),
    start_minutes       SMALLINT NOT NULL CHECK (start_minutes BETWEEN 0 AND 1439),
    end_minutes         SMALLINT NOT NULL CHECK (end_minutes BETWEEN 0 AND 1439),
    next_period_start   DATE NOT NULL,
    next_publish_at     TIMESTAMPTZ NOT NULL,
    channel_id          TEXT NOT NULL,
    enabled             BOOLEAN NOT NULL DEFAULT true,
    CHECK (start_minutes <> end_minutes)
);

CREATE TABLE IF NOT EXISTS schedule_weekdays (
    team_id     BIGINT NOT NULL REFERENCES schedule_settings(team_id) ON DELETE CASCADE,
    weekday     SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    PRIMARY KEY (team_id, weekday)
);

CREATE TABLE IF NOT EXISTS schedule_polls (
    id              BIGSERIAL PRIMARY KEY,
    team_id         BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    channel_id      TEXT NOT NULL,
    message_id      TEXT NOT NULL DEFAULT '',
    closed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, period_start)
);

CREATE TABLE IF NOT EXISTS poll_members (
    poll_id     BIGINT NOT NULL REFERENCES schedule_polls(id) ON DELETE CASCADE,
    member_id   TEXT NOT NULL,
    PRIMARY KEY (poll_id, member_id)
);

CREATE TABLE IF NOT EXISTS poll_occurrences (
    id          BIGSERIAL PRIMARY KEY,
    poll_id     BIGINT NOT NULL REFERENCES schedule_polls(id) ON DELETE CASCADE,
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ NOT NULL,
    status      TEXT NOT NULL DEFAULT 'proposed'
                CHECK (status IN ('proposed', 'confirmed', 'cancelled', 'attention_required')),
    UNIQUE (poll_id, starts_at),
    CHECK (ends_at > starts_at)
);

CREATE TABLE IF NOT EXISTS poll_submissions (
    poll_id         BIGINT NOT NULL REFERENCES schedule_polls(id) ON DELETE CASCADE,
    member_id       TEXT NOT NULL,
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (poll_id, member_id)
);

CREATE TABLE IF NOT EXISTS poll_availability (
    occurrence_id   BIGINT NOT NULL REFERENCES poll_occurrences(id) ON DELETE CASCADE,
    member_id       TEXT NOT NULL,
    available       BOOLEAN NOT NULL,
    PRIMARY KEY (occurrence_id, member_id)
);

CREATE INDEX IF NOT EXISTS schedule_settings_due_idx
    ON schedule_settings (next_publish_at) WHERE enabled;
CREATE INDEX IF NOT EXISTS poll_occurrences_poll_idx ON poll_occurrences (poll_id);

-- +goose Down
DROP TABLE IF EXISTS poll_availability;
DROP TABLE IF EXISTS poll_submissions;
DROP TABLE IF EXISTS poll_occurrences;
DROP TABLE IF EXISTS poll_members;
DROP TABLE IF EXISTS schedule_polls;
DROP TABLE IF EXISTS schedule_weekdays;
DROP TABLE IF EXISTS schedule_settings;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
