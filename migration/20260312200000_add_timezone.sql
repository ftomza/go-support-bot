-- +goose Up
-- +goose StatementBegin
ALTER TABLE categories ADD COLUMN timezone VARCHAR(50) DEFAULT 'UTC';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE categories DROP COLUMN timezone;
-- +goose StatementEnd