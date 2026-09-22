package kiks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
)

type testApp struct {
	UserID          string `json:"user_id,omitempty"`
	IsAuthenticated bool   `json:"is_authenticated,omitempty"`
}

func (a *testApp) MarshalCookie() ([]byte, error) {
	return json.Marshal(a)
}

func (a *testApp) UnmarshalCookie(data []byte) error {
	if len(data) == 0 {
		*a = testApp{}
		return nil
	}
	return json.Unmarshal(data, a)
}

type theme string

func (t *theme) MarshalCookie() ([]byte, error) {
	return []byte(*t), nil
}

func (t *theme) UnmarshalCookie(data []byte) error {
	*t = theme(data)
	return nil
}

func testKeys() Keys {
	return Keys{
		Authentication: []byte("01234567890123456789012345678901"),
		Encryption:     []byte("01234567890123456789012345678901"),
	}
}

func testJar(t *testing.T, defs ...Definition) *Jar {
	t.Helper()
	keys := testKeys()
	store, err := NewCookieStore(keys)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := NewJar(keys, store, defs...)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestCookieDriverRoundTrip(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", HTTPOnly(), MaxAge(60)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	handler := jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		AddFlash(c.Request().Context(), FlashSuccess, "saved")
		return c.Redirect(http.StatusSeeOther, "/next")
	})
	if err := handler(ctx); err != nil {
		t.Fatal(err)
	}

	cookies := recorder.Result().Cookies()
	appCookie := cookieByName(cookies, "app")
	flashCookie := cookieByName(cookies, "app_flash")
	if appCookie == nil || flashCookie == nil {
		t.Fatalf("cookies = %v, want app + app_flash", cookieNames(cookies))
	}

	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(appCookie)
	follow.AddCookie(flashCookie)
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	var got *testApp
	var flashes []FlashMessage
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		var err error
		got, err = Get[*testApp](c.Request().Context())
		if err != nil {
			return err
		}
		flashes = Flashes(c.Request().Context())
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.IsAuthenticated || got.UserID != "u1" {
		t.Fatalf("app = %+v", got)
	}
	if len(flashes) != 1 || flashes[0].Message != "saved" {
		t.Fatalf("flashes = %+v", flashes)
	}

	nextCookies := followRecorder.Result().Cookies()
	clearedFlash := cookieByName(nextCookies, "app_flash")
	if clearedFlash == nil || clearedFlash.MaxAge >= 0 {
		t.Fatalf("expected expired flash cookie, got %v", nextCookies)
	}
	third := httptest.NewRequest(http.MethodGet, "/", nil)
	third.AddCookie(appCookie)
	thirdRecorder := httptest.NewRecorder()
	thirdCtx := echo.New().NewContext(third, thirdRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		if got := Flashes(c.Request().Context()); len(got) != 0 {
			t.Fatalf("flashes persisted onto the next page: %v", got)
		}
		return c.NoContent(http.StatusOK)
	})(thirdCtx); err != nil {
		t.Fatal(err)
	}
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names
}

func TestSameRequestFlashIsVisible(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		AddFlash(c.Request().Context(), FlashError, "nope")
		got := Flashes(c.Request().Context())
		if len(got) != 1 || got[0].Message != "nope" {
			t.Fatalf("same-request flashes = %+v", got)
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptSessionIsRecovered(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "app", Value: "not-a-valid-cookie"})
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	called := false
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		called = true
		app, err := Get[*testApp](c.Request().Context())
		if err != nil {
			return err
		}
		if app != nil && app.IsAuthenticated {
			t.Fatal("corrupt cookie should yield nil App")
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not called")
	}
}

