package layout

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

func TestGeneratedErrorPagesDisplayEscapedMessages(t *testing.T) {
	for _, page := range []struct{ name, fallback string }{
		{"bad_request", "The request made was invalid."},
		{"not_found", "The page you are looking for could not be found."},
		{"conflict", "The request conflicts with existing data."},
	} {
		for _, adapter := range []struct{ prefix, declaration, expression string }{
			{"inertia_react", "message?: string", "{message || '" + page.fallback + "'}"},
			{"inertia", "defineProps<{ message?: string }>()", "{{ message || '" + page.fallback + "' }}"},
			{"inertia_svelte", "export let message: string = ''", "{message || '" + page.fallback + "'}"},
		} {
			name := adapter.prefix + "_assets_pages_errors_" + page.name + ".tmpl"
			t.Run(name, func(t *testing.T) {
				assertTemplateContains(t, name, adapter.declaration, adapter.expression)
				for _, unsafe := range []string{"dangerouslySetInnerHTML", "v-html", "{@html"} {
					assertTemplateMissing(t, name, unsafe)
				}
			})
		}
	}
}

// Exercise the generated handler against real anduin and inertia packages, not
// just the template text. The small module substitutes only domain sentinels.
func TestGeneratedErrorHandlerResponses(t *testing.T) {
	root := t.TempDir()
	if err := renderTemplate(root, "handlers_errors.tmpl", "handlers/errors.go", layouttemplates.Files, &TemplateData{ModuleName: "testapp"}); err != nil {
		t.Fatal(err)
	}
	writeLayoutTestFile(t, root, "domains/errors.go", `package domains
import "errors"
var ErrNotFound = errors.New("domain not found")
var ErrConflict = errors.New("domain conflict")
`)
	var module strings.Builder
	fmt.Fprintf(&module, "module testapp\n\ngo %s\n", goVersion)
	for _, name := range []string{"anduin", "inertia", "routing", "telemetry"} {
		path, err := filepath.Abs(filepath.Join("..", "pkg", name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&module, "\nrequire github.com/mbvlabs/andurel/pkg/%s v0.0.0\nreplace github.com/mbvlabs/andurel/pkg/%s => %q\n", name, name, filepath.ToSlash(path))
	}
	writeLayoutTestFile(t, root, "go.mod", module.String())
	writeLayoutTestFile(t, root, "handlers/errors_test.go", generatedErrorResponseTest)
	cmd := exec.Command("go", "test", "-v", "-mod=mod", "./handlers")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated error handler tests: %v\n%s", err, output)
	} else {
		t.Logf("generated scratch HTTP tests:\n%s", output)
	}
}

