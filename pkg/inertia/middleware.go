// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/textproto"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// Middleware parses request state and applies v3 version, empty-response, and
// redirect corrections only to Inertia visits. When the router has already
// rendered an error into the captured response, the capture is normalized,
// reflashed and committed, and the original error is still returned. A reflash
// failure still commits the normalized rendered response, with the reflash error
// joined to the original. An error with no rendered response is discarded as
// before, which leaves delayed CatchErrors scopes free to render after this
// middleware unwinds.
func (renderer *Renderer) Middleware() anduin.Middleware {
	return func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			request := anduin.Request(ctx)
			state, err := ParseRequest(ctx)
			if err != nil {
				return err
			}

			ctx = anduin.ReplaceRequest(ctx, requestWithState(request, state))
			if renderer.protocolDebug {
				// The middleware runs before the router matches a ServeMux route, so
				// emit the record on the way out when the route pattern is known.
				defer logProtocolRequest(ctx, request, state)
			}
			if !state.Inertia {
				return next(ctx)
			}

			if request.Method == http.MethodGet {
				version, versionErr := renderer.currentVersion(ctx)
				if versionErr != nil {
					return versionErr
				}

				if state.Version != version {
					header := anduin.ResponseWriter(ctx).Header()
					header.Del(HeaderInertia)
					header.Set(HeaderLocation, requestURL(ctx))
					header.Set(HeaderVersion, version)
					anduin.AddVary(header, HeaderInertia)
					return anduin.NoContent(ctx, http.StatusConflict)
				}
			}

			original := anduin.ResponseWriter(ctx)
			captured := newCaptureWriter(original)
			ctx = anduin.ReplaceResponseWriter(ctx, captured)
			err = next(ctx)
			ctx = anduin.ReplaceResponseWriter(ctx, original)
			if err != nil && anduin.RenderedError(ctx) == nil {
				return err
			}

			finishErr := captured.finish(ctx, renderer.reflash, err != nil)
			if err != nil {
				if finishErr != nil {
					return errors.Join(err, finishErr)
				}
				return err
			}
			return finishErr
		}
	}
}

func logProtocolRequest(ctx context.Context, request *http.Request, state Request) {
	slog.Default().Debug("inertia protocol request",
		slog.String("method", request.Method),
		slog.String("url", routePath(ctx)),
		slog.Bool("inertia", state.Inertia),
		slog.String("partial_component", state.PartialComponent),
		slog.Any("only", state.Only),
		slog.Any("except", state.Except),
		slog.Any("reset", state.Reset),
		slog.String("purpose", string(state.Purpose)),
	)
}

type captureWriter struct {
	http.ResponseWriter
	status     int
	body       bytes.Buffer
	baseHeader http.Header
}

var _ anduin.ResponseBuffer = (*captureWriter)(nil)

func newCaptureWriter(writer http.ResponseWriter) *captureWriter {
	captured := &captureWriter{ResponseWriter: writer}
	if writer != nil {
		captured.baseHeader = writer.Header().Clone()
	}
	return captured
}

// DiscardBuffered implements anduin.ResponseBuffer. The router calls it before
// rendering an error page into this writer, so a buffered success response
// cannot be concatenated with the error page. The status and body are reset,
// and headers are restored to the snapshot taken when capture began, which
// drops headers owned by the discarded response. Response metadata that belongs
// to the request rather than the discarded body is re-applied from the current
// headers: the router's Allow, cache Vary tokens, and security/CORS policy
// headers installed by middleware after capture started.
func (writer *captureWriter) DiscardBuffered() {
	if writer == nil {
		return
	}
	writer.status = 0
	writer.body.Reset()
	if writer.ResponseWriter == nil || writer.baseHeader == nil {
		return
	}
	header := writer.ResponseWriter.Header()
	current := header.Clone()
	clear(header)
	for key, values := range writer.baseHeader {
		header[key] = slices.Clone(values)
	}
	for key, values := range current {
		if isResponseMetadataHeader(key) {
			header[key] = slices.Clone(values)
		}
	}
}

