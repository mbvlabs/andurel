package anduin

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"slices"
	"sync"
	"time"
)

// ErrResponseCommitted is returned when a handler is invoked after an error
// response has already been rendered. The router renders once; middleware that
// needs to retry or translate an error must wrap the retry scope with
// CatchErrors.
var ErrResponseCommitted = errors.New("anduin: response already committed")

// CommittedError reports that a retry was blocked because the response was
// already committed. It unwraps to the error that was rendered and matches
// ErrResponseCommitted with errors.Is.
type CommittedError struct {
	Err error
}

func (err *CommittedError) Error() string {
	if err == nil || err.Err == nil {
		return ErrResponseCommitted.Error()
	}
	return ErrResponseCommitted.Error() + ": " + err.Err.Error()
}

// Unwrap exposes the rendered error.
func (err *CommittedError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

// Is matches ErrResponseCommitted and delegates to the rendered error.
func (err *CommittedError) Is(target error) bool {
	if target == ErrResponseCommitted {
		return true
	}
	if err == nil {
		return false
	}
	return errors.Is(err.Err, target)
}

// ResponseBuffer is implemented by response writers that buffer a response
// before delivering it (for example the Inertia capture writer). Before the
// router writes an error response it walks the writer Unwrap chain and calls
// DiscardBuffered on every buffer, so a successful page captured earlier cannot
// be combined with the error page. Implementations should discard the buffered
// status and body (and headers owned by the discarded response). The router
// restores router-owned error metadata such as the ServeMux Allow header after
// the discard. A buffer that renders an error response must deliver it; the
// router trusts ResponseBuffer implementations and does not write a fallback for
// them.
type ResponseBuffer interface {
	DiscardBuffered()
}

// RenderedError reports the error the router has rendered on this request, or
// nil. A middleware that receives an error after an early host/global render can
// use it to decide whether a response-buffer implementation should commit the
// captured error page.
func RenderedError(ctx context.Context) error {
	state := stateFrom(ctx)
	if state == nil {
		return nil
	}
	return state.renderedErr
}

// CatchErrors delays rendering of errors raised inside a middleware until that
// middleware returns. It is the explicit scope for global and host middleware
// that consumes, translates or retries errors: without it, an error is rendered
// at the failing boundary before an outer middleware can see it. Route
// middleware already gets this delayed scope implicitly, which preserves
// existing route error consumers.
//
// The returned middleware propagates errors unchanged; a middleware that
// returns nil after handling an error prevents the ErrorHandler from running.
// Errors are rendered once, after the scope unwinds, with the deepest values
// recorded from the latest failing pass; a retry after an earlier render is
// blocked with ErrResponseCommitted. When scopes are nested, the inner scope
// hands attribution to the enclosing scope instead of rendering, so the outer
// owner may consume, translate or retry first. The live request context supplies
// cancellation, deadline and cause; the middleware's own resources may already
// be released when the delayed render happens.
func CatchErrors(middleware Middleware) Middleware {
	if middleware == nil {
		return nil
	}
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			state := stateFrom(ctx)
			if state == nil || state.renderError == nil {
				return middleware(next)(ctx)
			}
			if state.rendered {
				return &CommittedError{Err: state.renderedErr}
			}

			scope := state.beginScope()
			defer state.dropScope(scope)
			err := middleware(next)(ctx)
			state.endScope(ctx, scope, err)
			return err
		}
	}
}

// errorFailure records the deepest boundary that failed in one pass. call grows
// monotonically for every boundary invocation in the request, so a later pass
// replaces an earlier pass even at a different depth.
type errorFailure struct {
	call uint64
	ctx  context.Context
}

// errorScope is one delayed-render region: the implicit route scope or an
// explicit CatchErrors wrapper.
type errorScope struct {
	failure *errorFailure
}

func (state *requestState) nextCall() uint64 {
	state.calls++
	return state.calls
}

func (state *requestState) currentScope() *errorScope {
	if len(state.scopes) == 0 {
		return nil
	}
	return state.scopes[len(state.scopes)-1]
}

func (state *requestState) beginScope() *errorScope {
	scope := &errorScope{}
	state.scopes = append(state.scopes, scope)
	return scope
}

// dropScope removes a delayed scope without rendering. It is deferred by each
// scope owner so a panic cannot leave a stale scope active for an outer Recover
// middleware's error boundary.
func (state *requestState) dropScope(scope *errorScope) {
	if scope == nil {
		return
	}
	for i, v := range slices.Backward(state.scopes) {
		if v != scope {
			continue
		}
		state.scopes = state.scopes[:i]
		return
	}
}

