package watcher

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

type TemplChange int8

const (
	TemplChangeNone               TemplChange = iota
	TemplChangeNeedsRestart                   // Full server restart needed (e.g., _templ.go changed)
	TemplChangeNeedsBrowserReload             // Just browser reload needed (e.g., template content changed)
)

var (
	bytesPrefixWarning      = []byte(`(!)`)
	bytesPrefixErr          = []byte(`(✗)`)
	bytesPrefixErrCleared   = []byte(`(✓) Error cleared`)
	bytesPrefixPostGenEvent = []byte(`(✓) Post-generation event received, processing...`)
	bytesNeedsRestart       = []byte(`needsRestart=true`)
	bytesNeedsBrowserReload = []byte(`needsBrowserReload=true`)
)

var templShutdownTimeout = 2 * time.Second

// TemplWatchDir is the only directory `templ generate --watch` covers. The
// Inertia root document in resources/ is the only page templ in the scaffold.
// Email templates are React Email sources under emails/, exported to
// emails/templates by andurel sync emails, so plain templ never touches them.
const TemplWatchDir = "resources"

type TemplWatcherConfig struct {
	// Dir is the project root. bin/templ and tmp/templ resolve against it.
	Dir        string
	Verbose    bool
	AddProcess func(*exec.Cmd)
	OnTemplErr func(msg string)
	Log        io.Writer
}

// TemplWatchPattern matches the .templ files under dir/resources. templ
// applies the watch pattern to absolute paths, both to its first full
// generation and to later change events, so nothing outside resources/ is
// generated. The Go watcher handles .go files.
func TemplWatchPattern(dir string) string {
	prefix := filepath.Join(dir, TemplWatchDir) + string(filepath.Separator)
	return "^" + regexp.QuoteMeta(prefix) + `.+\.templ$`
}

// TemplArgs returns the argv (after the binary) for the templ watch process
// of the project rooted at dir. -path stays the project root so generated
// code records file names relative to it (resources/root.templ), exactly as
// `andurel build` and verify generate them.
func TemplArgs(dir string) []string {
	return []string{
		"generate",
		"--watch",
		"-path", dir,
		"--log-level", "debug",
		"--watch-pattern", TemplWatchPattern(dir),
	}
}

func RunTemplWatcher(ctx context.Context, templChange chan<- TemplChange, cfg TemplWatcherConfig) error {
	dir := cfg.Dir
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = wd
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	templDir := filepath.Join(dir, "tmp", "templ")
	if err := os.MkdirAll(templDir, 0o755); err != nil {
		return fmt.Errorf("create templ directory %s: %w", templDir, err)
	}

	cmd := exec.Command(filepath.Join(dir, "bin", "templ"), TemplArgs(dir)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TMPDIR="+templDir)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("obtaining stderr pipe: %w", err)
	}

	logf(cfg.Log, "[run] Starting templ generate --watch\n")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting templ: %w", err)
	}

	if cfg.AddProcess != nil {
		cfg.AddProcess(cmd)
	}

	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			b := scanner.Bytes()
			line := scanner.Text()

			// Print templ output for debugging
			if cfg.Verbose {
				logf(cfg.Log, "[templ] %s\n", line)
			}

			switch {
			case bytes.HasPrefix(b, bytesPrefixWarning):
				logf(cfg.Log, "[run] templ warning: %s\n", line)
			case bytes.HasPrefix(b, bytesPrefixErr):
				logf(cfg.Log, "[run] templ error: %s\n", line)
				if cfg.OnTemplErr != nil {
					cfg.OnTemplErr(line)
				}
			case bytes.HasPrefix(b, bytesPrefixErrCleared):
				logf(cfg.Log, "[run] templ error cleared\n")
				if cfg.OnTemplErr != nil {
					cfg.OnTemplErr("")
				}
			}

			if after, found := bytes.CutPrefix(b, bytesPrefixPostGenEvent); found {
				switch {
				case bytes.Contains(after, bytesNeedsRestart):
					logf(cfg.Log, "[run] templ: needs restart (Go code changed)\n")
					select {
					case templChange <- TemplChangeNeedsRestart:
					default:
					}
				case bytes.Contains(after, bytesNeedsBrowserReload):
					logf(cfg.Log, "[run] templ: needs browser reload (template content changed)\n")
					select {
					case templChange <- TemplChangeNeedsBrowserReload:
					default:
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			logf(cfg.Log, "[run] error scanning templ output: %v\n", err)
		}
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		stopTemplProcess(cmd, done, cfg.Log)
		return nil
	case err := <-done:
		return err
	}
}

func stopTemplProcess(cmd *exec.Cmd, done <-chan error, log io.Writer) {
	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(os.Interrupt)

	timer := time.NewTimer(templShutdownTimeout)
	defer timer.Stop()

	select {
	case <-done:
		return
	case <-timer.C:
	}

	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		logf(log, "[run] templ kill fallback error: %v\n", err)
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		logf(log, "[run] templ did not exit after kill fallback\n")
	}
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		w = os.Stdout
	}
	_, _ = fmt.Fprintf(w, format, args...)
}
