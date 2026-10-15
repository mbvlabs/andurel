// Package anduintest builds httptest servers from real handler registrations so
// handler tests exercise the same Router a production process does.
package anduintest

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

// Option configures a test Server.
type Option func(*options)

type options struct {
	middleware []anduin.Middleware
}

// WithMiddleware installs global middleware on the test router (for example
// session or Inertia middleware).
func WithMiddleware(mw ...anduin.Middleware) Option {
	return func(o *options) {
		o.middleware = append(o.middleware, mw...)
	}
}

// Server is a running test HTTP server plus a cookie-persisting client.
type Server struct {
	t      *testing.T
	server *httptest.Server
	client *http.Client
	Router *anduin.Router
}

// New boots a test server, calls register to wire handlers, and registers a
// cleanup that stops the server.
func New(t *testing.T, register func(*anduin.Router), opts ...Option) *Server {
	t.Helper()

	cfg := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	rt := anduin.NewRouter()
	register(rt)
	rt.Use(cfg.middleware...)

	server := httptest.NewServer(rt.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("anduintest: cookie jar: %v", err)
	}

	return &Server{
		t:      t,
		server: server,
		client: &http.Client{
			Jar: jar,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Router: rt,
	}
}

// URL returns an absolute URL for a path on the test server.
func (s *Server) URL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return s.server.URL + path
}

// Get performs a GET request.
func (s *Server) Get(path string, headers ...Header) *Response {
	s.t.Helper()
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodGet, s.URL(path), nil)
	if err != nil {
		s.t.Fatalf("anduintest: build GET %s: %v", path, err)
	}
	return s.Do(req, headers...)
}

// PostForm performs an application/x-www-form-urlencoded POST request.
func (s *Server) PostForm(path string, form url.Values, headers ...Header) *Response {
	s.t.Helper()
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodPost, s.URL(path), body)
	if err != nil {
		s.t.Fatalf("anduintest: build POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.Do(req, headers...)
}

// PostJSON performs a JSON POST request.
func (s *Server) PostJSON(path string, payload any, headers ...Header) *Response {
	s.t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			s.t.Fatalf("anduintest: encode JSON: %v", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodPost, s.URL(path), body)
	if err != nil {
		s.t.Fatalf("anduintest: build POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return s.Do(req, headers...)
}

// Do performs req and returns the decoded response.
func (s *Server) Do(req *http.Request, headers ...Header) *Response {
	s.t.Helper()
	for _, header := range headers {
		req.Header.Set(header.Name, header.Value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("anduintest: do %s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatalf("anduintest: read body: %v", err)
	}

	return &Response{
		t:          s.t,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
	}
}

// Header is a request header used by Get/Post helpers.
type Header struct {
	Name  string
	Value string
}

// Response is a decoded HTTP response.
type Response struct {
	t          *testing.T
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Text returns the response body as a string.
func (r *Response) Text() string { return string(r.Body) }

// JSON decodes the response body into dst.
func (r *Response) JSON(dst any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		r.t.Fatalf("anduintest: decode JSON: %v\nbody: %s", err, r.Body)
	}
}

// AssertStatus fails unless the response status matches want.
func (r *Response) AssertStatus(want int) {
	r.t.Helper()
	if r.StatusCode != want {
		r.t.Fatalf("anduintest: status = %d, want %d\nbody: %s", r.StatusCode, want, r.Body)
	}
}

// AssertInertia fails unless the response is an Inertia page (or full document)
// for component.
func (r *Response) AssertInertia(component string) {
	r.t.Helper()
	if got, ok := r.inertiaComponent(); ok {
		if got != component {
			r.t.Fatalf("anduintest: inertia component = %q, want %q", got, component)
		}
		return
	}
	r.t.Fatalf("anduintest: response is not an Inertia page\nbody: %s", r.Body)
}

func (r *Response) inertiaComponent() (string, bool) {
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return decodeComponent(r.Body)
	}

	marker := []byte(`data-page="`)
	index := bytes.Index(r.Body, marker)
	if index == -1 {
		return "", false
	}
	rest := r.Body[index+len(marker):]
	end := bytes.IndexByte(rest, '"')
	if end == -1 {
		return "", false
	}
	page := html.UnescapeString(string(rest[:end]))
	return decodeComponent([]byte(page))
}

func decodeComponent(body []byte) (string, bool) {
	var page struct {
		Component string `json:"component"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return "", false
	}
	if page.Component == "" {
		return "", false
	}
	return page.Component, true
}
