package publishedpins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func TestPinsListsVerifiedModules(t *testing.T) {
	root := writeTestRoot(t, "inertia", "anduin")
	checker := New(Options{Root: root})

	pins, err := checker.Pins()
	if err != nil {
		t.Fatalf("Pins: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("pins = %d, want 2", len(pins))
	}
	if pins[0].Module != versions.PkgPrefix+"anduin" {
		t.Fatalf("first pin = %s, want anduin", pins[0].Module)
	}
	if pins[1].Module != versions.PkgPrefix+"inertia" {
		t.Fatalf("second pin = %s, want inertia", pins[1].Module)
	}
	for _, pin := range pins {
		want, ok := versions.PackageVersion(pin.Module)
		if !ok {
			t.Fatalf("%s is not in layout/versions", pin.Module)
		}
		if pin.Version != want {
			t.Fatalf("%s version = %s, want %s", pin.Module, pin.Version, want)
		}
	}
}

func TestPinsRejectsUnverifiedModule(t *testing.T) {
	root := writeTestRoot(t, "anduin", "not-a-package")
	checker := New(Options{Root: root})

	_, err := checker.Pins()
	if err == nil {
		t.Fatal("Pins succeeded, want error")
	}
	if !strings.Contains(err.Error(), "pkg/not-a-package") {
		t.Fatalf("error = %v, want the unverified module name", err)
	}
}

func TestPinsRequiresModules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module testroot\n\ngo 1.27.3\n"),
		0o644,
	); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	checker := New(Options{Root: root})

	if _, err := checker.Pins(); err == nil {
		t.Fatal("Pins succeeded without pkg modules, want error")
	}
}

func TestCheckVerifiesPublishedPins(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin", "inertia")
	runner := &fakeRunner{}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	t.Setenv("GOPRIVATE", "github.com/mbvlabs/*")
	t.Setenv("GOWORK", "/tmp/workspace")
	t.Setenv("GOPROXY", "file:///tmp/stale-proxy")

	results, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for _, result := range results {
		if !result.SumDB || !result.Proxy || !result.Resolved || !result.Checksum {
			t.Fatalf("result for %s is incomplete: %+v", result.Pin, result)
		}
	}

	if len(registry.requests) != 4 {
		t.Fatalf("registry requests = %d, want 2 sumdb + 2 proxy", len(registry.requests))
	}

	downloads, verifies := runner.summary()
	if downloads != 2 {
		t.Fatalf("downloads = %d, want 2", downloads)
	}
	if verifies != 1 {
		t.Fatalf("verifies = %d, want 1", verifies)
	}
	if !runner.ran("list", "-m", "all") {
		t.Fatal("checker did not resolve the module graph")
	}

	call := runner.firstCall(t)
	assertEnv(t, call.env, "GOWORK", "off")
	assertEnv(t, call.env, "GOENV", "off")
	assertEnv(t, call.env, "GOPROXY", server.URL+",direct")
	assertEnv(t, call.env, "GOSUMDB", "sum.golang.org")
	assertNoEnv(t, call.env, "GOPRIVATE")
	assertOwnedEnvDir(t, call.env, "GOMODCACHE", call.dir)
	assertOwnedEnvDir(t, call.env, "GOCACHE", call.dir)
	assertOwnedEnvDir(t, call.env, "GOTMPDIR", call.dir)

	if strings.Contains(call.goMod, "replace ") {
		t.Fatalf("pin resolution go.mod has a replace directive:\n%s", call.goMod)
	}
	for _, pin := range []Pin{
		{Module: versions.PkgPrefix + "anduin", Version: mustVersion(t, "anduin")},
		{Module: versions.PkgPrefix + "inertia", Version: mustVersion(t, "inertia")},
	} {
		if !strings.Contains(call.goMod, pin.Module+" "+pin.Version) {
			t.Fatalf("pin resolution go.mod is missing %s:\n%s", pin, call.goMod)
		}
	}

	if _, err := os.Stat(call.dir); !os.IsNotExist(err) {
		t.Fatalf("pin resolution dir still exists after Check: %v", err)
	}
}

