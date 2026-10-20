# Test integration

An integration test boots the entire real graph with `runtime.New(ctx, cfg)`.
There is no Fx and no reflection: the graph is explicit code, so the test
constructs exactly what production does and gets `Handler` and `DB`. The
database comes from the package's test cluster, so the test never touches the
development database.

## Files

### internal/runtime/runtime_test.go
```go
package runtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/runtime"
	"app/routes"
)

func TestAppServesHealth(t *testing.T) {
	app, err := runtime.New(t.Context(), testConfig(t))
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	// t.Context() is already canceled when cleanups run; keep the close alive.
	t.Cleanup(func() { _ = app.Close(context.WithoutCancel(t.Context())) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, routes.Health.Path(), nil)
	rec := httptest.NewRecorder()
	app.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
```

## Rules

- The scaffold ships `internal/runtime/main_test.go` with `testConfig(t)` and `internal/runtime/runtime_test.go` as the worked example. Add tests beside them instead of defining another `TestMain`.
- Boot the same graph production uses via `runtime.New`.
- `testConfig(t)` loads the project `.env` when present, falls back to fixed values for the required keys, and replaces `cfg.Database` with the `storage.Config` from `storagetest.Cluster.NewTestDBWithConfig`. A fresh scaffold has no `.env`; the fallbacks keep `verify --with-tests` hermetic.
- Integration tests cover cross-domain flows that handler tests cannot.
- Close the app with `app.Close` to release the database and telemetry, passing `context.WithoutCancel(t.Context())` because `t.Context()` is canceled before cleanup runs.
- Use a real Postgres test database from the package cluster; migrations are applied before `runtime.New` boots.
- Do not reach into unexported runtime fields.
- `routing.ConfigureHosts` writes a package-level registry. Tests that call `runtime.New` use the same hosts config and do not use `t.Parallel()`.

## Checklist

- [ ] `runtime.New` boots without error.
- [ ] `app.Close` runs in cleanup.
- [ ] Cross-domain behavior is exercised end to end.
- [ ] `go test ./internal/runtime/...` passes with Postgres available.
