-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS topic_ratings
(
    id          SERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    topic_id    INT NOT NULL,
    manager_id  BIGINT,
    score       INT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

ALTER TABLE customer_topics ADD COLUMN active_manager_id BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS topic_ratings CASCADE;
ALTER TABLE customer_topics DROP COLUMN active_manager_id;
-- +goose StatementEnd