package anduin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRouteScopeTranslationUsesDeepestValues pins that a route middleware can
// replace the error while the delayed render still uses the deepest context of
// the failing pass.
func TestRouteScopeTranslationUsesDeepestValues(t *testing.T) {
	t.Parallel()

	translated := errors.New("translated")
	var (
		observedMarker string
		observedErr    error
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		observedMarker, _ = ctx.Value(errorCtxKey{}).(string)
		observedErr = err
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Get("/", func(context.Context) error {
		return Forbidden(errErrorCtxGuard)
	}, func(next Handler) Handler {
		return func(ctx context.Context) error {
			_ = next(context.WithValue(ctx, errorCtxKey{}, "R"))
			return BadRequest(translated)
		}
	})

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if observedMarker != "R" {
		t.Fatalf("ErrorHandler marker = %q, want R", observedMarker)
	}
	if !errors.Is(observedErr, translated) {
		t.Fatalf("ErrorHandler err = %v, want the translated error", observedErr)
	}
	if writer.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", writer.status)
	}
}

// TestRecoveredPanicRendersAtGlobalBoundary pins panic unwinding: a route or
// CatchErrors scope must not stay active when an outer Recover middleware turns
// a panic into an error. Otherwise the error would miss the early boundary and
// its live global context.
func TestRecoveredPanicRendersAtGlobalBoundary(t *testing.T) {
	captureDefaultLogger(t)

	cases := []struct {
		name  string
		setup func(rt *Router)
	}{
		{
			name: "route panic",
			setup: func(rt *Router) {
				rt.Get("/", func(context.Context) error {
					panic("route panic")
				})
			},
		},
		{
			name: "catch scope panic",
			setup: func(rt *Router) {
				rt.Use(CatchErrors(func(next Handler) Handler {
					return func(context.Context) error {
						panic("scope panic")
					}
				}))
				rt.Get("/", textHandler("home"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				renders int
				marker  string
			)
			rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
				renders++
				marker, _ = ctx.Value(errorCtxKey{}).(string)
				return Text(ctx, StatusCode(err), "themed")
			}))
			rt.Use(appendErrorCtx("G"), Recover())
			tc.setup(rt)

			writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
			if renders != 1 {
				t.Fatalf("ErrorHandler renders = %d, want 1", renders)
			}
			if marker != "G" {
				t.Fatalf("ErrorHandler marker = %q, want G from the live global boundary", marker)
			}
			if writer.status != http.StatusInternalServerError || writer.body.String() != "themed" {
				t.Fatalf("response = %d %q, want 500 themed", writer.status, writer.body.String())
			}
		})
	}
}

// TestCatchErrorsConsumesHostError pins the explicit delayed scope used by
// existing global/host consumers. Without CatchErrors an inner guard renders
// early; with it the consumer returns nil and owns the response.
func TestCatchErrorsConsumesHostError(t *testing.T) {
	t.Parallel()

	record := &errorRenderRecord{}
	rt := newErrorContextRouter(t, record)
	rt.Use(CatchErrors(func(next Handler) Handler {
		return func(ctx context.Context) error {
			if err := next(ctx); err != nil {
				return Text(ctx, http.StatusTeapot, "recovered by scope")
			}
			return nil
		}
	}))
	rt.Use(appendErrorCtx("G"), guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if record.calls != 0 {
		t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed host error", record.calls)
	}
	if writer.status != http.StatusTeapot || writer.body.String() != "recovered by scope" {
		t.Fatalf("response = %d %q, want 418 recovered by scope", writer.status, writer.body.String())
	}
}

// TestRouteMiddlewareConsumesErrorWithoutErrorHandler pins the existing route
// consumer contract (the shape used by APIDocumentationErrors): route middleware
// consumes the handler error and no router render happens.
func TestRouteMiddlewareConsumesErrorWithoutErrorHandler(t *testing.T) {
	t.Parallel()

	record := &errorRenderRecord{}
	rt := newErrorContextRouter(t, record)
	rt.Get("/", func(context.Context) error {
		return Forbidden(errErrorCtxGuard)
	}, func(next Handler) Handler {
		return func(ctx context.Context) error {
			if err := next(ctx); err != nil {
				return JSONBlob(ctx, http.StatusOK, []byte(`{"handled":true}`))
			}
			return nil
		}
	})

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if record.calls != 0 {
		t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed route error", record.calls)
	}
	if writer.status != http.StatusOK || writer.body.String() != `{"handled":true}` {
		t.Fatalf("response = %d %q, want the route middleware JSON response", writer.status, writer.body.String())
	}
}

// TestCatchErrorsAllowsHostRetry pins that an explicit delayed scope lets a
// host/global middleware retry and render the latest failing pass.
func TestCatchErrorsAllowsHostRetry(t *testing.T) {
	t.Parallel()

	type passKey struct{}
	var (
		observedMarker string
		renders        int
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		observedMarker, _ = ctx.Value(passKey{}).(string)
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Use(CatchErrors(func(next Handler) Handler {
		return func(ctx context.Context) error {
			_ = next(context.WithValue(ctx, passKey{}, "A"))
			return next(context.WithValue(ctx, passKey{}, "B"))
		}
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			pass, _ := ctx.Value(passKey{}).(string)
			return Forbidden(fmt.Errorf("guard %s: %w", pass, errErrorCtxGuard))
		}
	})
	rt.Get("/", textHandler("unused"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 || observedMarker != "B" {
		t.Fatalf("ErrorHandler renders=%d marker=%q; want one render for pass B", renders, observedMarker)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
}

// TestDelayedScopeMergesLiveCancellation pins the delayed-scope context merge:
// values come from the deepest failing pass, while Err/Cause/Deadline come from
// the live request context, not the middleware's canceled child.
func TestDelayedScopeMergesLiveCancellation(t *testing.T) {
	t.Parallel()

	t.Run("deadline and values stay live", func(t *testing.T) {
		type deadlineKey struct{}
		var (
			observedDeadline time.Time
			observedErr      error
			observedMarker   string
		)

		rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
			observedErr = ctx.Err()
			observedMarker, _ = ctx.Value(deadlineKey{}).(string)
			observedDeadline, _ = ctx.Deadline()
			return Text(ctx, StatusCode(err), "themed")
		}))
		rt.Get("/", func(context.Context) error {
			return Forbidden(errErrorCtxGuard)
		}, func(next Handler) Handler {
			return func(ctx context.Context) error {
				child, cancel := context.WithCancel(ctx)
				defer cancel()
				child = context.WithValue(child, deadlineKey{}, "R")
				return next(child)
			}
		})

		root, cancelRoot := context.WithTimeout(context.Background(), time.Minute)
		defer cancelRoot()
		wantDeadline, _ := root.Deadline()

		request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(root)
		writer := newErrorCountingWriter()
		rt.Handler().ServeHTTP(writer, request)

		if observedErr != nil {
			t.Fatalf("ErrorHandler ctx.Err = %v, want live nil", observedErr)
		}
		if observedMarker != "R" {
			t.Fatalf("ErrorHandler marker = %q, want R", observedMarker)
		}
		if !observedDeadline.Equal(wantDeadline) {
			t.Fatalf("ErrorHandler deadline = %v, want live deadline %v", observedDeadline, wantDeadline)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})

	t.Run("root cancellation keeps cause", func(t *testing.T) {
		cause := errors.New("request aborted")
		var (
			observedErr   error
			observedCause error
		)
		rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
			observedErr = ctx.Err()
			observedCause = context.Cause(ctx)
			return Text(ctx, StatusCode(err), "themed")
		}))

		root, cancelRoot := context.WithCancelCause(context.Background())
		rt.Get("/", func(context.Context) error {
			cancelRoot(cause)
			return Forbidden(errErrorCtxGuard)
		})

		request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(root)
		writer := newErrorCountingWriter()
		rt.Handler().ServeHTTP(writer, request)

		if !errors.Is(observedErr, context.Canceled) {
			t.Fatalf("ErrorHandler ctx.Err = %v, want context.Canceled", observedErr)
		}
		if !errors.Is(observedCause, cause) {
			t.Fatalf("ErrorHandler context.Cause = %v, want %v", observedCause, cause)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})
}

// errorBufferWriter models the coordinated Inertia capture writer. It implements
// ResponseBuffer so the router can drop a successful buffered page before an
// error render, and it only delivers when its owner commits.
type errorBufferWriter struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	discarded   int
	committed   bool
	discardHook func()
}

