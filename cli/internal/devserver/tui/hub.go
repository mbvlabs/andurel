package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
)

const (
	StreamApp   = "app"
	StreamQueue = "queue"
	StreamBuild = "build"

	maxLines = 5000
)

// CoreStreams are always present in the runner TUI.
var CoreStreams = []string{StreamApp, StreamQueue, StreamBuild}

// Streams is the default tab order (core only). Prefer Hub.StreamNames() when
// tool sidecars may be enabled.
var Streams = CoreStreams

// Status is the live process indicator shown on each tab.
type Status int

const (
	StatusIdle Status = iota
	StatusStarting
	StatusReady
	StatusError
	StatusExited
)

func (s Status) String() string {
	switch s {
	case StatusStarting:
		return "starting"
	case StatusReady:
		return "ready"
	case StatusError:
		return "error"
	case StatusExited:
		return "exited"
	default:
		return "idle"
	}
}

// Line is one complete log line attributed to a named stream.
type Line struct {
	Stream string
	Text   string
}

// Hub fans child-process output into named ring buffers and optional subscribers.
type Hub struct {
	mu       sync.Mutex
	order    []string
	streams  map[string]*ring
	status   map[string]Status
	subs     []chan Line
	fallback io.Writer
	prefixes map[string]string
	writers  map[string]*streamWriter
}

func NewHub() *Hub {
	h := &Hub{
		order:   append([]string(nil), CoreStreams...),
		streams: make(map[string]*ring, len(CoreStreams)),
		status:  make(map[string]Status, len(CoreStreams)),
		prefixes: map[string]string{
			StreamApp:   "[app] ",
			StreamQueue: "[queue] ",
			StreamBuild: "[build] ",
		},
		writers: make(map[string]*streamWriter),
	}
	for _, name := range CoreStreams {
		h.streams[name] = newRing(maxLines)
		h.status[name] = StatusIdle
	}
	return h
}

// Enable registers an additional named stream (e.g. a sidecar tool tab).
// Idempotent: enabling an existing stream is a no-op.
func (h *Hub) Enable(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.streams[name]; ok {
		return
	}
	h.streams[name] = newRing(maxLines)
	h.status[name] = StatusIdle
	if _, ok := h.prefixes[name]; !ok {
		h.prefixes[name] = "[" + name + "] "
	}
	h.order = append(h.order, name)
}

// StreamNames returns the live tab order (core + enabled tools).
func (h *Hub) StreamNames() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.order))
	copy(out, h.order)
	return out
}

// SetFallback mirrors complete lines (with stream prefixes) to w. Used for --inline / non-TTY.
func (h *Hub) SetFallback(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fallback = w
}

func (h *Hub) Writer(stream string) io.Writer {
	h.mu.Lock()
	defer h.mu.Unlock()
	if w, ok := h.writers[stream]; ok {
		return w
	}
	w := &streamWriter{hub: h, stream: stream}
	h.writers[stream] = w
	return w
}

func (h *Hub) PrefixWriter(stream, prefix string) io.Writer {
	return &streamWriter{hub: h, stream: stream, linePrefix: prefix}
}

func (h *Hub) SetStatus(stream string, status Status) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status[stream] = status
}

func (h *Hub) Status(stream string) Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status[stream]
}

func (h *Hub) Lines(stream string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.streams[stream]
	if r == nil {
		return nil
	}
	return r.snapshot()
}

// Subscribe receives new lines. Dropped when the buffer is full so producers never block.
func (h *Hub) Subscribe() (<-chan Line, func()) {
	ch := make(chan Line, 256)
	h.mu.Lock()
	h.subs = append(h.subs, ch)
	h.mu.Unlock()

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			for i, existing := range h.subs {
				if existing == ch {
					h.subs = append(h.subs[:i], h.subs[i+1:]...)
					break
				}
			}
			close(ch)
		})
	}
	return ch, unsub
}

func (h *Hub) Dump(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, name := range h.order {
		r := h.streams[name]
		if r == nil || r.len() == 0 {
			continue
		}
		prefix := h.prefixes[name]
		fmt.Fprintf(w, "--- %s ---\n", name)
		for _, line := range r.snapshot() {
			fmt.Fprintf(w, "%s%s\n", prefix, line)
		}
	}
}

func (h *Hub) appendLine(stream, text string) {
	h.mu.Lock()
	r := h.streams[stream]
	if r == nil {
		r = newRing(maxLines)
		h.streams[stream] = r
		if _, ok := h.prefixes[stream]; !ok {
			h.prefixes[stream] = "[" + stream + "] "
		}
		h.order = append(h.order, stream)
	}
	r.add(text)
	line := Line{Stream: stream, Text: text}
	subs := append([]chan Line(nil), h.subs...)
	fallback := h.fallback
	prefix := h.prefixes[stream]
	h.mu.Unlock()

	if fallback != nil {
		fmt.Fprintf(fallback, "%s%s\n", prefix, text)
	}
	for _, sub := range subs {
		select {
		case sub <- line:
		default:
		}
	}
}

type streamWriter struct {
	hub        *Hub
	stream     string
	linePrefix string
	mu         sync.Mutex
	buf        bytes.Buffer
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(p)
	w.buf.Write(p)
	for {
		data := w.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := string(bytes.TrimRight(data[:idx], "\r"))
		copy(data, data[idx+1:])
		w.buf.Truncate(w.buf.Len() - idx - 1)
		if w.linePrefix != "" {
			line = w.linePrefix + line
		}
		w.hub.appendLine(w.stream, line)
	}
	return n, nil
}

type ring struct {
	lines []string
	start int
	count int
	cap   int
}

func newRing(capacity int) *ring {
	return &ring{
		lines: make([]string, capacity),
		cap:   capacity,
	}
}

func (r *ring) add(line string) {
	if r.count < r.cap {
		r.lines[(r.start+r.count)%r.cap] = line
		r.count++
		return
	}
	r.lines[r.start] = line
	r.start = (r.start + 1) % r.cap
}

func (r *ring) len() int {
	return r.count
}

func (r *ring) snapshot() []string {
	out := make([]string, r.count)
	for i := 0; i < r.count; i++ {
		out[i] = r.lines[(r.start+i)%r.cap]
	}
	return out
}

func JoinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
