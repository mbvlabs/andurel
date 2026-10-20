package unwrap

import "github.com/mbvlabs/andurel/pkg/storage"

func Bind(tx storage.Tx) storage.Transaction {
	return storage.Unwrap(tx) // want "tx-unwrap"
}

func BindAlias(tx storage.Tx) storage.Transaction {
	unwrap := storage.Unwrap // want "tx-unwrap"
	return unwrap(tx)
}

var unwrapValue = storage.Unwrap // want "tx-unwrap"
