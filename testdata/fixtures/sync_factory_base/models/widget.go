package models

// andurel:table widgets

import (
	"time"
	"uuid"
)

// Widget is a minimal Entity for factory sync goldens.
// Bootstrap: conceptually from `andurel generate model Widget`, then slimmed
// to the Entity + table marker that factory sync reads.
type Widget struct {
	ID        uuid.UUID `andurel:"id"`
	Name      string    `andurel:"name"`
	Quantity  int32     `andurel:"quantity"`
	Active    bool      `andurel:"active"`
	CreatedAt time.Time `andurel:"created_at"`
	UpdatedAt time.Time `andurel:"updated_at"`
}
