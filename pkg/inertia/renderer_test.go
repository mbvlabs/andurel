package inertia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// testContext builds an anduin request context around an httptest request and
// recorder so the renderer can read the request and write the response.
func testContext(request *http.Request, writer http.ResponseWriter) context.Context {
	ctx := anduin.ReplaceRequest(context.Background(), request)
	return anduin.ReplaceResponseWriter(ctx, writer)
}

func testSSRClient() (string, time.Duration, int64) {
	return "http://127.0.0.1:13714", 2 * time.Second, 2 << 20
}

func newTestRenderer(options ...Option) (*Renderer, error) {
	ssrURL, timeout, maxBytes := testSSRClient()
	return NewRenderer(
		RendererConfig{
			ContainerID:         "app",
			BuildPathURL:        "/assets/dist/vite/*",
			EntryPoint:          "resources/js/app.ts",
			ViteDevURL:          "http://localhost:5173/assets/dist",
			SSRURL:              ssrURL,
			SSRTimeout:          timeout,
			SSRMaxResponseBytes: maxBytes,
		},
		options...,
	)
}

type fakeSSRRenderer struct {
	calls    int
	response *SSRResponse
	err      error
}

func (renderer *fakeSSRRenderer) Render(context.Context, Page) (*SSRResponse, error) {
	renderer.calls++
	return renderer.response, renderer.err
}

func testRoot(captured *RootData) RootFunc {
	return func(writer io.Writer, data RootData) error {
		if captured != nil {
			*captured = data
		}
		if data.SSR != nil {
			_, err := io.WriteString(writer, string(data.SSRBody()))
			return err
		}
		_, err := io.WriteString(writer, "client")
		return err
	}
}

// pageScriptRoot renders only the page script, matching the smallest root a
// project can configure.
func pageScriptRoot(captured *RootData) RootFunc {
	return func(writer io.Writer, data RootData) error {
		if captured != nil {
			*captured = data
		}
		script, err := data.PageScript()
		if err != nil {
			return err
		}
		_, err = io.WriteString(writer, string(script))
		return err
	}
}

func TestWithSSRContactsRendererOnlyForInitialDocument(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Head: []string{"<title>SSR</title>"},
		Body: `<div data-server-rendered="true" data-page="app">SSR</div>`,
	}}
	renderer, err := newTestRenderer(WithRootFunc(testRoot(&captured)), WithSSRRenderer(ssr))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Dashboard", Props{"name": "Ada"}).SSR().Render(); err != nil {
		t.Fatalf("initial Page: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil || !strings.Contains(recorder.Body.String(), "SSR") {
		t.Fatalf(
			"SSR initial response calls=%d captured=%#v body=%q",
			ssr.calls,
			captured.SSR,
			recorder.Body.String(),
		)
	}

	request = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder = httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Dashboard", nil).SSR().Render(); err != nil {
		t.Fatalf("Inertia Page: %v", err)
	}
	if ssr.calls != 1 {
		t.Fatalf("SSR renderer contacted for JSON visit: %d calls", ssr.calls)
	}
}

func TestSSRFailureFallsBackUnlessFailFast(t *testing.T) {
	var captured RootData
	ssrErr := errors.New("renderer unavailable")
	ssr := &fakeSSRRenderer{err: ssrErr}
	renderer, err := newTestRenderer(WithRootFunc(testRoot(&captured)), WithSSRRenderer(ssr))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(testContext(request, recorder), "Home", nil).SSR().Render(); err != nil {
		t.Fatalf("fallback Page: %v", err)
	}
	if captured.SSR != nil || recorder.Body.String() != "client" {
		t.Fatalf(
			"SSR failure did not fall back: captured=%#v body=%q",
			captured.SSR,
			recorder.Body.String(),
		)
	}

	renderer, err = newTestRenderer(
		WithRootFunc(testRoot(&captured)),
		WithSSRRenderer(ssr),
		WithSSRFailFast(true),
	)
	if err != nil {
		t.Fatalf("NewRenderer fail-fast: %v", err)
	}
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	err = renderer.Page(testContext(request, recorder), "Home", nil).SSR().Render()
	var inertiaErr *Error
	if !errors.As(err, &inertiaErr) || inertiaErr.Kind != ErrorSSR {
		t.Fatalf("fail-fast error = %#v", err)
	}
}

func TestPageScriptEscapesClosingScript(t *testing.T) {
	output, err := (RootData{
		ContainerID: "app",
		PageJSON:    []byte(`{"value":"</script>"}`),
	}).PageScript()
	if err != nil {
		t.Fatalf("PageScript: %v", err)
	}
	if strings.Contains(string(output), "</script></script>") ||
		!strings.Contains(string(output), `\u003c\/script\u003e`) {
		t.Fatalf("unsafe page script: %s", output)
	}
}

func TestPageScriptRejectsInvalidJSON(t *testing.T) {
	if _, err := (RootData{ContainerID: "app", PageJSON: []byte(`{"value":`)}).PageScript(); err == nil {
		t.Fatal("expected an invalid page JSON error")
	}
	if _, err := (RootData{ContainerID: "app", PageJSON: []byte(`{} {}`)}).PageScript(); err == nil {
		t.Fatal("expected a trailing page JSON error")
	}
}