func (state *requestState) endScope(ctx context.Context, scope *errorScope, err error) {
	state.dropScope(scope)

	if err == nil || errors.Is(err, ErrResponseCommitted) || state.rendered {
		return
	}
	if state.renderError == nil {
		return
	}
	if outer := state.currentScope(); outer != nil {
		// A nested delayed scope propagates its failing attribution to the
		// enclosing scope instead of rendering. The outer owner may consume,
		// translate or retry before the single render.
		if scope != nil && scope.failure != nil {
			state.recordFailure(scope.failure.call, scope.failure.ctx)
		}
		return
	}

	renderCtx := ctx
	cleanup := func() {}
	if scope != nil && scope.failure != nil {
		renderCtx, cleanup = mergeErrorContext(scope.failure.ctx, ctx)
	}
	defer cleanup()
	state.render(renderCtx, err)
}

func (state *requestState) recordFailure(call uint64, ctx context.Context) {
	scope := state.currentScope()
	if scope == nil {
		return
	}
	if scope.failure != nil && call < scope.failure.call {
		return
	}
	scope.failure = &errorFailure{call: call, ctx: ctx}
}

func (state *requestState) clearFailure() {
	scope := state.currentScope()
	if scope != nil {
		scope.failure = nil
	}
}

// render invokes the router ErrorHandler exactly once. The rendered flag is set
// before the handler runs so a reentrant boundary cannot render a second page.
func (state *requestState) render(ctx context.Context, err error) {
	if err == nil || state.rendered || state.rendering || state.renderError == nil {
		return
	}
	if errors.Is(err, ErrResponseCommitted) {
		return
	}

	state.rendering = true
	state.rendered = true
	state.renderedErr = err
	defer func() { state.rendering = false }()

	writer := ResponseWriter(ctx)
	if writer == nil {
		slog.Error("anduin: cannot render error without a response writer", "error", err)
		return
	}

	// ServeMux sets Allow before producing a 405. A cooperative capture may
	// reset its header snapshot in DiscardBuffered, so keep the router-owned
	// error metadata across that discard rather than losing it.
	buffered := state.discardBufferedForError(writer, err)
	before := state.deliverySnapshot()

	previous := writer
	counted := &renderWriteWriter{ResponseWriter: writer}
	ReplaceResponseWriter(ctx, counted)
	defer ReplaceResponseWriter(ctx, previous)

	handlerErr := state.renderError(ctx, err)
	if handlerErr != nil {
		slog.Error("anduin: error handler failed", "error", handlerErr, "request_error", err)
		// A cooperative capture may hold a partial error page and its status.
		// Clear it before the single plain fallback so only the fallback is
		// delivered (or committed by the capture owner).
		state.discardBufferedForError(ResponseWriter(ctx), err)
		writePlainFallback(ctx)
	}

	after := state.deliverySnapshot()
	if counted.wrote && !buffered && after == before {
		if after.bytes == 0 {
			slog.Error("anduin: error response was discarded before delivery", "error", err)
			if state.rootWriter != nil {
				http.Error(state.rootWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}
		slog.Error("anduin: error response was discarded after the response started", "error", err)
	}
}

func (state *requestState) deliverySnapshot() deliverySnapshot {
	if state.delivery == nil {
		return deliverySnapshot{}
	}
	return state.delivery.snapshot()
}

// logPostRenderError records a final error that changed or was joined after the
// response was already rendered. It never renders again and logs at most once,
// from the router entry point after the chain unwinds.
func (state *requestState) logPostRenderError(err error) {
	if err == nil || !state.rendered || state.renderedErr == nil {
		return
	}
	if errors.Is(err, ErrResponseCommitted) {
		return
	}
	if sameError(err, state.renderedErr) {
		return
	}
	slog.Error("anduin: error changed after the response was rendered",
		"error", err,
		"rendered_error", state.renderedErr,
	)
}

// sameError compares two errors only when their actual dynamic values are
// comparable. Type.Comparable alone is not enough for a struct whose interface
// field holds a slice. Non-comparable values are conservatively treated as
// changed: extra logging is preferable to a panic or a false unchanged signal.
func sameError(left, right error) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Type() != rightValue.Type() ||
		!leftValue.Comparable() || !rightValue.Comparable() {
		return false
	}
	return left == right
}

// writePlainFallback writes one plain 500 for a failed or panicking
// ErrorHandler. It never invokes the ErrorHandler again.
func writePlainFallback(ctx context.Context) {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return
	}
	http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// discardBufferedForError drops buffered content before an error render while
// preserving router-owned error metadata across a capture that resets its
// header snapshot. The 405 Allow value is captured when ServeMux generates the
// error, so it does not depend on the buffer's current header map.
func (state *requestState) discardBufferedForError(writer http.ResponseWriter, err error) bool {
	allow := ""
	if errors.Is(err, ErrMethodNotAllowed) {
		allow = state.routeAllow
		if allow == "" {
			allow = writer.Header().Get("Allow")
		}
	}
	found := discardBufferedWriters(writer)
	if allow != "" {
		writer.Header().Set("Allow", allow)
	}
	return found
}

// discardBufferedWriters walks the Unwrap chain and drops content buffered by a
// successful response before the router writes the error page into that buffer.
func discardBufferedWriters(writer http.ResponseWriter) bool {
	found := false
	// Unwrap chains are short; cap the walk so an unusual cycle cannot hang.
	for range 64 {
		if writer == nil {
			break
		}
		if buffer, ok := writer.(ResponseBuffer); ok {
			buffer.DiscardBuffered()
			found = true
		}
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		next := unwrapper.Unwrap()
		if next == writer {
			break
		}
		writer = next
	}
	return found
}

// renderWriteWriter counts writes initiated by the ErrorHandler, so a render
// that only reached an uncommitted capture can be detected.
type renderWriteWriter struct {
	http.ResponseWriter
	wrote bool
}

func (writer *renderWriteWriter) WriteHeader(status int) {
	writer.wrote = true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *renderWriteWriter) Write(body []byte) (int, error) {
	writer.wrote = true
	return writer.ResponseWriter.Write(body)
}

func (writer *renderWriteWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *renderWriteWriter) Flush() {
	_ = writer.FlushError()
}

func (writer *renderWriteWriter) FlushError() error {
	return http.NewResponseController(writer.ResponseWriter).Flush()
}

func (writer *renderWriteWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(writer.ResponseWriter).Hijack()
}

func (writer *renderWriteWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := writer.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

// mergedErrorContext embeds a self-canceling bridge over the deepest values and
// reports the live request deadline. Value comes from the bridge, while Done and
// Err use a wrapper-owned channel and frozen classification. The wrapper Done is
// deliberately different from the bridge Done: standard context derivation then
// uses public propagation instead of discovering the bridge's internal cancel
// state. A live cancellation watcher propagates only live cancellation into the
// bridge, and every cancellation path closes the wrapper Done last.
type mergedErrorContext struct {
	context.Context
	live   context.Context
	cancel *errorCancellation
}

func (ctx mergedErrorContext) Deadline() (time.Time, bool) { return ctx.live.Deadline() }

func (ctx mergedErrorContext) Done() <-chan struct{} {
	if ctx.cancel == nil {
		return ctx.Context.Done()
	}
	return ctx.cancel.done
}

// Err stays nil until the standard bridge is canceled, then waits for the
// wrapper Done and returns the classification frozen by the first cancellation
// path. Waiting closes the tiny publication window between bridge cancellation
// and the wrapper Done close, so Cause readers never see a non-nil cause with a
// nil Err.
func (ctx mergedErrorContext) Err() error {
	if ctx.cancel == nil {
		return ctx.Context.Err()
	}
	if ctx.Context.Err() == nil {
		return nil
	}
	<-ctx.cancel.done
	return ctx.cancel.err()
}

// errorCancellation serializes the first cancellation classification, cancels
// the standard bridge, and closes the wrapper Done last. Readers observe the
// frozen error only through the closed Done channel.
type errorCancellation struct {
	once   sync.Once
	cancel context.CancelCauseFunc
	done   chan struct{}
	frozen error
}

func (state *errorCancellation) finish(cause, err error) {
	state.once.Do(func() {
		state.frozen = err
		state.cancel(cause)
		close(state.done)
	})
}

func (state *errorCancellation) err() error {
	select {
	case <-state.done:
		return state.frozen
	default:
		return nil
	}
}

// mergeErrorContext keeps the middleware values from values while propagating
// only live cancellation into the returned bridge. WithoutCancel prevents a
// canceled child context from leaking its cancellation through values; the
// wrapper's own Done/Err freeze the first classification while the bridge keeps
// the standard cause. Cleanup stops the watcher and closes the wrapper context
// after the synchronous render.
func mergeErrorContext(values, live context.Context) (context.Context, func()) {
	if values == nil {
		return live, func() {}
	}
	detached := context.WithoutCancel(values)
	if live == nil {
		return detached, func() {}
	}

	bridge, cancel := context.WithCancelCause(detached)
	state := &errorCancellation{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	stop := context.AfterFunc(live, func() {
		liveErr := live.Err()
		if liveErr == nil {
			liveErr = context.Canceled
		}
		state.finish(context.Cause(live), liveErr)
	})
	if liveErr := live.Err(); liveErr != nil {
		state.finish(context.Cause(live), liveErr)
	}

	cleanup := func() {
		stop()
		if liveErr := live.Err(); liveErr != nil {
			state.finish(context.Cause(live), liveErr)
			return
		}
		state.finish(nil, context.Canceled)
	}
	return mergedErrorContext{Context: bridge, live: live, cancel: state}, cleanup
}