// bufferedChainWriter proves the router walks Unwrap rather than only checking
// the current writer for ResponseBuffer.
type bufferedChainWriter struct {
	http.ResponseWriter
}

func (writer *bufferedChainWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *errorBufferWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *errorBufferWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.body.Write(body)
}

func (writer *errorBufferWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *errorBufferWriter) DiscardBuffered() {
	writer.discarded++
	writer.body.Reset()
	writer.status = 0
	if writer.discardHook != nil {
		writer.discardHook()
	}
}

func (writer *errorBufferWriter) commit() error {
	writer.committed = true
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.ResponseWriter.WriteHeader(writer.status)
	if writer.body.Len() == 0 {
		return nil
	}
	_, err := writer.ResponseWriter.Write(writer.body.Bytes())
	return err
}

// TestResponseBufferDiscardsBufferedSuccessBeforeError pins the coordinated
// capture contract: a buffered success is dropped before the error page is
// written, and a cooperative buffer commits the rendered error once.
func TestResponseBufferDiscardsBufferedSuccessBeforeError(t *testing.T) {
	t.Parallel()

	var (
		buffer  *errorBufferWriter
		renders int
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		return Text(ctx, StatusCode(err), "error page")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			original := ResponseWriter(ctx)
			buffer = &errorBufferWriter{ResponseWriter: original}
			ctx = ReplaceResponseWriter(ctx, &bufferedChainWriter{ResponseWriter: buffer})
			err := next(ctx)
			ReplaceResponseWriter(ctx, original)
			if err != nil {
				if RenderedError(ctx) != nil {
					return buffer.commit()
				}
				return err
			}
			return buffer.commit()
		}
	})
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			if err := Text(ctx, http.StatusOK, "stale success"); err != nil {
				return err
			}
			return Forbidden(errErrorCtxGuard)
		}
	})

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want 1", renders)
	}
	if buffer == nil || buffer.discarded != 1 {
		t.Fatalf("buffer discard count = %v, want 1", buffer)
	}
	if got := buffer.body.String(); got != "error page" {
		t.Fatalf("buffered body = %q, want only the error page", got)
	}
	if !buffer.committed {
		t.Fatal("buffer was not committed")
	}
	if writer.status != http.StatusForbidden || writer.body.String() != "error page" {
		t.Fatalf("response = %d %q, want 403 error page", writer.status, writer.body.String())
	}
}

// legacyCaptureWriter models the published Inertia middleware before the
// coordinated ResponseBuffer interface: it captures writes and drops them when
// the inner handler returns an error.
type legacyCaptureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (writer *legacyCaptureWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *legacyCaptureWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.body.Write(body)
}

func (writer *legacyCaptureWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

// TestLegacyCaptureDiscardFallsBackOnce pins mixed new-anduin/old-capture
// behavior: the router must not invoke the ErrorHandler twice. When the render
// was buffered but never delivered, it logs and writes one safe plain 500 to the
// real response.
func TestLegacyCaptureDiscardFallsBackOnce(t *testing.T) {
	capture := captureDefaultLogger(t)

	var (
		legacy  *legacyCaptureWriter
		renders int
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		return Text(ctx, StatusCode(err), "rendered into legacy capture")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			original := ResponseWriter(ctx)
			legacy = &legacyCaptureWriter{ResponseWriter: original}
			ctx = ReplaceResponseWriter(ctx, legacy)
			err := next(ctx)
			ReplaceResponseWriter(ctx, original)
			// Published inertia returns the error without committing capture.
			return err
		}
	})
	rt.Use(guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want exactly 1 (never double-render)", renders)
	}
	if legacy == nil || !strings.Contains(legacy.body.String(), "rendered into legacy capture") {
		t.Fatalf("legacy capture body = %v, want the discarded render", legacy)
	}
	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want safe plain 500", writer.status)
	}
	if got := writer.body.String(); got != "Internal Server Error\n" {
		t.Fatalf("body = %q, want one plain fallback", got)
	}
	if records := capture.recordsFor("anduin: error response was discarded before delivery"); len(records) != 1 {
		t.Fatalf("discard log records = %d, want 1", len(records))
	}
}

