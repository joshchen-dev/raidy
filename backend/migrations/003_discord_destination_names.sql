-- +goose Up
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS guild_name TEXT NOT NULL DEFAULT '';

ALTER TABLE schedule_settings
    ADD COLUMN IF NOT EXISTS channel_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE schedule_settings DROP COLUMN IF EXISTS channel_name;
ALTER TABLE teams DROP COLUMN IF EXISTS guild_name;
