package models

// andurel:table products

import (
	"time"
	"uuid"
)

// Product is a second Entity for bulk `sync factories` and missing-factory create.
type Product struct {
	ID        uuid.UUID `andurel:"id"`
	Name      string    `andurel:"name"`
	Price     int32     `andurel:"price"`
	CreatedAt time.Time `andurel:"created_at"`
}
