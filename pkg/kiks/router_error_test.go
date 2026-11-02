package kiks

// These tests drive the real kiks middleware through a real anduin.Router. They
// pin the persistence contract of the coordinated error lifecycle:
//
//   - Route errors render after route middleware unwinds, but still inside the
//     enclosing kiks middleware, so the ErrorHandler sees the request bag and
//     its writes to the bag are persisted with the rendered status.
//   - Host and global errors render at the failing boundary while the enclosing
//     kiks middleware is live, so the same bag visibility and persistence apply
//     to early error renders.
//   - A CatchErrors scope that reads or mutates the bag must be installed inside
//     the kiks middleware. A scope outside kiks has no bag on its own context,
//     and kiks has already saved by the time it sees the error.
//   - Persistence is fail-closed: load failure leaves no bag and writes no
//     cookies; save failure suppresses the body and leaves the joined error
//     observable, without a second response or ErrorHandler call.
//
// The tests use only kiks and anduin. Inertia capture is intentionally out of
// scope here so pkg/kiks does not gain a test-only inertia dependency.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

const kiksAdminHost routing.HostName = "kiks-admin"

const (
	kiksPrimaryAuthority = "andurel.com"
	kiksAdminAuthority   = "admin.andurel.com"
)

// returnTarget mirrors the scaffold's encrypted return-to cookie value used by
// an AuthOnly-style rejection.
type returnTarget struct {
	Host routing.HostName
	Path string
}

func newKiksHosts(t *testing.T) *routing.Hosts {
	t.Helper()
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: kiksPrimaryAuthority},
		kiksAdminHost:       {Hostname: kiksAdminAuthority},
	})
	if err != nil {
		t.Fatal(err)
	}
	return hosts
}

// countingStore records how often the kiks middleware persists a session. It
// wraps the real CookieStore, so cookies still round-trip.
type countingStore struct {
	inner Store

	mu       sync.Mutex
	loads    int
	saves    int
	destroys int
}

func (store *countingStore) Load(request *http.Request, name string) ([]byte, error) {
	store.mu.Lock()
	store.loads++
	store.mu.Unlock()
	return store.inner.Load(request, name)
}

func (store *countingStore) Save(
	writer http.ResponseWriter,
	request *http.Request,
	name string,
	payload []byte,
	attrs CookieAttrs,
) error {
	store.mu.Lock()
	store.saves++
	store.mu.Unlock()
	return store.inner.Save(writer, request, name, payload, attrs)
}

func (store *countingStore) Destroy(
	writer http.ResponseWriter,
	request *http.Request,
	name string,
	attrs CookieAttrs,
) error {
	store.mu.Lock()
	store.destroys++
	store.mu.Unlock()
	return store.inner.Destroy(writer, request, name, attrs)
}

func (store *countingStore) saveCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.saves
}

func newCountingJar(t *testing.T, defs ...Definition) (*Jar, *countingStore) {
	t.Helper()
	keys := testKeys()
	inner, err := NewCookieStore(keys)
	if err != nil {
		t.Fatal(err)
	}
	store := &countingStore{inner: inner}
	jar, err := NewJar(keys, store, defs...)
	if err != nil {
		t.Fatal(err)
	}
	return jar, store
}

// failingLoadStore fails session loading before kiks can install a bag or
// persistWriter.
type failingLoadStore struct {
	inner Store
	err   error
}

func (store failingLoadStore) Load(*http.Request, string) ([]byte, error) {
	return nil, store.err
}

func (store failingLoadStore) Save(
	writer http.ResponseWriter,
	request *http.Request,
	name string,
	payload []byte,
	attrs CookieAttrs,
) error {
	return store.inner.Save(writer, request, name, payload, attrs)
}

func (store failingLoadStore) Destroy(
	writer http.ResponseWriter,
	request *http.Request,
	name string,
	attrs CookieAttrs,
) error {
	return store.inner.Destroy(writer, request, name, attrs)
}

type logCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (capture *logCapture) Enabled(context.Context, slog.Level) bool { return true }

func (capture *logCapture) Handle(_ context.Context, record slog.Record) error {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.records = append(capture.records, record.Clone())
	return nil
}

