// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// Middleware parses request state and applies v3 version, empty-response, and
// redirect corrections only to Inertia visits.
func (renderer *Renderer) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(etx *echo.Context) error {
			state, err := ParseRequest(etx)
			if err != nil {
				return err
			}
			etx.SetRequest(requestWithState(etx.Request(), state))
			if renderer.protocolDebug {
				etx.Logger().Debug("inertia protocol request",
					slog.String("method", etx.Request().Method),
					slog.String("url", requestURL(etx)),
					slog.Bool("inertia", state.Inertia),
					slog.String("partial_component", state.PartialComponent),
					slog.Any("only", state.Only),
					slog.Any("except", state.Except),
					slog.Any("reset", state.Reset),
					slog.String("purpose", string(state.Purpose)),
				)
			}
			if !state.Inertia {
				return next(etx)
			}

			if etx.Request().Method == http.MethodGet {
				version, versionErr := renderer.currentVersion(etx)
				if versionErr != nil {
					return versionErr
				}
				if state.Version != version {
					return renderer.versionMismatch(etx, version)
				}
			}

			original := etx.Response()
			captured := &captureWriter{ResponseWriter: original}
			etx.SetResponse(captured)
			err = next(etx)
			etx.SetResponse(original)
			if err != nil {
				return err
			}
			wasRedirect := captured.status >= 300 && captured.status < 400
			captured.normalize(etx)
			if renderer.reflash != nil && (wasRedirect || captured.status >= 300 && captured.status < 400) {
				if err := renderer.reflash(etx); err != nil {
					return err
				}
			}
			return captured.commit()
		}
	}
}

func (renderer *Renderer) versionMismatch(etx *echo.Context, version string) error {
	header := etx.Response().Header()
	header.Del(HeaderInertia)
	header.Set(HeaderLocation, requestURL(etx))
	header.Set(HeaderVersion, version)
	appendVary(header, HeaderInertia)
	return etx.NoContent(http.StatusConflict)
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

func (writer *captureWriter) normalize(etx *echo.Context) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	header := writer.Header()
	appendVary(header, HeaderInertia)

	if writer.status == http.StatusOK && writer.body.Len() == 0 {
		location := etx.Request().Referer()
		if location == "" {
			location = "/"
		}
		header.Set("Location", location)
		writer.status = http.StatusFound
	}
	if writer.status == http.StatusFound && isUnsafeRedirectMethod(etx.Request().Method) {
		writer.status = http.StatusSeeOther
	}
	if writer.status >= 300 && writer.status < 400 && strings.Contains(header.Get("Location"), "#") && requestStateMust(etx).Purpose != PurposePrefetch {
		location := header.Get("Location")
		header.Del("Location")
		header.Del(HeaderInertia)
		header.Del("Content-Type")
		header.Set(HeaderRedirect, location)
		writer.status = http.StatusConflict
		writer.body.Reset()
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

func requestStateMust(etx *echo.Context) Request {
	state, _ := requestState(etx)
	return state
}

func isUnsafeRedirectMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}
