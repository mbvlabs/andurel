package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
)

// TestSetVersionNarsilcRuntimeMismatchIsNotSilent reproduces B7-002: changing
// the narsilc tool version leaves an older narsilc runtime pin in go.mod, and
// set-version must name the exact remediation instead of reporting a clean
// success.
func TestSetVersionNarsilcRuntimeMismatchIsNotSilent(t *testing.T) {
	resetCLITestSeams(t)

	root := t.TempDir()
	writeTestFile(t, root, "go.mod", `module example.com/app

go 1.27.3

require github.com/mbvlabs/narsilc v0.6.3
`)
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	installToolVersionAndLockFunc = func(
		projectRoot, name string,
		tool *layout.Tool,
		lock *layout.AndurelLock,
		goos, goarch string,
	) error {
		return lock.WriteLockFile(projectRoot)
	}

	stdout := captureProcessOutput(t, &os.Stdout)
	if err := setVersion(root, "narsilc", "v0.6.4", testChecksumArguments()...); err != nil {
		t.Fatalf("setVersion: %v", err)
	}
	out := stdout()

	for _, want := range []string{
		"narsilc runtime",
		"go get github.com/mbvlabs/narsilc@v0.6.4",
		"go mod tidy",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("set-version output missing %q:\n%s", want, out)
		}
	}

	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !bytes.Contains(goMod, []byte("github.com/mbvlabs/narsilc v0.6.3")) {
		t.Fatalf("set-version must not mutate go.mod:\n%s", goMod)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
		t.Fatalf("set-version must not create go.sum: %v", err)
	}
}

func TestInspectNarsilcRuntimeStates(t *testing.T) {
	const moduleHeader = "module example.com/app\n\ngo 1.27.3\n\n"
	tests := []struct {
		name            string
		goMod           string
		toolVersion     string
		wantStatus      string
		wantRequired    string
		wantReplacement string
		wantSteps       []string
	}{
		{
			name:         "aligned",
			goMod:        moduleHeader + "require github.com/mbvlabs/narsilc v0.6.4\n",
			toolVersion:  "v0.6.4",
			wantStatus:   narsilcRuntimeAligned,
			wantRequired: "v0.6.4",
		},
		{
			name:         "mismatched",
			goMod:        moduleHeader + "require github.com/mbvlabs/narsilc v0.6.3\n",
			toolVersion:  "0.6.4",
			wantStatus:   narsilcRuntimeMismatch,
			wantRequired: "v0.6.3",
			wantSteps: []string{
				"go get github.com/mbvlabs/narsilc@v0.6.4",
				"go mod tidy",
			},
		},
		{
			name:         "missing requirement",
			goMod:        moduleHeader + "require example.com/other v1.0.0\n",
			toolVersion:  "v0.6.4",
			wantStatus:   narsilcRuntimeMissing,
			wantRequired: "",
		},
		{
			name: "local replacement",
			goMod: moduleHeader +
				"require github.com/mbvlabs/narsilc v0.6.4\n\n" +
				"replace github.com/mbvlabs/narsilc => ../narsilc\n",
			toolVersion:     "v0.6.4",
			wantStatus:      narsilcRuntimeReplaced,
			wantRequired:    "v0.6.4",
			wantReplacement: "github.com/mbvlabs/narsilc => ../narsilc",
		},
		{
			name: "versioned replacement",
			goMod: moduleHeader +
				"require github.com/mbvlabs/narsilc v0.6.4\n\n" +
				"replace github.com/mbvlabs/narsilc v0.6.4 => example.com/fork v0.6.4\n",
			toolVersion:     "v0.6.4",
			wantStatus:      narsilcRuntimeReplaced,
			wantRequired:    "v0.6.4",
			wantReplacement: "github.com/mbvlabs/narsilc v0.6.4 => example.com/fork v0.6.4",
		},
		{
			name: "non-applying versioned replacement",
			goMod: moduleHeader +
				"require github.com/mbvlabs/narsilc v0.6.4\n\n" +
				"replace github.com/mbvlabs/narsilc v0.6.3 => ../old-narsilc\n",
			toolVersion:  "v0.6.4",
			wantStatus:   narsilcRuntimeAligned,
			wantRequired: "v0.6.4",
		},
		{
			name:         "missing go.mod",
			toolVersion:  "v0.6.4",
			wantStatus:   narsilcRuntimeUnverifiable,
			wantRequired: "",
		},
		{
			name:         "malformed go.mod",
			goMod:        "module example.com/app\n\nrequire (\n",
			toolVersion:  "v0.6.4",
			wantStatus:   narsilcRuntimeUnverifiable,
			wantRequired: "",
		},
		{
			name:         "invalid tool version",
			goMod:        moduleHeader + "require github.com/mbvlabs/narsilc v0.6.4\n",
			toolVersion:  "not-semver",
			wantStatus:   narsilcRuntimeUnverifiable,
			wantRequired: "v0.6.4",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.goMod != "" {
				writeTestFile(t, root, "go.mod", test.goMod)
			}

			state := inspectNarsilcRuntime(root, test.toolVersion)
			if state.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q (%#v)", state.Status, test.wantStatus, state)
			}
			if state.RequiredVersion != test.wantRequired {
				t.Fatalf("required version = %q, want %q", state.RequiredVersion, test.wantRequired)
			}
			if state.Replacement != test.wantReplacement {
				t.Fatalf("replacement = %q, want %q", state.Replacement, test.wantReplacement)
			}
			if got := state.remediation(); !reflect.DeepEqual(got, test.wantSteps) {
				t.Fatalf("remediation = %#v, want %#v", got, test.wantSteps)
			}
		})
	}
}

