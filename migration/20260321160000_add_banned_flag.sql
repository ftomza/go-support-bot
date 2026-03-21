-- +goose Up
-- +goose StatementBegin
ALTER TABLE customers ADD COLUMN is_banned BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE customers DROP COLUMN is_banned;
-- +goose StatementEnd