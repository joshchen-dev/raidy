-- +goose Up
CREATE TABLE web_sessions (
    token_hash  BYTEA PRIMARY KEY,
    data        JSONB NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX web_sessions_expires_idx ON web_sessions (expires_at);

-- +goose Down
DROP TABLE web_sessions;
