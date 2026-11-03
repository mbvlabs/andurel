package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

func TestEmailRenderBuildsCommandArgs(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "cmd/emails/main.go", "package main\n")

	originalFindGoModRoot := findGoModRoot
	originalRunner := runEmailRenderCommand
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runEmailRenderCommand = originalRunner
	})

	var calls [][]string
	runEmailRenderCommand = func(
		rootDir string,
		args []string,
		stdout, stderr io.Writer,
	) error {
		if rootDir != root {
			t.Fatalf("rootDir = %q, want %q", rootDir, root)
		}
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 0 && args[len(args)-1] == "-json" {
			_, _ = io.WriteString(
				stdout,
				`{"emails":[{"name":"verify_email","subject":"Verify your email","html":"tmp/emails/verify_email.html","text":"tmp/emails/verify_email.txt","bytes":42}]}`,
			)
		}
		return nil
	}

	var humanOut bytes.Buffer
	human := &cobra.Command{Use: "andurel"}
	human.SetOut(&humanOut)
	if err := runEmailRender(human, "verify_email", false, false); err != nil {
		t.Fatalf("runEmailRender human: %v", err)
	}
	if want := []string{"run", "./cmd/emails", "-out", "tmp/emails", "-only", "verify_email"}; !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("human args = %#v, want %#v", calls[0], want)
	}

	var structuredOut bytes.Buffer
	structured := newStructuredTestCommand(&structuredOut)
	if err := runEmailRender(structured, "", false, false); err != nil {
		t.Fatalf("runEmailRender structured: %v", err)
	}
	if want := []string{"run", "./cmd/emails", "-out", "tmp/emails", "-json"}; !reflect.DeepEqual(calls[1], want) {
		t.Fatalf("structured args = %#v, want %#v", calls[1], want)
	}

	var envelope struct {
		OK      bool              `json:"ok"`
		Data    emailRenderReport `json:"data"`
		Summary string            `json:"summary"`
	}
	if err := json.Unmarshal(structuredOut.Bytes(), &envelope); err != nil {
		t.Fatalf("decode structured output: %v\n%s", err, structuredOut.String())
	}
	if !envelope.OK || len(envelope.Data.Emails) != 1 {
		t.Fatalf("structured envelope = %#v", envelope)
	}
	if envelope.Data.Emails[0].Name != "verify_email" ||
		envelope.Data.Emails[0].HTML != "tmp/emails/verify_email.html" ||
		envelope.Data.Emails[0].Bytes != 42 {
		t.Fatalf("structured email = %#v", envelope.Data.Emails[0])
	}
	if !strings.Contains(envelope.Summary, "Rendered 1 email preview") {
		t.Fatalf("summary = %q", envelope.Summary)
	}
}

func TestEmailRenderMissingEntrypointExplainsHowToUpdate(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	err := runEmailRender(cmd, "", false, false)
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeMissingTool {
		t.Fatalf("missing entrypoint error = %T %[1]v", err)
	}
	if !strings.Contains(cliErr.Message, filepath.Join("cmd", "emails", "main.go")) {
		t.Fatalf("missing entrypoint message = %q", cliErr.Message)
	}
	if !strings.Contains(cliErr.Hint, "copy cmd/emails/main.go and email/previews.go") {
		t.Fatalf("missing entrypoint hint = %q", cliErr.Hint)
	}
}

func TestEmailRenderReportsRendererFailure(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "cmd/emails/main.go", "package main\n")

	originalFindGoModRoot := findGoModRoot
	originalRunner := runEmailRenderCommand
	findGoModRoot = func() (string, error) { return root, nil }
	runEmailRenderCommand = func(string, []string, io.Writer, io.Writer) error {
		return errors.New("exit status 1")
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runEmailRenderCommand = originalRunner
	})

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	err := runEmailRender(cmd, "", false, false)
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeExternalCommandFailed {
		t.Fatalf("renderer failure error = %T %[1]v", err)
	}
}

func TestEmailRenderOpensGalleryAfterRendering(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "cmd/emails/main.go", "package main\n")

	originalFindGoModRoot := findGoModRoot
	originalRunner := runEmailRenderCommand
	originalOpen := openEmailGallery
	findGoModRoot = func() (string, error) { return root, nil }
	runEmailRenderCommand = func(string, []string, io.Writer, io.Writer) error { return nil }
	var opened string
	openEmailGallery = func(indexPath string) error {
		opened = indexPath
		return nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runEmailRenderCommand = originalRunner
		openEmailGallery = originalOpen
	})

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "andurel"}
	cmd.SetOut(&out)
	if err := runEmailRender(cmd, "", true, false); err != nil {
		t.Fatalf("runEmailRender --open: %v", err)
	}
	want := filepath.Join(root, "tmp", "emails", "index.html")
	if opened != want {
		t.Fatalf("opened %q, want %q", opened, want)
	}
}

func TestEmailRenderWatchRejectsJSON(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "cmd/emails/main.go", "package main\n")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newStructuredTestCommand(&out)
	err := runEmailRender(cmd, "", false, true)
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != output.CodeOutputMode {
		t.Fatalf("watch --json error = %T %[1]v", err)
	}
	if !strings.Contains(cliErr.Message, "--watch") ||
		!strings.Contains(cliErr.Message, "--json") {
		t.Fatalf("watch --json message = %q", cliErr.Message)
	}
}

func TestEmailRenderWatchRendersWithWatchFlagAndOpensOnce(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root)
	writeTestFile(t, root, "cmd/emails/main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(root, "email"), 0o755); err != nil {
		t.Fatalf("mkdir email: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "emails"), 0o755); err != nil {
		t.Fatalf("mkdir emails: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	originalRunner := runEmailRenderCommand
	originalOpen := openEmailGallery
	findGoModRoot = func() (string, error) { return root, nil }
	var calls [][]string
	runEmailRenderCommand = func(
		_ string,
		args []string,
		stdout, _ io.Writer,
	) error {
		calls = append(calls, append([]string(nil), args...))
		_, _ = io.WriteString(
			stdout,
			`{"emails":[{"name":"verify_email","subject":"Verify your email","html":"tmp/emails/verify_email.html","text":"tmp/emails/verify_email.txt","bytes":42}]}`,
		)
		return nil
	}
	opened := 0
	openEmailGallery = func(string) error {
		opened++
		return nil
	}
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
		runEmailRenderCommand = originalRunner
		openEmailGallery = originalOpen
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{Use: "andurel"}
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := runEmailRender(cmd, "", true, true); err != nil {
		t.Fatalf("runEmailRender --watch: %v\nstderr: %s", err, errOut.String())
	}
	want := []string{"run", "./cmd/emails", "-out", "tmp/emails", "-json", "-watch"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("watch render calls = %#v, want one %#v", calls, want)
	}
	if opened != 1 {
		t.Fatalf("opened gallery %d times, want 1", opened)
	}
	if !strings.Contains(out.String(), "rendered 1 email in ") {
		t.Fatalf("watch status = %q", out.String())
	}
	if !strings.Contains(out.String(), "watching emails/") {
		t.Fatalf("watch status did not announce the watcher: %q", out.String())
	}
}
