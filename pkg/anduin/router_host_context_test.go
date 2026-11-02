package anduin

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestHostMiddlewareContextAndRequestPropagation(t *testing.T) {
	t.Parallel()
	type key string
	const chain key = "chain"
	rt := NewRouter(WithHosts(newAdminHosts(t)))
	rt.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			return next(context.WithValue(ctx, chain, "G"))
		}
	})
	admin := rt.Host(hostAdmin)
	admin.FallbackToPrimary()
	admin.Use(func(next Handler) Handler {
		return func(ctx context.Context) error {
			ctx = context.WithValue(ctx, chain, ctx.Value(chain).(string)+"H")
			req := Request(ctx).Clone(ctx)
			req.Header.Set("X-Host-Modified", "yes")
			return next(ReplaceRequest(ctx, req))
		}
	})
	route := func(next Handler) Handler {
		return func(ctx context.Context) error {
			if got := Request(ctx).Header.Get("X-Host-Modified"); got != "yes" {
				t.Errorf("route request header = %q", got)
			}
			return next(context.WithValue(ctx, chain, ctx.Value(chain).(string)+"R"))
		}
	}
	admin.Get("/admin/:id", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, fmt.Sprintf("%s:%s:%s", ctx.Value(chain), Request(ctx).Header.Get("X-Host-Modified"), PathValue(ctx, "id")))
	}, route)
	admin.Mount("/mount", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s:%s:%s:%s", r.Context().Value(chain), r.Header.Get("X-Host-Modified"), r.Method, r.URL.Path)
	}))
	rt.Get("/asset", func(ctx context.Context) error {
		return Text(ctx, http.StatusOK, fmt.Sprintf("%s:%s", ctx.Value(chain), Request(ctx).Header.Get("X-Host-Modified")))
	})
	for _, tc := range []struct{ method, path, want string }{
		{"GET", "/admin/42", "GHR:yes:42"},
		{"POST", "/mount/nested", "GH:yes:POST:/mount/nested"},
		{"GET", "/asset", "G:"},
	} {
		rec := serveOnHost(rt, tc.method, "admin.andurel.com", tc.path)
		if rec.Code != http.StatusOK || rec.Body.String() != tc.want {
			t.Errorf("%s %s = %d %q, want 200 %q", tc.method, tc.path, rec.Code, rec.Body.String(), tc.want)
		}
	}
}
