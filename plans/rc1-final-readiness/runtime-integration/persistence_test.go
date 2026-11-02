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

var errPersistedGuard = errors.New("integration: persisted guard rejected")

// TestErrorRendererBagMutationsPersistWithFinalStatus drives the real kiks
// persistWriter from inside the ErrorHandler, through the Inertia capture when
// the request is an X-Inertia visit. A 409/303 ErrorHandler flash and dirty
// session must persist with the rendered status; a 403 keeps the dirty session
// but clears the flash.
func TestErrorRendererBagMutationsPersistWithFinalStatus(t *testing.T) {
	t.Run("409", func(t *testing.T) {
		for _, visit := range []bool{false, true} {
			name := "document"
			if visit {
				name = "inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, rootData := newIntegrationRenderer(t)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "before",
					IsAuthenticated: true,
				}, false)
				store.reset()

				renders := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++
						current, getErr := kiks.Get[*sessionState](ctx)
						if getErr != nil {
							return getErr
						}
						if current == nil || current.UserID != "before" || !current.IsAuthenticated {
							return errors.New("ErrorHandler did not see the loaded session")
						}
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "after-409",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}
						if flashErr := kiks.AddFlash(ctx, kiks.FlashError, "persisted 409"); flashErr != nil {
							return flashErr
						}

						return renderer.Page(ctx, "Errors/Conflict", nil).
							Status(http.StatusConflict).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())
				rt.Use(func(next anduin.Handler) anduin.Handler {
					return func(context.Context) error {
						return anduin.Conflict(errPersistedGuard)
					}
				})
				rt.Get("/conflict", func(ctx context.Context) error {
					return anduin.Text(ctx, http.StatusOK, "unreachable")
				})

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    primaryAuthority,
					path:    "/conflict",
					inertia: visit,
					cookies: seedCookies,
				})

				if renders != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", renders)
				}
				if writer.status != http.StatusConflict {
					t.Fatalf("status = %d, want 409: %s", writer.status, writer.bodyString())
				}
				assertThemedComponent(t, writer.bodyString(), "Errors/Conflict")
				if !visit && rootData.Page.Component != "Errors/Conflict" {
					t.Fatalf("document component = %q, want Errors/Conflict", rootData.Page.Component)
				}
				if saves := store.saveCount(); saves != 1 {
					t.Fatalf("session saves = %d, want exactly 1", saves)
				}

				cookies := writer.cookies()
				sessionCookie := cookieByName(cookies, sessionCookieName)
				if sessionCookie == nil || sessionCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the dirty session saved", sessionCookieName, sessionCookie)
				}
				flashCookie := cookieByName(cookies, flashCookieName)
				if flashCookie == nil || flashCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the 409 flash persisted", flashCookieName, flashCookie)
				}

				follow := readBag(t, jar, sessionCookie, flashCookie)
				if follow.err != nil {
					t.Fatalf("follow-up bag read: %v", follow.err)
				}
				if follow.session == nil || follow.session.UserID != "after-409" {
					t.Fatalf("persisted session = %+v, want after-409", follow.session)
				}
				if len(follow.flashes) != 1 || follow.flashes[0].Message != "persisted 409" {
					t.Fatalf("persisted flashes = %+v, want the 409 flash", follow.flashes)
				}
			})
		}
	})

	t.Run("303", func(t *testing.T) {
		for _, visit := range []bool{false, true} {
			name := "document"
			if visit {
				name = "inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, _ := newIntegrationRenderer(t)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "before",
					IsAuthenticated: true,
				}, false)
				store.reset()

				renders := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++
						current, getErr := kiks.Get[*sessionState](ctx)
						if getErr != nil {
							return getErr
						}
						if current == nil || current.UserID != "before" {
							return errors.New("ErrorHandler did not see the loaded session")
						}
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "after-303",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}
						if flashErr := kiks.AddFlash(ctx, kiks.FlashSuccess, "persisted 303"); flashErr != nil {
							return flashErr
						}

						return anduin.Redirect(ctx, "/sign-in", http.StatusSeeOther)
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())
				rt.Use(func(next anduin.Handler) anduin.Handler {
					return func(context.Context) error {
						return anduin.BadRequest(errPersistedGuard)
					}
				})
				rt.Get("/redirect", func(ctx context.Context) error {
					return anduin.Text(ctx, http.StatusOK, "unreachable")
				})

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    primaryAuthority,
					path:    "/redirect",
					inertia: visit,
					cookies: seedCookies,
				})

				if renders != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", renders)
				}
				if writer.status != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303: %s", writer.status, writer.bodyString())
				}
				if location := writer.header.Get("Location"); location != "/sign-in" {
					t.Fatalf("Location = %q, want /sign-in", location)
				}
				if writer.bodyString() != "" {
					t.Fatalf("body = %q, want empty for the redirect", writer.bodyString())
				}
				if saves := store.saveCount(); saves != 1 {
					t.Fatalf("session saves = %d, want exactly 1", saves)
				}

				cookies := writer.cookies()
				sessionCookie := cookieByName(cookies, sessionCookieName)
				if sessionCookie == nil || sessionCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the dirty session saved", sessionCookieName, sessionCookie)
				}
				flashCookie := cookieByName(cookies, flashCookieName)
				if flashCookie == nil || flashCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the 303 flash persisted", flashCookieName, flashCookie)
				}

				follow := readBag(t, jar, sessionCookie, flashCookie)
				if follow.err != nil {
					t.Fatalf("follow-up bag read: %v", follow.err)
				}
				if follow.session == nil || follow.session.UserID != "after-303" {
					t.Fatalf("persisted session = %+v, want after-303", follow.session)
				}
				if len(follow.flashes) != 1 || follow.flashes[0].Message != "persisted 303" {
					t.Fatalf("persisted flashes = %+v, want the 303 flash", follow.flashes)
				}
			})
		}
	})

	t.Run("403 clears flash and saves dirty session", func(t *testing.T) {
		for _, visit := range []bool{false, true} {
			name := "document"
			if visit {
				name = "inertia"
			}

			t.Run(name, func(t *testing.T) {
				renderer, _ := newIntegrationRenderer(t)
				jar, store := newIntegrationJar(t)
				seedCookies := seedSession(t, jar, &sessionState{
					UserID:          "before",
					IsAuthenticated: true,
				}, true)
				store.reset()

				renders := 0
				rt := anduin.NewRouter(
					anduin.WithHosts(newIntegrationHosts(t)),
					anduin.WithErrorHandler(func(ctx context.Context, err error) error {
						renders++
						current, getErr := kiks.Get[*sessionState](ctx)
						if getErr != nil {
							return getErr
						}
						if current == nil || current.UserID != "before" {
							return errors.New("ErrorHandler did not see the loaded session")
						}
						if setErr := kiks.Set(ctx, &sessionState{
							UserID:          "after-403",
							IsAuthenticated: true,
						}); setErr != nil {
							return setErr
						}
						if flashErr := kiks.AddFlash(ctx, kiks.FlashError, "must clear"); flashErr != nil {
							return flashErr
						}

						return renderer.Page(ctx, "Errors/NotFound", nil).
							Status(http.StatusForbidden).
							Render()
					}),
				)
				rt.Use(jar.Middleware())
				rt.Use(renderer.Middleware())
				rt.Use(func(next anduin.Handler) anduin.Handler {
					return func(context.Context) error {
						return anduin.Forbidden(errPersistedGuard)
					}
				})
				rt.Get("/forbidden", func(ctx context.Context) error {
					return anduin.Text(ctx, http.StatusOK, "unreachable")
				})

				writer := serveRequest(t, rt.Handler(), requestOptions{
					method:  http.MethodGet,
					host:    primaryAuthority,
					path:    "/forbidden",
					inertia: visit,
					cookies: seedCookies,
				})

				if renders != 1 {
					t.Fatalf("ErrorHandler calls = %d, want 1", renders)
				}
				if writer.status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403: %s", writer.status, writer.bodyString())
				}
				assertThemedComponent(t, writer.bodyString(), "Errors/NotFound")
				if saves := store.saveCount(); saves != 1 {
					t.Fatalf("session saves = %d, want exactly 1", saves)
				}

				cookies := writer.cookies()
				sessionCookie := cookieByName(cookies, sessionCookieName)
				if sessionCookie == nil || sessionCookie.MaxAge < 0 {
					t.Fatalf("%s = %+v, want the dirty session saved", sessionCookieName, sessionCookie)
				}
				flashCookie := cookieByName(cookies, flashCookieName)
				if flashCookie == nil {
					t.Fatalf("%s missing, want the 403 flash clear", flashCookieName)
				}
				if flashCookie.MaxAge >= 0 {
					t.Fatalf("%s = %+v, want a cleared flash", flashCookieName, flashCookie)
				}

				follow := readBag(t, jar, sessionCookie)
				if follow.err != nil {
					t.Fatalf("follow-up bag read: %v", follow.err)
				}
				if follow.session == nil || follow.session.UserID != "after-403" {
					t.Fatalf("persisted session = %+v, want after-403", follow.session)
				}
				if len(follow.flashes) != 0 {
					t.Fatalf("persisted flashes = %+v, want none after a 403", follow.flashes)
				}
			})
		}
	})
}

