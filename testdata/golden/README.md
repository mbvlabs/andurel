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

Fixtures for these scenarios are under `testdata/fixtures/generate_base` and
`testdata/fixtures/generate_inertia_vue`. Migration SQL is reused from
`generator/testdata/migrations/` via the harness.

## Regenerating

Golden expected files are **not** hand-written. After installing pinned tools:

```bash
just install-dev-tools
just update-golden
```

That runs `go test ./golden/... -update` and refreshes every scenario under this
tree (including smoke). Commit the resulting diffs when Phase 3 syncs expectations.

## Verifying

```bash
just test-golden
```
