// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// Middleware parses request state and applies v3 version, empty-response, and
// redirect corrections only to Inertia visits.
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
				slog.Default().Debug("inertia protocol request",
					slog.String("method", request.Method),
					slog.String("url", requestURL(ctx)),
					slog.Bool("inertia", state.Inertia),
					slog.String("partial_component", state.PartialComponent),
					slog.Any("only", state.Only),
					slog.Any("except", state.Except),
					slog.Any("reset", state.Reset),
					slog.String("purpose", string(state.Purpose)),
				)
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
					appendVary(header, HeaderInertia)
					return anduin.NoContent(ctx, http.StatusConflict)
				}
			}

			original := anduin.ResponseWriter(ctx)
			captured := &captureWriter{ResponseWriter: original}
			ctx = anduin.ReplaceResponseWriter(ctx, captured)
			err = next(ctx)
			ctx = anduin.ReplaceResponseWriter(ctx, original)
			if err != nil {
				return err
			}

			wasRedirect := captured.status >= 300 && captured.status < 400
			captured.normalize(ctx)
			if renderer.reflash != nil &&
				(wasRedirect || captured.status >= 300 && captured.status < 400) {
				if err := renderer.reflash(ctx); err != nil {
					return err
				}
			}
			return captured.commit()
		}
	}
}

type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
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
	appendVary(header, HeaderInertia)

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
