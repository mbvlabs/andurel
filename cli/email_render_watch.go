package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver/watcher"
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

const (
	// emailRenderWatchDebounce coalesces the burst of events a single edit
	// produces into one sync and one render.
	emailRenderWatchDebounce = 200 * time.Millisecond
	// emailRenderWatchQuiet ignores template output events right after our
	// own export. Re-running the export for those would loop.
	emailRenderWatchQuiet = 2 * time.Second
)

// emailWatchEvent is what a changed path asks the gallery to do.
type emailWatchEvent int

const (
	emailWatchEventNone emailWatchEvent = iota
	// emailWatchEventRender re-renders without regenerating props or
	// templates, for a committed template edited outside the export.
	emailWatchEventRender
	// emailWatchEventSync regenerates emails/generated and runs the
	// project's email:export before re-rendering.
	emailWatchEventSync
)

// classifyEmailWatchEvent maps a changed path to the work it requires.
// Authored TSX and emails/*.go are sync inputs; committed templates are
// render-only because the export wrote them.
func classifyEmailWatchEvent(rootDir, path string) emailWatchEvent {
	switch {
	case watcher.IsEmailsSource(rootDir, path):
		return emailWatchEventSync
	case watcher.IsEmailsTemplateOutput(rootDir, path):
		return emailWatchEventRender
	default:
		return emailWatchEventNone
	}
}

// emailWatchCycle coalesces events into one pending action and suppresses the
// template events produced by our own export.
type emailWatchCycle struct {
	pending       emailWatchEvent
	suppressUntil time.Time
	quiet         time.Duration
}

func newEmailWatchCycle(quiet time.Duration) *emailWatchCycle {
	return &emailWatchCycle{quiet: quiet}
}

// observe records an event and reports whether it should arm the debounce
// timer. Render events during the quiet window are our own export output and
// are ignored.
func (c *emailWatchCycle) observe(event emailWatchEvent, now time.Time) bool {
	switch event {
	case emailWatchEventSync:
		c.pending = emailWatchEventSync
		return true
	case emailWatchEventRender:
		if now.Before(c.suppressUntil) {
			return false
		}
		if c.pending == emailWatchEventNone {
			c.pending = emailWatchEventRender
		}
		return true
	default:
		return false
	}
}

// take returns the coalesced action and clears it.
func (c *emailWatchCycle) take() emailWatchEvent {
	event := c.pending
	c.pending = emailWatchEventNone
	return event
}

// finishSync starts the quiet window after the export has written templates.
func (c *emailWatchCycle) finishSync(now time.Time) {
	c.suppressUntil = now.Add(c.quiet)
}

func runEmailRenderWatch(cmd *cobra.Command, rootDir, name string, openGallery bool) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	render := func() error {
		start := time.Now()
		var stdout, stderr bytes.Buffer
		err := runEmailRenderCommand(
			rootDir,
			emailRenderGoArgs(name, true, true),
			&stdout,
			&stderr,
		)
		if err != nil {
			return emailWatchError("render emails", err, stderr.String())
		}

		var report emailRenderReport
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &report); err != nil {
			return fmt.Errorf("decode email render report: %w", err)
		}
		_, _ = fmt.Fprintf(
			out,
			"rendered %s in %s\n",
			emailPreviewCount(len(report.Emails)),
			formatEmailElapsed(time.Since(start)),
		)
		return nil
	}

	if err := render(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	if openGallery {
		if err := openEmailGallery(filepath.Join(rootDir, "tmp", "emails", "index.html")); err != nil {
			return err
		}
	}

	fsWatcher, watched, err := newEmailRenderWatcher(rootDir)
	if err != nil {
		return err
	}
	if !watched {
		_ = fsWatcher.Close()
		return output.NewError(
			output.CodeError,
			"nothing to watch: emails/ is missing",
			output.ExitDependency,
			"Create emails/*.go before running andurel email render --watch.",
		)
	}
	defer func() {
		_ = fsWatcher.Close()
	}()

	_, _ = fmt.Fprintln(out, "watching emails/ and emails/templates/; press Ctrl-C to stop")

	syncEmails := func(ctx context.Context) error {
		start := time.Now()
		if _, err := generateEmailsFiles(rootDir); err != nil {
			return err
		}
		var exportOutput bytes.Buffer
		if err := runEmailsExport(ctx, rootDir, &exportOutput, &exportOutput); err != nil {
			return emailWatchError("sync emails", err, exportOutput.String())
		}
		_, _ = fmt.Fprintf(
			out,
			"synced email templates in %s\n",
			formatEmailElapsed(time.Since(start)),
		)
		return nil
	}

	return runEmailRenderWatchLoop(ctx, fsWatcher, rootDir, syncEmails, render, out, errOut)
}

func newEmailRenderWatcher(rootDir string) (*fsnotify.Watcher, bool, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, false, fmt.Errorf("create email watcher: %w", err)
	}

	root := filepath.Join(rootDir, "emails")
	if _, statErr := os.Stat(root); statErr != nil {
		if os.IsNotExist(statErr) {
			return fsWatcher, false, nil
		}
		_ = fsWatcher.Close()
		return nil, false, statErr
	}
	watched, err := addEmailRenderWatchTree(fsWatcher, root, rootDir)
	if err != nil {
		_ = fsWatcher.Close()
		return nil, false, err
	}
	return fsWatcher, watched, nil
}

// addEmailRenderWatchTree watches root and every directory below it, skipping
// generated props and node_modules.
func addEmailRenderWatchTree(
	fsWatcher *fsnotify.Watcher,
	root, projectRoot string,
) (bool, error) {
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
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == generatedEmailsDir || strings.HasPrefix(rel, generatedEmailsDir+"/") {
			return filepath.SkipDir
		}
		if err := fsWatcher.Add(path); err != nil {
			return fmt.Errorf("watch email directory %s: %w", path, err)
		}
		watched = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return watched, nil
}

func runEmailRenderWatchLoop(
	ctx context.Context,
	fsWatcher *fsnotify.Watcher,
	rootDir string,
	syncEmails func(context.Context) error,
	render func() error,
	out, errOut io.Writer,
) error {
	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	cycle := newEmailWatchCycle(emailRenderWatchQuiet)

	for {
		select {
		case <-ctx.Done():
			return nil
		case watchErr, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			_, _ = fmt.Fprintf(errOut, "email watcher: %v\n", watchErr)
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if _, err := addEmailRenderWatchTree(fsWatcher, event.Name, rootDir); err != nil {
						_, _ = fmt.Fprintf(errOut, "email watcher: %v\n", err)
					}
				}
			}
			if !cycle.observe(classifyEmailWatchEvent(rootDir, event.Name), time.Now()) {
				continue
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(emailRenderWatchDebounce)
		case <-debounce.C:
			kind := cycle.take()
			if kind == emailWatchEventNone {
				continue
			}
			if kind == emailWatchEventSync {
				err := syncEmails(ctx)
				cycle.finishSync(time.Now())
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					_, _ = fmt.Fprintf(errOut, "email sync failed: %v\n", err)
					continue
				}
			}
			if err := render(); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				_, _ = fmt.Fprintf(errOut, "email render failed: %v\n", err)
			}
		}
	}
}

func emailWatchError(action string, err error, details string) error {
	details = strings.TrimSpace(details)
	if details == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w\n%s", action, err, details)
}

func emailPreviewCount(count int) string {
	if count == 1 {
		return "1 email"
	}
	return fmt.Sprintf("%d emails", count)
}

func formatEmailElapsed(elapsed time.Duration) string {
	return fmt.Sprintf("%.1fs", elapsed.Seconds())
}