func TestCheckReportsSumDBFailure(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	pin := Pin{Module: versions.PkgPrefix + "inertia", Version: mustVersion(t, "inertia")}
	registry.setSumDB(pin, http.StatusNotFound)

	root := writeTestRoot(t, "anduin", "inertia")
	runner := &fakeRunner{}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded, want sumdb error")
	}
	if !strings.Contains(err.Error(), "sum.golang.org lookup") ||
		!strings.Contains(err.Error(), pin.String()) {
		t.Fatalf("error = %v, want a sumdb failure for %s", err, pin)
	}
	result := resultFor(t, results, pin)
	if result.SumDB || result.Resolved || result.Checksum {
		t.Fatalf("result for unpublished pin = %+v, want no sumdb proof", result)
	}
	if runner.downloaded(pin) {
		t.Fatalf("resolved %s even though its checksum lookup failed", pin)
	}
}

func TestCheckReportsProxyFailure(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	pin := Pin{Module: versions.PkgPrefix + "anduin", Version: mustVersion(t, "anduin")}
	registry.setProxy(pin, http.StatusGone)

	root := writeTestRoot(t, "anduin")
	runner := &fakeRunner{}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded, want proxy error")
	}
	if !strings.Contains(err.Error(), "proxy.golang.org") ||
		!strings.Contains(err.Error(), pin.String()) {
		t.Fatalf("error = %v, want a proxy failure for %s", err, pin)
	}
	result := resultFor(t, results, pin)
	if result.Proxy || result.Resolved || result.Checksum {
		t.Fatalf("result = %+v, want no proxy proof", result)
	}
}

func TestCheckRequiresBothChecksumLines(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin")
	pin := Pin{Module: versions.PkgPrefix + "anduin", Version: mustVersion(t, "anduin")}
	runner := &fakeRunner{skipSum: map[Pin]bool{pin: true}}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded without go.sum entries, want error")
	}
	if !strings.Contains(err.Error(), "go.sum is missing checksums") {
		t.Fatalf("error = %v, want a missing-checksum failure", err)
	}
	if result := resultFor(t, results, pin); result.Checksum {
		t.Fatalf("result = %+v, want no checksum proof", result)
	}
}

func TestCheckReportsDownloadFailure(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin")
	pin := Pin{Module: versions.PkgPrefix + "anduin", Version: mustVersion(t, "anduin")}
	runner := &fakeRunner{failDownload: map[Pin]error{pin: errors.New("module not found")}}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded, want download error")
	}
	if !strings.Contains(err.Error(), "resolve "+pin.String()) {
		t.Fatalf("error = %v, want a resolve failure for %s", err, pin)
	}
	if result := resultFor(t, results, pin); result.Resolved || result.Checksum {
		t.Fatalf("result = %+v, want no resolution proof", result)
	}
}

func TestCheckReportsGraphFailure(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin")
	runner := &fakeRunner{failGraph: true}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded, want module graph error")
	}
	if !strings.Contains(err.Error(), "resolve module graph") {
		t.Fatalf("error = %v, want a module graph failure", err)
	}
	for _, result := range results {
		if result.Resolved || result.Checksum {
			t.Fatalf("result = %+v, want no resolution proof after graph failure", result)
		}
	}
}

func TestCheckReportsVerifyFailure(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin")
	runner := &fakeRunner{failVerify: true}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	results, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check succeeded, want verify error")
	}
	if !strings.Contains(err.Error(), "verify module cache") {
		t.Fatalf("error = %v, want a verify failure", err)
	}
	for _, result := range results {
		if result.Resolved || result.Checksum {
			t.Fatalf("result = %+v, want no checksum proof after verify failure", result)
		}
	}
}

func TestCheckUsesRootGoDirective(t *testing.T) {
	registry := newFakeRegistry()
	server := httptest.NewServer(registry)
	defer server.Close()

	root := writeTestRoot(t, "anduin")
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module testroot\n\ngo 1.27.3\n"),
		0o644,
	); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	runner := &fakeRunner{}
	checker := New(Options{
		Root:     root,
		Client:   server.Client(),
		SumDBURL: server.URL,
		ProxyURL: server.URL,
		Runner:   runner,
	})

	if _, err := checker.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if call := runner.firstCall(t); !strings.Contains(call.goMod, "go 1.27.3") {
		t.Fatalf("pin resolution go.mod does not use the root go directive:\n%s", call.goMod)
	}
}

