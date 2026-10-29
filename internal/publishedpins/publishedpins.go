// Package publishedpins proves that every standalone pkg/* module pin recorded
// in layout/versions is published and resolvable from the public Go module
// proxy.
//
// It is the release gate that a local module proxy cannot satisfy: the checksum
// database, the public proxy, and go's own module cache are consulted without
// workspace, replace directives, or local package source.
package publishedpins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/v2/layout/versions"
)

const (
	defaultSumDBURL  = "https://sum.golang.org"
	defaultProxyURL  = "https://proxy.golang.org"
	defaultGoSumDB   = "sum.golang.org"
	defaultGoProxy   = "https://proxy.golang.org,direct"
	requestBodyLimit = 1 << 16
)

// Pin is one standalone Andurel module and the version the framework verifies.
type Pin struct {
	Module  string
	Version string
}

// String returns module@version.
func (pin Pin) String() string {
	return pin.Module + "@" + pin.Version
}

// Result records which publication proofs passed for one pin.
type Result struct {
	Pin      Pin
	SumDB    bool
	Proxy    bool
	Resolved bool
	Checksum bool
}

// Runner executes a command with an explicit environment. Tests inject a fake.
type Runner interface {
	Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error)
}

// ExecRunner runs commands with os/exec.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(
	ctx context.Context,
	dir string,
	env []string,
	name string,
	args ...string,
) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = env
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf(
			"%s %s: %w\n%s",
			name,
			strings.Join(args, " "),
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return output, nil
}

// Options configures a Checker. Zero values use the public endpoints and the go
// binary on PATH.
type Options struct {
	Root     string
	Client   *http.Client
	SumDBURL string
	ProxyURL string
	GoSumDB  string
	Runner   Runner
	GoBinary string
}

// Checker verifies the verified package pins under one repository root.
type Checker struct {
	root     string
	client   *http.Client
	sumDBURL string
	proxyURL string
	goSumDB  string
	runner   Runner
	goBinary string
}

