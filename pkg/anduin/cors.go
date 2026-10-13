package anduin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig configures the CORS middleware. Wildcard origins are allowed only
// when AllowCredentials is false; credentialed CORS reflects the request Origin.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
	Skipper          func(ctx context.Context) bool
}

// CORS returns middleware that answers cross-origin preflight requests and
// annotates actual responses with the matching Access-Control headers.
func CORS(cfg CORSConfig) Middleware {
	methods := cfg.AllowMethods
	if len(methods) == 0 {
		methods = []string{
			http.MethodGet, http.MethodHead, http.MethodPost,
			http.MethodPut, http.MethodPatch, http.MethodDelete,
		}
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

			header := responseHeader(ctx)
			if header == nil {
				return next(ctx)
			}
			header.Add("Vary", "Origin")

			origin := strings.TrimSpace(request.Header.Get("Origin"))
			if origin == "" {
				return next(ctx)
			}
			allowed, wildcard := allowedOrigin(cfg, origin)
			if !allowed {
				return next(ctx)
			}

			allowOrigin := origin
			if wildcard && !cfg.AllowCredentials {
				allowOrigin = "*"
			}
			header.Set("Access-Control-Allow-Origin", allowOrigin)
			if cfg.AllowCredentials {
				header.Set("Access-Control-Allow-Credentials", "true")
			}
			if len(cfg.ExposeHeaders) > 0 {
				header.Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
			}

			if request.Method != http.MethodOptions ||
				request.Header.Get("Access-Control-Request-Method") == "" {
				return next(ctx)
			}

			header.Add("Vary", "Access-Control-Request-Method")
			header.Add("Vary", "Access-Control-Request-Headers")
			header.Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
			if requested := strings.TrimSpace(request.Header.Get("Access-Control-Request-Headers")); requested != "" {
				header.Set("Access-Control-Allow-Headers", requested)
			} else if len(cfg.AllowHeaders) > 0 {
				header.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
			}
			if cfg.MaxAge > 0 {
				header.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
			return NoContent(ctx, http.StatusNoContent)
		}
	}
}

func responseHeader(ctx context.Context) http.Header {
	writer := ResponseWriter(ctx)
	if writer == nil {
		return nil
	}
	return writer.Header()
}

func allowedOrigin(cfg CORSConfig, origin string) (allowed bool, wildcard bool) {
	for _, candidate := range cfg.AllowOrigins {
		candidate = strings.TrimSpace(candidate)
		switch {
		case candidate == "":
			continue
		case candidate == "*":
			wildcard = true
		case strings.EqualFold(candidate, origin):
			return true, false
		}
	}
	if wildcard {
		return true, true
	}
	return false, false
}
