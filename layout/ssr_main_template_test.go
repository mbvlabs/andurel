package layout

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

// TestGeneratedSSRMainPropagatesChildFailure compiles the real cmd/ssr
// template and drives it with Node. A wrapper that logs a child failure but
// exits 0 would pass a mocked callback test, so this test uses the generated
// executable and its actual exit status.
func TestGeneratedSSRMainPropagatesChildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiled generated cmd/ssr test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("andurel v2 does not support Windows")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	root := t.TempDir()
	data := &TemplateData{ModuleName: "testapp", ProjectName: "testapp"}
	if err := renderTemplate(
		root,
		"cmd_ssr_main.tmpl",
		"cmd/ssr/main.go",
		layouttemplates.Files,
		data,
	); err != nil {
		t.Fatalf("render cmd/ssr: %v", err)
	}

	writeLayoutTestFile(t, root, "assets/assets.go", `package assets

import "embed"

//go:embed *
var Files embed.FS
`)
	writeLayoutTestFile(t, root, "assets/keep.txt", "keep\n")
	writeLayoutTestFile(t, root, "config/config.go", `package config

import (
	"fmt"
	"os"
	"time"
)

type Inertia struct {
	SSRRuntime          string
	SSRBundle           string
	SSRListen           string
	SSRStartupTimeout   time.Duration
	SSRMinimumMajor     int
	SSRRequestTimeout   time.Duration
}

func NewInertia() (Inertia, error) {
	if os.Getenv("TEST_INVALID_CONFIG") != "" {
		return Inertia{}, fmt.Errorf("invalid inertia config")
	}
	return Inertia{
		SSRRuntime:        envOr("INERTIA_SSR_RUNTIME", "node"),
		SSRBundle:         envOr("INERTIA_SSR_BUNDLE", "assets/dist/ssr/ssr.js"),
		SSRListen:         envOr("INERTIA_SSR_LISTEN", "http://127.0.0.1:13714"),
		SSRStartupTimeout: 3 * time.Second,
		SSRMinimumMajor:   22,
		SSRRequestTimeout: time.Second,
	}, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
`)

	var module strings.Builder
	fmt.Fprintf(&module, "module testapp\n\ngo %s\n", goVersion)
	for _, name := range []string{
		"anduin", "env", "inertia", "lifecycle", "routing", "telemetry",
	} {
		path, err := filepath.Abs(filepath.Join("..", "pkg", name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(
			&module,
			"\nrequire github.com/mbvlabs/andurel/pkg/%s v0.0.0\n"+
				"replace github.com/mbvlabs/andurel/pkg/%s => %q\n",
			name, name, filepath.ToSlash(path),
		)
	}
	writeLayoutTestFile(t, root, "go.mod", module.String())

	binary := filepath.Join(root, "ssr")
	build := exec.Command("go", "build", "-mod=mod", "-o", binary, "./cmd/ssr")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build generated cmd/ssr: %v\n%s", err, output)
	}

	writeLayoutTestFile(t, root, "ssr-steady.js", `const http = require('http')
const host = process.env.INERTIA_SSR_HOST
const port = Number(process.env.INERTIA_SSR_PORT)
const server = http.createServer((req, res) => {
  if (req.url === '/health') {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({status: 'ok'}))
    return
  }
  if (req.url === '/shutdown') {
    res.statusCode = 200
    res.end('{}')
    setTimeout(() => process.exit(0), 10)
    return
  }
  res.statusCode = 200
  res.end('{}')
})
server.on('error', err => { console.error(err.message); process.exit(1) })
server.listen(port, host)
`)
	writeLayoutTestFile(t, root, "ssr-crash.js", `const http = require('http')
const host = process.env.INERTIA_SSR_HOST
const port = Number(process.env.INERTIA_SSR_PORT)
const server = http.createServer((req, res) => {
  if (req.url === '/health') {
    res.setHeader('content-type', 'application/json')
    res.end(JSON.stringify({status: 'ok'}))
    setTimeout(() => process.exit(1), 250)
    return
  }
  res.statusCode = 200
  res.end('{}')
})
server.on('error', err => { console.error(err.message); process.exit(1) })
server.listen(port, host)
`)
	writeLayoutTestFile(t, root, "ssr-bind.js", `const http = require('http')
const host = process.env.INERTIA_SSR_HOST
const port = Number(process.env.INERTIA_SSR_PORT)
const server = http.createServer()
server.on('error', err => { console.error(err.message); process.exit(1) })
server.listen(port, host)
`)

	runSSRMain := func(t *testing.T, bundle string, listen string) (string, string, error) {
		t.Helper()
		cmd := exec.Command(binary)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"INERTIA_SSR_RUNTIME="+node,
			"INERTIA_SSR_BUNDLE="+bundle,
			"INERTIA_SSR_LISTEN="+listen,
		)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	t.Run("occupied port exits nonzero", func(t *testing.T) {
		listener, err := netListenLoopback(t)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()

		_, stderr, err := runSSRMain(
			t,
			filepath.Join(root, "ssr-bind.js"),
			"http://"+listener.Addr().String(),
		)
		if err == nil {
			t.Fatalf("cmd/ssr exited 0 on an occupied port\nstderr:\n%s", stderr)
		}
		if !strings.Contains(strings.ToLower(stderr), "eaddrinuse") {
			t.Fatalf("stderr does not report EADDRINUSE:\n%s", stderr)
		}
	})

	t.Run("post-start child crash exits nonzero", func(t *testing.T) {
		stdout, stderr, err := runSSRMain(
			t,
			filepath.Join(root, "ssr-crash.js"),
			fmt.Sprintf("http://127.0.0.1:%d", freeLoopbackPort(t)),
		)
		if err == nil {
			t.Fatalf("cmd/ssr exited 0 after the child crashed\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if !strings.Contains(stderr, "inertia SSR runtime stopped unexpectedly") {
			t.Fatalf("stderr does not report the child crash:\n%s", stderr)
		}
	})

	t.Run("missing bundle exits nonzero", func(t *testing.T) {
		_, stderr, err := runSSRMain(
			t,
			filepath.Join(root, "missing-bundle.js"),
			fmt.Sprintf("http://127.0.0.1:%d", freeLoopbackPort(t)),
		)
		if err == nil {
			t.Fatalf("cmd/ssr exited 0 with a missing bundle\nstderr:\n%s", stderr)
		}
		if !strings.Contains(stderr, "inertia SSR bundle") {
			t.Fatalf("stderr does not report the missing bundle:\n%s", stderr)
		}
	})

	t.Run("invalid config exits nonzero", func(t *testing.T) {
		cmd := exec.Command(binary)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"TEST_INVALID_CONFIG=1",
			"INERTIA_SSR_RUNTIME="+node,
			"INERTIA_SSR_BUNDLE="+filepath.Join(root, "ssr-steady.js"),
			fmt.Sprintf("INERTIA_SSR_LISTEN=http://127.0.0.1:%d", freeLoopbackPort(t)),
		)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			t.Fatalf("cmd/ssr exited 0 with invalid config\nstderr:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "inertia config") {
			t.Fatalf("stderr does not report invalid config:\n%s", stderr.String())
		}
	})

	t.Run("signal shutdown exits cleanly", func(t *testing.T) {
		cmd := exec.Command(binary)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"INERTIA_SSR_RUNTIME="+node,
			"INERTIA_SSR_BUNDLE="+filepath.Join(root, "ssr-steady.js"),
			fmt.Sprintf("INERTIA_SSR_LISTEN=http://127.0.0.1:%d", freeLoopbackPort(t)),
		)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}

		ready := make(chan struct{})
		go func() {
			buffer := make([]byte, 4096)
			var seen strings.Builder
			for {
				n, readErr := stdoutPipe.Read(buffer)
				if n > 0 {
					seen.Write(buffer[:n])
					if strings.Contains(seen.String(), "Inertia SSR listening") {
						close(ready)
						return
					}
				}
				if readErr != nil {
					return
				}
			}
		}()

		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("cmd/ssr did not report readiness\nstderr:\n%s", stderr.String())
		}

		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("cmd/ssr did not exit cleanly on SIGINT: %v\nstderr:\n%s", err, stderr.String())
		}
	})
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func netListenLoopback(t *testing.T) (net.Listener, error) {
	t.Helper()
	return net.Listen("tcp", "127.0.0.1:0")
}

func writeLayoutTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
