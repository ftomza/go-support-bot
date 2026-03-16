-- +goose Up
-- +goose StatementBegin

-- 1. Добавляем флаг блокировки для клиентов
ALTER TABLE customer_sessions ADD COLUMN is_blocked BOOLEAN NOT NULL DEFAULT FALSE;

-- 2. Таблица самих рассылок
CREATE TABLE IF NOT EXISTS broadcasts
(
    id         SERIAL PRIMARY KEY,
    text       TEXT NOT NULL,
    status     VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

-- 3. Таблица получателей рассылки (очередь отправки)
CREATE TABLE IF NOT EXISTS broadcast_recipients
(
    id           SERIAL PRIMARY KEY,
    broadcast_id INT NOT NULL REFERENCES broadcasts(id) ON DELETE CASCADE,
    customer_id  BIGINT NOT NULL,
    status       VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, sent, failed
    error_text   TEXT,
    sent_at      TIMESTAMP WITH TIME ZONE
                                                            );

-- Индекс для быстрого поиска тех, кому еще не отправили (нужно для воркера)
CREATE INDEX idx_broadcast_recipients_pending ON broadcast_recipients(broadcast_id, status) WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS broadcast_recipients CASCADE;
DROP TABLE IF EXISTS broadcasts CASCADE;
ALTER TABLE customer_sessions DROP COLUMN is_blocked;
-- +goose StatementEnd