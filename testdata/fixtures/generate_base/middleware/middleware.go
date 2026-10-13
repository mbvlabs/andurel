// Package middleware holds the application's HTTP middleware and cookie jar
// wiring. Generic middleware (recover, telemetry, logging, CORS, CSRF) lives in
// pkg/anduin; this package owns the pieces an application customises: the auth
// guard, the IP rate limiter, the CSRF policy, and the session jar.
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"example.com/app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/telemetry"
)

func isAPIPath(path string) bool {
	return matchesPathPrefix(path, routes.APIPrefix)
}

func matchesPathPrefix(path, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return false
	}

	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func hasNonEmptyBearerToken(authorization string) bool {
	parts := strings.Fields(authorization)
	return len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != ""
}

func hasApplicationSessionCookie(request *http.Request, appCookieSessionName string) bool {
	_, err := request.Cookie(appCookieSessionName)
	return err == nil
}

func mayBypassCSRF(request *http.Request, appCookieSessionName string) bool {
	return isAPIPath(request.URL.Path) &&
		hasNonEmptyBearerToken(request.Header.Get("Authorization")) &&
		!hasApplicationSessionCookie(request, appCookieSessionName)
}

// Telemetry attaches the telemetry pipeline to the request context and records
// the matched route on the active span. It is anduin's built-in Telemetry.
func Telemetry(tel *telemetry.Telemetry) anduin.Middleware {
	return anduin.Telemetry(tel)
}

// TraceRouteAttributes records the matched ServeMux route on the active span
// after the response has been produced.
func TraceRouteAttributes() anduin.Middleware {
	return func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			err := next(ctx)
			if pattern := anduin.RoutePattern(ctx); pattern != "" {
				telemetry.SetHTTPRoute(ctx, anduin.RoutePath(pattern))
			}

			return err
		}
	}
}

// Logger logs one completed request per line, skipping assets and API traffic.
func Logger() anduin.Middleware {
	return anduin.Logger(slog.Default(), routes.AssetsPrefix, routes.APIPrefix)
}

// CSRFMiddleware builds the CSRF policy for the configured strategy. The
// header_only strategy reads the token from the CSRF cookie; the
// header_or_legacy_token strategy also accepts the legacy _csrf form field.
func CSRFMiddleware(
	csrfStrategy string,
	csrfTrustedOrigins []string,
	csrfName string,
	environment string,
	appCookieSessionName string,
) (anduin.Middleware, error) {
	strategy := strings.TrimSpace(csrfStrategy)

	var tokenLookup string
	switch strategy {
	case "header_only":
		tokenLookup = "cookie:" + csrfName
	case "header_or_legacy_token":
		tokenLookup = "header:X-CSRF-Token,form:_csrf"
	default:
		return nil, errors.New("invalid CSRF strategy")
	}

	return anduin.CSRF(anduin.CSRFConfig{
		TokenLookup:    tokenLookup,
		CookieName:     csrfName,
		CookiePath:     "/",
		CookieSecure:   environment == "production",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		TrustedOrigins: csrfTrustedOrigins,
		Skipper: func(ctx context.Context) bool {
			return mayBypassCSRF(anduin.Request(ctx), appCookieSessionName)
		},
	}), nil
}
