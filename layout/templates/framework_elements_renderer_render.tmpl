// Package renderer provides utilities for rendering templ components.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package renderer

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

type CookieKey string

const BackURLKey CookieKey = "back_url_context"

func Render(ctx *echo.Context, t templ.Component, cookieKeys []CookieKey) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	extendedCtx := ctx.Request().Context()

	for _, cookie := range cookieKeys {
		cookieCtx := ctx.Get(string(cookie))
		bufCtx := context.WithValue(
			extendedCtx,
			cookie,
			cookieCtx,
		)

		extendedCtx = bufCtx
	}

	if err := t.Render(extendedCtx, buf); err != nil {
		return err
	}

	return ctx.HTML(http.StatusOK, buf.String())
}

func ResolveBackURL(ctx context.Context, fallback string) string {
	backURL, _ := ctx.Value(BackURLKey).(string)
	backURL = strings.TrimSpace(backURL)
	if !isSafeBackURL(backURL) {
		return fallback
	}

	return backURL
}

func isSafeBackURL(backURL string) bool {
	if backURL == "" {
		return false
	}

	return strings.HasPrefix(backURL, "/") &&
		!strings.HasPrefix(backURL, "//")
}