func TestSkipPrefixes(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware(SkipPrefixes("/assets"))(func(c *echo.Context) error {
		if bagFrom(c.Request().Context()) != nil {
			t.Fatal("skipped path should not have a bag")
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClientRedirectPersistsFlashes(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	handler := jar.EchoMiddleware()(func(c *echo.Context) error {
		AddFlash(c.Request().Context(), FlashSuccess, "next")
		c.Response().Header().Set(ClientRedirectHeader, "1")
		c.Response().WriteHeader(http.StatusOK)
		return nil
	})
	if err := handler(ctx); err != nil {
		t.Fatal(err)
	}
	flashCookie := cookieByName(recorder.Result().Cookies(), "app_flash")
	if flashCookie == nil {
		t.Fatal("missing flash cookie")
	}
	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(flashCookie)
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		got := Flashes(c.Request().Context())
		if len(got) != 1 || got[0].Message != "next" {
			t.Fatalf("redirect flashes = %+v", got)
		}
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
}

func TestNamedEncryptedCookieLazy(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app"),
		Bagged(Encrypted[*theme]("theme", HTTPOnly())),
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	dark := theme("dark")
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &dark); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
	themeCookie := cookieByName(recorder.Result().Cookies(), "theme")
	if themeCookie == nil {
		t.Fatal("missing theme cookie")
	}
	follow := httptest.NewRequest(http.MethodGet, "/", nil)
	follow.AddCookie(themeCookie)
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		exists, err := Exists[*theme](c.Request().Context())
		if err != nil {
			return err
		}
		if !exists {
			t.Fatal("theme should Exist after lazy Get path")
		}
		got, err := Get[*theme](c.Request().Context())
		if err != nil {
			return err
		}
		if got == nil || *got != "dark" {
			t.Fatalf("theme = %v", got)
		}
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
}

func TestDestroySession(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
	appCookie := cookieByName(recorder.Result().Cookies(), "app")
	if appCookie == nil {
		t.Fatal("missing app cookie")
	}

	destroyRecorder := httptest.NewRecorder()
	destroyReq := httptest.NewRequest(http.MethodGet, "/", nil)
	destroyReq.AddCookie(appCookie)
	destroyCtx := echo.New().NewContext(destroyReq, destroyRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		exists, err := Exists[*testApp](c.Request().Context())
		if err != nil {
			return err
		}
		if !exists {
			t.Fatal("expected session to exist")
		}
		if err := Destroy[*testApp](c.Request().Context()); err != nil {
			return err
		}
		exists, err = Exists[*testApp](c.Request().Context())
		if err != nil {
			return err
		}
		if exists {
			t.Fatal("expected session destroyed")
		}
		return c.NoContent(http.StatusOK)
	})(destroyCtx); err != nil {
		t.Fatal(err)
	}
	expired := cookieByName(destroyRecorder.Result().Cookies(), "app")
	if expired == nil || expired.MaxAge >= 0 {
		t.Fatalf("expected expired app cookie, got %+v", expired)
	}
}

func TestSetDestroyWithoutBag(t *testing.T) {
	if _, err := Get[*testApp](context.Background()); !errors.Is(err, ErrNoBag) {
		t.Fatalf("Get error = %v, want ErrNoBag", err)
	}
	if _, err := Exists[*testApp](context.Background()); !errors.Is(err, ErrNoBag) {
		t.Fatalf("Exists error = %v, want ErrNoBag", err)
	}
	if err := Set(context.Background(), &testApp{}); !errors.Is(err, ErrNoBag) {
		t.Fatalf("Set error = %v, want ErrNoBag", err)
	}
	if err := Destroy[*testApp](context.Background()); !errors.Is(err, ErrNoBag) {
		t.Fatalf("Destroy error = %v, want ErrNoBag", err)
	}
}

func TestGetUnknownType(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		// Value type when the jar registered *testApp.
		if _, err := Get[testApp](c.Request().Context()); !errors.Is(err, ErrUnknownType) {
			t.Fatalf("Get[testApp] error = %v, want ErrUnknownType", err)
		}
		if _, err := Exists[testApp](c.Request().Context()); !errors.Is(err, ErrUnknownType) {
			t.Fatalf("Exists[testApp] error = %v, want ErrUnknownType", err)
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestNativeCookieAPI(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app"),
		Encrypted[*theme]("theme", HTTPOnly(), MaxAge(3600)),
	)
	ctx := context.Background()

	if _, err := Read[*theme](ctx, nil, nil); err == nil {
		t.Fatal("Read(nil jar) want error")
	}
	if _, err := Read[*testApp](ctx, jar, nil); !errors.Is(err, ErrBagCookie) {
		t.Fatalf("Read(session) = %v, want ErrBagCookie", err)
	}
	if _, err := Read[*theme](ctx, testJar(t, NewSession[*testApp]("app")), nil); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("Read(unregistered) = %v, want ErrUnknownType", err)
	}

	recorder := httptest.NewRecorder()
	dark := theme("dark")
	if err := Write(ctx, jar, recorder, &dark); err != nil {
		t.Fatal(err)
	}
	themeCookie := cookieByName(recorder.Result().Cookies(), "theme")
	if themeCookie == nil {
		t.Fatal("missing theme cookie")
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(themeCookie)
	got, err := Read[*theme](ctx, jar, request)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != "dark" {
		t.Fatalf("theme = %v", got)
	}

	clearRecorder := httptest.NewRecorder()
	if err := Clear[*theme](ctx, jar, clearRecorder); err != nil {
		t.Fatal(err)
	}
	expired := cookieByName(clearRecorder.Result().Cookies(), "theme")
	if expired == nil || expired.MaxAge >= 0 {
		t.Fatalf("expected expired theme cookie, got %+v", expired)
	}
	if err := Clear[*testApp](ctx, jar, clearRecorder); !errors.Is(err, ErrBagCookie) {
		t.Fatalf("Clear(session) = %v, want ErrBagCookie", err)
	}
}

func TestNativePlainCookie(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app"),
		Plain[*theme]("theme", HTTPOnly()),
	)
	ctx := context.Background()
	recorder := httptest.NewRecorder()
	light := theme("light")
	if err := Write(ctx, jar, recorder, &light); err != nil {
		t.Fatal(err)
	}
	themeCookie := cookieByName(recorder.Result().Cookies(), "theme")
	if themeCookie == nil {
		t.Fatal("missing theme cookie")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(themeCookie)
	got, err := Read[*theme](ctx, jar, request)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != "light" {
		t.Fatalf("theme = %v", got)
	}
}

func TestBagRejectsNative(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app"),
		Encrypted[*theme]("theme", HTTPOnly()),
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		dark := theme("x")
		if err := Set(c.Request().Context(), &dark); !errors.Is(err, ErrNotBag) {
			t.Fatalf("Set(native) = %v, want ErrNotBag", err)
		}
		if _, err := Get[*theme](c.Request().Context()); !errors.Is(err, ErrNotBag) {
			t.Fatalf("Get(native) = %v, want ErrNotBag", err)
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
}

type memSessionDB struct {
	mu      sync.Mutex
	rows    map[string]memSessionRow
	loadErr error
}

type memSessionRow struct {
	payload   []byte
	expiresAt time.Time
}

func newMemSessionDB() *memSessionDB {
	return &memSessionDB{rows: make(map[string]memSessionRow)}
}

func (db *memSessionDB) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	switch {
	case len(args) == 3:
		id, _ := args[0].(string)
		payload, _ := args[1].([]byte)
		expiresAt, _ := args[2].(time.Time)
		db.rows[id] = memSessionRow{payload: append([]byte(nil), payload...), expiresAt: expiresAt}
	case len(args) == 1:
		id, _ := args[0].(string)
		delete(db.rows, id)
	default:
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}
	_ = query
	return pgconn.CommandTag{}, nil
}

type memScanner struct {
	row memSessionRow
	err error
}

func (s memScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	if len(dest) != 1 {
		return errors.New("expected one dest")
	}
	ptr, ok := dest[0].(*[]byte)
	if !ok {
		return errors.New("expected *[]byte")
	}
	*ptr = append([]byte(nil), s.row.payload...)
	return nil
}

func (db *memSessionDB) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.loadErr != nil {
		return memScanner{err: db.loadErr}
	}
	id, _ := args[0].(string)
	row, ok := db.rows[id]
	if !ok || time.Now().UTC().After(row.expiresAt) {
		return memScanner{err: pgx.ErrNoRows}
	}
	return memScanner{row: row}
}

func TestDatabaseStoreRoundTrip(t *testing.T) {
	keys := testKeys()
	db := newMemSessionDB()
	store, err := newDatabaseStore(keys, db)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := NewJar(keys, store, NewSession[*testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &testApp{UserID: "db1", IsAuthenticated: true}); err != nil {
			return err
		}
		AddFlash(c.Request().Context(), FlashInfo, "hi")
		return c.Redirect(http.StatusSeeOther, "/next")
	})(ctx); err != nil {
		t.Fatal(err)
	}
	appCookie := cookieByName(recorder.Result().Cookies(), "app")
	flashCookie := cookieByName(recorder.Result().Cookies(), "app_flash")
	if appCookie == nil || flashCookie == nil {
		t.Fatalf("cookies = %v", cookieNames(recorder.Result().Cookies()))
	}
	if len(db.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(db.rows))
	}

	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(appCookie)
	follow.AddCookie(flashCookie)
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		got, err := Get[*testApp](c.Request().Context())
		if err != nil {
			return err
		}
		if got == nil || got.UserID != "db1" || !got.IsAuthenticated {
			t.Fatalf("app = %+v", got)
		}
		if flashes := Flashes(c.Request().Context()); len(flashes) != 1 {
			t.Fatalf("flashes = %+v", flashes)
		}
		if err := Destroy[*testApp](c.Request().Context()); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
	if len(db.rows) != 0 {
		t.Fatalf("rows after destroy = %d", len(db.rows))
	}
}

