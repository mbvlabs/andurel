//go:build !andurel_golden

package testseed

import (
	"crypto/rand"
	"io"
	"time"
)

// Enabled reports whether this binary was built with the andurel_golden tag.
func Enabled() bool {
	return false
}

// RandomReader returns the production CSPRNG reader.
func RandomReader() io.Reader {
	return rand.Reader
}

// Now returns the current wall-clock time.
func Now() time.Time {
	return time.Now()
}
