-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY DEFAULT (uuid()),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    age INTEGER,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE users;