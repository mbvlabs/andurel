package golden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

type inspectViewsEnvelope[T any] struct {
	OK   bool `json:"ok"`
	Data T    `json:"data"`
}

type generatedPageComponent struct {
	Component string `json:"component"`
	File      string `json:"file"`
	Exists    bool   `json:"exists"`
	Routes    []struct {
		Name   string `json:"name"`
		Method string `json:"method"`
	} `json:"routes"`
}

type generatedPagesReport struct {
	Pages []generatedPageComponent `json:"pages"`
}

type generatedProcess struct {
	Name                string `json:"name"`
	ResponsibilityKnown bool   `json:"responsibility_known"`
}

type generatedProcessReport struct {
	Processes []generatedProcess `json:"processes"`
}

type generatedURLRoute struct {
	Name             string `json:"name"`
	Resolved         bool   `json:"resolved"`
	URL              string `json:"url"`
	UnresolvedReason string `json:"unresolved_reason"`
}

type generatedURLsReport struct {
	Routes []generatedURLRoute `json:"routes"`
}

func TestInspectViewsOnGeneratedApp(t *testing.T) {
	goldentest.RequireBinary(t)

	parent := t.TempDir()
	goldentest.RunCLI(t, parent, "new", "app")
	project := filepath.Join(parent, "app")

	goldentest.RunCLI(t, project, "commands", "--check")
	configKeys, configExit := goldentest.RunCLIExit(
		t,
		project,
		"inspect",
		"config",
		"--jq",
		".keys",
	)
	if configExit != 0 {
		t.Fatalf("inspect config --jq .keys exited %d\n%s", configExit, configKeys)
	}
	var keys []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(configKeys, &keys); err != nil || len(keys) == 0 {
		t.Fatalf("inspect config --jq .keys failed: %v\n%s", err, configKeys)
	}

	pages := runInspectReport[generatedPagesReport](t, project, "pages")
	welcome := false
	for _, page := range pages.Pages {
		if page.Component != "Welcome" {
			continue
		}
		welcome = true
		if !page.Exists || page.File == "" {
			t.Fatalf("generated Welcome page = %#v", page)
		}
		foundRoute := false
		for _, route := range page.Routes {
			if route.Name == "pages.home" && route.Method == "GET" {
				foundRoute = true
			}
		}
		if !foundRoute {
			t.Fatalf("generated Welcome page routes = %#v", page.Routes)
		}
	}
	if !welcome {
		t.Fatalf("generated app has no Welcome page: %#v", pages.Pages)
	}

	processes := runInspectReport[generatedProcessReport](t, project, "process")
	names := map[string]bool{}
	for _, process := range processes.Processes {
		names[process.Name] = true
	}
	for _, name := range []string{"app", "queue", "migrate", "ssr", "seeds"} {
		if !names[name] {
			t.Fatalf("generated process list missing %s: %#v", name, processes.Processes)
		}
	}

	urls := runInspectReport[generatedURLsReport](t, project, "urls")
	_, envErr := os.Stat(filepath.Join(project, ".env"))
	if os.IsNotExist(envErr) {
		for _, route := range urls.Routes {
			if route.Resolved || route.URL != "" {
				t.Fatalf("generated app without .env invented URL for %s: %#v", route.Name, route)
			}
			if !strings.Contains(route.UnresolvedReason, "HOST_PRIMARY") {
				t.Fatalf("generated app route %s unresolved reason = %q", route.Name, route.UnresolvedReason)
			}
		}
	}
}

func runInspectReport[T any](t *testing.T, project string, view string) T {
	t.Helper()
	out, exitCode := goldentest.RunCLIExit(t, project, "inspect", view, "--json")
	if exitCode != 0 {
		t.Fatalf("inspect %s exited %d\n%s", view, exitCode, out)
	}
	var envelope inspectViewsEnvelope[T]
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("decode inspect %s: %v\n%s", view, err, out)
	}
	if !envelope.OK {
		t.Fatalf("inspect %s not ok:\n%s", view, out)
	}
	return envelope.Data
}
