-- +goose Up
ALTER TABLE users ADD COLUMN phone VARCHAR(20) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN phone;