func TestStoreLoadInfraErrorPropagates(t *testing.T) {
	keys := testKeys()
	db := newMemSessionDB()
	store, err := newDatabaseStore(keys, db)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := NewJar(keys, store, NewSession[*testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		return Set(c.Request().Context(), &testApp{UserID: "u1", IsAuthenticated: true})
	})(ctx); err != nil {
		t.Fatal(err)
	}
	appCookie := cookieByName(recorder.Result().Cookies(), "app")
	if appCookie == nil {
		t.Fatal("missing app cookie")
	}

	db.loadErr = errors.New("connection refused")
	failReq := httptest.NewRequest(http.MethodGet, "/", nil)
	failReq.AddCookie(appCookie)
	failRecorder := httptest.NewRecorder()
	failCtx := echo.New().NewContext(failReq, failRecorder)
	called := false
	err = jar.EchoMiddleware()(func(c *echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})(failCtx)
	if err == nil {
		t.Fatal("expected load infra error")
	}
	if !errors.Is(err, db.loadErr) {
		t.Fatalf("error = %v, want wrapped connection refused", err)
	}
	if called {
		t.Fatal("handler should not run on load infra failure")
	}
}

type failSaveStore struct {
	inner Store
	err   error
}

func (s failSaveStore) Load(r *http.Request, name string) ([]byte, error) {
	return s.inner.Load(r, name)
}