func TestEscapeModulePath(t *testing.T) {
	tests := map[string]string{
		"github.com/mbvlabs/andurel/pkg/anduin": "github.com/mbvlabs/andurel/pkg/anduin",
		"github.com/Example/Mod":                "github.com/!example/!mod",
	}
	for input, want := range tests {
		if got := escapeModulePath(input); got != want {
			t.Fatalf("escapeModulePath(%q) = %q, want %q", input, got, want)
		}
	}
}

type registryRequest struct {
	path   string
	status int
}

type fakeRegistry struct {
	mu       sync.Mutex
	sumDB    map[Pin]int
	proxy    map[Pin]int
	requests []registryRequest
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		sumDB: make(map[Pin]int),
		proxy: make(map[Pin]int),
	}
}

func (registry *fakeRegistry) setSumDB(pin Pin, status int) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.sumDB[pin] = status
}

func (registry *fakeRegistry) setProxy(pin Pin, status int) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.proxy[pin] = status
}

func (registry *fakeRegistry) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	status := http.StatusOK
	path := request.URL.Path
	switch {
	case strings.HasPrefix(path, "/lookup/"):
		key := strings.TrimPrefix(path, "/lookup/")
		pin, ok := pinFromLookup(key)
		if !ok {
			status = http.StatusBadRequest
			break
		}
		if want, ok := registry.sumDB[pin]; ok {
			status = want
		}
	case strings.Contains(path, "/@v/"):
		key := strings.TrimPrefix(path, "/")
		key = strings.TrimSuffix(key, ".info")
		module, version, ok := strings.Cut(key, "/@v/")
		if !ok {
			status = http.StatusBadRequest
			break
		}
		pin := Pin{Module: module, Version: version}
		if want, ok := registry.proxy[pin]; ok {
			status = want
		}
	default:
		status = http.StatusNotFound
	}

	registry.requests = append(registry.requests, registryRequest{path: path, status: status})
	writer.WriteHeader(status)
}

func pinFromLookup(key string) (Pin, bool) {
	module, version, ok := strings.Cut(key, "@")
	if !ok {
		return Pin{}, false
	}
	return Pin{Module: module, Version: version}, true
}

type runnerCall struct {
	dir   string
	env   []string
	name  string
	args  []string
	goMod string
}

type fakeRunner struct {
	mu           sync.Mutex
	calls        []runnerCall
	skipSum      map[Pin]bool
	failDownload map[Pin]error
	failVerify   bool
	failGraph    bool
}

func (runner *fakeRunner) Run(
	_ context.Context,
	dir string,
	env []string,
	name string,
	args ...string,
) ([]byte, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	call := runnerCall{dir: dir, env: append([]string(nil), env...), name: name, args: append([]string(nil), args...)}
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		call.goMod = string(data)
	}
	runner.calls = append(runner.calls, call)

	if len(args) == 3 && args[0] == "list" && args[1] == "-m" && args[2] == "all" {
		if runner.failGraph {
			return []byte("module graph failed"), errors.New("module graph failed")
		}
		return nil, nil
	}

	if len(args) < 2 || args[0] != "mod" {
		return nil, fmt.Errorf("unexpected command %s %v", name, args)
	}
	switch args[1] {
	case "download":
		if len(args) != 3 {
			return nil, fmt.Errorf("download args = %v, want module@version", args)
		}
		pin, ok := pinFromLookup(args[2])
		if !ok {
			return nil, fmt.Errorf("download argument %q is not module@version", args[2])
		}
		if err := runner.failDownload[pin]; err != nil {
			return []byte("download failed"), err
		}
		if runner.skipSum[pin] {
			if err := appendPartialFakeSum(dir, pin); err != nil {
				return nil, err
			}
			return nil, nil
		}
		if err := appendFakeSum(dir, pin); err != nil {
			return nil, err
		}
		return nil, nil
	case "verify":
		if runner.failVerify {
			return []byte("checksum mismatch"), errors.New("checksum mismatch")
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected mod subcommand %q", args[1])
	}
}

