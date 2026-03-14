-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS customer_sessions
(
    customer_id   BIGINT PRIMARY KEY,
    waiting_name  BOOLEAN NOT NULL DEFAULT FALSE,
    full_name     VARCHAR(255) DEFAULT '',
    last_throttle TIMESTAMP WITH TIME ZONE
                                );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS customer_sessions CASCADE;
-- +goose StatementEnd