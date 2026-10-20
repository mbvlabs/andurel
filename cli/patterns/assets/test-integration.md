# Test integration

An integration test boots the entire real graph with `runtime.New(ctx, cfg)`.
There is no Fx and no reflection: the graph is explicit code, so the test
constructs exactly what production does and gets `Handler` and `DB`.

## Files

### internal/runtime/main_test.go
```go
package runtime_test

import (
	"testing"

	"app/config"
)

// testConfig loads the project's .env and builds the same config cmd/app uses.
// The test binary runs in internal/runtime, so .env is two directories up.
func testConfig(t *testing.T) config.Config {
	t.Helper()

	if err := config.LoadEnvironment("../../.env"); err != nil {
		t.Fatalf("load environment: %v", err)
	}

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	return cfg
}
```

### internal/runtime/runtime_test.go
```go
package runtime_test

import (
	"context"
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
	// t.Context() is already canceled when cleanups run; keep the close alive.
	t.Cleanup(func() { _ = app.Close(context.WithoutCancel(t.Context())) })

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
- Close the app with `app.Close` to release the database and telemetry, passing `context.WithoutCancel(t.Context())` because `t.Context()` is canceled before cleanup runs.
- `testConfig(t)` lives in `internal/runtime/main_test.go`; it loads the project `.env` and builds the same config `cmd/app` does.
- Use a real Postgres test database; migrations are applied at boot.
- Do not reach into unexported runtime fields.

## Checklist

- [ ] `runtime.New` boots without error.
- [ ] `app.Close` runs in cleanup.
- [ ] Cross-domain behavior is exercised end to end.
- [ ] `go test ./internal/runtime/...` passes with Postgres available.
