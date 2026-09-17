package kiks

import (
	"context"
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

func init() {
	gob.Register(testApp{})
}

type testApp struct {
	UserID          string
	IsAuthenticated bool
}

func testKeys() Keys {
	return Keys{
		Authentication: []byte("01234567890123456789012345678901"),
		Encryption:     []byte("01234567890123456789012345678901"),
	}
}

func TestCookieDriverRoundTrip(t *testing.T) {
	jar, err := New(testKeys(), CookieStore(), Session[testApp]("app", HTTPOnly(), MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	handler := jar.EchoMiddleware()(func(c *echo.Context) error {
		Set(c.Request().Context(), testApp{UserID: "u1", IsAuthenticated: true})
		AddFlash(c.Request().Context(), FlashSuccess, "saved")
		return c.Redirect(http.StatusSeeOther, "/next")
	})
	if err := handler(ctx); err != nil {
		t.Fatal(err)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}

	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(cookies[0])
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	var got testApp
	var flashes []FlashMessage
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		got = Get[testApp](c.Request().Context())
		flashes = Flashes(c.Request().Context())
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
	if !got.IsAuthenticated || got.UserID != "u1" {
		t.Fatalf("app = %+v", got)
	}
	if len(flashes) != 1 || flashes[0].Message != "saved" {
		t.Fatalf("flashes = %+v", flashes)
	}

	nextCookies := followRecorder.Result().Cookies()
	if len(nextCookies) != 1 {
		t.Fatalf("cleared-flash cookies = %d, want 1", len(nextCookies))
	}
	third := httptest.NewRequest(http.MethodGet, "/", nil)
	third.AddCookie(nextCookies[0])
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

func TestSameRequestFlashIsVisible(t *testing.T) {
	jar, err := New(testKeys(), CookieStore(), Session[testApp]("app"))
	if err != nil {
		t.Fatal(err)
	}
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
	jar, err := New(testKeys(), CookieStore(), Session[testApp]("app"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "app", Value: "not-a-valid-cookie"})
	recorder := httptest.NewRecorder()
	ctx := echo.New().NewContext(request, recorder)
	called := false
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		called = true
		if Get[testApp](c.Request().Context()).IsAuthenticated {
			t.Fatal("corrupt cookie should yield zero App")
		}
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not called")
	}
}

func TestSQLStoreRoundTrip(t *testing.T) {
	store := NewSQLStore(&memorySQL{rows: map[string][]byte{}})
	jar, err := New(testKeys(), store, Session[testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		Set(c.Request().Context(), testApp{UserID: "db", IsAuthenticated: true})
		return c.Redirect(http.StatusSeeOther, "/")
	})(ctx); err != nil {
		t.Fatal(err)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}

	follow := httptest.NewRequest(http.MethodGet, "/", nil)
	follow.AddCookie(cookies[0])
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		got := Get[testApp](c.Request().Context())
		if got.UserID != "db" || !got.IsAuthenticated {
			t.Fatalf("sql session = %+v", got)
		}
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
}

func TestSkipPrefixes(t *testing.T) {
	jar, err := New(testKeys(), CookieStore(), Session[testApp]("app"))
	if err != nil {
		t.Fatal(err)
	}
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
	jar, err := New(testKeys(), CookieStore(), Session[testApp]("app", MaxAge(60)))
	if err != nil {
		t.Fatal(err)
	}
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
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(cookies[0])
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

func TestNamedEncryptedCookie(t *testing.T) {
	type Theme string
	jar, err := New(
		testKeys(),
		CookieStore(),
		Session[testApp]("app"),
		Encrypted[Theme]("theme", HTTPOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := echo.New().NewContext(request, recorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		Set[Theme](c.Request().Context(), "dark")
		return c.NoContent(http.StatusOK)
	})(ctx); err != nil {
		t.Fatal(err)
	}
	var themeCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "theme" {
			themeCookie = cookie
		}
	}
	if themeCookie == nil {
		t.Fatal("missing theme cookie")
	}
	follow := httptest.NewRequest(http.MethodGet, "/", nil)
	follow.AddCookie(themeCookie)
	followRecorder := httptest.NewRecorder()
	followCtx := echo.New().NewContext(follow, followRecorder)
	if err := jar.EchoMiddleware()(func(c *echo.Context) error {
		got, ok := Lookup[Theme](c.Request().Context())
		if !ok || got != "dark" {
			t.Fatalf("theme = %q, ok = %t", got, ok)
		}
		return c.NoContent(http.StatusOK)
	})(followCtx); err != nil {
		t.Fatal(err)
	}
}

type memorySQL struct {
	mu   sync.Mutex
	rows map[string][]byte
}

func (db *memorySQL) Exec(_ context.Context, query string, args ...any) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if len(args) == 1 {
		delete(db.rows, args[0].(string))
		return nil
	}
	id := args[0].(string)
	payload := args[1].([]byte)
	db.rows[id] = payload
	_ = query
	if len(args) > 2 {
		_ = args[2].(time.Time)
	}
	return nil
}

func (db *memorySQL) QueryRow(_ context.Context, _ string, args ...any) Row {
	db.mu.Lock()
	defer db.mu.Unlock()
	id := args[0].(string)
	payload, ok := db.rows[id]
	return memoryRow{payload: payload, ok: ok}
}

type memoryRow struct {
	payload []byte
	ok      bool
}

func (row memoryRow) Scan(dest ...any) error {
	if !row.ok {
		return errNoRows{}
	}
	*(dest[0].(*[]byte)) = row.payload
	return nil
}

type errNoRows struct{}

func (errNoRows) Error() string { return "no rows in result set" }