func TestSetVersionNarsilcRuntimeReportMatrix(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		toolVersion string
		goMod       string
		wantStatus  string
		wantWarning bool
		wantSteps   []string
	}{
		{
			name:        "equal versions",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod:       "module example.com/app\n\ngo 1.27.3\n\nrequire github.com/mbvlabs/narsilc v0.6.4\n",
			wantStatus:  narsilcRuntimeAligned,
		},
		{
			name:        "mismatched versions",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod:       "module example.com/app\n\ngo 1.27.3\n\nrequire github.com/mbvlabs/narsilc v0.6.3\n",
			wantStatus:  narsilcRuntimeMismatch,
			wantWarning: true,
			wantSteps: []string{
				"go get github.com/mbvlabs/narsilc@v0.6.4",
				"go mod tidy",
			},
		},
		{
			name:        "missing runtime requirement is not injected",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod:       "module example.com/app\n\ngo 1.27.3\n",
			wantStatus:  narsilcRuntimeMissing,
		},
		{
			name:        "local replacement is unverifiable",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod: "module example.com/app\n\ngo 1.27.3\n\n" +
				"require github.com/mbvlabs/narsilc v0.6.4\n\n" +
				"replace github.com/mbvlabs/narsilc => ../narsilc\n",
			wantStatus:  narsilcRuntimeReplaced,
			wantWarning: true,
		},
		{
			name:        "versioned replacement is unverifiable",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod: "module example.com/app\n\ngo 1.27.3\n\n" +
				"require github.com/mbvlabs/narsilc v0.6.4\n\n" +
				"replace github.com/mbvlabs/narsilc v0.6.4 => example.com/fork v0.6.4\n",
			wantStatus:  narsilcRuntimeReplaced,
			wantWarning: true,
		},
		{
			name:        "unavailable module data is unverifiable",
			tool:        "narsilc",
			toolVersion: "v0.6.4",
			goMod:       "module example.com/app\n\nrequire (\n",
			wantStatus:  narsilcRuntimeUnverifiable,
			wantWarning: true,
		},
		{
			name:        "unrelated tool leaves narsilc alone",
			tool:        "templ",
			toolVersion: "v4.0.0",
			goMod:       "module example.com/app\n\ngo 1.27.3\n\nrequire github.com/mbvlabs/narsilc v0.6.3\n",
			wantStatus:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCLITestSeams(t)
			root := t.TempDir()
			writeTestFile(t, root, "go.mod", test.goMod)
			lock := layout.NewAndurelLock("test")
			if err := lock.WriteLockFile(root); err != nil {
				t.Fatalf("write lock: %v", err)
			}
			installToolVersionAndLockFunc = func(
				projectRoot, name string,
				tool *layout.Tool,
				lock *layout.AndurelLock,
				goos, goarch string,
			) error {
				return lock.WriteLockFile(projectRoot)
			}

			before, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil {
				t.Fatalf("read go.mod: %v", err)
			}

			stdout := captureProcessOutput(t, &os.Stdout)
			report, err := setVersionAndReport(
				root,
				test.tool,
				test.toolVersion,
				testChecksumArguments()...,
			)
			out := stdout()
			if err != nil {
				t.Fatalf("setVersionAndReport: %v", err)
			}

			if test.wantStatus == "" {
				if report.NarsilcRuntime != nil {
					t.Fatalf("unrelated tool produced narsilc report: %#v", report.NarsilcRuntime)
				}
			} else {
				if report.NarsilcRuntime == nil {
					t.Fatalf("missing narsilc runtime report")
				}
				if report.NarsilcRuntime.Status != test.wantStatus {
					t.Fatalf("status = %q, want %q", report.NarsilcRuntime.Status, test.wantStatus)
				}
			}
			if got := len(report.Warnings) > 0; got != test.wantWarning {
				t.Fatalf("warning = %v, want %v (%#v)", got, test.wantWarning, report.Warnings)
			}
			if !reflect.DeepEqual(report.NextSteps, test.wantSteps) {
				t.Fatalf("next steps = %#v, want %#v", report.NextSteps, test.wantSteps)
			}
			if test.wantWarning && !strings.Contains(out, "narsilc runtime") {
				t.Fatalf("human output missing warning:\n%s", out)
			}

			after, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil {
				t.Fatalf("read go.mod after: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("go.mod changed:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
				t.Fatalf("set-version must not create go.sum: %v", err)
			}
		})
	}
}