func (capture *logCapture) WithAttrs([]slog.Attr) slog.Handler { return capture }

func (capture *logCapture) WithGroup(string) slog.Handler { return capture }

func (capture *logCapture) all() []slog.Record {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]slog.Record(nil), capture.records...)
}

func (capture *logCapture) recordsFor(message string) []slog.Record {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	var matches []slog.Record
	for _, record := range capture.records {
		if record.Message == message {
			matches = append(matches, record)
		}
	}
	return matches
}

func captureDefaultLogger(t *testing.T) *logCapture {
	t.Helper()
	capture := &logCapture{}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return capture
}

func logAttribute(record slog.Record, key string) string {
	var value string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			value = attr.Value.String()
			return false
		}
		return true
	})
	return value
}

func serveKiksRouter(
	rt *anduin.Router,
	method, host, path string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Host = host
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	rt.Handler().ServeHTTP(recorder, request)
	return recorder
}

func cookiesNamed(cookies []*http.Cookie, name string) []*http.Cookie {
	matches := make([]*http.Cookie, 0, 1)
	for _, cookie := range cookies {
		if cookie.Name == name {
			matches = append(matches, cookie)
		}
	}
	return matches
}

// bagMutatingErrorHandler reads a session placed by the failing layer, mutates
// the bag, and only then writes the error response. This is the ordering the
// kiks persistWriter requires: the first response write saves the bag with the
// final status.
func bagMutatingErrorHandler(
	t *testing.T,
	write func(context.Context, int) error,
) anduin.ErrorHandler {
	t.Helper()
	return func(ctx context.Context, err error) error {
		app, getErr := Get[*testApp](ctx)
		if getErr != nil {
			t.Errorf("ErrorHandler Get session: %v", getErr)
		}
		if app == nil || app.UserID != "before" {
			t.Errorf("ErrorHandler session = %+v, want the failing layer's value", app)
		}
		if setErr := Set(ctx, &testApp{UserID: "after", IsAuthenticated: true}); setErr != nil {
			return setErr
		}
		if flashErr := AddFlash(ctx, FlashError, "error flash"); flashErr != nil {
			return flashErr
		}
		return write(ctx, anduin.StatusCode(err))
	}
}

