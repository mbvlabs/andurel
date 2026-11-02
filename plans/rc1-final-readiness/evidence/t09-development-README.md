# runtime-integration (DEVELOPMENT ONLY)

Cross-module integration tests for the rc.1 coordinated error lifecycle:
real `pkg/anduin` router + real `pkg/kiks` cookie middleware + real
`pkg/inertia` capture middleware.

## Not release proof

- This directory is a standalone Go module outside the repository's module and
  workspace graph.
- Its `go.mod` replaces `pkg/anduin`, `pkg/inertia`, `pkg/kiks`, `pkg/routing`
  and `pkg/telemetry` with local working-tree paths.
- It must be run with `GOWORK=off`. A green run proves only that the current
  local source works together; it does not prove published modules, public
  proxy/checksum resolution, release goldens or scaffold output.
- Do not publish, tag, pin or reference this module from an application.

Release-shaped proof remains T08/T09 after the anduin module is published and
all framework pins are updated.

## Run

```sh
cd plans/rc1-final-readiness/runtime-integration
GOWORK=off go test -count=1 ./...
GOWORK=off go vet ./...
```

Race validation uses the repository's Zig/cgo shim:

```sh
GOWORK=off CGO_ENABLED=1 \
  CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...
```

All tests are default-on. They use `httptest` only: synthetic hosts, synthetic
cookie/session values, a renderer root that writes a test document, and no
network, database, Docker or production data.
