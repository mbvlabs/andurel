package anduin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

type csrfContextKey struct{}

// CSRFConfig configures the CSRF middleware. TokenLookup is a comma-separated
// list of "source:name" locators, where source is one of cookie, header, form,
// or query (for example "header:X-CSRF-Token,form:_csrf").
type CSRFConfig struct {
	TokenLookup    string
	CookieName     string
	CookiePath     string
	CookieDomain   string
	CookieSecure   bool
	CookieHTTPOnly bool
	CookieSameSite http.SameSite
	TrustedOrigins []string
	TokenLength    int
	Skipper        func(ctx context.Context) bool
}

type csrfSource struct {
	kind string
	name string
}

// CSRF returns middleware that issues and verifies double-submit CSRF tokens.
// The current token is available to handlers via CSRFToken.
func CSRF(cfg CSRFConfig) Middleware {
	sources := parseCSRFLookup(cfg.TokenLookup)
	cookieName := cfg.CookieName
	if cookieName == "" {
		for _, source := range sources {
			if source.kind == "cookie" {
				cookieName = source.name
				break
			}
		}
	}
	if cookieName == "" {
		cookieName = "_csrf"
	}
	if cfg.TokenLength <= 0 {
		cfg.TokenLength = 32
	}

	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			if cfg.Skipper != nil && cfg.Skipper(ctx) {
				return next(ctx)
			}
			request := Request(ctx)
			if request == nil {
				return next(ctx)
			}

			token := extractCSRFToken(ctx, sources)
			if token == "" {
				generated, err := generateCSRFToken(cfg.TokenLength)
				if err != nil {
					return err
				}
				token = generated
			}
			setCSRFCookie(ctx, cfg, cookieName, token)
			ctx = context.WithValue(ctx, csrfContextKey{}, token)

			if isSafeMethod(request.Method) || csrfOriginTrusted(cfg.TrustedOrigins, request) {
				return next(ctx)
			}

			submitted := extractCSRFToken(ctx, sources)
			cookieToken := csrfCookieToken(ctx, cookieName)
			if submitted == "" || cookieToken == "" ||
				subtle.ConstantTimeCompare([]byte(submitted), []byte(cookieToken)) != 1 {
				return &HTTPError{Status: http.StatusForbidden, Message: "CSRF token verification failed"}
			}

			return next(ctx)
		}
	}
}

// CSRFToken returns the CSRF token attached to ctx by the CSRF middleware.
func CSRFToken(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	token, _ := ctx.Value(csrfContextKey{}).(string)
	return token
}

func parseCSRFLookup(lookup string) []csrfSource {
	var sources []csrfSource
	for part := range strings.SplitSeq(lookup, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kind, name, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		kind = strings.ToLower(strings.TrimSpace(kind))
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		switch kind {
		case "cookie", "header", "form", "query":
			sources = append(sources, csrfSource{kind: kind, name: name})
		}
	}
	return sources
}

func extractCSRFToken(ctx context.Context, sources []csrfSource) string {
	for _, source := range sources {
		switch source.kind {
		case "cookie":
			if cookie, err := Cookie(ctx, source.name); err == nil && cookie.Value != "" {
				return cookie.Value
			}
		case "header":
			if value := Header(ctx, source.name); value != "" {
				return value
			}
		case "form":
			if value := FormValue(ctx, source.name); value != "" {
				return value
			}
		case "query":
			if value := Query(ctx, source.name); value != "" {
				return value
			}
		}
	}
	return ""
}

func csrfCookieToken(ctx context.Context, name string) string {
	if cookie, err := Cookie(ctx, name); err == nil {
		return cookie.Value
	}
	return ""
}

func setCSRFCookie(ctx context.Context, cfg CSRFConfig, name, token string) {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return
	}
	path := cfg.CookiePath
	if path == "" {
		path = "/"
	}
	http.SetCookie(writer, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     path,
		Domain:   cfg.CookieDomain,
		Secure:   cfg.CookieSecure,
		HttpOnly: cfg.CookieHTTPOnly,
		SameSite: cfg.CookieSameSite,
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func csrfOriginTrusted(trusted []string, request *http.Request) bool {
	if len(trusted) == 0 {
		return false
	}
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		if referer := strings.TrimSpace(request.Header.Get("Referer")); referer != "" {
			if parsed, err := url.Parse(referer); err == nil {
				origin = parsed.Scheme + "://" + parsed.Host
			}
		}
	}
	if origin == "" {
		return false
	}
	for _, candidate := range trusted {
		if strings.EqualFold(strings.TrimSpace(candidate), origin) {
			return true
		}
	}
	return false
}

func generateCSRFToken(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: generate CSRF token", Err: err}
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
