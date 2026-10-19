# Test integration

An integration test boots the entire real graph with `runtime.New(ctx, cfg)`.
There is no Fx and no reflection: the graph is explicit code, so the test
constructs exactly what production does and gets `Handler` and `DB`.

## Files

### internal/runtime/runtime_test.go
```go
package runtime_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/runtime"
)

func TestAppBoots(t *testing.T) {
	app, err := runtime.New(t.Context(), testConfig(t))
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(t.Context()) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	if _, err := io.Copy(io.Discard, rec.Result().Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
}
```

## Rules

- Boot the same graph production uses via `runtime.New`.
- Integration tests cover cross-domain flows that handler tests cannot.
- Close the app with `app.Close` to release the database and telemetry.
- Use a real Postgres test database; migrations are applied at boot.
- Do not reach into unexported runtime fields.

## Checklist

- [ ] `runtime.New` boots without error.
- [ ] `app.Close` runs in cleanup.
- [ ] Cross-domain behavior is exercised end to end.
- [ ] `go test ./internal/runtime/...` passes with Postgres available.
