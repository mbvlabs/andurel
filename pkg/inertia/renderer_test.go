package inertia

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

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
	renderer, err := New(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	etx := e.NewContext(request, recorder)
	if err := renderer.Page(etx, "Dashboard", Props{"name": "Ada"}, WithSSR()); err != nil {
		t.Fatalf("initial Page: %v", err)
	}
	if ssr.calls != 1 || captured.SSR == nil || !strings.Contains(recorder.Body.String(), "SSR") {
		t.Fatalf("SSR initial response calls=%d captured=%#v body=%q", ssr.calls, captured.SSR, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	request.Header.Set(HeaderInertia, "true")
	recorder = httptest.NewRecorder()
	etx = e.NewContext(request, recorder)
	if err := renderer.Page(etx, "Dashboard", nil, WithSSR()); err != nil {
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
	renderer, err := New(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	if err := renderer.Page(e.NewContext(request, recorder), "Home", nil, WithSSR()); err != nil {
		t.Fatalf("fallback Page: %v", err)
	}
	if captured.SSR != nil || recorder.Body.String() != "client" {
		t.Fatalf("SSR failure did not fall back: captured=%#v body=%q", captured.SSR, recorder.Body.String())
	}

	renderer, err = New(WithRoot(testRoot(&captured)), WithSSRRenderer(ssr), WithSSRFailFast(true))
	if err != nil {
		t.Fatalf("New fail-fast: %v", err)
	}
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	err = renderer.Page(e.NewContext(request, recorder), "Home", nil, WithSSR())
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
	if strings.Contains(output.String(), "</script></script>") || !strings.Contains(output.String(), `<\/script>`) {
		t.Fatalf("unsafe page script: %s", output.String())
	}
}