func (s failSaveStore) Save(w http.ResponseWriter, r *http.Request, name string, payload []byte, attrs CookieAttrs) error {
	if s.err != nil {
		return s.err
	}
	return s.inner.Save(w, r, name, payload, attrs)
}

func (s failSaveStore) Destroy(w http.ResponseWriter, r *http.Request, name string, attrs CookieAttrs) error {
	return s.inner.Destroy(w, r, name, attrs)
}

func TestPersistSaveErrorFailsRequest(t *testing.T) {
	keys := testKeys()
	inner, err := NewCookieStore(keys)
	if err != nil {
		t.Fatal(err)
	}
	saveErr := errors.New("encode failed")
	jar, err := NewJar(keys, failSaveStore{inner: inner, err: saveErr}, NewSession[*testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	err = jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})(ctx)
	if err == nil {
		t.Fatal("expected persist error")
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("error = %v, want encode failed", err)
	}
}

type failMarshalApp struct {
	testApp
}

func (a *failMarshalApp) MarshalCookie() ([]byte, error) {
	return nil, errors.New("marshal boom")
}

func (a *failMarshalApp) UnmarshalCookie(data []byte) error {
	return a.testApp.UnmarshalCookie(data)
}

func TestPersistMarshalErrorFailsRequest(t *testing.T) {
	jar := testJar(t, NewSession[*failMarshalApp]("app", MaxAge(60)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	err := jar.EchoMiddleware()(func(c *echo.Context) error {
		if err := Set(c.Request().Context(), &failMarshalApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})(ctx)
	if err == nil {
		t.Fatal("expected marshal persist error")
	}
	if err.Error() != "marshal boom" {
		t.Fatalf("error = %v, want marshal boom", err)
	}
}