func assertSavedBag(
	t *testing.T,
	jar *Jar,
	recorder *httptest.ResponseRecorder,
	store *countingStore,
	wantStatus int,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
	}
	if saves := store.saveCount(); saves != 1 {
		t.Fatalf("session saves = %d, want exactly 1", saves)
	}

	cookies := recorder.Result().Cookies()
	if appCookies := cookiesNamed(cookies, "app"); len(appCookies) != 1 {
		t.Fatalf("app cookies = %d, want exactly 1 (%v)", len(appCookies), cookieNames(cookies))
	}
	appCookie := cookieByName(cookies, "app")
	if appCookie.MaxAge < 0 {
		t.Fatalf("app cookie was cleared, want the dirty session saved")
	}
	if flashCookies := cookiesNamed(cookies, "app_flash"); len(flashCookies) != 1 {
		t.Fatalf("flash cookies = %d, want exactly 1 (%v)", len(flashCookies), cookieNames(cookies))
	}
	flashCookie := cookieByName(cookies, "app_flash")
	if flashCookie.MaxAge < 0 {
		t.Fatalf("flash cookie = %+v, want a persisted flash", flashCookie)
	}

	follow := httptest.NewRequest(http.MethodGet, "/follow", nil)
	follow.AddCookie(appCookie)
	follow.AddCookie(flashCookie)
	var (
		gotApp     *testApp
		gotFlashes []FlashMessage
	)
	if err := anduin.Serve(httptest.NewRecorder(), follow, jar.Middleware()(func(ctx context.Context) error {
		var getErr error
		gotApp, getErr = Get[*testApp](ctx)
		if getErr != nil {
			return getErr
		}
		gotFlashes = FlashMessages(ctx)
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
	if gotApp == nil || gotApp.UserID != "after" || !gotApp.IsAuthenticated {
		t.Fatalf("persisted session = %+v, want the ErrorHandler's mutation", gotApp)
	}
	if len(gotFlashes) != 1 || gotFlashes[0].Message != "error flash" {
		t.Fatalf("persisted flashes = %+v, want the ErrorHandler's flash", gotFlashes)
	}
}

func TestErrorHandlerBagMutationsPersistWithFinalStatus(t *testing.T) {
	t.Run("route handler 409 saves dirty session and flash exactly once", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))
		renders := 0
		rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			return bagMutatingErrorHandler(t, func(ctx context.Context, _ int) error {
				return anduin.Text(ctx, http.StatusConflict, "conflict page")
			})(ctx, err)
		}))
		rt.Use(jar.Middleware())
		rt.Get("/", func(ctx context.Context) error {
			if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
				return err
			}
			return anduin.Conflict(errors.New("route conflict"))
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
		if renders != 1 {
			t.Fatalf("ErrorHandler renders = %d, want 1", renders)
		}
		if got := recorder.Body.String(); got != "conflict page" {
			t.Fatalf("body = %q, want the 409 page", got)
		}
		assertSavedBag(t, jar, recorder, store, http.StatusConflict)
	})

	t.Run("global middleware 409 saves dirty session and flash exactly once", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))
		renders := 0
		rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			return bagMutatingErrorHandler(t, func(ctx context.Context, _ int) error {
				return anduin.Text(ctx, http.StatusConflict, "global conflict page")
			})(ctx, err)
		}))
		rt.Use(jar.Middleware())
		rt.Use(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
					return err
				}
				return anduin.Conflict(errors.New("global conflict"))
			}
		})
		rt.Get("/", func(context.Context) error {
			return errors.New("global handler must not run")
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
		if renders != 1 {
			t.Fatalf("ErrorHandler renders = %d, want 1", renders)
		}
		if got := recorder.Body.String(); got != "global conflict page" {
			t.Fatalf("body = %q, want the global 409 page", got)
		}
		assertSavedBag(t, jar, recorder, store, http.StatusConflict)
	})

	t.Run("host middleware 409 saves dirty session and flash exactly once", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))
		rt := anduin.NewRouter(
			anduin.WithHosts(newKiksHosts(t)),
			anduin.WithErrorHandler(bagMutatingErrorHandler(t, func(ctx context.Context, _ int) error {
				return anduin.Text(ctx, http.StatusConflict, "host conflict page")
			})),
		)
		rt.Use(jar.Middleware())
		admin := rt.Host(kiksAdminHost)
		admin.Isolate()
		admin.Use(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
					return err
				}
				return anduin.Conflict(errors.New("host conflict"))
			}
		})
		admin.Get("/admin", func(context.Context) error {
			return errors.New("host handler must not run")
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksAdminAuthority, "/admin")
		if got := recorder.Body.String(); got != "host conflict page" {
			t.Fatalf("body = %q, want the host 409 page", got)
		}
		assertSavedBag(t, jar, recorder, store, http.StatusConflict)
	})

	t.Run("route 303 redirect saves flash with final status", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))
		rt := anduin.NewRouter(anduin.WithErrorHandler(bagMutatingErrorHandler(t, func(ctx context.Context, _ int) error {
			return anduin.Redirect(ctx, "/sign-in", http.StatusSeeOther)
		})))
		rt.Use(jar.Middleware())
		rt.Get("/", func(ctx context.Context) error {
			if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
				return err
			}
			return anduin.BadRequest(errors.New("redirect route error"))
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
		if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/sign-in" {
			t.Fatalf(
				"response = %d Location %q, want 303 /sign-in",
				recorder.Code,
				recorder.Header().Get("Location"),
			)
		}
		assertSavedBag(t, jar, recorder, store, http.StatusSeeOther)
	})
}

