package watcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunTemplWatcherFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tmp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunTemplWatcher(t.Context(), nil, TemplWatcherConfig{Dir: dir}); err == nil {
		t.Fatal("expected directory creation error")
	}
}

func TestRunTemplWatcherCancelUsesKillFallback(t *testing.T) {
	tmp := t.TempDir()
	createTemplScript(t, tmp, `#!/usr/bin/env sh
trap '' INT
while true; do
  sleep 1
done
`)

	oldTimeout := templShutdownTimeout
	templShutdownTimeout = 100 * time.Millisecond
	t.Cleanup(func() { templShutdownTimeout = oldTimeout })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		errCh <- RunTemplWatcher(ctx, make(chan TemplChange, 1), TemplWatcherConfig{Dir: tmp, Log: io.Discard})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
		if time.Since(start) > time.Second {
			t.Fatalf("shutdown took too long: %s", time.Since(start))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunTemplWatcher did not return after cancel")
	}
}

func TestRunTemplWatcherReturnsProcessError(t *testing.T) {
	tmp := t.TempDir()
	createTemplScript(t, tmp, `#!/usr/bin/env sh
echo "templ failed" 1>&2
exit 7
`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := RunTemplWatcher(ctx, make(chan TemplChange, 1), TemplWatcherConfig{Dir: tmp, Log: io.Discard})
	if err == nil {
		t.Fatal("expected process error, got nil")
	}
}

// TestTemplArgsLimitWatchToResources guards F1: plain templ must only
// regenerate resources/; React Email sources under emails/ are not templ.
func TestTemplArgsLimitWatchToResources(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "home", "dev", "my.app")
	args := TemplArgs(dir)
	if !slices.Contains(args, "--watch") {
		t.Fatalf("templ args = %q, want --watch", args)
	}
	// -path stays the project root so generated file names match
	// `andurel build` (resources/root.templ, not root.templ).
	index := slices.Index(args, "-path")
	if index < 0 || index+1 >= len(args) || args[index+1] != dir {
		t.Fatalf("templ args = %q, want -path %s", args, dir)
	}
	index = slices.Index(args, "--watch-pattern")
	if index < 0 || index+1 >= len(args) {
		t.Fatalf("templ args = %q, want --watch-pattern", args)
	}
	pattern := regexp.MustCompile(args[index+1])
	cases := map[string]bool{
		filepath.Join(dir, "resources", "root.templ"):             true,
		filepath.Join(dir, "resources", "nested", "page.templ"):   true,
		filepath.Join(dir, "emails", "verify_email.templ"):        false,
		filepath.Join(dir, "resources", "root_templ.go"):          false,
		filepath.Join(dir+"x", "resources", "root.templ"):         false,
		filepath.Join("/other", "resources", "my.app", "a.templ"): false,
	}
	for path, want := range cases {
		if got := pattern.MatchString(path); got != want {
			t.Errorf("pattern %q match %q = %v, want %v", pattern, path, got, want)
		}
	}
}

func TestRunTemplWatcherPassesPatternToTempl(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "templ.args")
	createTemplScript(t, tmp, "#!/usr/bin/env sh\nfor a in \"$@\"; do echo \"$a\"; done > \""+argsFile+"\"\nexit 0\n")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := RunTemplWatcher(ctx, make(chan TemplChange, 1), TemplWatcherConfig{Dir: tmp, Log: io.Discard}); err != nil {
		t.Fatalf("RunTemplWatcher: %v", err)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if want := TemplWatchPattern(tmp) + "\n"; !strings.Contains(string(got), want) {
		t.Fatalf("templ invoked with %q, want pattern %q", got, want)
	}
}

// TestTemplWatchSkipsEmailWithRealTempl runs the pinned templ binary when it
// is available (ANDUREL_TEST_TEMPL) and checks that a watch session generates
// resources/ but leaves emails/ alone.
func TestTemplWatchSkipsEmailWithRealTempl(t *testing.T) {
	templBin := os.Getenv("ANDUREL_TEST_TEMPL")
	if templBin == "" {
		t.Skip("set ANDUREL_TEST_TEMPL to a templ binary to run")
	}
	dir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("go.mod", "module example.com/app\n\ngo 1.24\n")
	writeFile("resources/root.templ", "package resources\n\ntempl Root() {\n\t<p>hi</p>\n}\n")
	writeFile("emails/mail.templ", "package emails\n\ntempl Mail() {\n\t<p class=\"p-4\">hi</p>\n}\n")
	writeFile("emails/mail_templ.go", "package emails\n// untouched by templ\n")
	binPath := filepath.Join(dir, "bin", "templ")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(templBin, binPath); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunTemplWatcher(ctx, make(chan TemplChange, 8), TemplWatcherConfig{Dir: dir, Log: io.Discard})
	}()
	generated := filepath.Join(dir, "resources", "root_templ.go")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(generated); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done

	rootGo, err := os.ReadFile(generated)
	if err != nil {
		t.Fatalf("resources/root_templ.go was not generated: %v", err)
	}
	if strings.Contains(string(rootGo), "FileName: `root.templ`") {
		t.Fatal("generated file names must be relative to the project root")
	}
	mailGo, err := os.ReadFile(filepath.Join(dir, "emails", "mail_templ.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(mailGo) != "package emails\n// untouched by templ\n" {
		t.Fatalf("templ watch rewrote emails/mail_templ.go:\n%s", mailGo)
	}
}

func createTemplScript(t *testing.T, root, content string) {
	t.Helper()

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(binDir, "templ")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
