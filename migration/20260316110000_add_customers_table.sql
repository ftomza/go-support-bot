-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS customers
(
    id         BIGINT PRIMARY KEY,
    full_name  VARCHAR(255) NOT NULL,
    username   VARCHAR(255),
    is_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- Переносим данные из сессий, если они там есть (на всякий случай)
INSERT INTO customers (id, full_name, is_blocked)
SELECT customer_id, full_name, is_blocked
FROM customer_sessions
    ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS customers CASCADE;
-- +goose StatementEnd