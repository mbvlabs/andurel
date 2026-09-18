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

Fixtures for these scenarios are under `testdata/fixtures/generate_base`,
`testdata/fixtures/generate_inertia_vue`, and `testdata/fixtures/sync_factory_base`.
Migration SQL for generate scenarios is reused from `generator/testdata/migrations/`
via the harness.

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
tree (smoke, generate, sync). Commit the resulting diffs when expectations change.

## Verifying

```bash
just test-golden
```
