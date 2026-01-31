-- +goose Up
CREATE TABLE testers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);

-- +goose Down
DROP TABLE testers;
