package cli

import (
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyEmailWatchEvent(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want emailWatchEvent
	}{
		{
			name: "authored tsx",
			path: filepath.Join(root, "emails", "verify-email.tsx"),
			want: emailWatchEventSync,
		},
		{
			name: "nested component",
			path: filepath.Join(root, "emails", "_components", "layout.tsx"),
			want: emailWatchEventSync,
		},
		{
			name: "email go struct",
			path: filepath.Join(root, "email", "verify_email.go"),
			want: emailWatchEventSync,
		},
		{
			name: "generated props",
			path: filepath.Join(root, "emails", "_generated", "props.ts"),
			want: emailWatchEventNone,
		},
		{
			name: "generated manifest",
			path: filepath.Join(root, "emails", "_generated", "manifest.ts"),
			want: emailWatchEventNone,
		},
		{
			name: "committed html template",
			path: filepath.Join(root, "email", "templates", "verify_email.html.tmpl"),
			want: emailWatchEventRender,
		},
		{
			name: "committed text template",
			path: filepath.Join(root, "email", "templates", "verify_email.txt.tmpl"),
			want: emailWatchEventRender,
		},
		{
			name: "email test file",
			path: filepath.Join(root, "email", "verify_email_test.go"),
			want: emailWatchEventNone,
		},
		{
			name: "nested email go file",
			path: filepath.Join(root, "email", "internal", "verify_email.go"),
			want: emailWatchEventNone,
		},
		{
			name: "render output",
			path: filepath.Join(root, "tmp", "emails", "index.html"),
			want: emailWatchEventNone,
		},
		{
			name: "outside project",
			path: filepath.Join(root, "..", "other", "emails", "theme.ts"),
			want: emailWatchEventNone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyEmailWatchEvent(root, test.path); got != test.want {
				t.Fatalf(
					"classifyEmailWatchEvent(%q) = %v, want %v",
					test.path,
					got,
					test.want,
				)
			}
		})
	}
}

func TestEmailWatchCycleCoalescesEvents(t *testing.T) {
	now := time.Unix(100, 0)
	cycle := newEmailWatchCycle(emailRenderWatchQuiet)

	if !cycle.observe(emailWatchEventSync, now) {
		t.Fatal("sync event should arm the debounce")
	}
	if !cycle.observe(emailWatchEventSync, now.Add(time.Millisecond)) {
		t.Fatal("second sync event should keep the debounce armed")
	}
	if !cycle.observe(emailWatchEventRender, now.Add(2*time.Millisecond)) {
		t.Fatal("render event should keep the debounce armed")
	}
	if got := cycle.take(); got != emailWatchEventSync {
		t.Fatalf("take() = %v, want sync", got)
	}
	if got := cycle.take(); got != emailWatchEventNone {
		t.Fatalf("second take() = %v, want none", got)
	}
}

func TestEmailWatchCycleSuppressesOwnExportOutput(t *testing.T) {
	now := time.Unix(100, 0)
	cycle := newEmailWatchCycle(emailRenderWatchQuiet)

	if !cycle.observe(emailWatchEventSync, now) {
		t.Fatal("sync event should arm the debounce")
	}
	if got := cycle.take(); got != emailWatchEventSync {
		t.Fatalf("take() = %v, want sync", got)
	}

	// The export writes email/templates/*.tmpl; those events must not start a
	// second sync or a second render.
	cycle.finishSync(now)
	if cycle.observe(emailWatchEventRender, now.Add(time.Second)) {
		t.Fatal("template event during the quiet window should be ignored")
	}
	if got := cycle.take(); got != emailWatchEventNone {
		t.Fatalf("take() after quiet template event = %v, want none", got)
	}

	// A real source edit still syncs during the quiet window.
	if !cycle.observe(emailWatchEventSync, now.Add(time.Second)) {
		t.Fatal("source edit should not be suppressed")
	}
	if got := cycle.take(); got != emailWatchEventSync {
		t.Fatalf("take() after source edit = %v, want sync", got)
	}
	cycle.finishSync(now.Add(time.Second))

	// Once the quiet window passes, an edited template renders again.
	if !cycle.observe(emailWatchEventRender, now.Add(3*time.Second)) {
		t.Fatal("template event after the quiet window should render")
	}
	if got := cycle.take(); got != emailWatchEventRender {
		t.Fatalf("take() after quiet window = %v, want render", got)
	}
}

func TestEmailWatchCycleIgnoresUnrelatedEvents(t *testing.T) {
	cycle := newEmailWatchCycle(emailRenderWatchQuiet)
	now := time.Unix(100, 0)
	if cycle.observe(emailWatchEventNone, now) {
		t.Fatal("unrelated event should not arm the debounce")
	}
	if got := cycle.take(); got != emailWatchEventNone {
		t.Fatalf("take() = %v, want none", got)
	}
}
