// Package domains owns business rules and persistence. Domain writes validate
// before querying; services coordinate work spanning multiple receivers.
//
// A receiver holds a storage.Transactor when one of its own methods must be
// atomic by itself, either because it writes more than once or because it
// reads, modifies and writes a row.
package domains

import "github.com/mbvlabs/andurel/pkg/storage"

var (
	ErrNotFound = storage.ErrNotFound
	ErrConflict = storage.ErrConflict
)