func (runner *fakeRunner) summary() (downloads, verifies int) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for _, call := range runner.calls {
		if len(call.args) < 2 {
			continue
		}
		switch call.args[1] {
		case "download":
			downloads++
		case "verify":
			verifies++
		}
	}
	return downloads, verifies
}

func (runner *fakeRunner) downloaded(pin Pin) bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for _, call := range runner.calls {
		if len(call.args) == 3 && call.args[1] == "download" && call.args[2] == pin.String() {
			return true
		}
	}
	return false
}

func (runner *fakeRunner) firstCall(t *testing.T) runnerCall {
	t.Helper()
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.calls) == 0 {
		t.Fatal("runner was not called")
	}
	return runner.calls[0]
}

func (runner *fakeRunner) ran(args ...string) bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for _, call := range runner.calls {
		if len(call.args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if call.args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func appendFakeSum(dir string, pin Pin) error {
	file, err := os.OpenFile(
		filepath.Join(dir, "go.sum"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = fmt.Fprintf(
		file,
		"%s %s h1:fake\n%s %s/go.mod h1:fake\n",
		pin.Module,
		pin.Version,
		pin.Module,
		pin.Version,
	)
	return err
}

// appendPartialFakeSum writes only the module checksum line, so the go.sum file
// exists but is missing the /go.mod line the checker requires.
func appendPartialFakeSum(dir string, pin Pin) error {
	file, err := os.OpenFile(
		filepath.Join(dir, "go.sum"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = fmt.Fprintf(file, "%s %s h1:fake\n", pin.Module, pin.Version)
	return err
}

func writeTestRoot(t *testing.T, modules ...string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module testroot\n\ngo 1.27.3\n"),
		0o644,
	); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	for _, module := range modules {
		dir := filepath.Join(root, "pkg", module)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create pkg/%s: %v", module, err)
		}
		content := "module " + versions.PkgPrefix + module + "\n\ngo 1.27.3\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
			t.Fatalf("write pkg/%s/go.mod: %v", module, err)
		}
	}
	return root
}

func mustVersion(t *testing.T, module string) string {
	t.Helper()
	version, ok := versions.PackageVersion(versions.PkgPrefix + module)
	if !ok {
		t.Fatalf("layout/versions has no pin for %s", module)
	}
	return version
}

func resultFor(t *testing.T, results []Result, pin Pin) Result {
	t.Helper()
	for _, result := range results {
		if result.Pin == pin {
			return result
		}
	}
	t.Fatalf("no result for %s", pin)
	return Result{}
}

func assertEnv(t *testing.T, env []string, key, want string) {
	t.Helper()
	values := envValues(env, key)
	if len(values) != 1 {
		t.Fatalf("%s has %d values, want 1: %v", key, len(values), values)
	}
	if values[0] != want {
		t.Fatalf("%s = %q, want %q", key, values[0], want)
	}
}

func assertNoEnv(t *testing.T, env []string, key string) {
	t.Helper()
	if values := envValues(env, key); len(values) != 0 {
		t.Fatalf("%s is set, want absent: %v", key, values)
	}
}

func assertOwnedEnvDir(t *testing.T, env []string, key, callDir string) {
	t.Helper()
	values := envValues(env, key)
	if len(values) != 1 {
		t.Fatalf("%s has %d values, want 1: %v", key, len(values), values)
	}
	want := ""
	switch key {
	case "GOMODCACHE":
		want = filepath.Join(callDir, "modcache")
	case "GOCACHE":
		want = filepath.Join(callDir, "gocache")
	case "GOTMPDIR":
		want = filepath.Join(callDir, "tmp")
	default:
		t.Fatalf("unexpected cache key %s", key)
	}
	if values[0] != want {
		t.Fatalf("%s = %q, want %q", key, values[0], want)
	}
}

func envValues(env []string, key string) []string {
	var values []string
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			values = append(values, value)
		}
	}
	return values
}
