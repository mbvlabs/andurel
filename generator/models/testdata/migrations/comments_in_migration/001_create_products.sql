-- +goose Up
-- This migration creates the products table for the e-commerce module
-- Author: test@example.com
-- Date: 2024-01-15

CREATE TABLE products (
    id UUID PRIMARY KEY, -- unique identifier for the product
    created_at TIMESTAMP WITH TIME ZONE NOT NULL, -- record creation timestamp
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL, -- last update timestamp
    name VARCHAR(255) NOT NULL, /* product display name */
    description TEXT, -- optional product description
    price NUMERIC(10, 2) NOT NULL, /* price in USD with 2 decimal places */
    sku VARCHAR(50) NOT NULL UNIQUE, -- stock keeping unit
    is_active BOOLEAN DEFAULT true /* whether product is available for sale */
);

/* 
 * Create index for faster lookups by SKU
 * This is commonly used in inventory management
 */
CREATE INDEX idx_products_sku ON products(sku);

-- +goose Down
DROP TABLE products;