// TestJoinedErrorAfterRenderIsLoggedOnce pins that a middleware which joins a
// later failure (for example a cookie persist error) after an early render does
// not trigger a second response; the final changed error is logged once.
func TestJoinedErrorAfterRenderIsLoggedOnce(t *testing.T) {
	capture := captureDefaultLogger(t)

	var renders int
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			err := next(ctx)
			if err == nil {
				return nil
			}
			return errors.Join(err, errors.New("persist failed"))
		}
	})
	rt.Use(guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want 1", renders)
	}
	if writer.status != http.StatusForbidden || writer.body.String() != "themed" {
		t.Fatalf("response = %d %q, want the first rendered 403", writer.status, writer.body.String())
	}
	logged := capture.recordsFor("anduin: error changed after the response was rendered")
	if len(logged) != 1 {
		t.Fatalf("post-render changed-error logs = %d, want 1", len(logged))
	}
	if got := attribute(logged[0], "error"); !strings.Contains(got, "persist failed") {
		t.Fatalf("logged final error = %q, want the joined persist error", got)
	}
}

// TestErrorHandlerPanicFallsBackOnce pins the renderer failure path: one panic
// is recovered, logged, and replaced by a single plain response.
func TestErrorHandlerPanicFallsBackOnce(t *testing.T) {
	t.Parallel()

	var renders int
	rt := NewRouter(WithErrorHandler(func(context.Context, error) error {
		renders++
		panic("renderer exploded")
	}))
	rt.Use(guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want 1", renders)
	}
	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", writer.status)
	}
	if got := writer.body.String(); got != "Internal Server Error\n" {
		t.Fatalf("body = %q, want one plain fallback", got)
	}
	if writer.statusWrites != 1 || writer.bodyWrites != 1 {
		t.Fatalf("writes = status %d body %d, want 1/1", writer.statusWrites, writer.bodyWrites)
	}
}

// TestCommittedErrorUnwrapsOriginal pins the committed sentinel API used by
// retries: it matches ErrResponseCommitted and still exposes the error that was
// rendered through errors.Is/errors.As.
func TestCommittedErrorUnwrapsOriginal(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("rendered sentinel")
	original := Forbidden(sentinel)
	err := &CommittedError{Err: original}

	if !errors.Is(err, ErrResponseCommitted) {
		t.Fatalf("errors.Is(%v, ErrResponseCommitted) = false, want true", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(%v, sentinel) = false, want true", err)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusForbidden {
		t.Fatalf("errors.As(%v, *HTTPError) = %v, want 403", err, httpErr)
	}
}

// TestResponseWriterSupportsFlushAndHijackThroughRouter pins that the delivery
// tracker keeps ResponseController flush/hijack working through the writer
// chain rather than hiding the underlying connection capabilities.
func TestResponseWriterSupportsFlushAndHijackThroughRouter(t *testing.T) {
	t.Parallel()

	rt := NewRouter()
	rt.Get("/flush", func(ctx context.Context) error {
		if err := http.NewResponseController(ResponseWriter(ctx)).Flush(); err != nil {
			return err
		}
		return Text(ctx, http.StatusOK, "flushed")
	})
	rt.Get("/hijack", func(ctx context.Context) error {
		conn, buffered, err := http.NewResponseController(ResponseWriter(ctx)).Hijack()
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = fmt.Fprint(buffered, "HTTP/1.1 200 OK\r\nContent-Length: 8\r\nConnection: close\r\n\r\nhijacked")
		if err != nil {
			return err
		}
		return buffered.Flush()
	})

	server := httptest.NewServer(rt.Handler())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/flush")
	if err != nil {
		t.Fatalf("flush request: %v", err)
	}
	body, _ := readAllString(response)
	if response.StatusCode != http.StatusOK || body != "flushed" {
		t.Fatalf("flush response = %d %q, want 200 flushed", response.StatusCode, body)
	}

	response, err = http.Get(server.URL + "/hijack")
	if err != nil {
		t.Fatalf("hijack request: %v", err)
	}
	body, _ = readAllString(response)
	if response.StatusCode != http.StatusOK || body != "hijacked" {
		t.Fatalf("hijack response = %d %q, want 200 hijacked", response.StatusCode, body)
	}
}

// TestNestedCatchErrorsDefersToEnclosingScope pins nested delayed scopes: an
// inner scope must not render while an outer CatchErrors scope is active. The
// outer scope gets the failing attribution and may consume, translate or retry.
func TestNestedCatchErrorsDefersToEnclosingScope(t *testing.T) {
	t.Parallel()

	type passKey struct{}

	t.Run("inner observer consumes", func(t *testing.T) {
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		var observed error

		rt.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				if err != nil {
					return Text(ctx, http.StatusBadRequest, "global should not handle this")
				}
				return nil
			}
		}))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				observed = err
				if err != nil {
					return Text(ctx, http.StatusConflict, "host observer consumed")
				}
				return nil
			}
		}), guardErrorCtx)
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if !errors.Is(observed, errErrorCtxGuard) {
			t.Fatalf("host observer error = %v, want the guard error", observed)
		}
		if record.calls != 0 {
			t.Fatalf("ErrorHandler calls = %d, want 0 before an outer scope consumes", record.calls)
		}
		if writer.status != http.StatusConflict || writer.body.String() != "host observer consumed" {
			t.Fatalf("response = %d %q, want the host observer response", writer.status, writer.body.String())
		}
	})

	t.Run("inner translation reaches outer consumer", func(t *testing.T) {
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		translated := errors.New("translated by host")
		var observed error

		rt.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				observed = err
				if err != nil {
					return Text(ctx, http.StatusUnprocessableEntity, "outer handled translation")
				}
				return nil
			}
		}))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				if err := next(ctx); err != nil {
					return BadRequest(translated)
				}
				return nil
			}
		}), guardErrorCtx)
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if !errors.Is(observed, translated) {
			t.Fatalf("outer consumer error = %v, want translated error", observed)
		}
		if record.calls != 0 {
			t.Fatalf("ErrorHandler calls = %d, want 0 before an outer scope handles", record.calls)
		}
		if writer.status != http.StatusUnprocessableEntity || writer.body.String() != "outer handled translation" {
			t.Fatalf("response = %d %q, want outer consumer response", writer.status, writer.body.String())
		}
	})

	t.Run("inner retry keeps latest attribution before outer render", func(t *testing.T) {
		var (
			events         []string
			observedMarker string
			observedErr    error
			renders        int
		)
		rt := NewRouter(
			WithHosts(newAdminHosts(t)),
			WithErrorHandler(func(ctx context.Context, err error) error {
				renders++
				events = append(events, "render")
				observedMarker, _ = ctx.Value(passKey{}).(string)
				observedErr = err
				return Text(ctx, StatusCode(err), "themed")
			}),
		)
		rt.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				if err != nil {
					events = append(events, "outer:see")
				}
				return err
			}
		}))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				_ = next(context.WithValue(ctx, passKey{}, "A"))
				return next(context.WithValue(ctx, passKey{}, "B"))
			}
		}), func(next Handler) Handler {
			return func(ctx context.Context) error {
				pass, _ := ctx.Value(passKey{}).(string)
				return Forbidden(fmt.Errorf("guard %s: %w", pass, errErrorCtxGuard))
			}
		})
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if renders != 1 || observedMarker != "B" || !strings.Contains(observedErr.Error(), "guard B") {
			t.Fatalf(
				"ErrorHandler renders=%d marker=%q err=%v; want one outer render for the latest pass B",
				renders,
				observedMarker,
				observedErr,
			)
		}
		if got, want := strings.Join(events, ","), "outer:see,render"; got != want {
			t.Fatalf("events = %q, want %q (inner scope must not render)", got, want)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})

	t.Run("outer retry uses latest host pass", func(t *testing.T) {
		var (
			observedMarker string
			observedErr    error
			renders        int
			hostSaw        []string
		)
		rt := NewRouter(
			WithHosts(newAdminHosts(t)),
			WithErrorHandler(func(ctx context.Context, err error) error {
				renders++
				observedMarker, _ = ctx.Value(passKey{}).(string)
				observedErr = err
				return Text(ctx, StatusCode(err), "themed")
			}),
		)
		rt.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				_ = next(context.WithValue(ctx, passKey{}, "A"))
				return next(context.WithValue(ctx, passKey{}, "B"))
			}
		}))
		admin := rt.Host(hostAdmin)
		admin.Isolate()
		admin.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				pass, _ := ctx.Value(passKey{}).(string)
				err := next(ctx)
				hostSaw = append(hostSaw, pass)
				return err
			}
		}), func(next Handler) Handler {
			return func(ctx context.Context) error {
				pass, _ := ctx.Value(passKey{}).(string)
				return Forbidden(fmt.Errorf("guard %s: %w", pass, errErrorCtxGuard))
			}
		})
		admin.Get("/admin", textHandler("admin"))

		writer := serveErrorRequest(rt, http.MethodGet, "admin.andurel.com", "/admin")
		if got, want := strings.Join(hostSaw, ","), "A,B"; got != want {
			t.Fatalf("host observer passes = %q, want %q", got, want)
		}
		if renders != 1 || observedMarker != "B" || !strings.Contains(observedErr.Error(), "guard B") {
			t.Fatalf(
				"ErrorHandler renders=%d marker=%q err=%v; want one render for outer-retry pass B",
				renders,
				observedMarker,
				observedErr,
			)
		}
		if writer.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", writer.status)
		}
	})

	t.Run("route error reaches enclosing scope", func(t *testing.T) {
		record := &errorRenderRecord{}
		rt := newErrorContextRouter(t, record)
		var observed error

		rt.Use(CatchErrors(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				observed = err
				if err != nil {
					return Text(ctx, http.StatusBadRequest, "route consumed by outer scope")
				}
				return nil
			}
		}))
		rt.Get("/", func(context.Context) error {
			return Forbidden(errErrorCtxGuard)
		})

		writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
		if !errors.Is(observed, errErrorCtxGuard) {
			t.Fatalf("outer scope error = %v, want route error", observed)
		}
		if record.calls != 0 {
			t.Fatalf("ErrorHandler calls = %d, want 0 for a consumed route error", record.calls)
		}
		if writer.status != http.StatusBadRequest || writer.body.String() != "route consumed by outer scope" {
			t.Fatalf("response = %d %q, want outer scope response", writer.status, writer.body.String())
		}
	})
}