const generatedErrorResponseTest = `package handlers

import (
 "context"
 "encoding/json"
 "errors"
  "fmt"
  "io"
  "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
 "testapp/domains"

 "github.com/mbvlabs/andurel/pkg/anduin"
 "github.com/mbvlabs/andurel/pkg/inertia"
)

type fieldError struct{}
func (fieldError) Error() string { return "validation" }
func (fieldError) ToMap() map[string]string { return map[string]string{"email":"required"} }

func TestResponses(t *testing.T) {
 renderer, err := inertia.NewRenderer(inertia.RendererConfig{ContainerID:"app", SSRURL:"http://127.0.0.1:13714", SSRTimeout:time.Second, SSRMaxResponseBytes:1<<20}, inertia.WithRootFunc(func(w io.Writer, data inertia.RootData) error {
  script, err := data.PageScript()
  if err != nil { return err }
  _, err = io.WriteString(w, string(script))
  return err
 }))
 if err != nil { t.Fatal(err) }
 tests := []struct {
  name string; err error; accept string; inertia bool; status int; component, message, contains string
 }{
  {"400 explicit", anduin.Errorf(400, "bad <input>"), "", true, 400, "Errors/BadRequest", "bad <input>", ""},
  {"404 explicit", anduin.Errorf(404, "missing item"), "", true, 404, "Errors/NotFound", "missing item", ""},
  {"drive 400", &anduin.HTTPError{Status:400, Message:"drive: invoice id must be numeric"}, "", true, 400, "Errors/BadRequest", "drive: invoice id must be numeric", ""},
  {"drive 404", &anduin.HTTPError{Status:404, Message:"drive: invoice 42 does not exist"}, "", true, 404, "Errors/NotFound", "drive: invoice 42 does not exist", ""},
  {"wrapped domain sentinel", fmt.Errorf("outer: %w", &anduin.HTTPError{Status:404, Message:"missing domain", Err:domains.ErrNotFound}), "", true, 404, "Errors/NotFound", "missing domain", ""},
  {"wrapped router sentinel", &anduin.HTTPError{Status:404, Message:"missing route", Err:anduin.ErrNotFound}, "", true, 404, "Errors/NotFound", "missing route", ""},
  {"wrapped conflict", &anduin.HTTPError{Status:409, Message:"already exists", Err:domains.ErrConflict}, "", true, 409, "Errors/Conflict", "already exists", ""},
  {"wrapped method sentinel", &anduin.HTTPError{Status:405, Message:"method unavailable", Err:anduin.ErrMethodNotAllowed}, "", true, 405, "Errors/NotFound", "method unavailable", ""},
  {"plain 403", anduin.Errorf(403, "access denied"), "", true, 403, "", "", "access denied"},
  {"plain cause 403", anduin.Forbidden(errors.New("private cause")), "", true, 403, "", "", "Forbidden"},
  {"sentinel fallback", domains.ErrNotFound, "", true, 404, "Errors/NotFound", "", ""},
  {"cause is not public", anduin.BadRequest(errors.New("private cause")), "", true, 400, "Errors/BadRequest", "", ""},
  {"server message hidden", anduin.Errorf(500, "private server detail"), "", true, 500, "Errors/InternalError", "", ""},
  {"server wrapping mapped sentinel", &anduin.HTTPError{Status:500, Message:"private server detail", Err:domains.ErrNotFound}, "", true, 404, "Errors/NotFound", "", ""},
  {"validation precedence", &anduin.HTTPError{Status:400, Message:"wrong", Err:fieldError{}}, "", true, 422, "Errors/Validation", "", "required"},
  {"JSON explicit", anduin.Errorf(400, "bad input"), "application/json", false, 400, "", "", ` + "`" + `"error":"bad input"` + "`" + `},
  {"JSON wrapped explicit mapped", fmt.Errorf("outer: %w", &anduin.HTTPError{Status:400, Message:"missing domain", Err:domains.ErrNotFound}), "application/json", false, 404, "", "", ` + "`" + `"error":"missing domain"` + "`" + `},
  {"JSON cause private", anduin.BadRequest(errors.New("private cause")), "application/json", false, 400, "", "", ` + "`" + `"error":"Bad Request"` + "`" + `},
  {"JSON server mapped private", &anduin.HTTPError{Status:500, Message:"private server detail", Err:domains.ErrNotFound}, "application/json", false, 404, "", "", ` + "`" + `"error":"Not Found"` + "`" + `},
  {"JSON validation precedence", &anduin.HTTPError{Status:400, Message:"wrong", Err:fieldError{}}, "application/json", false, 422, "", "", ` + "`" + `"fields":{"email":"required"}` + "`" + `},
  {"Inertia wins over JSON accept", &anduin.HTTPError{Status:404, Message:"visible", Err:domains.ErrNotFound}, "application/json", true, 404, "Errors/NotFound", "visible", ""},
 }
 for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.Header.Set("Accept", tt.accept)
  if tt.inertia { req.Header.Set(inertia.HeaderInertia, "true") }
  rec := httptest.NewRecorder()
  ctx := anduin.ReplaceResponseWriter(anduin.ReplaceRequest(context.Background(), req), rec)
  if err := MapError(renderer)(ctx, tt.err); err != nil { t.Fatalf("MapError: %v", err) }
  if rec.Code != tt.status { t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body.String()) }
  body := rec.Body.String()
  if tt.contains != "" && !strings.Contains(body, tt.contains) { t.Errorf("body %q missing %q", body, tt.contains) }
   if strings.Contains(body, "private cause") || strings.Contains(body, "private server detail") { t.Errorf("private message leaked: %s", body) }
  if tt.component != "" {
   var page struct { Component string; Props map[string]any }
   if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil { t.Fatalf("decode: %v: %s", err, body) }
   if page.Component != tt.component { t.Errorf("component %q, want %q", page.Component, tt.component) }
   got, _ := page.Props["message"].(string)
   if got != tt.message { t.Errorf("message %q, want %q", got, tt.message) }
  }
 }) }
 t.Run("real HTTP scratch probe", func(t *testing.T) {
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   ctx := anduin.ReplaceResponseWriter(anduin.ReplaceRequest(context.Background(), r), w)
   var failure error
   switch r.URL.Path {
   case "/explicit": failure = fmt.Errorf("wrapped: %w", anduin.Errorf(400, "bad <input>"))
   case "/explicit404": failure = anduin.Errorf(404, "missing item")
   case "/cause": failure = anduin.BadRequest(errors.New("private cause"))
   case "/mapped": failure = &anduin.HTTPError{Status:500, Message:"private server detail", Err:domains.ErrNotFound}
   }
   if err := MapError(renderer)(ctx, failure); err != nil { t.Errorf("handler: %v", err) }
  }))
  defer server.Close()
  for _, probe := range []struct{path, accept, inertia, want string; status int}{
   {"/explicit", "application/json", "", ` + "`" + `"error":"bad \u003cinput\u003e"` + "`" + `, 400},
   {"/explicit404", "application/json", "", ` + "`" + `"error":"missing item"` + "`" + `, 404},
   {"/cause", "application/json", "", ` + "`" + `"error":"Bad Request"` + "`" + `, 400},
   {"/mapped", "application/json", "", ` + "`" + `"error":"Not Found"` + "`" + `, 404},
   {"/explicit", "", "true", ` + "`" + `"message":"bad \u003cinput\u003e"` + "`" + `, 400},
  } {
   req, _ := http.NewRequest(http.MethodGet, server.URL+probe.path, nil)
   req.Header.Set("Accept", probe.accept); req.Header.Set(inertia.HeaderInertia, probe.inertia)
   resp, err := http.DefaultClient.Do(req); if err != nil { t.Fatal(err) }
   body, err := io.ReadAll(resp.Body); resp.Body.Close(); if err != nil { t.Fatal(err) }
   t.Logf("%s accept=%q inertia=%q status=%d body=%s", probe.path, probe.accept, probe.inertia, resp.StatusCode, strings.TrimSpace(string(body)))
   if resp.StatusCode != probe.status || !strings.Contains(string(body), probe.want) { t.Fatalf("unexpected response") }
  }
 })
 t.Run("initial document escapes message", func(t *testing.T) {
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  rec := httptest.NewRecorder()
  ctx := anduin.ReplaceResponseWriter(anduin.ReplaceRequest(context.Background(), req), rec)
  if err := MapError(renderer)(ctx, anduin.Errorf(400, "bad </script><script>alert(1)</script>")); err != nil { t.Fatal(err) }
  if rec.Code != 400 { t.Fatalf("status %d: %s", rec.Code, rec.Body.String()) }
  body := rec.Body.String()
  if strings.Contains(body, "</script><script>alert") || !strings.Contains(body, "alert(1)") { t.Fatalf("unsafe or missing message: %s", body) }
 })
}
`
