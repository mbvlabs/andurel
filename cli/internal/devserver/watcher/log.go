package watcher

import (
	"fmt"
	"io"
	"os"
)

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		w = os.Stdout
	}
	_, _ = fmt.Fprintf(w, format, args...)
}
