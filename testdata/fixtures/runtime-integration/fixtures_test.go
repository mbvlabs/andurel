package runtimeintegration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
	"github.com/mbvlabs/andurel/pkg/kiks"
	"github.com/mbvlabs/andurel/pkg/routing"
)

// This file is the shared fixture for the development-only integration module.
// It builds the real anduin router, kiks cookie middleware and Inertia capture
// middleware over synthetic hosts and synthetic cookie values. No production,
// login, database or network data is used.

const (
	adminHost routing.HostName = "admin"

	primaryAuthority = "primary.test"
	adminAuthority   = "admin.test"

	rendererVersion = "rc1-integration-v1"

	sessionCookieName = "app_sess"
	flashCookieName   = "app_sess_flash"
	returnCookieName  = "app_return_to"
)

type sessionState struct {
	UserID          string `json:"user_id,omitempty"`
	IsAuthenticated bool   `json:"is_authenticated,omitempty"`
}

// returnTarget mirrors the encrypted return-to value used by an AuthOnly-style
// host rejection. Kiks encodes it as JSON because it has no custom Cookie codec.
type returnTarget struct {
	Host routing.HostName `json:"host"`
	Path string           `json:"path"`
}

// countingStore wraps the real kiks CookieStore so a test can assert exactly-once
// session persistence without changing cookie round-trips.
type countingStore struct {
	inner kiks.Store

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
	attrs kiks.CookieAttrs,
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
	attrs kiks.CookieAttrs,
) error {
	store.mu.Lock()
	store.destroys++
	store.mu.Unlock()

	return store.inner.Destroy(writer, request, name, attrs)
}

func (store *countingStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.loads = 0
	store.saves = 0
	store.destroys = 0
}

func (store *countingStore) saveCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()

	return store.saves
}

func (store *countingStore) destroyCount() int {
	store.mu.Lock()
	defer store.mu.Unlock()

	return store.destroys
}

func newIntegrationJar(t *testing.T) (*kiks.Jar, *countingStore) {
	t.Helper()

	keys := kiks.Keys{
		Authentication: bytes.Repeat([]byte{0x41}, 32),
		Encryption:     bytes.Repeat([]byte{0x42}, 32),
	}
	cookieStore, err := kiks.NewCookieStore(keys)
	if err != nil {
		t.Fatalf("new cookie store: %v", err)
	}
	store := &countingStore{inner: cookieStore}
	jar, err := kiks.NewJar(
		keys,
		store,
		kiks.NewSession[*sessionState](sessionCookieName, kiks.MaxAge(60), kiks.HTTPOnly()),
		kiks.Bagged(kiks.Encrypted[*returnTarget](returnCookieName, kiks.MaxAge(600), kiks.HTTPOnly())),
	)
	if err != nil {
		t.Fatalf("new jar: %v", err)
	}

	return jar, store
}

func newIntegrationHosts(t *testing.T) *routing.Hosts {
	t.Helper()

	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: primaryAuthority, Protocol: "http"},
		adminHost:           {Hostname: adminAuthority, Protocol: "http"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	return hosts
}

func newIntegrationRenderer(t *testing.T, options ...inertia.Option) (*inertia.Renderer, *inertia.RootData) {
	t.Helper()

	captured := new(inertia.RootData)
	options = append(
		[]inertia.Option{
			inertia.WithRoot(integrationRoot(captured)),
			inertia.WithVersion(rendererVersion),
		},
		options...,
	)
	renderer, err := inertia.NewRenderer(
		inertia.RendererConfig{
			ContainerID: "app",
			// Valid but never contacted: these tests render client-side only.
			SSRURL:              "http://127.0.0.1:1",
			SSRTimeout:          time.Second,
			SSRMaxResponseBytes: 2 << 20,
		},
		options...,
	)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	return renderer, captured
}

func integrationRoot(captured *inertia.RootData) inertia.RootFunc {
	return func(data inertia.RootData) templ.Component {
		*captured = data

		return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
			_, err := fmt.Fprintf(
				writer,
				`<!doctype html><html><body data-themed="true"><div id="%s"></div><script data-page="%s" type="application/json">%s</script></body></html>`,
				data.ContainerID,
				data.ContainerID,
				data.PageJSON,
			)

			return err
		})
	}
}

// deliveryWriter is the real response sink. It records every header and body
// write so tests can prove a capture delivered the error page exactly once.
type deliveryWriter struct {
	header http.Header
	status int

	headerWrites int
	bodyWrites   int
	body         bytes.Buffer
	writeErr     error
}

func newDeliveryWriter() *deliveryWriter {
	return &deliveryWriter{header: make(http.Header)}
}

func (writer *deliveryWriter) Header() http.Header { return writer.header }

func (writer *deliveryWriter) WriteHeader(status int) {
	writer.headerWrites++
	if writer.status == 0 {
		writer.status = status
	}
}

func (writer *deliveryWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	if writer.writeErr != nil {
		return 0, writer.writeErr
	}
	writer.bodyWrites++

	return writer.body.Write(body)
}

func (writer *deliveryWriter) bodyString() string { return writer.body.String() }

func (writer *deliveryWriter) cookies() []*http.Cookie {
	return (&http.Response{Header: writer.header}).Cookies()
}

type requestOptions struct {
	method  string
	host    string
	path    string
	inertia bool
	accept  string
	ctx     context.Context
	cookies []*http.Cookie
}