// New returns a Checker for opts.
func New(opts Options) *Checker {
	root := opts.Root
	if root == "" {
		root = "."
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	sumDBURL := opts.SumDBURL
	if sumDBURL == "" {
		sumDBURL = defaultSumDBURL
	}
	proxyURL := opts.ProxyURL
	if proxyURL == "" {
		proxyURL = defaultProxyURL
	}
	goSumDB := opts.GoSumDB
	if goSumDB == "" {
		goSumDB = defaultGoSumDB
	}
	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	goBinary := opts.GoBinary
	if goBinary == "" {
		goBinary = "go"
	}
	return &Checker{
		root:     root,
		client:   client,
		sumDBURL: strings.TrimSuffix(sumDBURL, "/"),
		proxyURL: strings.TrimSuffix(proxyURL, "/"),
		goSumDB:  goSumDB,
		runner:   runner,
		goBinary: goBinary,
	}
}

// Pins returns every pkg/* module that layout/versions verifies, sorted by
// module path.
//
// A pkg/* directory without a verified pin is an error: the scaffold would
// either pin nothing or pin a version the release gate never checked.
func (checker *Checker) Pins() ([]Pin, error) {
	matches, err := filepath.Glob(filepath.Join(checker.root, "pkg", "*", "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("list pkg modules: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no pkg/*/go.mod modules under %s", checker.root)
	}

	pins := make([]Pin, 0, len(matches))
	for _, match := range matches {
		name := filepath.Base(filepath.Dir(match))
		module := versions.PkgPrefix + name
		version, ok := versions.PackageVersion(module)
		if !ok {
			return nil, fmt.Errorf(
				"pkg/%s has no verified version in layout/versions",
				name,
			)
		}
		if version == "" {
			return nil, fmt.Errorf("pkg/%s has an empty verified version", name)
		}
		pins = append(pins, Pin{Module: module, Version: version})
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].Module < pins[j].Module })
	return pins, nil
}

// Check verifies every pin and returns the per-pin results plus a joined error.
// It does not stop at the first failure: a release engineer should see every
// unpublished pin in one run.
func (checker *Checker) Check(ctx context.Context) ([]Result, error) {
	pins, err := checker.Pins()
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(pins))
	var errs []error
	var resolvable []Pin
	for i, pin := range pins {
		results[i].Pin = pin

		if err := checker.checkSumDB(ctx, pin); err != nil {
			errs = append(errs, err)
		} else {
			results[i].SumDB = true
		}

		if err := checker.checkProxy(ctx, pin); err != nil {
			errs = append(errs, err)
		} else {
			results[i].Proxy = true
		}

		if results[i].SumDB && results[i].Proxy {
			resolvable = append(resolvable, pin)
		}
	}

	if len(resolvable) > 0 {
		verified, resolveErr := checker.resolve(ctx, resolvable)
		if resolveErr != nil {
			errs = append(errs, resolveErr)
		}
		for i := range results {
			if !results[i].SumDB || !results[i].Proxy {
				continue
			}
			if !verified[results[i].Pin] {
				continue
			}
			results[i].Resolved = true
			results[i].Checksum = true
		}
	}

	return results, errors.Join(errs...)
}

func (checker *Checker) checkSumDB(ctx context.Context, pin Pin) error {
	endpoint := checker.sumDBURL + "/lookup/" + escapeModulePath(pin.Module) + "@" + pin.Version
	status, err := checker.get(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("sum.golang.org lookup %s: %w", pin, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf(
			"sum.golang.org lookup %s: HTTP %d (module version is not published)",
			pin,
			status,
		)
	}
	return nil
}

func (checker *Checker) checkProxy(ctx context.Context, pin Pin) error {
	endpoint := checker.proxyURL + "/" + escapeModulePath(pin.Module) + "/@v/" + pin.Version + ".info"
	status, err := checker.get(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("proxy.golang.org %s: %w", pin, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf(
			"proxy.golang.org %s: HTTP %d (module version is not published)",
			pin,
			status,
		)
	}
	return nil
}

func (checker *Checker) get(ctx context.Context, endpoint string) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	response, err := checker.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, requestBodyLimit))
	return response.StatusCode, nil
}

// resolve writes a throwaway module that requires pins, downloads each pin with
// a fresh cache, verifies the cache against go.sum, and requires both checksum
// lines for every pin.
func (checker *Checker) resolve(ctx context.Context, pins []Pin) (map[Pin]bool, error) {
	dir, err := os.MkdirTemp("", "andurel-published-pins-*")
	if err != nil {
		return nil, fmt.Errorf("create pin resolution dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	goMod := checker.goModFile(pins)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return nil, fmt.Errorf("write pin resolution go.mod: %w", err)
	}
	for _, cache := range []string{"modcache", "gocache", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, cache), 0o755); err != nil {
			return nil, fmt.Errorf("create pin resolution cache %s: %w", cache, err)
		}
	}

	env := checker.goEnv(dir)
	verified := make(map[Pin]bool, len(pins))
	var errs []error
	for _, pin := range pins {
		argument := pin.Module + "@" + pin.Version
		if _, err := checker.runner.Run(ctx, dir, env, checker.goBinary, "mod", "download", argument); err != nil {
			errs = append(errs, fmt.Errorf("resolve %s: %w", pin, err))
			continue
		}
		verified[pin] = true
	}
	if len(verified) == 0 {
		return verified, errors.Join(errs...)
	}

	// Resolve the full module graph, not just the pinned zips: a pin whose
	// transitive requirements cannot be loaded is not installable either.
	if _, err := checker.runner.Run(ctx, dir, env, checker.goBinary, "list", "-m", "all"); err != nil {
		return map[Pin]bool{}, errors.Join(append(errs, fmt.Errorf("resolve module graph: %w", err))...)
	}

	if _, err := checker.runner.Run(ctx, dir, env, checker.goBinary, "mod", "verify"); err != nil {
		return map[Pin]bool{}, errors.Join(append(errs, fmt.Errorf("verify module cache: %w", err))...)
	}

	sums, err := os.ReadFile(filepath.Join(dir, "go.sum"))
	if err != nil {
		return map[Pin]bool{}, errors.Join(append(errs, fmt.Errorf("read go.sum: %w", err))...)
	}
	for pin := range verified {
		if !hasChecksumLines(string(sums), pin) {
			delete(verified, pin)
			errs = append(errs, fmt.Errorf("go.sum is missing checksums for %s", pin))
		}
	}
	return verified, errors.Join(errs...)
}

func (checker *Checker) goModFile(pins []Pin) string {
	var builder strings.Builder
	builder.WriteString("module andurel-published-pin-check\n\n")
	builder.WriteString("go " + checker.goDirective() + "\n\n")
	builder.WriteString("require (\n")
	for _, pin := range pins {
		builder.WriteString("\t" + pin.Module + " " + pin.Version + "\n")
	}
	builder.WriteString(")\n")
	return builder.String()
}

func (checker *Checker) goDirective() string {
	data, err := os.ReadFile(filepath.Join(checker.root, "go.mod"))
	if err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "go" {
				return fields[1]
			}
		}
	}
	version := strings.TrimPrefix(runtime.Version(), "go")
	if version == "" || !strings.HasPrefix(version, "1.") {
		return "1.27"
	}
	return version
}

// goEnv builds a module-resolution environment that cannot be satisfied by the
// caller's workspace, replace directives, or module cache.
func (checker *Checker) goEnv(dir string) []string {
	env := make([]string, 0, len(os.Environ())+8)
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && (strings.HasPrefix(name, "GO") || name == "GOWORK") {
			continue
		}
		env = append(env, entry)
	}
	return append(
		env,
		"GOWORK=off",
		"GOENV=off",
		"GOPROXY="+checker.proxyURL+",direct",
		"GOSUMDB="+checker.goSumDB,
		"GOMODCACHE="+filepath.Join(dir, "modcache"),
		"GOCACHE="+filepath.Join(dir, "gocache"),
		"GOTMPDIR="+filepath.Join(dir, "tmp"),
	)
}

func hasChecksumLines(goSum string, pin Pin) bool {
	moduleLine := false
	goModLine := false
	for line := range strings.SplitSeq(goSum, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != pin.Module || !strings.HasPrefix(fields[2], "h1:") {
			continue
		}
		switch fields[1] {
		case pin.Version:
			moduleLine = true
		case pin.Version + "/go.mod":
			goModLine = true
		}
	}
	return moduleLine && goModLine
}

// escapeModulePath applies the Go module proxy's uppercase escaping. Andurel
// module paths are lowercase today; keeping the rule explicit prevents a future
// mixed-case module from producing a misleading 404.
func escapeModulePath(module string) string {
	var builder strings.Builder
	builder.Grow(len(module))
	for _, char := range module {
		if char >= 'A' && char <= 'Z' {
			builder.WriteByte('!')
			builder.WriteRune(char + ('a' - 'A'))
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}