// TestForbiddenErrorClearsFlashAndSavesDirtySession pins the 403 control: the
// ErrorHandler may read the bag and dirty the session, but a 403 is not a flash
// status, so the flash is cleared rather than persisted.
func TestForbiddenErrorClearsFlashAndSavesDirtySession(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))

	seedRecorder := httptest.NewRecorder()
	seedRequest := httptest.NewRequest(http.MethodGet, "/seed", nil)
	if err := anduin.Serve(seedRecorder, seedRequest, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
			return err
		}
		if err := AddFlash(ctx, FlashInfo, "old flash"); err != nil {
			return err
		}
		return anduin.Redirect(ctx, "/seed", http.StatusSeeOther)
	})); err != nil {
		t.Fatal(err)
	}
	seedCookies := seedRecorder.Result().Cookies()
	sessionCookie := cookieByName(seedCookies, "app")
	flashCookie := cookieByName(seedCookies, "app_flash")
	if sessionCookie == nil || flashCookie == nil {
		t.Fatalf("seed cookies = %v, want app and app_flash", cookieNames(seedCookies))
	}

	rt := anduin.NewRouter(
		anduin.WithHosts(newKiksHosts(t)),
		anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			app, getErr := Get[*testApp](ctx)
			if getErr != nil {
				t.Errorf("ErrorHandler Get session: %v", getErr)
			}
			if app == nil || app.UserID != "before" {
				t.Errorf("ErrorHandler session = %+v, want the loaded session", app)
			}
			if setErr := Set(ctx, &testApp{UserID: "after", IsAuthenticated: true}); setErr != nil {
				return setErr
			}
			if flashErr := AddFlash(ctx, FlashError, "must clear"); flashErr != nil {
				return flashErr
			}
			return anduin.Text(ctx, http.StatusForbidden, "forbidden page")
		}),
	)
	rt.Use(jar.Middleware())
	admin := rt.Host(kiksAdminHost)
	admin.Isolate()
	admin.Use(func(next anduin.Handler) anduin.Handler {
		return func(context.Context) error {
			return anduin.Forbidden(errors.New("admin denied"))
		}
	})
	admin.Get("/admin", func(context.Context) error {
		return errors.New("admin handler must not run")
	})

	recorder := serveKiksRouter(rt, http.MethodGet, kiksAdminAuthority, "/admin", sessionCookie, flashCookie)
	if recorder.Code != http.StatusForbidden || recorder.Body.String() != "forbidden page" {
		t.Fatalf("response = %d %q, want 403 forbidden page", recorder.Code, recorder.Body.String())
	}

	cookies := recorder.Result().Cookies()
	if appCookies := cookiesNamed(cookies, "app"); len(appCookies) != 1 {
		t.Fatalf("app cookies = %d, want exactly 1 (%v)", len(appCookies), cookieNames(cookies))
	}
	if updated := cookieByName(cookies, "app"); updated == nil || updated.MaxAge < 0 {
		t.Fatalf("app cookie = %+v, want the dirty session saved", updated)
	}
	if cleared := cookiesNamed(cookies, "app_flash"); len(cleared) != 1 {
		t.Fatalf("flash cookies = %d, want exactly one clear (%v)", len(cleared), cookieNames(cookies))
	}
	clearedFlash := cookieByName(cookies, "app_flash")
	if clearedFlash.MaxAge >= 0 {
		t.Fatalf("flash cookie = %+v, want a cleared flash", clearedFlash)
	}

	follow := httptest.NewRequest(http.MethodGet, "/follow", nil)
	follow.AddCookie(cookieByName(cookies, "app"))
	var (
		gotApp     *testApp
		gotFlashes []FlashMessage
	)
	if err := anduin.Serve(httptest.NewRecorder(), follow, jar.Middleware()(func(ctx context.Context) error {
		var getErr error
		gotApp, getErr = Get[*testApp](ctx)
		if getErr != nil {
			return getErr
		}
		gotFlashes = FlashMessages(ctx)
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
	if gotApp == nil || gotApp.UserID != "after" {
		t.Fatalf("persisted session = %+v, want the ErrorHandler's mutation", gotApp)
	}
	if len(gotFlashes) != 0 {
		t.Fatalf("flashes survived a 403 = %+v, want a clear", gotFlashes)
	}
}

// TestCatchErrorsInsideKiksPersistsBagMutations pins the required ordering for
// an explicit delayed scope: kiks must be outside CatchErrors so the scope
// consumer runs with the bag attached and the first response write still goes
// through the kiks persistWriter.
func TestCatchErrorsInsideKiksPersistsBagMutations(t *testing.T) {
	t.Run("inside kiks", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))
		renders := 0
		var consumerErr error
		rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			return anduin.Text(ctx, http.StatusInternalServerError, "must not render")
		}))
		rt.Use(jar.Middleware())
		rt.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				if err == nil {
					return nil
				}
				app, getErr := Get[*testApp](ctx)
				if getErr != nil {
					consumerErr = getErr
					return err
				}
				if app == nil || app.UserID != "before" {
					consumerErr = errors.New("consumer did not see the bag value")
					return err
				}
				if setErr := Set(ctx, &testApp{UserID: "after", IsAuthenticated: true}); setErr != nil {
					consumerErr = setErr
					return err
				}
				if flashErr := AddFlash(ctx, FlashSuccess, "caught flash"); flashErr != nil {
					consumerErr = flashErr
					return err
				}
				consumerErr = anduin.Text(ctx, http.StatusConflict, "caught conflict")
				return nil
			}
		}))
		rt.Use(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
					return err
				}
				return anduin.Conflict(errors.New("guard conflict"))
			}
		})
		rt.Get("/", func(context.Context) error {
			return errors.New("handler must not run")
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
		if consumerErr != nil {
			t.Fatalf("CatchErrors consumer: %v", consumerErr)
		}
		if renders != 0 {
			t.Fatalf("ErrorHandler renders = %d, want 0 for a consumed error", renders)
		}
		if recorder.Code != http.StatusConflict || recorder.Body.String() != "caught conflict" {
			t.Fatalf("response = %d %q, want the consumer's 409", recorder.Code, recorder.Body.String())
		}
		if saves := store.saveCount(); saves != 1 {
			t.Fatalf("session saves = %d, want exactly 1", saves)
		}
		cookies := recorder.Result().Cookies()
		if flashCookies := cookiesNamed(cookies, "app_flash"); len(flashCookies) != 1 {
			t.Fatalf("flash cookies = %d, want exactly 1 (%v)", len(flashCookies), cookieNames(cookies))
		}
		flashCookie := cookieByName(cookies, "app_flash")
		if flashCookie.MaxAge < 0 {
			t.Fatalf("flash cookie = %+v, want the consumer's flash persisted", flashCookie)
		}

		follow := httptest.NewRequest(http.MethodGet, "/follow", nil)
		follow.AddCookie(cookieByName(cookies, "app"))
		follow.AddCookie(flashCookie)
		var (
			gotApp     *testApp
			gotFlashes []FlashMessage
		)
		if err := anduin.Serve(httptest.NewRecorder(), follow, jar.Middleware()(func(ctx context.Context) error {
			var getErr error
			gotApp, getErr = Get[*testApp](ctx)
			if getErr != nil {
				return getErr
			}
			gotFlashes = FlashMessages(ctx)
			return anduin.NoContent(ctx, http.StatusOK)
		})); err != nil {
			t.Fatal(err)
		}
		if gotApp == nil || gotApp.UserID != "after" {
			t.Fatalf("persisted session = %+v, want the consumer's mutation", gotApp)
		}
		if len(gotFlashes) != 1 || gotFlashes[0].Message != "caught flash" {
			t.Fatalf("persisted flashes = %+v, want the consumer's flash", gotFlashes)
		}
	})

	t.Run("outside kiks has no bag and saves before the consumer", func(t *testing.T) {
		jar, store := newCountingJar(t, NewSession[*testApp]("app", MaxAge(60)))

		seedRecorder := httptest.NewRecorder()
		seedRequest := httptest.NewRequest(http.MethodGet, "/seed", nil)
		if err := anduin.Serve(seedRecorder, seedRequest, jar.Middleware()(func(ctx context.Context) error {
			if err := AddFlash(ctx, FlashInfo, "incoming flash"); err != nil {
				return err
			}
			return anduin.Redirect(ctx, "/seed", http.StatusSeeOther)
		})); err != nil {
			t.Fatal(err)
		}
		incomingFlash := cookieByName(seedRecorder.Result().Cookies(), "app_flash")
		if incomingFlash == nil || incomingFlash.MaxAge < 0 {
			t.Fatalf("seed flash = %+v, want a valid incoming flash", incomingFlash)
		}
		if saves := store.saveCount(); saves != 0 {
			t.Fatalf("seed session saves = %d, want 0 before the error request", saves)
		}

		renders := 0
		var (
			consumerAddFlash error
			savesAtConsumer  = -1
		)
		rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
			renders++
			return anduin.Text(ctx, http.StatusConflict, "rendered conflict")
		}))
		rt.Use(anduin.CatchErrors(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				err := next(ctx)
				if err == nil {
					return nil
				}
				// The outer scope sees the error only after kiks has unwound.
				// This save count proves persistence already happened, and the
				// expired incoming flash below proves it used status 200, not
				// the later 409 render.
				savesAtConsumer = store.saveCount()
				consumerAddFlash = AddFlash(ctx, FlashError, "too late")
				return err
			}
		}))
		rt.Use(jar.Middleware())
		rt.Use(func(next anduin.Handler) anduin.Handler {
			return func(ctx context.Context) error {
				if err := Set(ctx, &testApp{UserID: "before", IsAuthenticated: true}); err != nil {
					return err
				}
				return anduin.Conflict(errors.New("guard conflict"))
			}
		})
		rt.Get("/", func(context.Context) error {
			return errors.New("handler must not run")
		})

		recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/", incomingFlash)
		if savesAtConsumer != 1 {
			t.Fatalf("session saves when the outer consumer runs = %d, want 1 before the consumer", savesAtConsumer)
		}
		if !errors.Is(consumerAddFlash, ErrNoBag) {
			t.Fatalf("AddFlash outside kiks = %v, want ErrNoBag", consumerAddFlash)
		}
		if renders != 1 || recorder.Code != http.StatusConflict {
			t.Fatalf("renders = %d, status = %d; want one 409 render", renders, recorder.Code)
		}
		if saves := store.saveCount(); saves != 1 {
			t.Fatalf("session saves after render = %d, want the pre-consumer kiks save only", saves)
		}
		flashCookies := cookiesNamed(recorder.Result().Cookies(), "app_flash")
		if len(flashCookies) != 1 {
			t.Fatalf("flash cookies = %d, want exactly the pre-consumer clear (%v)", len(flashCookies), cookieNames(recorder.Result().Cookies()))
		}
		if flashCookies[0].MaxAge >= 0 {
			t.Fatalf(
				"flash cookie = %+v, want the pre-consumer status 200 save to clear the incoming flash",
				flashCookies[0],
			)
		}
	})
}