// TestErrorRendererFlashStaysOutOfTheResponseBody is a small consistency
// control: the flash is a cookie concern, never a reason to append it to the
// themed body.
func TestErrorRendererFlashStaysOutOfTheResponseBody(t *testing.T) {
	renderer, _ := newIntegrationRenderer(t)
	jar, _ := newIntegrationJar(t)
	seedCookies := seedSession(t, jar, &sessionState{
		UserID:          "before",
		IsAuthenticated: true,
	}, false)

	rt := anduin.NewRouter(
		anduin.WithHosts(newIntegrationHosts(t)),
		anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			if flashErr := kiks.AddFlash(ctx, kiks.FlashInfo, "cookie-only-flash"); flashErr != nil {
				return flashErr
			}

			return renderer.Page(ctx, "Errors/Conflict", nil).
				Status(http.StatusConflict).
				Render()
		}),
	)
	rt.Use(jar.Middleware())
	rt.Use(renderer.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(context.Context) error {
			return anduin.Conflict(errPersistedGuard)
		}
	})
	rt.Get("/conflict", func(ctx context.Context) error {
		return anduin.Text(ctx, http.StatusOK, "unreachable")
	})

	writer := serveRequest(t, rt.Handler(), requestOptions{
		method:  http.MethodGet,
		host:    primaryAuthority,
		path:    "/conflict",
		cookies: seedCookies,
	})
	if strings.Contains(writer.bodyString(), "cookie-only-flash") {
		t.Fatalf("response body leaked the flash value: %s", writer.bodyString())
	}
	if cookieByName(writer.cookies(), flashCookieName) == nil {
		t.Fatal("flash cookie missing from the 409 response")
	}
}
