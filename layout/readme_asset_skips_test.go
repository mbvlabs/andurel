package layout

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestReadmeDocumentsFrontendAssetGatedTests(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		body, err := fs.ReadFile(templates.Files, name)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	runtime := read("runtime_runtime_test.tmpl")
	readme := read("readme.tmpl")
	start := strings.Index(readme, "A fresh scaffold skips these app tests")
	if start < 0 {
		t.Fatal("README missing frontend asset skip guidance")
	}
	section, _, _ := strings.Cut(readme[start:], "\n\n")
	functions := regexp.MustCompile(`(?m)^func (\w+)\(.*`)
	names := regexp.MustCompile("`(Test[a-zA-Z0-9_]+)`")
	gated := map[string]bool{}
	matches := functions.FindAllStringSubmatchIndex(runtime, -1)
	for i, match := range matches {
		name := runtime[match[2]:match[3]]
		end := len(runtime)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := runtime[match[1]:end]
		if strings.HasPrefix(name, "Test") && (strings.Contains(body, "firstViteAssetPath(t)") || strings.Contains(body, "requireProductionAssets(t)")) {
			gated[name] = true
		}
	}
	if len(gated) == 0 {
		t.Fatal("no frontend asset-gated tests found")
	}
	documented := map[string]bool{}
	for _, match := range names.FindAllStringSubmatch(section, -1) {
		documented[match[1]] = true
	}
	for name := range gated {
		if !documented[name] {
			t.Errorf("README missing asset-gated test %s", name)
		}
	}
	for name := range documented {
		if !gated[name] {
			t.Errorf("README lists non-gated test %s", name)
		}
	}
}