// TestAddFlashAfterResponseWriteIsNotPersisted documents existing kiks timing:
// persistWriter saves on the first response write, so a flash added after the
// response has started is visible on the request bag but is not sent to the
// client. Add flashes before writing the response.
func TestAddFlashAfterResponseWriteIsNotPersisted(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))
	rt := anduin.NewRouter()
	rt.Use(jar.Middleware())
	rt.Get("/", func(ctx context.Context) error {
		if err := anduin.Redirect(ctx, "/next", http.StatusSeeOther); err != nil {
			return err
		}
		// Too late: the redirect response already made persistWriter save.
		if err := AddFlash(ctx, FlashSuccess, "too late"); err != nil {
			return err
		}
		return nil
	})

	recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", recorder.Code)
	}
	if flashCookie := cookieByName(recorder.Result().Cookies(), "app_flash"); flashCookie != nil {
		t.Fatalf("flash cookie = %+v, want no persisted flash after the write", flashCookie)
	}
}

// TestLoadFailureLeavesNoBagOrCookies pins that a middleware load failure runs
// the ErrorHandler without a kiks bag or persistWriter. Bag access fails with
// ErrNoBag and the failed load must not emit any cookies.
func TestLoadFailureLeavesNoBagOrCookies(t *testing.T) {
	keys := testKeys()
	inner, err := NewCookieStore(keys)
	if err != nil {
		t.Fatal(err)
	}
	loadErr := errors.New("load failed")
	store := &countingStore{inner: failingLoadStore{inner: inner, err: loadErr}}
	jar, err := NewJar(keys, store, NewSession[*testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}

	var (
		handlerCalled bool
		observedErr   error
		getErr        error
		flashErr      error
		setErr        error
	)
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		observedErr = err
		_, getErr = Get[*testApp](ctx)
		flashErr = AddFlash(ctx, FlashInfo, "lost")
		setErr = Set(ctx, &testApp{UserID: "lost", IsAuthenticated: true})
		return anduin.Text(ctx, http.StatusInternalServerError, "load failed")
	}))
	rt.Use(jar.Middleware())
	rt.Get("/", func(context.Context) error {
		handlerCalled = true
		return nil
	})

	recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
	if handlerCalled {
		t.Fatal("handler ran after a kiks load failure")
	}
	if !errors.Is(observedErr, loadErr) {
		t.Fatalf("ErrorHandler error = %v, want the load error", observedErr)
	}
	if !errors.Is(getErr, ErrNoBag) {
		t.Fatalf("Get error = %v, want ErrNoBag", getErr)
	}
	if !errors.Is(flashErr, ErrNoBag) {
		t.Fatalf("AddFlash error = %v, want ErrNoBag", flashErr)
	}
	if !errors.Is(setErr, ErrNoBag) {
		t.Fatalf("Set error = %v, want ErrNoBag", setErr)
	}
	if recorder.Code != http.StatusInternalServerError || recorder.Body.String() != "load failed" {
		t.Fatalf("response = %d %q, want the ErrorHandler's 500", recorder.Code, recorder.Body.String())
	}
	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("load failure emitted cookies = %v, want none", cookieNames(cookies))
	}
	if saves := store.saveCount(); saves != 0 {
		t.Fatalf("session saves = %d, want none after a load failure", saves)
	}
}

