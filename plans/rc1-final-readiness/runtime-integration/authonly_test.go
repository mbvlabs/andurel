package runtimeintegration_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/kiks"
)

// TestAuthOnlyStyle303RedirectKeepsKiksCookies protects the successful
// AuthOnly-style rejection path through the real Inertia middleware: the host
// middleware destroys the session, stores an encrypted return target, writes a
// 303 and returns nil. Kiks must still persist the destroyed session and the
// return cookie instead of the error lifecycle swallowing or duplicating them.
func TestAuthOnlyStyle303RedirectKeepsKiksCookies(t *testing.T) {
	for _, visit := range []bool{false, true} {
		name := "document"
		if visit {
			name = "inertia"
		}

		t.Run(name, func(t *testing.T) {
			renderer, _ := newIntegrationRenderer(t)
			jar, store := newIntegrationJar(t)
			seedCookies := seedSession(t, jar, &sessionState{
				UserID:          "authonly-user",
				IsAuthenticated: true,
			}, false)
			store.reset()

			errorCalls := 0
			rt := anduin.NewRouter(
				anduin.WithHosts(newIntegrationHosts(t)),
				anduin.WithErrorHandler(func(ctx context.Context, err error) error {
					errorCalls++

					return renderer.Page(ctx, "Errors/NotFound", nil).
						Status(http.StatusForbidden).
						Render()
				}),
			)
			rt.Use(jar.Middleware())
			rt.Use(renderer.Middleware())

			admin := rt.Host(adminHost)
			admin.Isolate()
			admin.Use(func(next anduin.Handler) anduin.Handler {
				return func(ctx context.Context) error {
					current, getErr := kiks.Get[*sessionState](ctx)
					if getErr != nil {
						return getErr
					}
					if current == nil || !current.IsAuthenticated {
						return errors.New("AuthOnly middleware did not see the seed session")
					}
					if destroyErr := kiks.Destroy[*sessionState](ctx); destroyErr != nil {
						return destroyErr
					}
					target := &returnTarget{
						Host: anduin.RequestHost(ctx),
						Path: anduin.Request(ctx).URL.RequestURI(),
					}
					if setErr := kiks.Set(ctx, target); setErr != nil {
						return setErr
					}

					return anduin.Redirect(
						ctx,
						"https://"+primaryAuthority+"/session/new",
						http.StatusSeeOther,
					)
				}
			})
			admin.Get("/dashboard", func(ctx context.Context) error {
				return anduin.Text(ctx, http.StatusOK, "unreachable")
			})

			writer := serveRequest(t, rt.Handler(), requestOptions{
				method:  http.MethodGet,
				host:    adminAuthority,
				path:    "/dashboard?tab=settings",
				inertia: visit,
				cookies: seedCookies,
			})

			if errorCalls != 0 {
				t.Fatalf("ErrorHandler calls = %d, want 0 for a successful redirect", errorCalls)
			}
			if writer.status != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303: %s", writer.status, writer.bodyString())
			}
			if location := writer.header.Get("Location"); location != "https://"+primaryAuthority+"/session/new" {
				t.Fatalf("Location = %q, want the primary sign-in", location)
			}
			if destroys := store.destroyCount(); destroys != 1 {
				t.Fatalf("session destroys = %d, want exactly 1", destroys)
			}
			if saves := store.saveCount(); saves != 0 {
				t.Fatalf("session saves = %d, want none for the destroyed session", saves)
			}

			cookies := writer.cookies()
			sessionCookie := cookieByName(cookies, sessionCookieName)
			if sessionCookie == nil || sessionCookie.MaxAge >= 0 {
				t.Fatalf("%s = %+v, want the session destroyed", sessionCookieName, sessionCookie)
			}
			returnCookie := cookieByName(cookies, returnCookieName)
			if returnCookie == nil || returnCookie.MaxAge < 0 {
				t.Fatalf("%s = %+v, want the return target persisted", returnCookieName, returnCookie)
			}
			if flashCookie := cookieByName(cookies, flashCookieName); flashCookie != nil && flashCookie.MaxAge >= 0 {
				t.Fatalf("%s = %+v, want no live flash on the redirect", flashCookieName, flashCookie)
			}

			follow := readBag(t, jar, returnCookie)
			if follow.err != nil {
				t.Fatalf("follow-up bag read: %v", follow.err)
			}
			if follow.session != nil {
				t.Fatalf("session after AuthOnly rejection = %+v, want nil", follow.session)
			}
			if follow.target == nil ||
				follow.target.Host != adminHost ||
				follow.target.Path != "/dashboard?tab=settings" {
				t.Fatalf(
					"return target = %+v, want admin /dashboard?tab=settings",
					follow.target,
				)
			}
		})
	}
}
