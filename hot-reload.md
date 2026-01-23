# Hot Reload Improvements Plan

## Overview

Replace air with a custom, purpose-built watcher that integrates tightly with templ and serves assets directly from disk in dev mode. Linux and macOS only.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     cmd/run/main.go                         │
├─────────────────────────────────────────────────────────────┤
│  runGoWatcher()          - fsnotify for .go files           │
│  runTemplWatcher()       - templ generate --watch + parser  │
│  runTailwindWatcher()    - tailwind CLI (if tailwind)       │
│  runAppServer()          - build + run + restart lifecycle  │
│  runProxyServer()        - proxy + WS + JS injection        │
└─────────────────────────────────────────────────────────────┘

Signal Flow:
─────────────
.go file changed ──────────────────────────┐
                                           ▼
.templ changed → templ parser ─┬─ needsRestart ──→ rebuild channel
                               │
                               └─ needsBrowserReload ──→ broadcaster

CSS changed (tailwind) ────────────────────────────→ broadcaster

rebuild channel → runAppServer() → build → restart → health check → broadcaster
```

## Components

### 1. Go File Watcher (`cmd_run_gowatcher.tmpl`)

New file - fsnotify-based watcher for `.go` files.

```go
package main

import (
    "context"
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

func runGoWatcher(ctx context.Context, rebuildChan chan<- struct{}) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()

    wd, _ := os.Getwd()

    // Recursively add directories
    filepath.WalkDir(wd, func(path string, d os.DirEntry, err error) error {
        if err != nil || !d.IsDir() {
            return nil
        }
        name := d.Name()
        if excludeDirs[name] || strings.HasPrefix(name, ".") {
            return filepath.SkipDir
        }
        watcher.Add(path)
        return nil
    })

    // Debounce timer
    var debounceTimer *time.Timer
    debounceDelay := 500 * time.Millisecond

    for {
        select {
        case <-ctx.Done():
            return nil
        case event := <-watcher.Events:
            if !isGoFile(event.Name) || isTemplGenerated(event.Name) {
                continue
            }
            if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
                continue
            }

            // Debounce
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(debounceDelay, func() {
                logInfo("[andurel] Go file changed: %s", filepath.Base(event.Name))
                select {
                case rebuildChan <- struct{}{}:
                default:
                }
            })
        case err := <-watcher.Errors:
            logDebug("[andurel] watcher error: %v", err)
        }
    }
}

func isGoFile(path string) bool {
    return strings.HasSuffix(path, ".go")
}

func isTemplGenerated(path string) bool {
    return strings.HasSuffix(path, "_templ.go")
}
```

### 2. App Server Manager (`cmd_run_appserver.tmpl`)

New file - handles build/run/restart lifecycle.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "syscall"
    "time"
)

type AppServer struct {
    cmd         *exec.Cmd
    buildCmd    string
    binPath     string
    appPort     string
    broadcaster *Broadcaster
}

func NewAppServer(appPort string, broadcaster *Broadcaster) *AppServer {
    wd, _ := os.Getwd()
    return &AppServer{
        buildCmd:    "go build -o tmp/bin/main cmd/app/main.go",
        binPath:     wd + "/tmp/bin/main",
        appPort:     appPort,
        broadcaster: broadcaster,
    }
}

func (s *AppServer) Run(ctx context.Context, rebuildChan <-chan struct{}) error {
    // Initial build and start
    if err := s.rebuild(ctx); err != nil {
        logInfo("[andurel] Initial build failed: %v", err)
    }

    for {
        select {
        case <-ctx.Done():
            s.stop()
            return nil
        case <-rebuildChan:
            s.stop()
            if err := s.rebuild(ctx); err != nil {
                logInfo("[andurel] Build failed: %v", err)
                continue
            }
        }
    }
}

func (s *AppServer) rebuild(ctx context.Context) error {
    logInfo("[andurel] Building...")

    buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "tmp/bin/main", "cmd/app/main.go")
    buildCmd.Stdout = os.Stdout
    buildCmd.Stderr = os.Stderr

    if err := buildCmd.Run(); err != nil {
        return fmt.Errorf("build failed: %w", err)
    }

    logInfo("[andurel] Starting server...")
    s.cmd = exec.CommandContext(ctx, s.binPath)
    s.cmd.Env = append(os.Environ(), "TEMPL_DEV_MODE=true")
    s.cmd.Stdout = os.Stdout
    s.cmd.Stderr = os.Stderr

    if err := s.cmd.Start(); err != nil {
        return fmt.Errorf("start failed: %w", err)
    }

    addProcess(s.cmd)

    // Wait for healthy, then broadcast
    go func() {
        healthURL := fmt.Sprintf("http://localhost:%s/", s.appPort)
        BroadcastWhenHealthy(ctx, healthURL, s.broadcaster)
    }()

    return nil
}

func (s *AppServer) stop() {
    if s.cmd != nil && s.cmd.Process != nil {
        s.cmd.Process.Signal(syscall.SIGTERM)
        done := make(chan error, 1)
        go func() { done <- s.cmd.Wait() }()

        select {
        case <-done:
        case <-time.After(3 * time.Second):
            s.cmd.Process.Kill()
        }
    }
}
```

### 3. Updated Main (`cmd_run_main.tmpl`)

Simplified orchestration - remove air, add rebuild channel.

