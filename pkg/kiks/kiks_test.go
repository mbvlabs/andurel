package kiks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mbvlabs/andurel/pkg/anduin"
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

// jsonApp has no Cookie methods; kiks encodes it with encoding/json.
type jsonApp struct {
	UserID          string `json:"user_id,omitempty"`
	IsAuthenticated bool   `json:"is_authenticated,omitempty"`
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
	handler := jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		AddFlash(ctx, FlashSuccess, "saved")
		return anduin.Redirect(ctx, "/next", http.StatusSeeOther)
	})
	if err := anduin.Serve(recorder, request, handler); err != nil {
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
	var got *testApp
	var flashes []FlashMessage
	if err := anduin.Serve(followRecorder, follow, jar.Middleware()(func(ctx context.Context) error {
		var err error
		got, err = Get[*testApp](ctx)
		if err != nil {
			return err
		}
		flashes = Flashes(ctx)
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(thirdRecorder, third, jar.Middleware()(func(ctx context.Context) error {
		if got := Flashes(ctx); len(got) != 0 {
			t.Fatalf("flashes persisted onto the next page: %v", got)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
}

func TestConstructorsRejectInvalidEncryptionKeyLength(t *testing.T) {
	valid := testKeys()
	store, err := NewCookieStore(valid)
	if err != nil {
		t.Fatal(err)
	}

	for _, size := range []int{0, 1, 15, 17, 20, 23, 25, 31, 33} {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			keys := Keys{
				Authentication: valid.Authentication,
				Encryption:     make([]byte, size),
			}
			if _, err := NewCookieStore(keys); err == nil {
				t.Fatalf("NewCookieStore accepted a %d-byte encryption key", size)
			}
			if _, err := NewJar(keys, store, NewSession[*testApp]("app")); err == nil {
				t.Fatalf("NewJar accepted a %d-byte encryption key", size)
			}
		})
	}

	for _, size := range []int{16, 24, 32} {
		t.Run(fmt.Sprintf("%d bytes valid", size), func(t *testing.T) {
			keys := Keys{
				Authentication: valid.Authentication,
				Encryption:     make([]byte, size),
			}
			if _, err := NewCookieStore(keys); err != nil {
				t.Fatalf("NewCookieStore rejected a %d-byte encryption key: %v", size, err)
			}
			if _, err := NewJar(keys, store, NewSession[*testApp]("app")); err != nil {
				t.Fatalf("NewJar rejected a %d-byte encryption key: %v", size, err)
			}
		})
	}
}

func TestCookieMaxAgeIsPerDefinition(t *testing.T) {
	jar := testJar(t,
		NewSession[*testApp]("app", MaxAge(1)),
		Encrypted[*theme]("theme", MaxAge(1)),
		Signed[jsonApp]("prefs", MaxAge(3600)),
	)
	ctx := context.Background()

	themeRecorder := httptest.NewRecorder()
	dark := theme("dark")
	if err := Write(ctx, jar, themeRecorder, &dark); err != nil {
		t.Fatal(err)
	}
	prefsRecorder := httptest.NewRecorder()
	if err := Write(ctx, jar, prefsRecorder, jsonApp{UserID: "u1"}); err != nil {
		t.Fatal(err)
	}

	// securecookie compares whole seconds, so a 1-second MaxAge needs the
	// clock to advance past the next tick to be observably expired.
	time.Sleep(2100 * time.Millisecond)

	themeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	themeRequest.AddCookie(cookieByName(themeRecorder.Result().Cookies(), "theme"))
	if _, err := Read[*theme](ctx, jar, themeRequest); err == nil {
		t.Fatal("expected the 1-second theme cookie to expire")
	}

	prefsRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	prefsRequest.AddCookie(cookieByName(prefsRecorder.Result().Cookies(), "prefs"))
	prefs, err := Read[jsonApp](ctx, jar, prefsRequest)
	if err != nil {
		t.Fatalf("longer-lived cookie was rejected: %v", err)
	}
	if prefs.UserID != "u1" {
		t.Fatalf("prefs = %+v", prefs)
	}
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names
}

func TestDefinitionsWithoutCookieMethodsUseJSON(t *testing.T) {
	definitions := map[string]Definition{
		"session": NewSession[*jsonApp]("app", MaxAge(60)),
		"plain":   Plain[jsonApp]("prefs"),
	}
	want := jsonApp{UserID: "u1", IsAuthenticated: true}

	for name, definition := range definitions {
		t.Run(name, func(t *testing.T) {
			var input any = &want
			if name == "plain" {
				input = want
			}
			payload, err := definition.encodePayload(input)
			if err != nil {
				t.Fatalf("encodePayload: %v", err)
			}
			if string(payload) != `{"user_id":"u1","is_authenticated":true}` {
				t.Fatalf("payload = %q", payload)
			}

			decoded, err := definition.decodePayload(payload)
			if err != nil {
				t.Fatalf("decodePayload: %v", err)
			}
			switch value := decoded.(type) {
			case *jsonApp:
				if value == nil || *value != want {
					t.Fatalf("decoded = %#v, want %#v", value, want)
				}
			case jsonApp:
				if value != want {
					t.Fatalf("decoded = %#v, want %#v", value, want)
				}
			default:
				t.Fatalf("decoded type = %T", decoded)
			}

			zero, err := definition.decodePayload(nil)
			if err != nil {
				t.Fatalf("decodePayload empty: %v", err)
			}
			if name == "session" && zero.(*jsonApp) != nil {
				t.Fatalf("empty session = %#v, want nil", zero)
			}
			if name == "plain" && zero.(jsonApp) != (jsonApp{}) {
				t.Fatalf("empty plain = %#v, want zero", zero)
			}
		})
	}
}

func TestSessionWithoutCookieMethodsRoundTripsAsJSON(t *testing.T) {
	jar := testJar(t, NewSession[*jsonApp]("app", HTTPOnly(), MaxAge(60)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		return Set(ctx, &jsonApp{UserID: "u2", IsAuthenticated: true})
	})); err != nil {
		t.Fatal(err)
	}

	appCookie := cookieByName(recorder.Result().Cookies(), "app")
	if appCookie == nil {
		t.Fatal("app cookie missing")
	}

	follow := httptest.NewRequest(http.MethodGet, "/", nil)
	follow.AddCookie(appCookie)
	var got *jsonApp
	if err := anduin.Serve(httptest.NewRecorder(), follow, jar.Middleware()(func(ctx context.Context) error {
		var err error
		got, err = Get[*jsonApp](ctx)
		return err
	})); err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != (jsonApp{UserID: "u2", IsAuthenticated: true}) {
		t.Fatalf("app = %#v", got)
	}
}

func TestSameRequestFlashIsVisible(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		AddFlash(ctx, FlashError, "nope")
		got := Flashes(ctx)
		if len(got) != 1 || got[0].Message != "nope" {
			t.Fatalf("same-request flashes = %+v", got)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptSessionIsRecovered(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "app", Value: "not-a-valid-cookie"})
	recorder := httptest.NewRecorder()
	called := false
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		called = true
		app, err := Get[*testApp](ctx)
		if err != nil {
			return err
		}
		if app != nil && app.IsAuthenticated {
			t.Fatal("corrupt cookie should yield nil App")
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(recorder, request, jar.Middleware(SkipPrefixes("/assets"))(func(ctx context.Context) error {
		if bagFrom(ctx) != nil {
			t.Fatal("skipped path should not have a bag")
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
}

func TestClientRedirectPersistsFlashes(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler := jar.Middleware()(func(ctx context.Context) error {
		AddFlash(ctx, FlashSuccess, "next")
		anduin.SetHeader(ctx, ClientRedirectHeader, "1")
		return anduin.NoContent(ctx, http.StatusOK)
	})
	if err := anduin.Serve(recorder, request, handler); err != nil {
		t.Fatal(err)
	}
	flashCookie := cookieByName(recorder.Result().Cookies(), "app_flash")
	if flashCookie == nil {
		t.Fatal("missing flash cookie")
	}
	follow := httptest.NewRequest(http.MethodGet, "/next", nil)
	follow.AddCookie(flashCookie)
	followRecorder := httptest.NewRecorder()
	if err := anduin.Serve(followRecorder, follow, jar.Middleware()(func(ctx context.Context) error {
		got := Flashes(ctx)
		if len(got) != 1 || got[0].Message != "next" {
			t.Fatalf("redirect flashes = %+v", got)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	dark := theme("dark")
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &dark); err != nil {
			return err
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
	themeCookie := cookieByName(recorder.Result().Cookies(), "theme")
	if themeCookie == nil {
		t.Fatal("missing theme cookie")
	}
	follow := httptest.NewRequest(http.MethodGet, "/", nil)
	follow.AddCookie(themeCookie)
	followRecorder := httptest.NewRecorder()
	if err := anduin.Serve(followRecorder, follow, jar.Middleware()(func(ctx context.Context) error {
		exists, err := Exists[*theme](ctx)
		if err != nil {
			return err
		}
		if !exists {
			t.Fatal("theme should Exist after lazy Get path")
		}
		got, err := Get[*theme](ctx)
		if err != nil {
			return err
		}
		if got == nil || *got != "dark" {
			t.Fatalf("theme = %v", got)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
}

func TestDestroySession(t *testing.T) {
	jar := testJar(t, NewSession[*testApp]("app", MaxAge(60)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
		t.Fatal(err)
	}
	appCookie := cookieByName(recorder.Result().Cookies(), "app")
	if appCookie == nil {
		t.Fatal("missing app cookie")
	}

	destroyRecorder := httptest.NewRecorder()
	destroyReq := httptest.NewRequest(http.MethodGet, "/", nil)
	destroyReq.AddCookie(appCookie)
	if err := anduin.Serve(destroyRecorder, destroyReq, jar.Middleware()(func(ctx context.Context) error {
		exists, err := Exists[*testApp](ctx)
		if err != nil {
			return err
		}
		if !exists {
			t.Fatal("expected session to exist")
		}
		if err := Destroy[*testApp](ctx); err != nil {
			return err
		}
		exists, err = Exists[*testApp](ctx)
		if err != nil {
			return err
		}
		if exists {
			t.Fatal("expected session destroyed")
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		// Value type when the jar registered *testApp.
		if _, err := Get[testApp](ctx); !errors.Is(err, ErrUnknownType) {
			t.Fatalf("Get[testApp] error = %v, want ErrUnknownType", err)
		}
		if _, err := Exists[testApp](ctx); !errors.Is(err, ErrUnknownType) {
			t.Fatalf("Exists[testApp] error = %v, want ErrUnknownType", err)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		dark := theme("x")
		if err := Set(ctx, &dark); !errors.Is(err, ErrNotBag) {
			t.Fatalf("Set(native) = %v, want ErrNotBag", err)
		}
		if _, err := Get[*theme](ctx); !errors.Is(err, ErrNotBag) {
			t.Fatalf("Get(native) = %v, want ErrNotBag", err)
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &testApp{UserID: "db1", IsAuthenticated: true}); err != nil {
			return err
		}
		AddFlash(ctx, FlashInfo, "hi")
		return anduin.Redirect(ctx, "/next", http.StatusSeeOther)
	})); err != nil {
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
	if err := anduin.Serve(followRecorder, follow, jar.Middleware()(func(ctx context.Context) error {
		got, err := Get[*testApp](ctx)
		if err != nil {
			return err
		}
		if got == nil || got.UserID != "db1" || !got.IsAuthenticated {
			t.Fatalf("app = %+v", got)
		}
		if flashes := Flashes(ctx); len(flashes) != 1 {
			t.Fatalf("flashes = %+v", flashes)
		}
		if err := Destroy[*testApp](ctx); err != nil {
			return err
		}
		return anduin.NoContent(ctx, http.StatusOK)
	})); err != nil {
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
	if err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		return Set(ctx, &testApp{UserID: "u1", IsAuthenticated: true})
	})); err != nil {
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
	called := false
	err = anduin.Serve(failRecorder, failReq, jar.Middleware()(func(ctx context.Context) error {
		called = true
		return anduin.NoContent(ctx, http.StatusOK)
	}))
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
	err = anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &testApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return anduin.NoContent(ctx, http.StatusOK)
	}))
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
	err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if err := Set(ctx, &failMarshalApp{UserID: "u1", IsAuthenticated: true}); err != nil {
			return err
		}
		return anduin.NoContent(ctx, http.StatusOK)
	}))
	if err == nil {
		t.Fatal("expected marshal persist error")
	}
	if err.Error() != "marshal boom" {
		t.Fatalf("error = %v, want marshal boom", err)
	}
}
