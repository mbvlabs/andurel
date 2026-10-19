# Golden files

CLI-driven golden tests live under `golden/` and compare **raw bytes** written by
the `andurel` binary (built with `-tags andurel_golden` and
`-ldflags -X main.version=latest`) against committed files here.

There are two tracks:

| Track | Recipe | What runs |
|-------|--------|-----------|
| **PR** | `just test-golden` / `just update-golden` | `generate migration`, `sync`, and a slim `andurel new` capture. Fast, curated `AssertFiles` / `AssertDir`. |
| **Nightly** | `just test-golden-full` / `just update-golden-full` | Same suite **plus** full-tree `andurel new` for the three Inertia adapters. |

Full-tree `andurel new` tests skip unless `ANDUREL_GOLDEN_FULL=1`. The PR track
still runs a slim capture for `andurel new` (Inertia react and vue).

## How it works

`go test ./golden/...` exercises the real CLI end-to-end:

1. `TestMain` (`golden/main_test.go`) locates the repo root, builds the
   `andurel` binary with `-tags andurel_golden -ldflags -X main.version=latest`,
   `go install`s templ/golines/goimports at the versions from `layout/versions`,
   and downloads narsilc, tailwindcli, and goose from the same lock URL
   templates and SHA-256 digests as `andurel.lock`.
2. Each generate/sync test clones a fixture from `testdata/fixtures/`, seeds
   pinned tools into the project's `bin/`, then runs the prebuilt CLI in that
   project directory. Fixtures carry their own `migrations/`.
3. The harness reads files the CLI wrote and hands the **raw bytes** to
   `goldie/v2`, asserting byte-for-byte equality against the mirror path under
   this tree. The golden name is the mirrored relative path (e.g.
   `domains/internal/queries/widget.sql.go` under `sync/queries/`).
4. Running tests **compare** against the committed `.golden` files. Passing
   `-update` (via `just update-golden` or `just update-golden-full`) **writes**
   them instead. Expected files are never hand-written.

The harness (`internal/goldentest/harness.go`) isolates the process
environment: it prepends the pinned-tool bin to `PATH`, sets `ANDUREL_TOOL_BIN`,
and forces `GOWORK=off` so the CLI behaves like a standalone user install.

## Determinism contract

The harness **never normalizes, scrubs, or reformats captured bytes**
(`harness.go` doc comment). Byte stability is engineered into the product and
fixtures, not the test harness:

- `-tags andurel_golden` switches `internal/testseed` to a fixed seed: database
  secrets use a deterministic stream and the CLI date banner is pinned to
  `2025-01-01T00:00:00Z`. Scaffold migrations are written with finished
  sequential versions (`00001`–`00008`), so filenames need no goose fix.
  Without the tag, `internal/testseed/seed_default.go` is compiled instead.
- Under the tag, `layout.Scaffold` skips `go mod tidy` and `go fmt`
  **unless** `testseed.FullScaffold()` is true (`andurel_golden_full` tag or
  `ANDUREL_GOLDEN_FULL=1`). Compiled templ (`*_templ.go`) and narsilc query
  packages are written from embeds during template rendering (no `templ` /
  `narsilc` invocation on `andurel new`). PR goldens stay offline; nightly
  full-tree `new` still runs tidy/fmt with pinned tools on
  `ANDUREL_TOOL_BIN`.
- The golden CLI is built with `-ldflags -X main.version=latest`, so
  `andurel.lock`'s `version` field and any version banners stay stable without
  scrubbing.
- Pinned formatter and lock-tool versions make `generate`/`sync` output identical
  on every machine.

Adding a golden expectation therefore means proving a path is byte-stable
without network, external tools, or absolute temp paths — never adding a
scrubber. If a path floats, denylist it (nightly) or omit it from the allowlist
(PR).

## Layout

| Path | Source |
|------|--------|
| `generate/migration/` | `andurel generate migration …` (dummy SQL body) |
| `sync/queries/` | `andurel sync queries` |
| `sync/routes/` | `andurel sync routes` |
| `sync/payloads/` | `andurel sync payloads` |
| `sync/email/` | `andurel sync email` |
| `new/slim/` | PR slim `andurel new` dirs (templates only, no go fmt) |
| `new/` | Nightly only: `andurel new app` full tree via `AssertTree` |

