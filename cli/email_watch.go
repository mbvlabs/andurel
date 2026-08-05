package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watchEmailProject(ctx context.Context, rootDir, tailwindPath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create email watcher: %w", err)
	}

	emailDir := filepath.Join(rootDir, "email")
	if err := addEmailWatchDirectories(watcher, emailDir); err != nil {
		_ = watcher.Close()
		return err
	}
	if err := watcher.Add(filepath.Join(rootDir, "css")); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("watch email CSS: %w", err)
	}

	go runEmailWatcher(ctx, watcher, rootDir, tailwindPath)
	return nil
}

func addEmailWatchDirectories(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("watch email directory %s: %w", path, err)
		}
		return nil
	})
}

func runEmailWatcher(ctx context.Context, watcher *fsnotify.Watcher, rootDir, tailwindPath string) {
	defer watcher.Close()
	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	compilePending := false

	for {
		select {
		case <-ctx.Done():
			return
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "email watcher: %v\n", watchErr)
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := addEmailWatchDirectories(watcher, event.Name); err != nil {
						fmt.Fprintf(os.Stderr, "email watcher: %v\n", err)
					}
				}
			}
			if !isEmailCompilerSource(rootDir, event.Name) {
				continue
			}
			compilePending = true
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(200 * time.Millisecond)
		case <-debounce.C:
			if !compilePending {
				continue
			}
			compilePending = false
			if err := compileEmailWithTailwind(ctx, rootDir, tailwindPath); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func isEmailCompilerSource(rootDir, path string) bool {
	cleanPath := filepath.Clean(path)
	if cleanPath == filepath.Join(rootDir, "css", "email.css") {
		return true
	}
	emailDir := filepath.Join(rootDir, "email") + string(filepath.Separator)
	return strings.HasPrefix(cleanPath, emailDir) && filepath.Ext(cleanPath) == ".templ"
}
