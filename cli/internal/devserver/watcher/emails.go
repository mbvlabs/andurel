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

const (
	emailsSyncDebounce = 200 * time.Millisecond
	// emailsTemplateQuiet suppresses template events produced by the sync's
	// own export. Those events arrive right after Sync returns and must not
	// trigger a second rebuild.
	emailsTemplateQuiet = 2 * time.Second
)

// EmailsWatcherConfig configures RunEmailsWatcher.
type EmailsWatcherConfig struct {
	// Dir is the project root holding emails/.
	Dir string
	// Sync runs andurel sync emails: props.ts and manifest.ts generation plus
	// the project's email:export script.
	Sync func(ctx context.Context) error
	// OnSynced runs after a successful sync, so the app and queue are rebuilt
	// with the new embedded emails/templates.
	OnSynced func()
	// OnTemplatesChanged runs after emails/templates/*.tmpl changed outside a
	// sync (git checkout, manual export), so the app and queue are rebuilt
	// with the new embedded templates without re-running the export.
	OnTemplatesChanged func()
	// OnSyncError reports a sync error for the TUI. An empty message clears
	// the previous error.
	OnSyncError func(msg string)
	Log         io.Writer
}

// IsEmailsSource reports whether path is an input of andurel sync emails:
// top-level emails/*.go structs and authored .ts/.tsx files under emails/.
// Generated emails/generated files, committed emails/templates and Go tests
// are ignored, as is anything under node_modules.
func IsEmailsSource(rootDir, path string) bool {
	rel, ok := relativeProjectPath(rootDir, path)
	if !ok {
		return false
	}

	if strings.HasPrefix(rel, "emails/templates/") ||
		strings.HasPrefix(rel, "emails/generated/") ||
		strings.Contains(rel, "/node_modules/") {
		return false
	}
	if !strings.HasPrefix(rel, "emails/") {
		return false
	}
	if strings.HasSuffix(rel, ".go") {
		return filepath.Dir(rel) == "emails" &&
			!strings.HasSuffix(rel, "_test.go")
	}
	return strings.HasSuffix(rel, ".ts") || strings.HasSuffix(rel, ".tsx")
}

// IsEmailsTemplateOutput reports whether path is a committed exported
// template. The Go watcher only rebuilds for .go files, so this watcher treats
// template writes as a rebuild-only signal: re-running the export would loop.
func IsEmailsTemplateOutput(rootDir, path string) bool {
	rel, ok := relativeProjectPath(rootDir, path)
	if !ok {
		return false
	}
	return strings.HasPrefix(rel, "emails/templates/") &&
		strings.HasSuffix(rel, ".tmpl")
}

func relativeProjectPath(rootDir, path string) (string, bool) {
	rel, err := filepath.Rel(rootDir, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// RunEmailsWatcher watches the React Email inputs and syncs on change. It
// returns an error only when the watcher cannot be created; a missing emails/
// directory is tolerated because Go-only scaffolds have no React Email tree.
// Sync failures are reported through OnSyncError and the watcher keeps
// running.
func RunEmailsWatcher(ctx context.Context, cfg EmailsWatcherConfig) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create emails watcher: %w", err)
	}
	defer func() {
		if err := fsWatcher.Close(); err != nil {
			logf(cfg.Log, "[run] close emails watcher: %v\n", err)
		}
	}()

	added, err := addEmailsWatchDirectories(fsWatcher, filepath.Join(cfg.Dir, "emails"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !added {
		return nil
	}

	runEmailsWatcher(ctx, fsWatcher, cfg)
	return nil
}

// addEmailsWatchDirectories watches root and every directory below it. It
// skips node_modules and the generated emails/generated directory. It
// reports whether at least one directory was watched.
func addEmailsWatchDirectories(fsWatcher *fsnotify.Watcher, root string) (bool, error) {
	watched := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if name == "node_modules" || name == ".git" {
			return filepath.SkipDir
		}
		if filepath.Base(path) == "generated" &&
			filepath.Base(filepath.Dir(path)) == "emails" {
			return filepath.SkipDir
		}
		if err := fsWatcher.Add(path); err != nil {
			return fmt.Errorf("watch emails directory %s: %w", path, err)
		}
		watched = true
		return nil
	})
	return watched, err
}

func runEmailsWatcher(ctx context.Context, fsWatcher *fsnotify.Watcher, cfg EmailsWatcherConfig) {
	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	syncPending := false
	templatePending := false
	var lastSync time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case watchErr, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			logf(cfg.Log, "[run] emails watcher: %v\n", watchErr)
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if _, err := addEmailsWatchDirectories(fsWatcher, event.Name); err != nil {
						logf(cfg.Log, "[run] emails watcher: %v\n", err)
					}
				}
			}
			if IsEmailsTemplateOutput(cfg.Dir, event.Name) {
				templatePending = true
			} else if IsEmailsSource(cfg.Dir, event.Name) {
				syncPending = true
			} else {
				continue
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(emailsSyncDebounce)
		case <-debounce.C:
			if syncPending {
				syncPending = false
				templatePending = false
				if cfg.Sync == nil {
					continue
				}
				logf(cfg.Log, "[run] React Email source changed, syncing\n")
				err := cfg.Sync(ctx)
				lastSync = time.Now()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					logf(cfg.Log, "[run] email sync failed: %v\n", err)
					if cfg.OnSyncError != nil {
						cfg.OnSyncError(err.Error())
					}
					continue
				}
				if cfg.OnSyncError != nil {
					cfg.OnSyncError("")
				}
				if cfg.OnSynced != nil {
					cfg.OnSynced()
				}
				continue
			}
			if !templatePending {
				continue
			}
			templatePending = false
			// Template events from the sync's own export arrive just after
			// Sync returns; the OnSynced rebuild already covers them.
			if time.Since(lastSync) < emailsTemplateQuiet {
				continue
			}
			logf(cfg.Log, "[run] Email templates changed, rebuilding\n")
			if cfg.OnTemplatesChanged != nil {
				cfg.OnTemplatesChanged()
			}
		}
	}
}
