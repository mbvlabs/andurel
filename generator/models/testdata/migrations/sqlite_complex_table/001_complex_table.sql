-- +goose Up
CREATE TABLE products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description CLOB,
    price NUMERIC(10,2),
    weight REAL,
    quantity INTEGER NOT NULL DEFAULT 0,
    in_stock BOOLEAN DEFAULT true,
    tags TEXT,
    metadata BLOB,
    created_date DATE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME,
    category_id INTEGER,
    is_featured BOOLEAN,
    discount_rate REAL DEFAULT 0.0
);

-- +goose Down
DROP TABLE products;