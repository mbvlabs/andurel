package devserver

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/reload"
)

func TestRunProxyServerFailsFastWhenPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	})

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err = runProxyServer(t.Context(), t.TempDir(), port, "8080", reload.NewBroadcaster(), nil)
	if err == nil {
		t.Fatal("expected bind error when proxy port is already in use")
	}

	if time.Since(start) > time.Second {
		t.Fatalf("expected startup failure to return quickly, took %s", time.Since(start))
	}
}

func TestOptionsNormalize(t *testing.T) {
	if err := (&Options{}).normalize(); err == nil {
		t.Fatal("expected an error without a project directory")
	}

	opts := Options{Dir: t.TempDir()}
	if err := opts.normalize(); err != nil {
		t.Fatal(err)
	}
	if opts.PackageManager != DefaultPackageManager {
		t.Fatalf("PackageManager = %q, want %q", opts.PackageManager, DefaultPackageManager)
	}
	if opts.Stdin == nil || opts.Stdout == nil || opts.Stderr == nil {
		t.Fatal("expected standard streams to default")
	}

	bad := Options{Dir: t.TempDir(), Tools: []string{"dblab"}}
	if err := bad.normalize(); err == nil || !strings.Contains(err.Error(), "dblab") {
		t.Fatalf("normalize() = %v, want unknown tool error", err)
	}
}

func TestRunRequiresQueueEntrypoint(t *testing.T) {
	err := Run(t.Context(), Options{Dir: t.TempDir(), Inline: true, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "cmd/queue/main.go") {
		t.Fatalf("Run() = %v, want missing queue entrypoint error", err)
	}
}
