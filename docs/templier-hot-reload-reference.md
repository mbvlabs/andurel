# Templier Hot Reload Reference

Reference document for porting templier's hot reload functionality into andurel.

## Overview

Templier's hot reload works through these components:

1. **Templ stderr parser** - Watches `templ generate --watch` output for reload signals
2. **SignalBroadcaster** - Thread-safe pub/sub for reload events
3. **WebSocket endpoint** - Sends 'r' message to connected browsers
4. **Injected JavaScript** - Listens for WebSocket messages, calls `window.location.reload()`
5. **Proxy server** - Intercepts HTML responses, injects the JS
6. **Health check** - Polls app before broadcasting reload

---

## 1. Templ Stderr Parser

Runs `templ generate --watch` and parses stderr for post-generation events.

```go
// internal/cmdrun/cmdrun.go

type TemplChange int8

const (
	_ TemplChange = iota
	TemplChangeNeedsRestart
	TemplChangeNeedsBrowserReload
)

var (
	bytesPrefixWarning      = []byte(`(!)`)
	bytesPrefixErr          = []byte(`(✗)`)
	bytesPrefixErrCleared   = []byte(`(✓) Error cleared`)
	bytesPrefixPostGenEvent = []byte(`(✓) Post-generation event received, processing...`)
	bytesNeedsRestart       = []byte(`needsRestart=true`)
	bytesNeedsBrowserReload = []byte(`needsBrowserReload=true`)
)

// RunTemplWatch starts `templ generate --log-level debug --watch` and reads its
// stderr pipe for failure and success logs.
func RunTemplWatch(
	ctx context.Context,
	workDir string,
	templChange chan<- TemplChange,
) error {
	cmd := exec.Command(
		"templ", "generate",
		"--watch",
		"--log-level", "debug",
		"--watch-pattern", `(.+\.templ$)`,
	)
	cmd.Dir = workDir

	stdout, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("obtaining stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			b := scanner.Bytes()

			if after, found := bytes.CutPrefix(b, bytesPrefixPostGenEvent); found {
				switch {
				case bytes.Contains(after, bytesNeedsRestart):
					select {
					case templChange <- TemplChangeNeedsRestart:
					default:
					}
				case bytes.Contains(after, bytesNeedsBrowserReload):
					select {
					case templChange <- TemplChangeNeedsBrowserReload:
					default:
					}
				}
			}
		}
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("interrupting templ watch process: %w", err)
		}
		if err := <-done; err != nil {
			return fmt.Errorf("process did not exit cleanly: %w", err)
		}
	case err := <-done:
		return err
	}
	return nil
}
```

---

## 2. Signal Broadcaster

Thread-safe pub/sub for notifying multiple WebSocket connections.

```go
// internal/broadcaster/broadcaster.go

package broadcaster

import "sync"

type SignalBroadcaster struct {
	lock      sync.Mutex
	listeners map[chan<- struct{}]struct{}
}

func NewSignalBroadcaster() *SignalBroadcaster {
	return &SignalBroadcaster{listeners: map[chan<- struct{}]struct{}{}}
}

func (b *SignalBroadcaster) Len() int {
	b.lock.Lock()
	defer b.lock.Unlock()
	return len(b.listeners)
}

func (b *SignalBroadcaster) AddListener(c chan<- struct{}) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.listeners[c] = struct{}{}
}

func (b *SignalBroadcaster) RemoveListener(c chan<- struct{}) {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.listeners, c)
}

// BroadcastNonblock writes to all listeners in a non-blocking manner.
// Ignores unresponsive listeners.
func (b *SignalBroadcaster) BroadcastNonblock() {
	b.lock.Lock()
	defer b.lock.Unlock()
	for l := range b.listeners {
		select {
		case l <- struct{}{}:
		default: // Ignore unresponsive listeners
		}
	}
}
```

---

## 3. WebSocket Endpoint

Handles browser connections and sends reload signals.

```go
// internal/server/server.go

const PathProxyEvents = "/__templier/events"

var bytesMsgReload = []byte("r")

func (s *Server) handleProxyEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "expecting method GET", http.StatusMethodNotAllowed)
		return
	}

	c, err := s.webSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrading to websocket", http.StatusInternalServerError)
		return
	}
	defer c.Close()

	notifyReload := make(chan struct{})
	s.reload.AddListener(notifyReload)

	ctx, cancel := context.WithCancel(r.Context())

	defer func() {
		s.reload.RemoveListener(notifyReload)
		cancel()
	}()

	// Goroutine to detect client disconnect
	go func() {
		defer cancel()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifyReload:
			if !writeWSMsg(c, bytesMsgReload) {
				return // Disconnect
			}
		}
	}
}

func writeWSMsg(c *websocket.Conn, msg []byte) (ok bool) {
	err := c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return false
	}
	err = c.WriteMessage(websocket.TextMessage, msg)
	return err == nil
}
```

**WebSocket upgrader config:**

```go
webSocketUpgrader: websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Ignore CORS
},
```

---

## 4. Injected JavaScript

The JS that gets injected into HTML responses.