func TestSetVersionInstallFailureDoesNotWarnAboutRuntime(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	const goMod = `module example.com/app

go 1.27.3

require github.com/mbvlabs/narsilc v0.6.3
`
	writeTestFile(t, root, "go.mod", goMod)
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	installToolVersionAndLockFunc = func(
		string, string, *layout.Tool, *layout.AndurelLock, string, string,
	) error {
		return errors.New("download failed")
	}

	stdout := captureProcessOutput(t, &os.Stdout)
	report, err := setVersionAndReport(root, "narsilc", "v0.6.4", testChecksumArguments()...)
	out := stdout()
	if err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("install failure = %v", err)
	}
	if report.NarsilcRuntime != nil || len(report.Warnings) > 0 || len(report.NextSteps) > 0 {
		t.Fatalf("failed install must not report runtime guidance: %#v", report)
	}
	if strings.Contains(out, "go get github.com/mbvlabs/narsilc") {
		t.Fatalf("failed install emitted remediation:\n%s", out)
	}
	after, readErr := os.ReadFile(filepath.Join(root, "go.mod"))
	if readErr != nil {
		t.Fatalf("read go.mod: %v", readErr)
	}
	if string(after) != goMod {
		t.Fatalf("failed install mutated go.mod:\n%s", after)
	}
}

func TestSetVersionStructuredNarsilcMismatch(t *testing.T) {
	resetCLITestSeams(t)
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", `module example.com/app

go 1.27.3

require github.com/mbvlabs/narsilc v0.6.3
`)
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	installToolVersionAndLockFunc = func(
		projectRoot, name string,
		tool *layout.Tool,
		lock *layout.AndurelLock,
		goos, goarch string,
	) error {
		return lock.WriteLockFile(projectRoot)
	}
	findGoModRoot = func() (string, error) { return root, nil }

	cmd := newSetVersionCommand()
	output.RegisterPersistentFlags(cmd)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	args := []string{"narsilc", "v0.6.4", "--json"}
	for _, checksum := range testChecksumArguments() {
		args = append(args, "--sha256", checksum)
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute set-version --json: %v\nstderr:\n%s", err, stderr.String())
	}

	if strings.Contains(stdout.String(), "Setting narsilc") ||
		strings.Contains(stdout.String(), "Successfully updated") {
		t.Fatalf("structured output leaked human progress:\n%s", stdout.String())
	}

	var envelope struct {
		OK          bool              `json:"ok"`
		Summary     string            `json:"summary"`
		Data        toolVersionReport `json:"data"`
		Breadcrumbs []struct {
			Command     string `json:"cmd"`
			Description string `json:"description"`
		} `json:"breadcrumbs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode set-version envelope: %v\n%s", err, stdout.String())
	}
	if !envelope.OK {
		t.Fatalf("set-version envelope not ok: %#v", envelope)
	}
	if envelope.Data.NarsilcRuntime == nil ||
		envelope.Data.NarsilcRuntime.Status != narsilcRuntimeMismatch {
		t.Fatalf("unexpected runtime data: %#v", envelope.Data.NarsilcRuntime)
	}
	if !reflect.DeepEqual(envelope.Data.NextSteps, []string{
		"go get github.com/mbvlabs/narsilc@v0.6.4",
		"go mod tidy",
	}) {
		t.Fatalf("next steps = %#v", envelope.Data.NextSteps)
	}
	if len(envelope.Data.Warnings) != 1 ||
		!strings.Contains(envelope.Data.Warnings[0], "narsilc runtime mismatch") {
		t.Fatalf("warnings = %#v", envelope.Data.Warnings)
	}
	if !strings.Contains(envelope.Summary, "narsilc runtime requires v0.6.3") {
		t.Fatalf("summary = %q", envelope.Summary)
	}
	if len(envelope.Breadcrumbs) != 2 ||
		envelope.Breadcrumbs[0].Command != "go get github.com/mbvlabs/narsilc@v0.6.4" ||
		envelope.Breadcrumbs[1].Command != "go mod tidy" {
		t.Fatalf("breadcrumbs = %#v", envelope.Breadcrumbs)
	}
}
