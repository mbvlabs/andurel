# Golden files

CLI-driven golden tests live under `golden/` and compare **raw bytes** written by
the `andurel` binary (built with `-tags andurel_golden`) against committed files
here.

## Layout

| Path | Source |
|------|--------|
| `smoke/` | Smoke scenarios (e.g. `generate job`) |
| `generate/model/` | `andurel generate model …` |
| `generate/scaffold/` | `andurel generate scaffold …` |
| `generate/controller/` | `andurel generate controller …` |
| `sync/factory/` | `andurel sync factory … --sync` |
| `sync/factories/` | `andurel sync factories --sync` |
| `new/` | `andurel new app` (empty parent temp dir; no fixture copy) |

Fixtures for generate/sync scenarios are under `testdata/fixtures/generate_base`,
`testdata/fixtures/generate_inertia_vue`, and `testdata/fixtures/sync_factory_base`.
Migration SQL for generate scenarios is reused from `generator/testdata/migrations/`
via the harness.

`andurel new` scenarios create an empty parent directory and run
`andurel new app` (optionally `--inertia vue`). The module path is the project
name (`app`), not `example.com/app`.

### `andurel new` capture strategy

Assertions use a **curated allowlist** of relative paths (not a full tree walk).
Under `-tags andurel_golden`, `layout.Scaffold` writes templates/lock/migrations
then skips goose fix, templ generate, narsilc generate, `go mod tidy`, and
`go fmt` so the run stays offline and byte-stable.

**Intentionally not goldened** (unstable or absent without network/tools):

| Skip | Reason |
|------|--------|
| `bin/` | Tool downloads happen via `andurel tool sync`, not `new` |
| `go.sum` | Skipped tidy; would float with the module proxy |
| `.git/` | Local `git init` metadata |
| `models/internal/queries/` | narsilc generate skipped under golden |
| Extra `*_templ.go` | templ generate skipped; only template-emitted copies exist |
| Full tree | Prefer allowlist over denylist walk until every path is proven raw-stable |

Secrets in `.env.example` and migration timestamps come from `internal/testseed`
(fixed RNG + `2025-01-01T00:00:00Z`). No test normalizers or content scrubbers.

### Sync factory fixture

`sync_factory_base` is a minimal project with:

- `models/widget.go` / `models/product.go` — Entity structs + `andurel:table` markers
  (slimmed as if after `generate model`, without query/CRUD bodies)
- `models/factories/widget.go` — intentionally stale factory plus a hand-written
  `CustomWidgetScore` helper that sync must preserve
- no `models/factories/product.go` — missing factory created by sync

Tests only run `andurel sync …`; they do not regenerate models first.

`--check --json` stdout is **not** goldened: result paths are absolute temp dirs.

## Regenerating

Golden expected files are **not** hand-written. After installing pinned tools:

```bash
just install-dev-tools
just update-golden
```

That runs `go test ./golden/... -update` and refreshes every scenario under this
tree (smoke, generate, sync, new). Commit the resulting diffs when expectations change.

## Verifying

```bash
just test-golden
```
