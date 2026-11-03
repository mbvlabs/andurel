package watcher

import (
	"path/filepath"
	"testing"
)

func TestIsRebuildSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tests := []struct {
		path string
		want bool
	}{
		{filepath.Join(dir, "main.go"), true},
		{filepath.Join(dir, "resources", "root.html"), true},
		{filepath.Join(dir, "resources", "nested", "page.html"), true},
		{filepath.Join(dir, "resources", "root.templ"), false},
		{filepath.Join(dir, "emails", "templates", "reset.html.tmpl"), false},
		{filepath.Join(dir, "README.md"), false},
		{filepath.Join(dir, "..", "resources", "root.html"), false},
	}
	for _, test := range tests {
		if got := isRebuildSource(dir, test.path); got != test.want {
			t.Errorf("isRebuildSource(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}