```go
func main() {
    // ... setup code ...

    broadcaster := NewBroadcaster()
    rebuildChan := make(chan struct{}, 1)
    templChange := make(chan TemplChange, 64)

    // Start goroutines
    go runProxyServer(ctx, proxyPort, appPort, broadcaster)
    go runGoWatcher(ctx, rebuildChan)
    go runLiveTemplWithParser(ctx, templChange)
    {{- if eq .CSSFramework "tailwind" }}
    go runLiveTailwind(ctx, broadcaster)
    {{- end }}

    // App server manager
    appServer := NewAppServer(appPort, broadcaster)
    go appServer.Run(ctx, rebuildChan)

    // Handle templ changes
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case change := <-templChange:
                switch change {
                case TemplChangeNeedsBrowserReload:
                    logInfo("[andurel] Template changed, reloading browser")
                    broadcaster.Broadcast()
                case TemplChangeNeedsRestart:
                    logInfo("[andurel] Template Go code changed, rebuilding")
                    select {
                    case rebuildChan <- struct{}{}:
                    default:
                    }
                }
            }
        }
    }()

    // ... signal handling ...
}
```

### 4. Dev Asset Serving (`controllers_assets.tmpl`)

Modify to serve from disk in dev mode.

```go
import (
    "os"
    "path/filepath"
    // ... existing imports
)

func (a Assets) readAssetFile(path string) ([]byte, error) {
    if config.Env != server.ProdEnvironment {
        return os.ReadFile(filepath.Join("assets", path))
    }
    return assets.Files.ReadFile(path)
}

func (a Assets) Stylesheet(etx *echo.Context) error {
    stylesheet, err := a.readAssetFile("css/style.css")
    if err != nil {
        return err
    }
    if config.Env == server.ProdEnvironment {
        etx = a.enableCaching(etx, stylesheet)
    }
    return etx.Blob(http.StatusOK, "text/css", stylesheet)
}

// Apply same pattern to Scripts(), Script(), StyleImport()
```

### 5. Logging Improvements

Add to `cmd_run_main.tmpl`:

```go
var verbose = os.Getenv("ANDUREL_VERBOSE") == "true"

func logDebug(format string, args ...interface{}) {
    if verbose {
        fmt.Printf(format+"\n", args...)
    }
}

func logInfo(format string, args ...interface{}) {
    fmt.Printf(format+"\n", args...)
}
```

Update `cmd_run_templ.tmpl` line 63:
```go
logDebug("[templ] %s", line)  // was: fmt.Printf("[templ] %s\n", line)
```

### 6. Debounced Broadcaster (`cmd_run_broadcaster.tmpl`)

Add debouncing to prevent rapid reloads:

```go
type Broadcaster struct {
    mu            sync.RWMutex
    listeners     map[chan struct{}]struct{}
    lastBroadcast time.Time
    debounceTime  time.Duration
}

func NewBroadcaster() *Broadcaster {
    return &Broadcaster{
        listeners:    make(map[chan struct{}]struct{}),
        debounceTime: 50 * time.Millisecond,
    }
}

func (b *Broadcaster) Broadcast() {
    b.mu.Lock()
    now := time.Now()
    if now.Sub(b.lastBroadcast) < b.debounceTime {
        b.mu.Unlock()
        return
    }
    b.lastBroadcast = now
    b.mu.Unlock()

    b.mu.RLock()
    defer b.mu.RUnlock()
    for ch := range b.listeners {
        select {
        case ch <- struct{}{}:
        default:
        }
    }
}
```

---

## Files Summary

| File | Action | Description |
|------|--------|-------------|
| `layout/templates/cmd_run_gowatcher.tmpl` | CREATE | fsnotify-based Go file watcher |
| `layout/templates/cmd_run_appserver.tmpl` | CREATE | Build/run/restart lifecycle manager |
| `layout/templates/cmd_run_main.tmpl` | MODIFY | Remove air, add rebuild channel orchestration |
| `layout/templates/cmd_run_templ.tmpl` | MODIFY | Use logDebug for raw output |
| `layout/templates/cmd_run_broadcaster.tmpl` | MODIFY | Add debouncing |
| `layout/templates/controllers_assets.tmpl` | MODIFY | Dev mode disk serving |
| `layout/templates/gitignore.tmpl` | MODIFY | Remove trigger.txt (no longer needed) |
| `layout/layout.go` | MODIFY | Remove air from DefaultGoTools, add new template mappings |
| `layout/versions/versions.go` | MODIFY | Remove Air version |

---

## Dependencies

**Add to scaffolded go.mod:**
- `github.com/fsnotify/fsnotify` - File system notifications

**Remove:**
- `air` binary no longer needed in `bin/`

---

## Benefits

1. **No air dependency** - One less tool to sync/maintain
2. **Faster rebuilds** - Purpose-built, no air overhead
3. **No trigger.txt hack** - Direct channel communication
4. **Instant JS changes** - Assets served from disk in dev
5. **Cleaner logs** - Verbose mode for debugging
6. **Tight integration** - Templ and Go watcher coordinate properly

---

## Verification

1. Scaffold new project: `andurel new testproj`
2. Run `andurel tool sync`
3. Start dev server: `./bin/run`
4. Test scenarios:
   - Edit `.templ` file content → browser reloads (no rebuild)
   - Edit `.templ` that changes Go code → rebuild + reload
   - Edit `.go` file → rebuild + reload
   - Edit `_templ.go` manually → ignored (templ handles this)
   - Edit CSS (Tailwind) → CSS rebuilds + reload
   - Edit JS file → instant reload (no rebuild)
5. Check `ANDUREL_VERBOSE=true ./bin/run` shows debug output
6. Verify no air binary needed
