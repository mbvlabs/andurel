package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledToolVersionAndMatching(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "tool")
	writeVersionScript(t, binary, "tool version v1.2.3")

	actual, err := installedToolVersion(binary, &VersionCheck{Args: []string{"--version"}})
	if err != nil || actual != "1.2.3" {
		t.Fatalf("generic version check = %q, %v", actual, err)
	}

	actual, err = installedToolVersion(binary, &VersionCheck{
		Args:   []string{"version"},
		Regexp: `tool version (v[0-9.]+)`,
	})
	if err != nil || actual != "v1.2.3" {
		t.Fatalf("custom version check = %q, %v", actual, err)
	}

	for _, test := range []struct {
		name  string
		check *VersionCheck
		want  string
	}{
		{name: "missing check", want: "versionCheck.args is required"},
		{name: "missing args", check: &VersionCheck{}, want: "versionCheck.args is required"},
		{name: "invalid regexp", check: &VersionCheck{Args: []string{"version"}, Regexp: "["}, want: "error parsing regexp"},
		{name: "no match", check: &VersionCheck{Args: []string{"version"}, Regexp: `release ([0-9.]+)`}, want: "did not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := installedToolVersion(binary, test.check)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	if _, err := installedToolVersion(filepath.Join(root, "missing"), &VersionCheck{Args: []string{"--version"}}); err == nil {
		t.Fatal("missing executable should fail")
	}

	for _, test := range []struct {
		expected string
		actual   string
		want     bool
	}{
		{expected: "v1.2.3", actual: "1.2.3", want: true},
		{expected: " 1.2.3 ", actual: "v1.2.3", want: true},
		{expected: "v1.2.3", actual: "v1.2.4", want: false},
	} {
		if got := lockVersionsMatch(test.expected, test.actual); got != test.want {
			t.Fatalf("lockVersionsMatch(%q, %q) = %t", test.expected, test.actual, got)
		}
	}
}

func TestDownloadToolBinaryValidationErrors(t *testing.T) {
	digest := strings.Repeat("a", 64)
	tool := &Tool{
		Version: "v1.2.3",
		Download: &ToolDownload{
			URLTemplate: "https://example.com/{{version}}/{{os}}/{{arch}}",
			Archive:     "binary",
			BinaryName:  "tool",
			SHA256:      map[string]string{"linux/amd64": digest},
		},
	}

	tests := []struct {
		name     string
		toolName string
		tool     *Tool
		goos     string
		goarch   string
		want     string
	}{
		{name: "nil tool", toolName: "tool", want: "configuration is nil"},
		{name: "unsupported platform", toolName: "tool", tool: tool, goos: "plan9", goarch: "amd64", want: "unsupported platform"},
		{name: "missing digest", toolName: "tool", tool: &Tool{Download: &ToolDownload{URLTemplate: "https://example.com"}}, goos: "linux", goarch: "amd64", want: "missing SHA-256"},
		{name: "default archive", toolName: "tool", tool: &Tool{Version: "v1.0.0", Download: &ToolDownload{URLTemplate: "http://example.com", SHA256: map[string]string{"linux/amd64": digest}}}, goos: "linux", goarch: "amd64", want: "must use HTTPS"},
		{name: "source only", toolName: "tool", tool: &Tool{Source: "example.com/tool"}, want: "source downloads require explicit"},
		{name: "tailwind without metadata", toolName: "tailwindcli", tool: &Tool{}, want: "tailwindcli downloads require explicit"},
		{name: "no metadata", toolName: "tool", tool: &Tool{}, want: "no download metadata"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := downloadToolBinary(test.toolName, test.tool, test.goos, test.goarch, filepath.Join(t.TempDir(), "tool"))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLockSyncSkipsCurrentBinaryAndRejectsStaleUndownloadableTool(t *testing.T) {
	newLock := func() *AndurelLock {
		lock := NewAndurelLock("v1.0.0")
		lock.Tools["tool"] = NewBuiltTool("cmd/tool/main.go", "v1.2.3")
		return lock
	}

	currentRoot := t.TempDir()
	currentBinary := filepath.Join(currentRoot, "bin", "tool")
	writeVersionScript(t, currentBinary, "tool v1.2.3")
	if err := newLock().Sync(currentRoot, true); err != nil {
		t.Fatalf("sync current binary: %v", err)
	}
	if _, err := ReadLockFile(currentRoot); err != nil {
		t.Fatalf("sync did not write a valid lock: %v", err)
	}

	staleRoot := t.TempDir()
	staleBinary := filepath.Join(staleRoot, "bin", "tool")
	writeVersionScript(t, staleBinary, "tool v1.0.0")
	err := newLock().Sync(staleRoot, true)
	if err == nil || !strings.Contains(err.Error(), "tool has no download metadata") {
		t.Fatalf("stale undownloadable tool error = %v", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(staleRoot, "bin", ".andurel-candidate-*"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("candidate files were not cleaned up: %v, %v", matches, globErr)
	}

	missingRoot := t.TempDir()
	err = newLock().Sync(missingRoot, true)
	if err == nil || !strings.Contains(err.Error(), "tool has no download metadata") {
		t.Fatalf("missing undownloadable tool error = %v", err)
	}

	if err := (&AndurelLock{}).Sync(t.TempDir(), true); err == nil || !strings.Contains(err.Error(), "invalid lock file") {
		t.Fatalf("invalid lock sync error = %v", err)
	}
}

func TestLockRuntimeValidationAndLookupErrors(t *testing.T) {
	if spec, ok := GetDefaultToolVersionCheck("missing"); ok || spec != nil {
		t.Fatalf("unknown version check = %#v, %t", spec, ok)
	}
	if spec, ok := GetDefaultToolDownload("missing"); ok || spec != nil {
		t.Fatalf("unknown download = %#v, %t", spec, ok)
	}
	if cloned := cloneStringMap(nil); cloned != nil {
		t.Fatalf("cloneStringMap(nil) = %#v", cloned)
	}

	if err := (&AndurelLock{}).WriteLockFile(t.TempDir()); err == nil || !strings.Contains(err.Error(), "failed to validate") {
		t.Fatalf("invalid lock write error = %v", err)
	}
	valid := NewAndurelLock("v1.0.0")
	if err := valid.WriteLockFile(filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "failed to write") {
		t.Fatalf("missing directory write error = %v", err)
	}
	if _, err := ReadLockFile(filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("missing lock read error = %v", err)
	}
}

func writeVersionScript(t *testing.T, path, output string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "#!/bin/sh\nprintf '%s\\n' '" + output + "'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
