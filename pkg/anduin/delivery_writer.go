package anduin

import (
	"bufio"
	"net"
	"net/http"
	"sync"
)

// deliveryTracker observes writes that reach the real response. Buffered
// response writers may accept writes without delivering them, so the count is
// distinct from bytes accepted by the current writer chain.
type deliveryTracker struct {
	mu     sync.Mutex
	writes int64
	bytes  int64
}

type deliverySnapshot struct {
	writes int64
	bytes  int64
}

func (tracker *deliveryTracker) header() {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.writes++
}

func (tracker *deliveryTracker) body(n int) {
	if n <= 0 {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.writes++
	tracker.bytes += int64(n)
}

func (tracker *deliveryTracker) snapshot() deliverySnapshot {
	if tracker == nil {
		return deliverySnapshot{}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return deliverySnapshot{writes: tracker.writes, bytes: tracker.bytes}
}

// deliveryWriter wraps the original http.ResponseWriter for the whole request.
// Middleware may replace the writer carried on the context; this outer wrapper
// still sees every byte that is actually delivered and preserves
// ResponseController flush/hijack behavior through Unwrap.
type deliveryWriter struct {
	http.ResponseWriter
	tracker *deliveryTracker
}

func (writer *deliveryWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *deliveryWriter) WriteHeader(status int) {
	writer.tracker.header()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *deliveryWriter) Write(body []byte) (int, error) {
	written, err := writer.ResponseWriter.Write(body)
	writer.tracker.body(written)
	return written, err
}

func (writer *deliveryWriter) Flush() {
	_ = http.NewResponseController(writer.ResponseWriter).Flush()
}

func (writer *deliveryWriter) FlushError() error {
	return http.NewResponseController(writer.ResponseWriter).Flush()
}

func (writer *deliveryWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(writer.ResponseWriter).Hijack()
}

func (writer *deliveryWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := writer.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

var (
	_ http.Flusher                              = (*deliveryWriter)(nil)
	_ http.Hijacker                             = (*deliveryWriter)(nil)
	_ http.Pusher                               = (*deliveryWriter)(nil)
	_ interface{ FlushError() error }           = (*deliveryWriter)(nil)
	_ interface{ Unwrap() http.ResponseWriter } = (*deliveryWriter)(nil)
)