func TestRenderedDocumentCannotBreakOutOfPageScript(t *testing.T) {
	malicious := `</script><!--<script`
	renderer, err := newTestRenderer(WithRootFunc(pageScriptRoot(nil)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	page := renderer.Page(testContext(request, recorder), "Home", Props{"value": malicious})
	if err := page.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, malicious) || strings.Contains(body, "<!--") {
		t.Fatalf("document contains an unescaped prop: %s", body)
	}
	if got := strings.Count(body, "</script>"); got != 1 {
		t.Fatalf("document has %d closing script tags, want 1: %s", got, body)
	}
	if !strings.Contains(body, `\u003c\/script\u003e`) {
		t.Fatalf("document did not HTML-escape the prop: %s", body)
	}
}

func TestInertiaJSONEscapesHTML(t *testing.T) {
	var captured RootData
	renderer, err := newTestRenderer(WithRootFunc(testRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	malicious := `</script><!--<script`
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	page := renderer.Page(testContext(request, recorder), "Home", Props{"value": malicious})
	if err := page.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	if strings.Contains(body, malicious) || strings.Contains(body, "<!--") {
		t.Fatalf("JSON contains an unescaped prop: %s", body)
	}
	if !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatalf("JSON did not HTML-escape the prop: %s", body)
	}
}

func TestPageBuilderURLOverridesPageURL(t *testing.T) {
	t.Parallel()

	const pageURL = "/users/confirmation/new"
	var captured RootData
	renderer, err := newTestRenderer(WithRootFunc(pageScriptRoot(&captured)))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/users", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder := httptest.NewRecorder()
	page := renderer.Page(testContext(request, recorder), "Users/Confirmation", nil).URL(pageURL)
	if err := page.Render(); err != nil {
		t.Fatalf("JSON Render: %v", err)
	}

	var responsePage Page
	if err := json.Unmarshal(recorder.Body.Bytes(), &responsePage); err != nil {
		t.Fatalf("decode page JSON: %v", err)
	}
	if responsePage.URL != pageURL {
		t.Fatalf("JSON page.url = %q, want %q", responsePage.URL, pageURL)
	}

	request = httptest.NewRequest(http.MethodPost, "/users", nil)
	recorder = httptest.NewRecorder()
	page = renderer.Page(testContext(request, recorder), "Users/Confirmation", nil).URL(pageURL)
	if err := page.Render(); err != nil {
		t.Fatalf("document Render: %v", err)
	}
	if captured.Page.URL != pageURL {
		t.Fatalf("document page.url = %q, want %q", captured.Page.URL, pageURL)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"url":"\/users\/confirmation\/new"`) {
		t.Fatalf("document page script missing overridden url: %s", body)
	}
}

func assertAnonymousCrossOrigin(t *testing.T, tag string) {
	t.Helper()

	if !strings.Contains(tag, "crossorigin") {
		t.Fatalf("tag %q has no crossorigin attribute", tag)
	}
	if strings.Contains(tag, "use-credentials") {
		t.Fatalf("tag %q uses credentialed crossorigin", tag)
	}
	if strings.Contains(tag, `crossorigin="`) && !strings.Contains(tag, `crossorigin="anonymous"`) {
		t.Fatalf("tag %q has an unexpected crossorigin value", tag)
	}
}

func TestProductionViteTagsUseBuildPathPrefixAndAnonymousCrossOrigin(t *testing.T) {
	t.Parallel()

	assetFS := fstest.MapFS{
		"dist/vite/manifest.json": &fstest.MapFile{Data: []byte(
			`{"resources/js/app.ts":{"file":"app-abc.js","css":["app-abc.css"]}}`,
		)},
	}

	renderer, err := NewRenderer(
		RendererConfig{
			ContainerID:         "app",
			BuildPathURL:        "https://example.com/assets/dist/*",
			EntryPoint:          "resources/js/app.ts",
			ViteDevURL:          "http://localhost:5173/assets/dist",
			SSRURL:              "http://127.0.0.1:13714",
			SSRTimeout:          2 * time.Second,
			SSRMaxResponseBytes: 2 << 20,
		},
		WithRootFunc(testRoot(nil)),
		WithAssetFS(assetFS),
		WithEnvironment("production"),
	)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	wantCSS := `href="https://example.com/assets/dist/app-abc.css"`
	if !strings.Contains(renderer.viteTags.head, wantCSS) {
		t.Fatalf("vite head = %q", renderer.viteTags.head)
	}
	assertAnonymousCrossOrigin(t, renderer.viteTags.head)

	wantJS := `src="https://example.com/assets/dist/app-abc.js"`
	if !strings.Contains(renderer.viteTags.body, wantJS) ||
		!strings.Contains(renderer.viteTags.body, `type="module"`) {
		t.Fatalf("vite body = %q", renderer.viteTags.body)
	}
	assertAnonymousCrossOrigin(t, renderer.viteTags.body)
}