```javascript
// internal/server/templates.templ

(() => {
	// Prevent double initialization
	if (window._templier__jsInjection_initialized === true) {
		return
	}

	let params = JSON.parse(
		document.getElementById('_templier__jsInjection').textContent
	)

	let reconnectingOverlay

	function showReconnecting() {
		if (reconnectingOverlay != null) {
			return
		}
		reconnectingOverlay = document.createElement('p')
		reconnectingOverlay.innerHTML = '<span>🔌 reconnecting...</span>'
		reconnectingOverlay.style.margin = 0
		reconnectingOverlay.style.display = 'flex'
		reconnectingOverlay.style.justifyContent = 'center'
		reconnectingOverlay.style.alignItems = 'center'
		reconnectingOverlay.style.position = 'fixed'
		reconnectingOverlay.style.top = 0
		reconnectingOverlay.style.left = 0
		reconnectingOverlay.style.fontSize = '1.25rem'
		reconnectingOverlay.style.width = '100%'
		reconnectingOverlay.style.height = '100%'
		reconnectingOverlay.style.background = 'rgba(0,0,0,.8)'
		reconnectingOverlay.style.color = 'white'
		document.body.appendChild(reconnectingOverlay)
	}

	function hideReconnecting() {
		if (reconnectingOverlay == null) {
			return
		}
		reconnectingOverlay.remove()
		reconnectingOverlay = null
	}

	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'

	function connectWebsocket() {
		const wsURL = `${protocol}//${window.location.host}${params.WSEventsEndpoint}`
		ws = new WebSocket(wsURL)

		ws.onopen = function (e) {
			// If reconnecting, reload immediately
			if (reconnectingOverlay != null) {
				window.location.reload()
				return
			}
			hideReconnecting()
		}

		ws.onmessage = function (e) {
			switch (e.data) {
			case 'r': // Reload
				window.location.reload()
			case 's': // Shutdown (placeholder)
				break
			}
		}

		ws.onclose = function (e) {
			showReconnecting()
			setTimeout(() => connectWebsocket(), 300)
		}
	}

	connectWebsocket()
	window._templier__jsInjection_initialized = true
})();
```

**Passing config to JS via JSON script tag:**

```go
// templates.templ uses templ.JSONScript to pass params:
@templ.JSONScript("_templier__jsInjection", struct {
	PrintDebugLogs   bool
	WSEventsEndpoint string
}{
	PrintDebugLogs:   printDebugLogs,
	WSEventsEndpoint: wsEventsEndpoint,  // "/__templier/events"
})
```

---

## 5. HTML Injection Logic

Finds `</head>` or `</body>` and injects JS before it.

```go
// internal/server/server.go

var (
	bytesHeadClosingTag          = []byte("</head>")
	bytesHeadClosingTagUppercase = []byte("</HEAD>")
	bytesBodyClosingTag          = []byte("</body>")
	bytesBodyClosingTagUppercase = []byte("</BODY>")
)

// WriteWithInjection writes body with injection at end of head or body.
func WriteWithInjection(w io.Writer, body []byte, injection []byte) error {
	if bytes.Contains(body, bytesHeadClosingTag) {
		modified := bytes.Replace(body, bytesHeadClosingTag,
			append(injection, bytesHeadClosingTag...), 1)
		_, err := w.Write(modified)
		return err
	} else if bytes.Contains(body, bytesHeadClosingTagUppercase) {
		modified := bytes.Replace(body, bytesHeadClosingTagUppercase,
			append(injection, bytesHeadClosingTagUppercase...), 1)
		_, err := w.Write(modified)
		return err
	} else if bytes.Contains(body, bytesBodyClosingTag) {
		modified := bytes.Replace(body, bytesBodyClosingTag,
			append(injection, bytesBodyClosingTag...), 1)
		_, err := w.Write(modified)
		return err
	} else if bytes.Contains(body, bytesBodyClosingTagUppercase) {
		modified := bytes.Replace(body, bytesBodyClosingTagUppercase,
			append(injection, bytesBodyClosingTagUppercase...), 1)
		_, err := w.Write(modified)
		return err
	}

	// No closing tag found, prepend injection
	if _, err := w.Write(injection); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}
```

---

## 6. Proxy Response Modification

Intercepts responses, handles gzip/brotli, injects JS into HTML.

```go
// internal/server/server.go

func (s *Server) modifyResponse(r *http.Response) error {
	// Skip modification for HTMX/Datastar requests
	if r.Header.Get("templ-skip-modify") == "true" {
		return nil
	}

	// Set up decompression based on Content-Encoding
	newReader := func(in io.Reader) (io.Reader, error) { return in, nil }
	newWriter := func(out io.Writer) io.WriteCloser { return passthroughWriteCloser{out} }

	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		newReader = func(in io.Reader) (io.Reader, error) { return gzip.NewReader(in) }
		newWriter = func(out io.Writer) io.WriteCloser { return gzip.NewWriter(out) }
	case "br":
		newReader = func(in io.Reader) (io.Reader, error) { return brotli.NewReader(in), nil }
		newWriter = func(out io.Writer) io.WriteCloser { return brotli.NewWriter(out) }
	}

	// Read and decompress body
	encr, err := newReader(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := io.ReadAll(encr)
	if err != nil {
		return err
	}

	// Re-encode with injection
	var buf bytes.Buffer
	encw := newWriter(&buf)

	if strings.HasPrefix(http.DetectContentType(body), "text/html") {
		if err = WriteWithInjection(encw, body, s.jsInjection); err != nil {
			return fmt.Errorf("injecting JS: %w", err)
		}
	} else {
		if _, err := encw.Write(body); err != nil {
			return err
		}
	}

	if err := encw.Close(); err != nil {
		return err
	}

	// Update response
	r.Body = io.NopCloser(&buf)
	r.ContentLength = int64(buf.Len())
	r.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	return nil
}
```

---

## 7. Health Check Before Reload

Polls app server before broadcasting reload to browsers.

```go
// main.go (runAppLauncher -> rerun function)