func serveRequest(t *testing.T, handler http.Handler, options requestOptions) *deliveryWriter {
	t.Helper()

	request := httptest.NewRequest(options.method, options.path, nil)
	if options.ctx != nil {
		request = request.WithContext(options.ctx)
	}
	request.Host = options.host
	if options.inertia {
		request.Header.Set(inertia.HeaderInertia, "true")
		request.Header.Set(inertia.HeaderVersion, rendererVersion)
	}
	if options.accept != "" {
		request.Header.Set("Accept", options.accept)
	}
	for _, cookie := range options.cookies {
		request.AddCookie(cookie)
	}

	writer := newDeliveryWriter()
	handler.ServeHTTP(writer, request)

	return writer
}

// seedSession drives the real kiks middleware once and returns the cookies a
// browser would replay. withFlash seeds an existing flash cookie through a 303
// so the 403-clear control starts from a persisted value.
func seedSession(
	t *testing.T,
	jar *kiks.Jar,
	state *sessionState,
	withFlash bool,
) []*http.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://"+primaryAuthority+"/__seed", nil)
	err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		if setErr := kiks.Set(ctx, state); setErr != nil {
			return setErr
		}
		if !withFlash {
			return anduin.NoContent(ctx, http.StatusNoContent)
		}
		if flashErr := kiks.AddFlash(ctx, kiks.FlashInfo, "seed flash"); flashErr != nil {
			return flashErr
		}

		return anduin.Redirect(ctx, "/__seeded", http.StatusSeeOther)
	}))
	if err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if recorder.Code != http.StatusNoContent && recorder.Code != http.StatusSeeOther {
		t.Fatalf("seed status = %d, want 204 or 303", recorder.Code)
	}

	return recorder.Result().Cookies()
}

type bagSnapshot struct {
	session *sessionState
	target  *returnTarget
	flashes []kiks.FlashMessage
	err     error
}

func readBag(t *testing.T, jar *kiks.Jar, cookies ...*http.Cookie) bagSnapshot {
	t.Helper()

	var snapshot bagSnapshot
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://"+primaryAuthority+"/__read", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	err := anduin.Serve(recorder, request, jar.Middleware()(func(ctx context.Context) error {
		var getErr error
		snapshot.session, getErr = kiks.Get[*sessionState](ctx)
		if getErr != nil {
			snapshot.err = getErr

			return getErr
		}
		snapshot.target, getErr = kiks.Get[*returnTarget](ctx)
		if getErr != nil {
			snapshot.err = getErr

			return getErr
		}
		snapshot.flashes = kiks.FlashMessages(ctx)

		return anduin.NoContent(ctx, http.StatusNoContent)
	}))
	if err != nil {
		t.Fatalf("read bag: %v", err)
	}

	return snapshot
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, len(cookies))
	for index, cookie := range cookies {
		names[index] = cookie.Name
	}

	return names
}

// strictAuthProvider is the application's pre-workaround shared provider: it
// requires the kiks cookie bag and propagates ErrNoBag. A host/global error
// render must reach it with an authenticated session, not a detached context.
type strictAuthProvider struct {
	mu      sync.Mutex
	calls   int
	err     error
	session *sessionState
}

func (provider *strictAuthProvider) provide(ctx context.Context) (inertia.Props, error) {
	current, err := kiks.Get[*sessionState](ctx)

	provider.mu.Lock()
	provider.calls++
	provider.err = err
	provider.session = current
	provider.mu.Unlock()

	if err != nil {
		return nil, err
	}
	authenticated := current != nil && current.IsAuthenticated

	return inertia.Props{"auth": inertia.Props{"isAuthenticated": authenticated}}, nil
}

func (provider *strictAuthProvider) snapshot() (calls int, err error, session *sessionState) {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	return provider.calls, provider.err, provider.session
}

type errorObservation struct {
	calls   int
	err     error
	session *sessionState
	getErr  error
	ctxErr  error
}

func (observation *errorObservation) record(ctx context.Context, err error) {
	observation.calls++
	observation.err = err
	observation.ctxErr = ctx.Err()
	observation.session, observation.getErr = kiks.Get[*sessionState](ctx)
}

func assertAuthenticatedSession(t *testing.T, state *sessionState) {
	t.Helper()

	if state == nil {
		t.Fatal("session = nil, want the seeded authenticated session")
	}
	if !state.IsAuthenticated || state.UserID == "" {
		t.Fatalf("session = %+v, want an authenticated user", state)
	}
}

func assertThemedComponent(t *testing.T, body, component string) {
	t.Helper()

	if !contains(body, `"component":"`+component+`"`) {
		t.Fatalf("body does not contain component %q: %s", component, body)
	}
}

func assertAuthProp(t *testing.T, props map[string]any) {
	t.Helper()

	var auth map[string]any
	switch value := props["auth"].(type) {
	case map[string]any:
		auth = value
	case inertia.Props:
		auth = value
	default:
		t.Fatalf("props[auth] = %#v, want a shared auth object", props["auth"])
	}
	authenticated, ok := auth["isAuthenticated"].(bool)
	if !ok || !authenticated {
		t.Fatalf("props[auth] = %#v, want isAuthenticated true", auth)
	}
}

func assertInertiaJSONPage(t *testing.T, body []byte) inertia.Page {
	t.Helper()

	var page inertia.Page
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode X-Inertia page: %v: %s", err, body)
	}

	return page
}

func contains(value, needle string) bool {
	return bytes.Contains([]byte(value), []byte(needle))
}
