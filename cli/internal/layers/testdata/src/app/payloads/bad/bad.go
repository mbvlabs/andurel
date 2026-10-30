package bad

import (
	"net/http" // want "payloads-import"

	"app/domains" // want "payloads-import"

	"github.com/mbvlabs/andurel/pkg/anduin"  // want "payloads-import"
	"github.com/mbvlabs/andurel/pkg/inertia" // want "payloads-import"
	"github.com/mbvlabs/andurel/pkg/storage" // want "payloads-import"
)

var (
	_ http.Handler
	_ domains.User
	_ = anduin.Query
	_ inertia.Renderer
	_ storage.Connection
)
