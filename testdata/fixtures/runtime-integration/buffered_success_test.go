package runtimeintegration_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/kiks"
)

var errBufferedHost = errors.New("integration: host rejected a buffered success")

// TestBufferedSuccessIsReplacedByOuterHostError pins the response-buffer path:
// an Inertia success page is captured, an outer host middleware then fails, and
// the error page must replace the captured status/body/success-only headers
// without concatenation. The kiks session mutation is saved with the final 403.
func TestBufferedSuccessIsReplacedByOuterHostError(t *testing.T) {
	renderer, _ := newIntegrationRenderer(t)
	jar, store := newIntegrationJar(t)
	seedCookies := seedSession(t, jar, &sessionState{
		UserID:          "buffered-user",
		IsAuthenticated: true,
	}, false)
	store.reset()

	renders := 0
	var handlerErr error
	var staleHeaderAtRender string
	var baseHeaderAtRender string
	rt := anduin.NewRouter(
		anduin.WithHosts(newIntegrationHosts(t)),
		anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			handlerErr = err
			staleHeaderAtRender = anduin.ResponseWriter(ctx).Header().Get("X-Success")
			baseHeaderAtRender = anduin.ResponseWriter(ctx).Header().Get("X-Base")

			current, getErr := kiks.Get[*sessionState](ctx)
			if getErr != nil {
				return getErr
			}
			if current == nil || current.UserID != "buffered-success" {
				return errors.New("ErrorHandler did not see the buffered handler's session")
			}
			if setErr := kiks.Set(ctx, &sessionState{
				UserID:          "buffered-error",
				IsAuthenticated: true,
			}); setErr != nil {
				return setErr
			}

			return renderer.Page(ctx, "Errors/NotFound", nil).
				Status(http.StatusForbidden).
				Render()
		}),
	)
	rt.Use(jar.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			anduin.SetHeader(ctx, "X-Base", "keep")

			return next(ctx)
		}
	})
	rt.Use(renderer.Middleware())

	admin := rt.Host(adminHost)
	admin.Isolate()
	admin.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			if err := next(ctx); err != nil {
				return err
			}

			return anduin.Forbidden(errBufferedHost)
		}
	})
	admin.Get("/dashboard", func(ctx context.Context) error {
		if setErr := kiks.Set(ctx, &sessionState{
			UserID:          "buffered-success",
			IsAuthenticated: true,
		}); setErr != nil {
			return setErr
		}
		anduin.SetHeader(ctx, "X-Success", "stale")

		return renderer.Page(ctx, "Success", nil).Render()
	})

	writer := serveRequest(t, rt.Handler(), requestOptions{
		method:  http.MethodGet,
		host:    adminAuthority,
		path:    "/dashboard",
		inertia: true,
		cookies: seedCookies,
	})

	if renders != 1 {
		t.Fatalf("ErrorHandler calls = %d, want 1", renders)
	}
	if !errors.Is(handlerErr, errBufferedHost) {
		t.Fatalf("ErrorHandler err = %v, want the outer host error", handlerErr)
	}
	if staleHeaderAtRender != "" {
		t.Fatalf("ErrorHandler saw stale X-Success = %q", staleHeaderAtRender)
	}
	if baseHeaderAtRender != "keep" {
		t.Fatalf("ErrorHandler saw X-Base = %q, want keep", baseHeaderAtRender)
	}
	if writer.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
	}
	if writer.headerWrites != 1 || writer.bodyWrites != 1 {
		t.Fatalf(
			"delivery writes = headers %d body %d, want 1/1",
			writer.headerWrites,
			writer.bodyWrites,
		)
	}
	if got := writer.header.Get("X-Success"); got != "" {
		t.Fatalf("response kept stale X-Success = %q", got)
	}
	if got := writer.header.Get("X-Base"); got != "keep" {
		t.Fatalf("response X-Base = %q, want keep", got)
	}

	body := writer.bodyString()
	if strings.Contains(body, `"component":"Success"`) {
		t.Fatalf("body contains the discarded success page: %s", body)
	}
	if count := strings.Count(body, `"component":"Errors/NotFound"`); count != 1 {
		t.Fatalf("error component count = %d, want 1: %s", count, body)
	}
	if saves := store.saveCount(); saves != 1 {
		t.Fatalf("session saves = %d, want exactly 1 final save", saves)
	}

	sessionCookie := cookieByName(writer.cookies(), sessionCookieName)
	if sessionCookie == nil || sessionCookie.MaxAge < 0 {
		t.Fatalf("%s = %+v, want the final session saved", sessionCookieName, sessionCookie)
	}
	follow := readBag(t, jar, sessionCookie)
	if follow.err != nil {
		t.Fatalf("follow-up bag read: %v", follow.err)
	}
	if follow.session == nil || follow.session.UserID != "buffered-error" {
		t.Fatalf("persisted session = %+v, want buffered-error", follow.session)
	}
}