// nonComparableError has a dynamic type that cannot be compared with ==. The
// post-render logging path must not panic while deciding whether the final
// error is the rendered one or a wrapper/join.
type nonComparableError []string

func (err nonComparableError) Error() string { return strings.Join(err, ":") }

// interfacePayloadError has a comparable struct type but a non-comparable value
// when Payload holds a slice. Type.Comparable alone is not enough to compare it.
type interfacePayloadError struct {
	Payload any
}

func (err interfacePayloadError) Error() string { return fmt.Sprintf("payload: %v", err.Payload) }

type sameMessageErrorA string

func (err sameMessageErrorA) Error() string { return string(err) }

type sameMessageErrorB string

func (err sameMessageErrorB) Error() string { return string(err) }

// runPostRenderComparison installs a guard that returns rendered and an outer
// middleware that returns final(err), then reports the request's rendered state,
// status and changed-error logs.
func runPostRenderComparison(
	t *testing.T,
	rendered error,
	final func(error) error,
) (*errorCountingWriter, int, *captureHandler) {
	t.Helper()

	capture := captureDefaultLogger(t)
	renders := 0
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		return Text(ctx, StatusCode(err), "themed")
	}))
	if final != nil {
		rt.Use(func(next Handler) Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				if err == nil {
					return nil
				}
				return final(err)
			}
		})
	}
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			return rendered
		}
	})
	rt.Get("/", textHandler("home"))

	return serveErrorRequest(rt, http.MethodGet, "andurel.com", "/"), renders, capture
}

func TestPostRenderLoggingErrorComparison(t *testing.T) {
	changedMessage := "anduin: error changed after the response was rendered"

	t.Run("same comparable sentinel stays quiet", func(t *testing.T) {
		sentinel := errors.New("same comparable")
		writer, renders, capture := runPostRenderComparison(t, sentinel, nil)
		if renders != 1 || writer.status != http.StatusInternalServerError {
			t.Fatalf("renders=%d status=%d, want one 500 render", renders, writer.status)
		}
		if got := len(capture.recordsFor(changedMessage)); got != 0 {
			t.Fatalf("changed-error logs = %d, want 0 for the same comparable error", got)
		}
	})

	t.Run("named slice is conservatively logged", func(t *testing.T) {
		rendered := nonComparableError{"guard", "boom"}
		writer, renders, capture := runPostRenderComparison(t, rendered, nil)
		if renders != 1 || writer.status != http.StatusInternalServerError {
			t.Fatalf("renders=%d status=%d, want one 500 render", renders, writer.status)
		}
		if got := len(capture.recordsFor(changedMessage)); got != 1 {
			t.Fatalf("changed-error logs = %d, want 1 for an uncomparable named slice", got)
		}
	})

	t.Run("comparable struct with interface slice payload is conservatively logged", func(t *testing.T) {
		rendered := interfacePayloadError{Payload: []string{"guard", "boom"}}
		writer, renders, capture := runPostRenderComparison(t, rendered, nil)
		if renders != 1 || writer.status != http.StatusInternalServerError {
			t.Fatalf("renders=%d status=%d, want one 500 render", renders, writer.status)
		}
		if got := len(capture.recordsFor(changedMessage)); got != 1 {
			t.Fatalf("changed-error logs = %d, want 1 for a comparable type with slice payload", got)
		}
	})

	t.Run("distinct comparable types with same message log changed", func(t *testing.T) {
		rendered := sameMessageErrorA("same message")
		final := func(error) error { return sameMessageErrorB("same message") }
		writer, renders, capture := runPostRenderComparison(t, rendered, final)
		if renders != 1 || writer.status != http.StatusInternalServerError {
			t.Fatalf("renders=%d status=%d, want one 500 render", renders, writer.status)
		}
		if got := len(capture.recordsFor(changedMessage)); got != 1 {
			t.Fatalf("changed-error logs = %d, want 1 for a different comparable type", got)
		}
	})

	t.Run("wrapper with same message logs changed", func(t *testing.T) {
		rendered := errors.New("same wrapper message")
		final := func(error) error { return fmt.Errorf("same wrapper message") }
		writer, renders, capture := runPostRenderComparison(t, rendered, final)
		if renders != 1 || writer.status != http.StatusInternalServerError {
			t.Fatalf("renders=%d status=%d, want one 500 render", renders, writer.status)
		}
		if got := len(capture.recordsFor(changedMessage)); got != 1 {
			t.Fatalf("changed-error logs = %d, want 1 for a same-message wrapper", got)
		}
	})
}

