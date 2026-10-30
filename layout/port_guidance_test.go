package layout

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestReadmeExplainsVersionDiscoveryAndDevelopmentSeeds(t *testing.T) {
	body, err := fs.ReadFile(templates.Files, "readme.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"andurel --version",
		"andurel inspect project --json",
		"Development seeds create demo accounts with known passwords",
		"Never run these seeds against production",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("scaffold README missing %q", want)
		}
	}
}
