package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

// configCheckOutput mirrors the JSON the generated internal/configcheck prints.
type configCheckOutput struct {
	OK     bool `json:"ok"`
	Config struct {
		Status string   `json:"status"`
		Reason string   `json:"reason"`
		Errors []string `json:"errors"`
	} `json:"config"`
	Database *struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"database"`
}

// TestGeneratedConfigCheckRuntime renders the generated helper into a throwaway
// module and runs it the way andurel doctor does. It proves the helper uses the
// project's own config constructor (no hard-coded key list), accepts
// exported-only configuration, and reports an unreachable database instead of
// passing.
func TestGeneratedConfigCheckRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated config check runtime test in short mode")
	}

	root := renderConfigCheckModule(t)

	t.Run("missing configuration", func(t *testing.T) {
		out := runConfigCheck(t, root, nil)
		if out.Config.Status != "invalid" {
			t.Fatalf("config status = %q, want invalid", out.Config.Status)
		}
		if len(out.Config.Errors) == 0 {
			t.Fatal("missing configuration reported no errors")
		}
		if !strings.Contains(strings.Join(out.Config.Errors, "\n"), "HOST_PRIMARY is required") {
			t.Fatalf("errors do not name HOST_PRIMARY: %v", out.Config.Errors)
		}
		if out.Database != nil {
			t.Fatalf("database probe ran without valid config: %#v", out.Database)
		}
	})

	t.Run("exported-only configuration with unreachable database", func(t *testing.T) {
		out := runConfigCheck(t, root, map[string]string{
			"HOST_PRIMARY":           "localhost:8080",
			"TOKEN_SIGNING_KEY":      "test-token-signing-key",
			"PEPPER":                 "test-pepper",
			"SESSION_KEY":            strings.Repeat("ab", 32),
			"SESSION_ENCRYPTION_KEY": strings.Repeat("cd", 32),
			// Port 1 on loopback refuses connections, so the probe is bounded
			// and deterministic without a database.
			"DB_HOST":     "127.0.0.1",
			"DB_PORT":     "1",
			"DB_NAME":     "configcheck",
			"DB_USER":     "postgres",
			"DB_PASSWORD": "postgres",
		})
		if out.Config.Status != "valid" {
			t.Fatalf("config status = %q, errors = %v", out.Config.Status, out.Config.Errors)
		}
		if out.Database == nil || out.Database.Status != "unreachable" {
			t.Fatalf("database = %#v, want unreachable", out.Database)
		}
		if out.OK {
			t.Fatal("unreachable database reported ok")
		}
	})

	t.Run("invalid configuration does not echo the rejected value", func(t *testing.T) {
		secret := "not-hexadecimal-session-key"
		out := runConfigCheck(t, root, map[string]string{
			"HOST_PRIMARY":           "localhost:8080",
			"TOKEN_SIGNING_KEY":      "test-token-signing-key",
			"PEPPER":                 "test-pepper",
			"SESSION_KEY":            secret,
			"SESSION_ENCRYPTION_KEY": strings.Repeat("cd", 32),
		})
		if out.Config.Status != "invalid" {
			t.Fatalf("config status = %q, want invalid", out.Config.Status)
		}
		joined := strings.Join(out.Config.Errors, "\n")
		if !strings.Contains(joined, "SESSION_KEY") {
			t.Fatalf("errors do not name SESSION_KEY: %v", out.Config.Errors)
		}
		if strings.Contains(joined, secret) {
			t.Fatalf("config error leaked the rejected secret value:\n%s", joined)
		}
	})

	t.Run("database errors do not leak the password", func(t *testing.T) {
		password := "super-secret-password"
		out := runConfigCheck(t, root, map[string]string{
			"HOST_PRIMARY":           "localhost:8080",
			"TOKEN_SIGNING_KEY":      "test-token-signing-key",
			"PEPPER":                 "test-pepper",
			"SESSION_KEY":            strings.Repeat("ab", 32),
			"SESSION_ENCRYPTION_KEY": strings.Repeat("cd", 32),
			// An invalid port makes the driver reject the URL it would have
			// dialed, which is the message most likely to quote it.
			"DB_PORT":     "notaport",
			"DB_NAME":     "configcheck",
			"DB_USER":     "postgres",
			"DB_PASSWORD": password,
		})
		if out.Database == nil || out.Database.Status != "unreachable" {
			t.Fatalf("database = %#v, want unreachable", out.Database)
		}
		if strings.Contains(out.Database.Reason, password) {
			t.Fatalf("database error leaked the password:\n%s", out.Database.Reason)
		}
	})
}

// renderConfigCheckModule writes the config package and the config check into a
// temporary module wired to this checkout's pkg modules.
func renderConfigCheckModule(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	data := &TemplateData{ModuleName: "testapp", ProjectName: "testapp"}
	for _, name := range []string{
		"config", "app", "hosts", "http", "session", "database", "queue", "telemetry",
		"email", "auth", "cache", "inertia",
	} {
		if err := renderTemplate(
			root, "config_"+name+".tmpl", "config/"+name+".go", layouttemplates.Files, data,
		); err != nil {
			t.Fatalf("render config %s: %v", name, err)
		}
	}
	if err := renderTemplate(
		root,
		"internal_configcheck_main.tmpl",
		"internal/configcheck/main.go",
		layouttemplates.Files,
		data,
	); err != nil {
		t.Fatalf("render config check: %v", err)
	}

	var module strings.Builder
	if _, err := fmt.Fprintf(&module, `module testapp

go %s

require (
	github.com/gosimple/slug v1.15.0
)
`, goVersion); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"anduin", "cache", "env", "inertia", "routing", "storage", "telemetry", "validation",
	} {
		path, err := filepath.Abs(filepath.Join("..", "pkg", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(
			&module,
			"\nrequire github.com/mbvlabs/andurel/pkg/%s v0.0.0\n"+
				"replace github.com/mbvlabs/andurel/pkg/%s => %q\n",
			name, name, filepath.ToSlash(path),
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

// runConfigCheck runs the generated helper with only the supplied environment,
// so the host's own configuration cannot make a case pass or fail.
func runConfigCheck(t *testing.T, root string, env map[string]string) configCheckOutput {
	t.Helper()

	cmd := exec.Command("go", "run", "./internal/configcheck")
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"GOWORK=off",
		"GOFLAGS=-mod=mod",
	}
	if cache := os.Getenv("GOCACHE"); cache != "" {
		cmd.Env = append(cmd.Env, "GOCACHE="+cache)
	}
	if moduleCache := os.Getenv("GOMODCACHE"); moduleCache != "" {
		cmd.Env = append(cmd.Env, "GOMODCACHE="+moduleCache)
	}
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("config check failed: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("run config check: %v", err)
	}

	var result configCheckOutput
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode config check output: %v\n%s", err, output)
	}

	return result
}
