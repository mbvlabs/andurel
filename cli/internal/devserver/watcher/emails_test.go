package watcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestIsEmailsSource(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "email struct", path: filepath.Join(root, "emails", "verify_email.go"), want: true},
		{name: "email render", path: filepath.Join(root, "emails", "render.go"), want: true},
		{
			name: "email theme file",
			path: filepath.Join(root, "emails", "theme.ts"),
			want: true,
		},
		{
			name: "nested email source",
			path: filepath.Join(root, "emails", "_components", "layout.tsx"),
			want: true,
		},
		{
			name: "generated props",
			path: filepath.Join(root, "emails", "_generated", "props.ts"),
			want: false,
		},
		{
			name: "generated manifest",
			path: filepath.Join(root, "emails", "_generated", "manifest.ts"),
			want: false,
		},
		{
			name: "email template output",
			path: filepath.Join(root, "emails", "templates", "verify_email.html.tmpl"),
			want: false,
		},
		{
			name: "email test file",
			path: filepath.Join(root, "emails", "verify_email_test.go"),
			want: false,
		},
		{name: "web go file", path: filepath.Join(root, "handlers", "pages.go"), want: false},
		{
			name: "outside project",
			path: filepath.Join(root, "..", "other", "emails", "theme.ts"),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEmailsSource(root, test.path); got != test.want {
				t.Fatalf("IsEmailsSource(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestRunEmailsWatcherSyncsChangedSource(t *testing.T) {
	root := t.TempDir()
	emailDir := filepath.Join(root, "emails")
	emailsDir := filepath.Join(root, "emails", "_components")
	for _, dir := range []string{emailDir, emailsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	componentPath := filepath.Join(emailsDir, "layout.tsx")
	if err := os.WriteFile(componentPath, []byte("export default function Layout() {}\n"), 0o644); err != nil {
		t.Fatalf("write component: %v", err)
	}

	var synced atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEmailsWatcher(ctx, EmailsWatcherConfig{
			Dir: root,
			Sync: func(context.Context) error {
				return nil
			},
			OnSynced:    func() { synced.Add(1) },
			OnSyncError: func(string) {},
			Log:         io.Discard,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Give the watcher time to register its directories.
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(componentPath, []byte("export default function Layout() { return null }\n"), 0o644); err != nil {
		t.Fatalf("update component: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if synced.Load() > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("emails watcher did not sync the changed source before timeout")
}

func TestIsEmailsTemplateOutput(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "html template",
			path: filepath.Join(root, "emails", "templates", "verify_email.html.tmpl"),
			want: true,
		},
		{
			name: "text template",
			path: filepath.Join(root, "emails", "templates", "reset_password.txt.tmpl"),
			want: true,
		},
		{
			name: "source struct",
			path: filepath.Join(root, "emails", "verify_email.go"),
			want: false,
		},
		{
			name: "email source",
			path: filepath.Join(root, "emails", "verify-email.tsx"),
			want: false,
		},
		{
			name: "outside project",
			path: filepath.Join(root, "..", "other", "emails", "templates", "x.html.tmpl"),
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEmailsTemplateOutput(root, test.path); got != test.want {
				t.Fatalf("IsEmailsTemplateOutput(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestRunEmailsWatcherRebuildsChangedTemplates(t *testing.T) {
	root := t.TempDir()
	templatesDir := filepath.Join(root, "emails", "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	templatePath := filepath.Join(templatesDir, "verify_email.html.tmpl")
	if err := os.WriteFile(templatePath, []byte("<p>old</p>\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var synced, rebuilt atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEmailsWatcher(ctx, EmailsWatcherConfig{
			Dir: root,
			Sync: func(context.Context) error {
				synced.Add(1)
				return nil
			},
			OnSynced:           func() {},
			OnTemplatesChanged: func() { rebuilt.Add(1) },
			OnSyncError:        func(string) {},
			Log:                io.Discard,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Give the watcher time to register its directories.
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(templatePath, []byte("<p>new</p>\n"), 0o644); err != nil {
		t.Fatalf("update template: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rebuilt.Load() > 0 {
			if synced.Load() != 0 {
				t.Fatal("template change must rebuild without re-running the export sync")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("emails watcher did not rebuild for the changed template before timeout")
}

func TestRunEmailsWatcherIgnoresTemplatesWrittenBySync(t *testing.T) {
	root := t.TempDir()
	emailDir := filepath.Join(root, "emails")
	emailsDir := filepath.Join(root, "emails")
	templatesDir := filepath.Join(emailDir, "templates")
	for _, dir := range []string{emailDir, emailsDir, templatesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	sourcePath := filepath.Join(emailDir, "verify_email.go")
	if err := os.WriteFile(sourcePath, []byte("package emails\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	templatePath := filepath.Join(templatesDir, "verify_email.html.tmpl")
	if err := os.WriteFile(templatePath, []byte("<p>old</p>\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var synced, rebuilt atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEmailsWatcher(ctx, EmailsWatcherConfig{
			Dir: root,
			Sync: func(context.Context) error {
				// The real export writes emails/templates; those events must not
				// schedule a second rebuild after OnSynced already did.
				if err := os.WriteFile(templatePath, []byte("<p>exported</p>\n"), 0o644); err != nil {
					return err
				}
				return nil
			},
			OnSynced:           func() { synced.Add(1) },
			OnTemplatesChanged: func() { rebuilt.Add(1) },
			OnSyncError:        func(string) {},
			Log:                io.Discard,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(sourcePath, []byte("package emails\n\n// changed\n"), 0o644); err != nil {
		t.Fatalf("update source: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if synced.Load() > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if synced.Load() == 0 {
		t.Fatal("emails watcher did not sync the changed source before timeout")
	}
	time.Sleep(500 * time.Millisecond)
	if rebuilt.Load() != 0 {
		t.Fatalf("template write from sync triggered %d extra rebuild(s)", rebuilt.Load())
	}
}

func TestRunEmailsWatcherReportsSyncErrors(t *testing.T) {
	root := t.TempDir()
	emailDir := filepath.Join(root, "emails")
	if err := os.MkdirAll(emailDir, 0o755); err != nil {
		t.Fatalf("mkdir emails: %v", err)
	}
	structPath := filepath.Join(emailDir, "verify_email.go")
	if err := os.WriteFile(structPath, []byte("package emails\n"), 0o644); err != nil {
		t.Fatalf("write struct: %v", err)
	}

	var reported atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEmailsWatcher(ctx, EmailsWatcherConfig{
			Dir: root,
			Sync: func(context.Context) error {
				return io.ErrUnexpectedEOF
			},
			OnSynced: func() {},
			OnSyncError: func(msg string) {
				if strings.Contains(msg, io.ErrUnexpectedEOF.Error()) {
					reported.Add(1)
				}
			},
			Log: io.Discard,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(structPath, []byte("package emails\n\n// changed\n"), 0o644); err != nil {
		t.Fatalf("update struct: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if reported.Load() > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("emails watcher did not report the sync error before timeout")
}

func TestRunEmailsWatcherStopsWithContext(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "emails"), 0o755); err != nil {
		t.Fatalf("mkdir emails: %v", err)
	}
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	t.Cleanup(func() { _ = fsWatcher.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runEmailsWatcher(ctx, fsWatcher, EmailsWatcherConfig{Dir: root, Log: io.Discard})
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runEmailsWatcher did not stop after context cancellation")
	}
}

func TestRunEmailsWatcherToleratesMissingDirectories(t *testing.T) {
	if err := RunEmailsWatcher(
		context.Background(),
		EmailsWatcherConfig{Dir: t.TempDir(), Log: io.Discard},
	); err != nil {
		t.Fatalf("RunEmailsWatcher without email directories = %v, want nil", err)
	}
}