// rerunActive is checked before broadcasting templ-only reloads
var rerunActive atomic.Bool

func rerun(ctx context.Context) {
	rerunActive.Store(true)
	defer rerunActive.Store(false)

	// Stop existing server...
	// Start new server...

	// Health check loop
	const maxRetries = 100
	for retry := 0; ; retry++ {
		if ctx.Err() != nil {
			return // Canceled
		}
		if retry > maxRetries {
			log.Errorf("waiting for server: %d retries failed", maxRetries)
			return
		}

		r, err := http.NewRequest(http.MethodOptions, appHostURL, http.NoBody)
		r = r.WithContext(ctx)
		if err != nil {
			continue
		}

		resp, err := healthCheckClient.Do(r)
		if err == nil {
			resp.Body.Close()
			break // Server is ready
		}

		if errors.Is(err, context.Canceled) {
			return
		}

		time.Sleep(50 * time.Millisecond) // ServerHealthPreflightWaitInterval
	}

	// Server is healthy, now broadcast reload
	reload.BroadcastNonblock()
}
```

**Templ-only changes (no Go rebuild):**

```go
// main.go - handling templ change events

case c := <-templChange:
	switch c {
	case cmdrun.TemplChangeNeedsRestart:
		// Go code changed, need full rebuild
		onChangeHandler.recompile(ctx, fsnotify.Event{})
	case cmdrun.TemplChangeNeedsBrowserReload:
		// Only templ changed, just reload browser
		// But skip if app is currently restarting
		if !rerunActive.Load() {
			reload.BroadcastNonblock()
		}
	}
```

---

## 8. Proxy Server Setup

Reverse proxy with retry logic.

```go
// internal/server/server.go

const (
	ReverseProxyRetries         = 20
	ReverseProxyInitialDelay    = 100 * time.Millisecond
	ReverseProxyBackoffExponent = 1.5
)

func New(httpClient *http.Client, reload *SignalBroadcaster, appHostURL *url.URL) *Server {
	s := &Server{
		reload:      reload,
		jsInjection: MustRenderJSInjection(context.Background()),
		webSocketUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}

	s.reverseProxy = httputil.NewSingleHostReverseProxy(appHostURL)
	s.reverseProxy.Transport = &roundTripper{
		maxRetries:      ReverseProxyRetries,
		initialDelay:    ReverseProxyInitialDelay,
		backoffExponent: ReverseProxyBackoffExponent,
	}
	s.reverseProxy.ModifyResponse = s.modifyResponse

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle WebSocket endpoint
	if r.Method == http.MethodGet && r.URL.Path == PathProxyEvents {
		s.handleProxyEvents(w, r)
		return
	}

	// Proxy to app server
	s.reverseProxy.ServeHTTP(w, r)
}
```

**Retry round tripper:**

```go
type roundTripper struct {
	maxRetries      int
	initialDelay    time.Duration
	backoffExponent float64
}

func (rt *roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// Buffer body for retries
	var bodyBytes []byte
	if r.Body != nil && r.Body != http.NoBody {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}

	var resp *http.Response
	var err error
	for retries := range rt.maxRetries {
		req := r.Clone(r.Context())
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err = http.DefaultTransport.RoundTrip(req)
		if err != nil {
			dur := time.Duration(math.Pow(rt.backoffExponent, float64(retries)))
			time.Sleep(rt.initialDelay * dur)
			continue
		}
		return resp, nil
	}

	return nil, fmt.Errorf("max retries reached: %q", r.URL.String())
}
```

---

## Dependencies

```
github.com/gorilla/websocket  - WebSocket handling
github.com/andybalholm/brotli - Brotli decompression (for proxied responses)
compress/gzip                 - Gzip decompression (stdlib)
```

---

## Flow Summary

```
.templ file saved
       ↓
templ generate --watch detects change
       ↓
templ writes to stderr: "(✓) Post-generation event received...needsBrowserReload=true"
       ↓
RunTemplWatch parses this, sends TemplChangeNeedsBrowserReload to channel
       ↓
Main loop receives, checks !rerunActive.Load()
       ↓
reload.BroadcastNonblock() sends to all listener channels
       ↓
handleProxyEvents receives on notifyReload channel
       ↓
Writes "r" to WebSocket connection
       ↓
Browser JS receives "r", calls window.location.reload()
```
