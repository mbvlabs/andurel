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

	"github.com/a-h/templ"
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
	return func(data RootData) templ.Component {
		*captured = data
		return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
			if data.SSR != nil {
				_, err := io.WriteString(writer, data.SSR.Body)
				return err
			}
			_, err := io.WriteString(writer, "client")
			return err
		})
	}
}

func TestWithSSRContactsRendererOnlyForInitialDocument(t *testing.T) {
	var captured RootData
	ssr := &fakeSSRRenderer{response: &SSRResponse{
		Head: []string{"<title>SSR</title>"},
		Body: `<div data-server-rendered="true" data-page="app">SSR</div>`,
	}}
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr))
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
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr))
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
		WithRoot(testRoot(&captured)),
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
	var output strings.Builder
	component := PageScript("app", []byte(`{"value":"</script>"}`))
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(output.String(), "</script></script>") ||
		!strings.Contains(output.String(), `\u003c\/script\u003e`) {
		t.Fatalf("unsafe page script: %s", output.String())
	}
}

func TestRenderedDocumentCannotBreakOutOfPageScript(t *testing.T) {
	malicious := `</script><!--<script`
	renderer, err := newTestRenderer(WithRoot(func(data RootData) templ.Component {
		return PageScript(data.ContainerID, data.PageJSON)
	}))
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
	renderer, err := newTestRenderer(WithRoot(testRoot(&captured)))
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
	renderer, err := newTestRenderer(WithRoot(func(data RootData) templ.Component {
		captured = data
		return PageScript(data.ContainerID, data.PageJSON)
	}))
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
		WithRoot(testRoot(nil)),
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