// isResponseMetadataHeader reports whether a response header describes the
// request/transport rather than the buffered success body. These values must
// survive an error replacement: Allow is router error metadata, Vary keeps
// CORS/cache behavior correct, the security/CORS headers are policies that
// apply to every response, and Retry-After/WWW-Authenticate/Proxy-Authenticate
// are standards-defined error challenge metadata. This is deliberately a
// narrow list: arbitrary success headers stay owned by the discarded response.
func isResponseMetadataHeader(key string) bool {
	key = textproto.CanonicalMIMEHeaderKey(key)
	switch key {
	case "Allow",
		"Proxy-Authenticate",
		"Retry-After",
		"Vary",
		"Www-Authenticate",
		"Origin-Agent-Cluster",
		"Permissions-Policy",
		"Referrer-Policy",
		"Strict-Transport-Security",
		"Timing-Allow-Origin",
		"X-Content-Type-Options",
		"X-Dns-Prefetch-Control",
		"X-Download-Options",
		"X-Frame-Options",
		"X-Permitted-Cross-Domain-Policies",
		"X-Xss-Protection":
		return true
	}
	return strings.HasPrefix(key, "Access-Control-") ||
		strings.HasPrefix(key, "Content-Security-Policy") ||
		strings.HasPrefix(key, "Cross-Origin-")
}

func (writer *captureWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *captureWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *captureWriter) Write(value []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}

	return writer.body.Write(value)
}

func (writer *captureWriter) normalize(ctx context.Context) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	header := writer.Header()
	anduin.AddVary(header, HeaderInertia)

	if writer.status == http.StatusOK && writer.body.Len() == 0 {
		location := anduin.Request(ctx).Referer()
		if location == "" {
			location = "/"
		}
		header.Set("Location", location)
		writer.status = http.StatusFound
	}
	if writer.status == http.StatusFound && isUnsafeRedirectMethod(anduin.Request(ctx).Method) {
		writer.status = http.StatusSeeOther
	}

	if writer.status >= 300 && writer.status < 400 &&
		strings.Contains(header.Get("Location"), "#") {
		state, _ := requestState(ctx)
		if state.Purpose != PurposePrefetch {
			location := header.Get("Location")
			header.Del("Location")
			header.Del(HeaderInertia)
			header.Del("Content-Type")
			header.Set(HeaderRedirect, location)
			writer.status = http.StatusConflict
			writer.body.Reset()
		}
	}
}

func (writer *captureWriter) finish(
	ctx context.Context,
	reflash ReflashHandler,
	commitOnReflashError bool,
) error {
	wasRedirect := writer.status >= 300 && writer.status < 400
	writer.normalize(ctx)

	var reflashErr error
	if reflash != nil &&
		(wasRedirect || writer.status >= 300 && writer.status < 400) {
		reflashErr = reflash(ctx)
	}
	if reflashErr != nil && !commitOnReflashError {
		return reflashErr
	}

	// A router-rendered error has already marked the response committed and
	// trusts this buffer to deliver it. If reflash failed, still deliver the
	// normalized response instead of leaving an empty implicit 200; the caller
	// joins the preserved reflash error with the original request error.
	commitErr := writer.commit()
	switch {
	case reflashErr != nil && commitErr != nil:
		return errors.Join(reflashErr, commitErr)
	case reflashErr != nil:
		return reflashErr
	default:
		return commitErr
	}
}

func (writer *captureWriter) commit() error {
	writer.ResponseWriter.WriteHeader(writer.status)
	if writer.body.Len() == 0 {
		return nil
	}
	_, err := writer.ResponseWriter.Write(writer.body.Bytes())
	return err
}

func isUnsafeRedirectMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch ||
		method == http.MethodDelete
}