// TestSaveFailureFailsClosedAndLogsJoinedErrorOnce pins persist failure
// behavior after an early host/global render: the ErrorHandler's response body
// is suppressed because the save failed, kiks preserves the original error by
// joining the persist error, and the router logs that final changed error once
// without rendering a second response.
func TestSaveFailureFailsClosedAndLogsJoinedErrorOnce(t *testing.T) {
	capture := captureDefaultLogger(t)

	saveErr := errors.New("save failed")
	guardErr := errors.New("guard rejected for save failure")
	keys := testKeys()
	inner, err := NewCookieStore(keys)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := NewJar(keys, failSaveStore{inner: inner, err: saveErr}, NewSession[*testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}

	renders := 0
	var renderWriteErr error
	rt := anduin.NewRouter(anduin.WithErrorHandler(func(ctx context.Context, err error) error {
		renders++
		if _, getErr := Get[*testApp](ctx); getErr != nil {
			t.Errorf("ErrorHandler Get session: %v", getErr)
		}
		// The save failure is observable on the ErrorHandler's write. Return
		// nil so the render itself is not replaced by a fallback; kiks still
		// records the persist error and joins it with the original.
		renderWriteErr = anduin.Text(ctx, anduin.StatusCode(err), "error page")
		return nil
	}))
	rt.Use(jar.Middleware())
	rt.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			if setErr := Set(ctx, &testApp{UserID: "save-failure", IsAuthenticated: true}); setErr != nil {
				return setErr
			}
			return anduin.Forbidden(guardErr)
		}
	})
	rt.Get("/", func(context.Context) error {
		return errors.New("handler must not run")
	})

	recorder := serveKiksRouter(rt, http.MethodGet, kiksPrimaryAuthority, "/")
	if renders != 1 {
		t.Fatalf("ErrorHandler renders = %d, want 1", renders)
	}
	if !errors.Is(renderWriteErr, saveErr) {
		t.Fatalf("ErrorHandler write error = %v, want the save failure observable", renderWriteErr)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want the first rendered 403", recorder.Code)
	}
	if body := recorder.Body.String(); body != "" {
		t.Fatalf("body = %q, want the failed save to suppress the body", body)
	}
	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("failed save emitted cookies = %v, want none", cookieNames(cookies))
	}

	records := capture.recordsFor("anduin: error changed after the response was rendered")
	if len(records) != 1 {
		t.Fatalf("post-render changed-error logs = %d, want 1", len(records))
	}
	if all := capture.all(); len(all) != 1 {
		t.Fatalf("total log records = %d, want only the joined-error record", len(all))
	}
	joined := logAttribute(records[0], "error")
	if !strings.Contains(joined, guardErr.Error()) || !strings.Contains(joined, saveErr.Error()) {
		t.Fatalf("logged final error = %q, want the joined original and save error", joined)
	}
	if rendered := logAttribute(records[0], "rendered_error"); !strings.Contains(rendered, guardErr.Error()) {
		t.Fatalf("logged rendered error = %q, want the original guard error", rendered)
	}
}

