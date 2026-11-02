# runtime-integration (published modules)

Standalone cross-module integration tests for the rc.1 coordinated error
lifecycle: published `pkg/anduin` router + published `pkg/kiks` cookie
middleware + published `pkg/inertia` capture middleware, with the published
`pkg/routing` host registry and `pkg/telemetry` dependency.

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
  framework/site release CI, not release goldens, scaffold/upgrade/E2E or T07
  live/image proof, and not a release approval.
- Do not import, publish or tag this module, or use it as an application pin.

Full evidence and the published origin/checksum record are in
`plans/rc1-final-readiness/t09-published-integration.md`.

## Run

```sh
cd plans/rc1-final-readiness/runtime-integration
unset GOPRIVATE GONOSUMDB GONOPROXY GOFLAGS
export GOWORK=off
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org

gofmt -l .
go fix ./...
go vet ./...
go test -mod=readonly -count=1 ./...
go test -count=10 ./...
CGO_ENABLED=1 \
  CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' \
  go test -race -count=1 ./...
go mod verify
go mod tidy -diff
```

All tests are default-on. They use `httptest` only: synthetic hosts, synthetic
cookie/session values, a renderer root that writes a test document, and no
network, database, Docker or production data.

## Historical development mode

Before the published-module proof, this harness was marked **DEVELOPMENT ONLY**
and replaced all five Andurel packages with local working-tree paths. That run
proved the then-current local source only; it did not prove published modules,
public proxy/checksum resolution or a release-shaped path. It remains
historical evidence, not current proof:

- `plans/rc1-final-readiness/t02-integration.md` — unchanged development report.
- `plans/rc1-final-readiness/evidence/t02-integration-*.txt` — unchanged
  development red/green/race logs.
- `plans/rc1-final-readiness/evidence/t09-development-go.mod.txt`,
  `...-go.sum.txt`, `...-README.md`, `...-doc.go.txt` — snapshots of the
  pre-conversion development module and wording.
