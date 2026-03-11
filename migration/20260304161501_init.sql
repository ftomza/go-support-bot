-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bot_settings
(
    key   VARCHAR(50) PRIMARY KEY,
    value TEXT NOT NULL
    );

CREATE TABLE IF NOT EXISTS categories
(
    id          SERIAL PRIMARY KEY,
    parent_id   INT REFERENCES categories (id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    prompt_text TEXT,
    manager_id  BIGINT,
    work_hours  VARCHAR(50)
    );

CREATE TABLE IF NOT EXISTS customer_topics
(
    customer_id BIGINT PRIMARY KEY,
    topic_id    INT     NOT NULL,
    category_id INT     REFERENCES categories (id) ON DELETE SET NULL,
    is_closed   BOOLEAN NOT NULL DEFAULT FALSE,
    lang_code   VARCHAR(10)      DEFAULT 'en'
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS customer_topics CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS bot_settings CASCADE;
-- +goose StatementEnd