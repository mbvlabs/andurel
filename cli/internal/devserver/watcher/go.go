package watcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var excludeDirs = map[string]bool{
	"tmp": true, "bin": true, "node_modules": true,
	".git": true, "assets": true, "vendor": true,
}

// RunGoWatcher watches .go files under dir (skipping generated _templ.go files
// and tool directories) and signals rebuildChan after a short debounce.
func RunGoWatcher(ctx context.Context, dir string, rebuildChan chan<- struct{}, verbose bool, log io.Writer) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Recursively add directories.
	if err := addWatchRecursive(watcher, dir); err != nil {
		return err
	}

	// Debounce timer
	var debounceTimer *time.Timer
	debounceDelay := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Add new directories to the watcher as they are created.
			if event.Op&fsnotify.Create != 0 {
				if stat, err := os.Stat(event.Name); err == nil && stat.IsDir() {
					if shouldSkipDir(filepath.Base(event.Name)) {
						continue
					}
					if err := addWatchRecursive(watcher, event.Name); err != nil && verbose {
						logf(log, "[run] failed to watch directory %s: %v\n", event.Name, err)
					}
					continue
				}
			}

			if !isGoFile(event.Name) || isTemplGenerated(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}

			// Debounce
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				logf(log, "[run] Go file changed: %s\n", filepath.Base(event.Name))
				select {
				case rebuildChan <- struct{}{}:
				default:
				}
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if verbose {
				logf(log, "[run] watcher error: %v\n", err)
			}
		}
	}
}

func addWatchRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		if shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}

		// fsnotify can race when directories are removed quickly; ignore missing paths.
		if err := w.Add(path); err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	})
}

func shouldSkipDir(name string) bool {
	return excludeDirs[name] || strings.HasPrefix(name, ".")
}

func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}

func isTemplGenerated(path string) bool {
	return strings.HasSuffix(path, "_templ.go")
}
