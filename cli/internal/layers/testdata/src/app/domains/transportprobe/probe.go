package transportprobe

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/anduin" // want "domain-transport"
)

// Shape 8: HTTP helper leaking into the domain layer.
func ProbeHTTP(ctx context.Context) string {
	return anduin.Query(ctx, "x")
}
