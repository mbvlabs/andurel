package layout

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestReadmeExplainsHostFallbackVerification(t *testing.T) {
	body, err := fs.ReadFile(templates.Files, "readme.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	section := strings.SplitN(string(body), "## Multiple Domains", 2)
	if len(section) != 2 {
		t.Fatal("README missing Multiple Domains section")
	}
	for _, want := range []string{
		"Plain `andurel verify` does not statically check for missing host fallback policies",
		"andurel verify --with-tests",
		"application startup test runs",
		"scoped or skipped tests may not exercise startup",
	} {
		if !strings.Contains(section[1], want) {
			t.Errorf("README multi-domain guidance missing %q", want)
		}
	}
}
