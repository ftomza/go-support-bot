-- +goose Up
-- +goose StatementBegin
ALTER TABLE categories ADD COLUMN image VARCHAR(1024);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE categories DROP COLUMN image;
-- +goose StatementEnd