// TestMergeErrorContextPreservesLiveSemantics pins the delayed render context:
// deepest values (including nil masks), live Deadline, own coherent Done/Err/
// Cause, custom deadline causes and no deadline-timer race or child-cancel leak.
func TestMergeErrorContextPreservesLiveSemantics(t *testing.T) {
	type valueKey struct{}

	values, cancelValues := context.WithCancel(context.Background())
	cancelValues()
	values = context.WithValue(values, valueKey{}, "deep")

	t.Run("expired custom deadline cause", func(t *testing.T) {
		cause := errors.New("custom deadline cause")
		live, cancelLive := context.WithDeadlineCause(
			context.Background(),
			time.Now().Add(-time.Millisecond),
			cause,
		)
		defer cancelLive()

		merged, cleanup := mergeErrorContext(values, live)
		defer cleanup()

		if !errors.Is(merged.Err(), context.DeadlineExceeded) {
			t.Fatalf("merged Err = %v, want context.DeadlineExceeded", merged.Err())
		}
		if got := context.Cause(merged); !errors.Is(got, cause) {
			t.Fatalf("merged Cause = %v, want custom deadline cause", got)
		}
		wantDeadline, _ := live.Deadline()
		gotDeadline, ok := merged.Deadline()
		if !ok || !gotDeadline.Equal(wantDeadline) {
			t.Fatalf("merged Deadline = %v/%t, want %v", gotDeadline, ok, wantDeadline)
		}
		if got := merged.Value(valueKey{}); got != "deep" {
			t.Fatalf("merged value = %v, want deep", got)
		}
	})

	t.Run("firing custom deadline cause", func(t *testing.T) {
		for i := range 100 {
			cause := fmt.Errorf("custom deadline %d", i)
			live, cancelLive := context.WithDeadlineCause(
				context.Background(),
				time.Now().Add(time.Millisecond),
				cause,
			)
			merged, cleanup := mergeErrorContext(values, live)
			<-merged.Done()
			if !errors.Is(merged.Err(), context.DeadlineExceeded) {
				cancelLive()
				cleanup()
				t.Fatalf("iteration %d merged Err = %v, want context.DeadlineExceeded", i, merged.Err())
			}
			if got := context.Cause(merged); !errors.Is(got, cause) {
				cancelLive()
				cleanup()
				t.Fatalf("iteration %d merged Cause = %v, want custom deadline cause", i, got)
			}
			cancelLive()
			cleanup()
		}
	})

	t.Run("cancelled deep child does not leak", func(t *testing.T) {
		live, cancelLive := context.WithCancel(context.Background())
		defer cancelLive()

		merged, cleanup := mergeErrorContext(values, live)
		defer cleanup()

		select {
		case <-merged.Done():
			t.Fatal("merged context already canceled by the deep child")
		default:
		}
		if merged.Err() != nil {
			t.Fatalf("merged Err = %v, want nil while live is active", merged.Err())
		}
		if cause := context.Cause(merged); cause != nil {
			t.Fatalf("merged Cause = %v, want nil while live is active", cause)
		}
		if got := merged.Value(valueKey{}); got != "deep" {
			t.Fatalf("merged value = %v, want deep", got)
		}

		cancelLive()
		<-merged.Done()
	})

	t.Run("nil value mask survives cancellation", func(t *testing.T) {
		live := context.WithValue(context.Background(), valueKey{}, "live")
		live, cancelLive := context.WithCancel(live)
		defer cancelLive()
		deep := context.WithValue(live, valueKey{}, nil)

		merged, cleanup := mergeErrorContext(deep, live)
		defer cleanup()

		if got := merged.Value(valueKey{}); got != nil {
			t.Fatalf("merged value = %v before cancel, want nil mask", got)
		}
		cancelLive()
		<-merged.Done()
		if got := merged.Value(valueKey{}); got != nil {
			t.Fatalf("merged value = %v after cancel, want stable nil mask", got)
		}
	})

	t.Run("concurrent cause reads stay coherent", func(t *testing.T) {
		cause := errors.New("concurrent cancel cause")
		live, cancelLive := context.WithCancelCause(context.Background())
		merged, cleanup := mergeErrorContext(values, live)
		defer cleanup()

		var wait sync.WaitGroup
		stop := make(chan struct{})
		for range 8 {
			wait.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
					}
					// Once Cause is non-nil the context is permanently
					// canceled. A separate Err read may observe the transition,
					// so check the permanent direction only.
					if cause := context.Cause(merged); cause != nil && merged.Err() == nil {
						t.Errorf("merged Cause = %v but Err is nil", cause)
						return
					}
					if err := merged.Err(); err != nil && context.Cause(merged) == nil {
						t.Errorf("merged Err = %v but Cause is nil", err)
						return
					}
				}
			})
		}
		cancelLive(cause)
		<-merged.Done()
		close(stop)
		wait.Wait()

		if got := context.Cause(merged); !errors.Is(got, cause) {
			t.Fatalf("merged Cause = %v, want root cause", got)
		}
	})

	t.Run("cleanup versus live cancellation race is stable", func(t *testing.T) {
		for i := range 50 {
			cause := fmt.Errorf("raced cancel %d", i)
			live, cancelLive := context.WithCancelCause(context.Background())
			merged, cleanup := mergeErrorContext(values, live)
			cleanupDone := make(chan struct{})
			go func() {
				cleanup()
				close(cleanupDone)
			}()
			cancelLive(cause)
			<-merged.Done()
			<-cleanupDone

			firstErr := merged.Err()
			firstCause := context.Cause(merged)
			if !errors.Is(firstErr, context.Canceled) {
				cancelLive(nil)
				t.Fatalf("iteration %d Err = %v, want Canceled", i, firstErr)
			}
			if !errors.Is(firstCause, context.Canceled) && !errors.Is(firstCause, cause) {
				cancelLive(nil)
				t.Fatalf("iteration %d Cause = %v, want cleanup or live cause", i, firstCause)
			}

			cancelLive(nil)
			if !errors.Is(merged.Err(), firstErr) {
				t.Fatalf("iteration %d Err changed from %v to %v", i, firstErr, merged.Err())
			}
			if !errors.Is(context.Cause(merged), firstCause) {
				t.Fatalf("iteration %d Cause changed from %v to %v", i, firstCause, context.Cause(merged))
			}
		}
	})

	t.Run("root custom cancellation cause", func(t *testing.T) {
		cause := errors.New("root canceled")
		live, cancelLive := context.WithCancelCause(context.Background())
		merged, cleanup := mergeErrorContext(values, live)
		defer cleanup()

		if err := merged.Err(); err != nil {
			t.Fatalf("merged Err before bridge Done = %v, want nil", err)
		}
		cancelLive(cause)
		<-merged.Done()
		if !errors.Is(merged.Err(), context.Canceled) {
			t.Fatalf("merged Err = %v, want context.Canceled", merged.Err())
		}
		if got := context.Cause(merged); !errors.Is(got, cause) {
			t.Fatalf("merged Cause = %v, want root cause", got)
		}
	})

	t.Run("cleanup cancellation stays Canceled", func(t *testing.T) {
		live := t.Context()
		merged, cleanup := mergeErrorContext(values, live)

		cleanup()
		<-merged.Done()
		if !errors.Is(merged.Err(), context.Canceled) {
			t.Fatalf("merged Err after cleanup = %v, want context.Canceled", merged.Err())
		}
		if cause := context.Cause(merged); !errors.Is(cause, context.Canceled) {
			t.Fatalf("merged Cause after cleanup = %v, want context.Canceled", cause)
		}
	})

	t.Run("cleanup then live deadline keeps classification", func(t *testing.T) {
		cause := errors.New("late deadline cause")
		live, cancelLive := context.WithDeadlineCause(
			context.Background(),
			time.Now().Add(5*time.Millisecond),
			cause,
		)
		defer cancelLive()
		merged, cleanup := mergeErrorContext(values, live)

		cleanup()
		<-merged.Done()
		if !errors.Is(merged.Err(), context.Canceled) {
			t.Fatalf("merged Err after cleanup = %v, want context.Canceled", merged.Err())
		}
		if got := context.Cause(merged); !errors.Is(got, context.Canceled) {
			t.Fatalf("merged Cause after cleanup = %v, want context.Canceled", got)
		}

		<-live.Done()
		if !errors.Is(merged.Err(), context.Canceled) {
			t.Fatalf("merged Err after late deadline = %v, want frozen context.Canceled", merged.Err())
		}
		if got := context.Cause(merged); !errors.Is(got, context.Canceled) {
			t.Fatalf("merged Cause after late deadline = %v, want frozen context.Canceled", got)
		}
	})

	t.Run("cleanup then live cancellation keeps classification", func(t *testing.T) {
		cause := errors.New("late cancel cause")
		live, cancelLive := context.WithCancelCause(context.Background())
		defer cancelLive(nil)
		merged, cleanup := mergeErrorContext(values, live)

		cleanup()
		<-merged.Done()
		cancelLive(cause)
		<-live.Done()

		if !errors.Is(merged.Err(), context.Canceled) {
			t.Fatalf("merged Err after late cancel = %v, want frozen context.Canceled", merged.Err())
		}
		if got := context.Cause(merged); !errors.Is(got, context.Canceled) {
			t.Fatalf("merged Cause after late cancel = %v, want frozen context.Canceled", got)
		}
	})

	t.Run("cleanup versus live deadline race is stable", func(t *testing.T) {
		for i := range 50 {
			cause := fmt.Errorf("raced deadline %d", i)
			live, cancelLive := context.WithDeadlineCause(
				context.Background(),
				time.Now().Add(time.Millisecond),
				cause,
			)
			merged, cleanup := mergeErrorContext(values, live)
			cleanupDone := make(chan struct{})
			go func() {
				cleanup()
				close(cleanupDone)
			}()
			<-live.Done()
			<-cleanupDone
			<-merged.Done()

			firstErr := merged.Err()
			firstCause := context.Cause(merged)
			cleanupWon := errors.Is(firstErr, context.Canceled) && errors.Is(firstCause, context.Canceled)
			liveWon := errors.Is(firstErr, context.DeadlineExceeded) && errors.Is(firstCause, cause)
			if !cleanupWon && !liveWon {
				cancelLive()
				t.Fatalf("iteration %d inconsistent classification Err=%v Cause=%v", i, firstErr, firstCause)
			}

			cancelLive()
			<-live.Done()
			if !errors.Is(merged.Err(), firstErr) {
				t.Fatalf("iteration %d Err changed from %v to %v", i, firstErr, merged.Err())
			}
			if !errors.Is(context.Cause(merged), firstCause) {
				t.Fatalf("iteration %d Cause changed from %v to %v", i, firstCause, context.Cause(merged))
			}
		}
	})
}