Fixtures for generate/sync scenarios are under `testdata/fixtures/generate_base`,
`testdata/fixtures/generate_inertia_vue`, `testdata/fixtures/job_smoke`, and
`testdata/fixtures/sync_email_base`.

`andurel new` scenarios create an empty parent directory and run
`andurel new app` (optionally `--ui vue/pnpm`). The module path is the project
name (`app`), not `example.com/app`.

PR CI captures the slim scaffold under `new/slim/` without running tidy/fmt.
Compiled `*_templ.go` and narsilc query packages are included from embeds.
Nightly full-tree capture lives under `new/<scenario>/` for every adapter with
tidy/fmt on. Those directories must stay separate: formatted nightly trees must
not be the PR baseline.

### Nightly `andurel new` capture strategy

Assertions use `goldentest.AssertTree`: walk the project, goldie-assert every
file except the denylist, then fail (or, with `-update`, delete) orphan
`.golden` files that have no project counterpart.

`ANDUREL_GOLDEN_FULL=1` makes `layout.Scaffold` run `go fmt` and
`go mod tidy` with pinned tools from `ANDUREL_TOOL_BIN`. Compiled templ and
narsilc query packages are always written from embeds (no scaffold-time
`templ` / `narsilc` generate). Migration files are written with finished
sequential versions, so goose fix is not part of scaffold.

**Denylist** (no content scrubbers):

| Skip | Reason |
|------|--------|
| `bin/` | Tool downloads / seeded binaries |
| `go.sum` | tidy churn until proven stable across CI runs |
| `.git/` | Local `git init` metadata |
| `.env` / `.env.example` | Secrets (even when seeded) |
| `node_modules/` | JS package install artifacts if present |

**Captured** (generators have run): `domains/internal/queries/`, `*_templ.go`,
`go.mod`, and the rest of the tree.

Secrets in `.env.example` still come from `internal/testseed` (fixed RNG +
`2025-01-01T00:00:00Z`) for product behavior; they are omitted from capture.
No test normalizers or content scrubbers.

`generate migration` calls goose `create`. The filename uses goose's wall-clock
UTC timestamp (`YYYYMMDDHHMMSS_create_widgets.sql`). The test globs that path
and goldie-asserts the file bytes under a stable name
(`migrations/create_widgets.sql`). The body is goose's default SQL stub.

## Coverage map

| Family | Scenarios | What is asserted |
|--------|-----------|------------------|
| `generate/migration/` | 1 | goose SQL stub (timestamped filename globbed, body goldened as `migrations/create_widgets.sql`) |
| `sync/queries/` | 1 | narsilc output under `domains/internal/queries/` from the fixture's `domains/widget.sql` and `domains/user.sql` |
| `sync/routes/` + `sync/payloads/` | 2 | Inertia `routes.ts` / `payloads.ts` |
| `sync/email/` | 1 | compiled `*_templ.go` renderers |
| `new/` (PR) | 2 | slim Inertia react + vue dirs (`domains`, `services`, `handlers`, `routes`, `middleware`, `resources/js/Pages`) |
| `new/` (nightly) | 3 | vue, react, svelte — full tree minus denylist |

## Adding a golden scenario

1. Mirror an existing test in `golden/`: pick a fixture (or an empty temp dir
   for nightly `new`), the CLI args, and the exact relative paths to capture
   (`AssertFiles`) or `AssertTree` for full-tree nightly `new`.
2. Run `just install-dev-tools` once, then `just update-golden` (PR track) or
   `just update-golden-full` (includes `new/`) to write the new `.golden` files.
3. Review the diff. `-update` blesses whatever the binary produced, so confirm
   the new goldens match intent and contain no machine-specific bytes
   (absolute paths, timestamps, versions).

## Regenerating

Golden expected files are **not** hand-written. After installing pinned tools:

```bash
just install-dev-tools
just update-golden
```

That runs `go test ./golden/... -update` for the PR generate+sync suite.
CI never blesses goldens: PR jobs run `just ci-pr` / `just test-golden`.
Nightly runs `just ci-nightly` (`just test-golden-full`).

```bash
just update-golden-full
```

sets `ANDUREL_GOLDEN_FULL=1` and also blesses nightly full-tree `new/` goldens.
Commit the resulting diffs when expectations change.

## Verifying

```bash
just test-golden
just test-golden-full   # just ci-nightly; used by e2e-nightly
```
