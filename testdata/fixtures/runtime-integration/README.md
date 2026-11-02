# runtime-integration fixture

Standalone cross-module integration fixture for the coordinated router, kiks
and Inertia error lifecycle: the published `pkg/anduin` router, the published
`pkg/kiks` cookie middleware, the published `pkg/inertia` capture middleware,
and the published `pkg/routing` host registry and `pkg/telemetry` dependency.

The fixture lives at `testdata/fixtures/runtime-integration/`, alongside the
other standalone fixture modules under `testdata/fixtures/`. It is not part of
the root module or workspace graph.

## Mode

- This directory is a standalone Go module outside the repository's module and
  workspace graph.
- `go.mod` has **no `replace` directives**. It requires public
  `anduin v0.8.0`, `inertia v0.14.1`, `kiks v0.6.1`, `routing v0.9.1` and
  `telemetry v0.2.3`; all resolution goes through
  `GOPROXY=https://proxy.golang.org` and `GOSUMDB=sum.golang.org`.
- Run with `GOWORK=off` and no `GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` or
  `GOFLAGS` overrides.
- A green run proves these published module versions work together through the
  real router, kiks, Inertia, routing and telemetry code. It is **not** full
  framework/site release CI, not release goldens, scaffold/upgrade/E2E or live
  image proof, and not a release approval.
- Do not import, publish or tag this module, or use it as an application pin.

## Run

```sh
cd testdata/fixtures/runtime-integration
unset GOPRIVATE GONOSUMDB GONOPROXY GOFLAGS
export GOWORK=off
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org

gofmt -l .
go fix ./...
go vet ./...
go test -mod=readonly -count=1 ./...
go test -mod=readonly -count=10 ./...
CGO_ENABLED=1 CC="$(command -v zig) cc" \
  go test -mod=readonly -race -count=1 ./...
go mod verify
go mod tidy -diff
```

All tests are default-on. They use `httptest` only: synthetic hosts, synthetic
cookie/session values, a renderer root that writes a test document, and no
network, database, Docker or production data.

## Provenance

This fixture was promoted from a local validation harness. Its test `.go` files,
`go.mod` and `go.sum` are byte-identical; only package documentation and this
README were adjusted for the durable `testdata/fixtures/runtime-integration/`
path. The local plans archive is intentionally excluded from the tracked tree.