// TestMergedContextErrWaitsForFrozenPublication pins the publication window:
// after the standard bridge is canceled but before wrapper Done closes, Err must
// wait rather than return nil while Cause is already non-nil.
func TestMergedContextErrWaitsForFrozenPublication(t *testing.T) {
	t.Parallel()

	cause := errors.New("pending publication cause")
	bridge, cancelBridge := context.WithCancelCause(context.WithoutCancel(context.Background()))
	state := &errorCancellation{
		cancel: cancelBridge,
		done:   make(chan struct{}),
		frozen: context.DeadlineExceeded,
	}
	cancelBridge(cause)
	merged := mergedErrorContext{
		Context: bridge,
		live:    context.Background(),
		cancel:  state,
	}

	if got := context.Cause(bridge); !errors.Is(got, cause) {
		t.Fatalf("bridge Cause = %v, want custom cause", got)
	}

	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		result <- merged.Err()
	}()
	<-started
	select {
	case err := <-result:
		t.Fatalf("merged Err = %v before wrapper Done closed, want blocked", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(state.done)
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("merged Err = %v after publication, want DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("merged Err did not unblock after wrapper Done closed")
	}
	if got := context.Cause(merged); !errors.Is(got, cause) {
		t.Fatalf("merged Cause = %v, want custom cause", got)
	}
}

// TestMergedContextChildrenPreserveClassification pins derived child behavior:
// standard WithCancel/WithCancelCause children must see the same frozen deadline
// classification and cause as the merged context, including after cleanup and
// under a cleanup/deadline race.
func TestMergedContextChildrenPreserveClassification(t *testing.T) {
	t.Parallel()

	t.Run("before firing deadline", func(t *testing.T) {
		cause := errors.New("child deadline cause")
		live, cancelLive := context.WithDeadlineCause(
			context.Background(),
			time.Now().Add(20*time.Millisecond),
			cause,
		)
		defer cancelLive()
		merged, cleanup := mergeErrorContext(valuesWithDeepValue(), live)
		defer cleanup()

		child, cancelChild := context.WithCancel(merged)
		defer cancelChild()
		causeChild, cancelCauseChild := context.WithCancelCause(merged)
		defer cancelCauseChild(nil)

		<-child.Done()
		<-causeChild.Done()
		if !errors.Is(child.Err(), context.DeadlineExceeded) {
			t.Fatalf("WithCancel child Err = %v, want DeadlineExceeded", child.Err())
		}
		if got := context.Cause(child); !errors.Is(got, cause) {
			t.Fatalf("WithCancel child Cause = %v, want custom deadline cause", got)
		}
		if !errors.Is(causeChild.Err(), context.DeadlineExceeded) {
			t.Fatalf("WithCancelCause child Err = %v, want DeadlineExceeded", causeChild.Err())
		}
		if got := context.Cause(causeChild); !errors.Is(got, cause) {
			t.Fatalf("WithCancelCause child Cause = %v, want custom deadline cause", got)
		}
	})

	t.Run("already expired deadline", func(t *testing.T) {
		cause := errors.New("expired child cause")
		live, cancelLive := context.WithDeadlineCause(
			context.Background(),
			time.Now().Add(-time.Millisecond),
			cause,
		)
		defer cancelLive()
		merged, cleanup := mergeErrorContext(valuesWithDeepValue(), live)
		defer cleanup()

		child, cancelChild := context.WithCancel(merged)
		defer cancelChild()
		if !errors.Is(child.Err(), context.DeadlineExceeded) {
			t.Fatalf("child Err = %v, want DeadlineExceeded", child.Err())
		}
		if got := context.Cause(child); !errors.Is(got, cause) {
			t.Fatalf("child Cause = %v, want custom deadline cause", got)
		}
	})

	t.Run("cleanup before deadline", func(t *testing.T) {
		cause := errors.New("late child deadline cause")
		live, cancelLive := context.WithDeadlineCause(
			context.Background(),
			time.Now().Add(5*time.Millisecond),
			cause,
		)
		defer cancelLive()
		merged, cleanup := mergeErrorContext(valuesWithDeepValue(), live)
		cleanup()
		<-merged.Done()

		child, cancelChild := context.WithCancel(merged)
		defer cancelChild()
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatalf("child Err after cleanup = %v, want Canceled", child.Err())
		}
		if got := context.Cause(child); !errors.Is(got, context.Canceled) {
			t.Fatalf("child Cause after cleanup = %v, want Canceled", got)
		}
		<-live.Done()
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatalf("child Err after late deadline = %v, want frozen Canceled", child.Err())
		}
		if got := context.Cause(child); !errors.Is(got, context.Canceled) {
			t.Fatalf("child Cause after late deadline = %v, want frozen Canceled", got)
		}
	})

	t.Run("cleanup versus deadline race", func(t *testing.T) {
		for i := range 20 {
			cause := fmt.Errorf("child raced deadline %d", i)
			live, cancelLive := context.WithDeadlineCause(
				context.Background(),
				time.Now().Add(time.Millisecond),
				cause,
			)
			merged, cleanup := mergeErrorContext(valuesWithDeepValue(), live)
			child, cancelChild := context.WithCancel(merged)
			cleanupDone := make(chan struct{})
			go func() {
				cleanup()
				close(cleanupDone)
			}()
			<-child.Done()
			<-cleanupDone

			firstErr := child.Err()
			firstCause := context.Cause(child)
			cleanupWon := errors.Is(firstErr, context.Canceled) && errors.Is(firstCause, context.Canceled)
			liveWon := errors.Is(firstErr, context.DeadlineExceeded) && errors.Is(firstCause, cause)
			if !cleanupWon && !liveWon {
				cancelChild()
				cancelLive()
				t.Fatalf("iteration %d inconsistent child Err=%v Cause=%v", i, firstErr, firstCause)
			}

			cancelChild()
			cancelLive()
			<-live.Done()
			if !errors.Is(child.Err(), firstErr) {
				t.Fatalf("iteration %d child Err changed from %v to %v", i, firstErr, child.Err())
			}
			if !errors.Is(context.Cause(child), firstCause) {
				t.Fatalf("iteration %d child Cause changed from %v to %v", i, firstCause, context.Cause(child))
			}
		}
	})

	t.Run("cleanup versus live cancellation race", func(t *testing.T) {
		for i := range 20 {
			cause := fmt.Errorf("child raced cancel %d", i)
			live, cancelLive := context.WithCancelCause(context.Background())
			merged, cleanup := mergeErrorContext(valuesWithDeepValue(), live)
			child, cancelChild := context.WithCancel(merged)
			cleanupDone := make(chan struct{})
			go func() {
				cleanup()
				close(cleanupDone)
			}()
			cancelLive(cause)
			<-child.Done()
			<-cleanupDone

			firstErr := child.Err()
			firstCause := context.Cause(child)
			if !errors.Is(firstErr, context.Canceled) {
				cancelChild()
				cancelLive(nil)
				t.Fatalf("iteration %d child Err = %v, want Canceled", i, firstErr)
			}
			if !errors.Is(firstCause, context.Canceled) && !errors.Is(firstCause, cause) {
				cancelChild()
				cancelLive(nil)
				t.Fatalf("iteration %d child Cause = %v, want cleanup or live cause", i, firstCause)
			}

			cancelChild()
			cancelLive(nil)
			if !errors.Is(child.Err(), firstErr) {
				t.Fatalf("iteration %d child Err changed from %v to %v", i, firstErr, child.Err())
			}
			if !errors.Is(context.Cause(child), firstCause) {
				t.Fatalf("iteration %d child Cause changed from %v to %v", i, firstCause, context.Cause(child))
			}
		}
	})
}