// TestAuthOnlyStyleHostRedirectStillPersistsBag pins the site's successful
// AuthOnly rejection path: a host middleware destroys the session, stores the
// return target, writes a 303 and returns nil. kiks must still persist those
// mutations with the redirect status, exactly as it did before the coordinated
// error lifecycle.
func TestAuthOnlyStyleHostRedirectStillPersistsBag(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app", MaxAge(60)),
		Bagged(Encrypted[*returnTarget]("app_return_to", HTTPOnly(), MaxAge(600))),
	)

	seedRecorder := httptest.NewRecorder()
	seedRequest := httptest.NewRequest(http.MethodGet, "/seed", nil)
	if err := anduin.Serve(seedRecorder, seedRequest, jar.Middleware()(func(ctx context.Context) error {
		return Set(ctx, &testApp{UserID: "user-1", IsAuthenticated: true})
	})); err != nil {
		t.Fatal(err)
	}
	sessionCookie := cookieByName(seedRecorder.Result().Cookies(), "app")
	if sessionCookie == nil {
		t.Fatal("seed session cookie missing")
	}

	rt := anduin.NewRouter(anduin.WithHosts(newKiksHosts(t)))
	rt.Use(jar.Middleware())
	admin := rt.Host(kiksAdminHost)
	admin.Isolate()
	admin.Use(func(next anduin.Handler) anduin.Handler {
		return func(ctx context.Context) error {
			session, err := Get[*testApp](ctx)
			if err != nil {
				return err
			}
			if session != nil {
				if err := Destroy[*testApp](ctx); err != nil {
					return err
				}
			}
			target := &returnTarget{
				Host: anduin.RequestHost(ctx),
				Path: anduin.Request(ctx).URL.RequestURI(),
			}
			if err := Set(ctx, target); err != nil {
				return err
			}
			return anduin.Redirect(ctx, "https://andurel.com/session/new", http.StatusSeeOther)
		}
	})
	admin.Get("/dashboard", func(context.Context) error {
		return errors.New("rejected handler must not run")
	})

	recorder := serveKiksRouter(
		rt,
		http.MethodGet,
		kiksAdminAuthority,
		"/dashboard?tab=settings",
		sessionCookie,
	)
	if recorder.Code != http.StatusSeeOther ||
		recorder.Header().Get("Location") != "https://andurel.com/session/new" {
		t.Fatalf(
			"response = %d Location %q, want 303 to the primary sign-in",
			recorder.Code,
			recorder.Header().Get("Location"),
		)
	}

	cookies := recorder.Result().Cookies()
	if expired := cookieByName(cookies, "app"); expired == nil || expired.MaxAge >= 0 {
		t.Fatalf("session cookie = %+v, want it destroyed", expired)
	}
	returnCookie := cookieByName(cookies, "app_return_to")
	if returnCookie == nil || returnCookie.MaxAge < 0 {
		t.Fatalf("return-to cookie = %+v, want it persisted with the redirect", returnCookie)
	}

	follow := httptest.NewRequest(http.MethodGet, "/session/new", nil)
	follow.AddCookie(returnCookie)
	var (
		gotSession *testApp
		gotTarget  *returnTarget
	)
	if err := anduin.Serve(httptest.NewRecorder(), follow, jar.Middleware()(func(ctx context.Context) error {
		var getErr error
		gotSession, getErr = Get[*testApp](ctx)
		if getErr != nil {
			return getErr
		}
		gotTarget, getErr = Get[*returnTarget](ctx)
		if getErr != nil {
			return getErr
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
	if gotSession != nil {
		t.Fatalf("session after AuthOnly rejection = %+v, want nil", gotSession)
	}
	if gotTarget == nil || gotTarget.Host != kiksAdminHost || gotTarget.Path != "/dashboard?tab=settings" {
		t.Fatalf("return target = %+v, want the admin dashboard path", gotTarget)
	}
}
