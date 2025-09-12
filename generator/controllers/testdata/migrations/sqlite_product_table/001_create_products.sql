-- +goose Up
CREATE TABLE products (
    id TEXT PRIMARY KEY DEFAULT (uuid()),
    name TEXT NOT NULL,
    price REAL NOT NULL,
    description TEXT,
    category_id INTEGER NOT NULL,
    in_stock BOOLEAN DEFAULT 1,
    metadata TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE products;