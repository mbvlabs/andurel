package watcher

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const emailCompileDebounce = 200 * time.Millisecond

// EmailWatcherConfig configures RunEmailWatcher.
type EmailWatcherConfig struct {
	// Dir is the project root holding email/ and css/.
	Dir string
	// Compile runs the andurel email compiler. It writes email/*_templ.go.
	Compile func(ctx context.Context) error
	// OnCompiled runs after a successful compile, so the app and queue are
	// rebuilt with the new renderers (the Go watcher skips _templ.go files).
	OnCompiled func()
	Log        io.Writer
}

// IsEmailCompilerSource reports whether path is an input of the email
// compiler: email/**/*.templ, css/email.css or css/theme.css.
func IsEmailCompilerSource(rootDir, path string) bool {
	cleanPath := filepath.Clean(path)
	if cleanPath == filepath.Join(rootDir, "css", "email.css") ||
		cleanPath == filepath.Join(rootDir, "css", "theme.css") {
		return true
	}
	emailDir := filepath.Join(rootDir, "email") + string(filepath.Separator)
	return strings.HasPrefix(cleanPath, emailDir) && filepath.Ext(cleanPath) == ".templ"
}

// RunEmailWatcher watches the email compiler inputs and recompiles on change.
// It returns an error when email/ or css/ cannot be watched, and nil once ctx
// is done. Compile failures are logged and the watcher keeps running.
func RunEmailWatcher(ctx context.Context, cfg EmailWatcherConfig) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create email watcher: %w", err)
	}
	defer func() {
		if err := fsWatcher.Close(); err != nil {
			logf(cfg.Log, "[run] close email watcher: %v\n", err)
		}
	}()

	if err := addEmailWatchDirectories(fsWatcher, filepath.Join(cfg.Dir, "email")); err != nil {
		return err
	}
	if err := fsWatcher.Add(filepath.Join(cfg.Dir, "css")); err != nil {
		return fmt.Errorf("watch email CSS: %w", err)
	}

	runEmailWatcher(ctx, fsWatcher, cfg)
	return nil
}

func addEmailWatchDirectories(fsWatcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if err := fsWatcher.Add(path); err != nil {
			return fmt.Errorf("watch email directory %s: %w", path, err)
		}
		return nil
	})
}

func runEmailWatcher(ctx context.Context, fsWatcher *fsnotify.Watcher, cfg EmailWatcherConfig) {
	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	compilePending := false

	for {
		select {
		case <-ctx.Done():
			return
		case watchErr, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			logf(cfg.Log, "[run] email watcher: %v\n", watchErr)
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := addEmailWatchDirectories(fsWatcher, event.Name); err != nil {
						logf(cfg.Log, "[run] email watcher: %v\n", err)
					}
				}
			}
			if !IsEmailCompilerSource(cfg.Dir, event.Name) {
				continue
			}
			compilePending = true
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(emailCompileDebounce)
		case <-debounce.C:
			if !compilePending {
				continue
			}
			compilePending = false
			if cfg.Compile == nil {
				continue
			}
			logf(cfg.Log, "[run] Email template changed, compiling\n")
			if err := cfg.Compile(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				logf(cfg.Log, "[run] email compile failed: %v\n", err)
				continue
			}
			if cfg.OnCompiled != nil {
				cfg.OnCompiled()
			}
		}
	}
}