// valuesWithDeepValue returns a detached value context carrying a known deep
// value with its own cancellation already triggered.
func valuesWithDeepValue() context.Context {
	type valueKey struct{}
	values, cancel := context.WithCancel(context.Background())
	cancel()
	return context.WithValue(values, valueKey{}, "deep")
}

// cooperativeErrorBufferMiddleware installs the fake coordinated capture and

// commits a rendered error page, like the planned Inertia implementation.
func cooperativeErrorBufferMiddleware(target **errorBufferWriter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			original := ResponseWriter(ctx)
			buffer := &errorBufferWriter{ResponseWriter: original}
			*target = buffer
			ctx = ReplaceResponseWriter(ctx, &bufferedChainWriter{ResponseWriter: buffer})
			err := next(ctx)
			ReplaceResponseWriter(ctx, original)
			if err != nil && RenderedError(ctx) == nil {
				return err
			}
			return buffer.commit()
		}
	}
}

// TestFailedRendererDiscardsCooperativeBufferBeforePlainFallback pins that a
// renderer which writes part of a page and then fails or panics leaves only the
// safe plain 500 in a cooperative capture, not the partial page or its status.
func TestFailedRendererDiscardsCooperativeBufferBeforePlainFallback(t *testing.T) {
	cases := []struct {
		name  string
		write func(ctx context.Context) error
	}{
		{
			name: "returned error",
			write: func(ctx context.Context) error {
				if err := Text(ctx, http.StatusCreated, "partial render"); err != nil {
					return err
				}
				return Forbidden(errors.New("renderer failed"))
			},
		},
		{
			name: "panic",
			write: func(ctx context.Context) error {
				if err := Text(ctx, http.StatusCreated, "partial panic"); err != nil {
					return err
				}
				panic("renderer panic")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captureDefaultLogger(t)
			var (
				buffer  *errorBufferWriter
				renders int
			)
			rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
				renders++
				return tc.write(ctx)
			}))
			rt.Use(cooperativeErrorBufferMiddleware(&buffer))
			rt.Use(guardErrorCtx)
			rt.Get("/", textHandler("home"))

			writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
			if renders != 1 {
				t.Fatalf("ErrorHandler renders = %d, want 1", renders)
			}
			if buffer == nil || buffer.discarded < 2 {
				t.Fatalf("buffer discards = %v, want partial render and fallback discarded", buffer)
			}
			if got := buffer.body.String(); got != "Internal Server Error\n" {
				t.Fatalf("buffered body = %q, want only the plain fallback", got)
			}
			if buffer.status != http.StatusInternalServerError {
				t.Fatalf("buffered status = %d, want 500", buffer.status)
			}
			if writer.status != http.StatusInternalServerError || writer.body.String() != "Internal Server Error\n" {
				t.Fatalf("response = %d %q, want plain 500 only", writer.status, writer.body.String())
			}
		})
	}
}

