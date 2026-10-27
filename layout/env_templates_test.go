package layout

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestEnvTemplateListsConfigKeys(t *testing.T) {
	configFiles, err := fs.Glob(layouttemplates.Files, "config_*.tmpl")
	if err != nil {
		t.Fatalf("glob config templates: %v", err)
	}
	if len(configFiles) == 0 {
		t.Fatal("no config templates found")
	}
	envBytes, err := fs.ReadFile(layouttemplates.Files, "env.tmpl")
	if err != nil {
		t.Fatalf("read env.tmpl: %v", err)
	}
	env := string(envBytes)

	// Match env reads even when the call spans lines.
	readPattern := regexp.MustCompile(
		`(?s)e\.(?:String|RequiredString|Strings|Int|Int32|Int64|Bool|Float64|Duration)\(\s*"([A-Z0-9_]+)"`,
	)
	// Ignore commented examples such as the HOST_ADMIN snippet in
	// config_hosts.tmpl.
	lineComment := regexp.MustCompile(`(?m)^\s*//.*$`)

	for _, name := range configFiles {
		body, err := fs.ReadFile(layouttemplates.Files, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := lineComment.ReplaceAllString(string(body), "")
		for _, match := range readPattern.FindAllStringSubmatch(source, -1) {
			key := match[1]
			if !strings.Contains(env, key+"=") {
				t.Errorf("%s reads %s but env.tmpl does not list it", name, key)
			}
		}
	}
}
