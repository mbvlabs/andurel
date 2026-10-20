package connection

import "github.com/mbvlabs/andurel/pkg/storage"

type Holder struct {
	conn storage.Connection  // want "db-handle"
	tx   storage.Transaction // want "db-handle"
}