// flushErrorWriter rejects flushes with a specific error and records whether
// the renderer wrapper forwarded FlushError rather than swallowing it.
type flushErrorWriter struct {
	http.ResponseWriter
	err error
}

func (writer *flushErrorWriter) FlushError() error { return writer.err }

func (writer *flushErrorWriter) Flush() { _ = writer.err }

func TestRendererWriterForwardsFlushError(t *testing.T) {
	t.Parallel()

	flushErr := errors.New("flush rejected")
	var (
		hasFlusher    bool
		hasFlushError bool
		hasHijacker   bool
		hasUnwrap     bool
		observedFlush error
	)
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		writer := ResponseWriter(ctx)
		_, hasFlusher = writer.(http.Flusher)
		_, hasFlushError = writer.(interface{ FlushError() error })
		_, hasHijacker = writer.(http.Hijacker)
		_, hasUnwrap = writer.(interface{ Unwrap() http.ResponseWriter })
		observedFlush = http.NewResponseController(writer).Flush()
		return Text(ctx, StatusCode(err), "themed")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			ctx = ReplaceResponseWriter(ctx, &flushErrorWriter{
				ResponseWriter: ResponseWriter(ctx),
				err:            flushErr,
			})
			return next(ctx)
		}
	})
	rt.Use(guardErrorCtx)
	rt.Get("/", textHandler("home"))

	writer := serveErrorRequest(rt, http.MethodGet, "andurel.com", "/")
	if !hasFlusher {
		t.Fatal("renderer writer does not expose http.Flusher")
	}
	if !hasFlushError {
		t.Fatal("renderer writer does not expose FlushError")
	}
	if !hasHijacker {
		t.Fatal("renderer writer does not expose http.Hijacker")
	}
	if !hasUnwrap {
		t.Fatal("renderer writer does not expose Unwrap")
	}
	if !errors.Is(observedFlush, flushErr) {
		t.Fatalf("renderer Flush error = %v, want underlying flush error", observedFlush)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", writer.status)
	}
}

// TestMethodNotAllowedAllowSurvivesBufferDiscard pins router-owned error
// metadata: a cooperative buffer that resets its header snapshot during
// DiscardBuffered cannot strip the ServeMux Allow header from a 405 error page.
func TestMethodNotAllowedAllowSurvivesBufferDiscard(t *testing.T) {
	t.Parallel()

	var buffer *errorBufferWriter
	var renders int
	rt := NewRouter(WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		return Text(ctx, StatusCode(err), "themed method not allowed")
	}))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			original := ResponseWriter(ctx)
			buffer = &errorBufferWriter{
				ResponseWriter: original,
				discardHook: func() {
					original.Header().Del("Allow")
				},
			}
			ctx = ReplaceResponseWriter(ctx, buffer)
			err := next(ctx)
			ReplaceResponseWriter(ctx, original)
			if err != nil && RenderedError(ctx) == nil {
				return err
			}
			return buffer.commit()
		}
	})
	rt.Get("/only-get", func(ctx context.Context) error {
		return NoContent(ctx, http.StatusNoContent)
	})

	writer := serveErrorRequest(rt, http.MethodPost, "andurel.com", "/only-get")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want 1", renders)
	}
	if got, want := writer.header.Get("Allow"), "GET, HEAD"; got != want {
		t.Fatalf("Allow = %q, want %q after buffer discard", got, want)
	}
	if writer.status != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", writer.status)
	}
}

func readAllString(response *http.Response) (string, error) {
	defer response.Body.Close()
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(response.Body)
	return buffer.String(), err
}
