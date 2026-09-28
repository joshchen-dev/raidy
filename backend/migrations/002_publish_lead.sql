-- +goose Up
ALTER TABLE schedule_settings
    ADD COLUMN IF NOT EXISTS publish_lead_days SMALLINT NOT NULL DEFAULT 3
    CHECK (publish_lead_days IN (1, 2, 3, 5, 7, 10, 14));

-- +goose Down
ALTER TABLE schedule_settings DROP COLUMN IF EXISTS publish_lead_days;
