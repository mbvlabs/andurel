# Agent gate hardening — next alpha

- **Status:** Complete; all workstreams landed
- **Date:** 2026-10-20
- **Baseline:** `master` @ `71510391` (post `v2.0.0-alpha.4`)
- **Compared against:** `v2.0.0-alpha.3` (last Echo/Fx v2) and `v1.5.5` (stable)
- **Relates to:** `plans/rfc-agent-first-go-native.md` (rev 6), `plans/v2-alpha-readiness-review.md`

## Why

The agent-first rebuild delivered its structural goals: `net/http` via `pkg/anduin`, a single
explicit `runtime.New`, flat `domains/` with colocated SQL, Inertia only, read-only `patterns`,
and a `verify` gate with structured violations and exit code 9.

The two load-bearing pieces of the agent loop are weaker than the docs claim:

1. **Patterns are the API, but they are untested** — several snippets do not compile.
2. **`verify` is the gate, but it is narrow and can report green on skipped work** — most
   transaction-escape shapes pass, raw SQL handles pass, and missing tool binaries pass.

Removing the v1/alpha generators also removed the only deterministic schema → entity path
(`generator/internal/ddl` + `catalog`) without moving that determinism into verification.

---

## Handover: read this before starting any workstream

### Ground rules

- Follow the repo `AGENTS.md`: run `gofmt`, `go fix` and `go vet` on everything you touch.
  Never use python or perl, including for file edits.
- Any change under `pkg/<module>/` adds an entry under `## Unreleased` in
  `pkg/<module>/CHANGELOG.md`.
- **Never add `Co-Authored-By` or any AI/Claude attribution** to commits or PR bodies.
- Commits use conventional commits; CI enforces this with `.github/workflows/conventional_commit.yml`.
  Types: `feat fix docs test ci refactor perf chore revert`. Look at `git log` for scopes
  (`cli`, `scaffold`, `pkg`, `golden`). Mark breaking changes with `!`.
- Domain entities use `Validate()` (via `pkg/validation`), not `ParseX` newtypes. Don't
  "improve" this.
- `.env.example` dev secrets are intentional. Leave them.
- Line numbers in this doc are as of `71510391`. If they have drifted, search by symbol.

### Git and releases

- One branch per workstream off `master`: `<type>/w<N>-<slug>`, where `<type>` is the
  conventional-commit type of the work (for example `fix/w2-verify-skipped`). W10 items get
  one branch each: `<type>/w10a-…`. Land each commit on `master` individually
  (fast-forward or rebase), never as a merge commit.
- Commit locally. **Do not push, tag, or open PRs** unless the human asks.
- `pkg/*` modules are released by the human. They are tagged as `pkg/<name>/vX.Y.Z` and the
  versions live in `layout/versions/versions.go` and the root `go.mod`. See
  `docs/releasing.md`. When a workstream changes `pkg/`, stop after the code + changelog
  commit and report "needs release: pkg/<name>". The pin bump in `layout/versions` and
  `go.mod`, plus golden refresh, happens in a follow-up commit after the tag exists. CI and
  `just` recipes run with `GOWORK=off`, so CLI code that uses an unreleased `pkg/` symbol is
  red until then. That is intended.

### Commands

| What | Command |
|------|---------|
| Build the CLI | `just build` (writes `./dev-andurel`) |
| Vet (root + every `pkg/*` module) | `just vet` |
| Unit tests | `just test` (or `go test ./cli/...` for a package) |
| PR golden suite | `just test-golden` |
| Full-scaffold goldens (45 min timeout) | `just test-golden-full` |
| Refresh goldens after an intended scaffold change | `just update-golden` / `just update-golden-full` |
| Command metadata / generated docs check | `just commands-check` |
| What PR CI runs | `just ci-pr` |

Full-scaffold tests gate on `ANDUREL_GOLDEN_FULL=1` via
`goldentest.RequireFullScaffold` (`internal/goldentest/tools.go:96`). A manual smoke test:
`./dev-andurel new app` in a temp dir, then `just local-replace <dir>` to point it at local
`pkg/*`.

### Definition of done (every workstream)

1. Every acceptance box in that workstream is satisfied and ticked in this file on the branch.
2. `just vet`, `just test`, `just commands-check` and `just test-golden` pass. If the scaffold or
   `verify` changed, `just test-golden-full` passes as well.
3. Goldens are refreshed only for intended changes. Explain each changed golden in the commit
   body.
4. A short report: what changed, what was deliberately not done, and any "needs release" or
   follow-up.

### Execution order and conflicts

Workstreams in the same wave can run in parallel. Each wave assumes the previous wave is merged.

| Wave | Workstreams | Notes |
|------|-------------|-------|
| 1 | W1, W2, W3, W9-drift, W10a | Disjoint files. W9-drift is only W9 proposal item 3. |
| 2 | W4, W8 | W4 and W2 both edit `cli/verify.go`, so W4 waits for W2. W8 ends at "needs release: pkg/storage". |
| 3 | W5, W7 | W5 reuses W4's loader and rule catalog. W7 needs W8 tagged and pinned, and W1's pattern text. |
| 4 | W10b, W10c, W10d | One at a time: W7, W10c and W10d all rewrite the same scaffold goldens. |
| 5 | W6, W9 (generation), W10e | All read the W4 rule catalog or the W5 column data. |

Shared hot spots:

- **Test-cluster call sites.** These are the patterns plus the scaffold tests from W7. W1 fixes
  them against `storage.NewTestCluster`. W8 then renames them mechanically to `storagetest`.
  Do not fold W8 into W1, because W8 needs a release.
- **Rule catalog.** W4 creates it. W9, W10e and W5 consume it.
- **`cli/verify.go`.** W2, then W4, then W10e, in that order.

---

## Priority overview

| ID | Workstream | Priority | Size | Touches `pkg/` |
|----|------------|----------|------|----------------|
| W1 | Pattern snippets compile, and a test proves it | P0 | S–M | no |
| W2 | `verify` never reports a skipped check as a pass | P0 | S | no |
| W3 | Upgrade gate for pre-rebuild v2 projects | P0 | S | no |
| W4 | Layer lints on `go/types` + close known gaps | P1 | L | no |
| W5 | Entity ↔ query column check (`entity-columns`) | P1 | M | no |
| W6 | `patterns show domain --entity` emits real code | P2 | M | no |
| W7 | Scaffold ships one test per layer | P1 | M | no |
| W8 | Move test containers out of `pkg/storage` | P2 | S | yes (`pkg/storage`) |
| W9 | One source of truth for agent docs + drift fixes | P2 | M | no |
| W10 | Small agent-surface fixes | P2 | S each | no |

---

## W1. Pattern snippets compile, and a test proves it — P0

### Problem

`andurel patterns show <kind>` is the replacement for generators, but `cli/patterns` has no tests
and several snippets do not compile against the real APIs.

### Evidence

- `storage.NewTestCluster` returns `(*TestCluster, error)` (`pkg/storage/psql.go:242`), but the
  patterns chain it as single-valued:
  - `cli/patterns/assets/domain.md:196`
  - `cli/patterns/assets/test-domain.md:45` and `:62`, plus the rule text at `:76`
  - `cli/patterns/assets/test-handler.md:29`
- Chaining per test also starts one Postgres container **per test** and never calls `Close`.
- `testRenderer(t)` (`test-handler.md:42`) and `testConfig(t)` (`test-integration.md`) are
  referenced but never defined, so the agent has to invent them.
- `test-integration.md` registers `t.Cleanup(func() { _ = app.Close(t.Context()) })`.
  `t.Context()` is canceled before cleanup functions run, so `Close` runs with a dead context.
- `domain.md` rules say "map unique-constraint failures to `ErrConflict`", but the worked
  `Create` example does not.
- `domain.md:131` has `### Atomic multi-statement methods (when needed)` under `## Files`.
  `parseMarkdown` (`cli/patterns/patterns.go:162`) treats every `###` in Files as a file path,
  so this becomes a "file" with that title as its path.

### Decisions (settled)

- **Test cluster shape.** Use one package-level cluster created in `TestMain`. The cluster is
  closed with `context.Background()` after `m.Run()`, and each test calls
  `cluster.NewTestDB(t, migrations.Migrations, ".")`. If `TestMain` cannot start the cluster
  (no Docker), it prints the reason and exits 0 with the tests skipped. It must not panic.
- **Cleanup context.** Cleanups use `context.WithoutCancel(t.Context())`.
- **Undefined helpers.** Define `testRenderer` and `testConfig` inline in the pattern now, in a
  `### handlers/main_test.go` / `### internal/runtime/main_test.go` file block. W7 later makes
  the scaffold ship the same helpers, and the patterns then reference those files.
- **`ErrConflict`.** Add the mapping to the `Create` example. Detect it with
  `errors.As(err, &pgErr)` and `pgErr.Code == "23505"`, using whatever
  `storage`/`pgconn` helper the scaffold already uses for that; if there is none, use
  `pgconn.PgError` directly.
- **Fragment marker.** The fence info string is split on whitespace. The first token stays
  `Language`, and a `fragment` token sets a new `File.Fragment bool`
  (`json:"fragment,omitempty"`). A file is **compile-tested** only if all of these hold:
  - its path ends in `.go`;
  - its path has no `<`;
  - its path has no `(edit)`;
  - it is not marked `fragment`.
- **Non-path headings.** Every `###` under `## Files` must be a path. Rename the
  atomic-methods heading to `### domains/product.go (edit: atomic methods)`.

### Proposal

1. Fix the snippets per the decisions above.
2. Add `cli/patterns/patterns_test.go`:
   - Every asset has a title, at least one file, rules, and a checklist.
   - Every `### ` heading under Files is a path, meaning it contains `/` or `.`.
   - Every `go` block parses with `go/parser`. Fragments are parsed as a file body after
     `package x`, or as statements wrapped in a func, whichever succeeds.
3. Add a full-scaffold compile test in `golden/` (gated by `RequireFullScaffold`):
   1. Scaffold `andurel new app`.
   2. Apply the `migration` pattern for `products`.
   3. Write every compile-tested file from all patterns.
   4. Run `andurel sync queries` to generate `ProductRow` and the queries.
   5. Run `go vet ./...` and `go test -run XXX_NONE ./...`, which compiles the tests without
      running them.
   - The golden `TestMain` already provides narsilc. See `install-dev-tools` and
     `internal/goldentest`.

### Acceptance

- [x] Every `NewTestCluster` call site in the patterns compiles and the cluster is closed.
- [x] No pattern references an undefined helper.
- [x] `go test ./cli/patterns/...` fails if any Go block stops parsing, or if a Files heading is
      not a path.
- [x] The full-scaffold compile test fails if a compile-tested snippet stops compiling against
      the scaffold.
- [x] `patterns show <kind> --json` output only gains `fragment` where marked. Other fields are
      unchanged, and `just commands-check` passes.

---

## W2. `verify` never reports a skipped check as a pass — P0

### Problem

On a fresh scaffold without tools synced, `andurel verify --json` returns `"ok": true` with:

```
templ generate: templ binary not found (skipping check)
narsilc generate: narsilc binary not found (skipping check)
```

`verifyGenerated` (`cli/verify.go:273`) turns `statusWarn` into a `Detail` note and leaves the
step `OK`. An agent reads that as a pass. `cli/` has **no `verify` tests at all** today.

### Decisions (settled)

- Each step in `data.steps[]` gains `status: "ok" | "fail" | "skipped"`. Keep the boolean
  `ok` for compatibility: it is `true` only when `status == "ok"`.
- A missing templ or narsilc binary fails the `generated` step with a violation:
  - `rule`: `missing-tool`
  - `message`: names the binary
  - `fix`: `andurel tool sync`
- `--skip-generated` gives the step `status: "skipped"` with a `reason`. An explicit skip does
  not fail the run.
- Top-level `ok` is true when every step is `ok` or explicitly `skipped`.
- Human output prints `SKIP` for skipped steps and `FAIL` for failed ones.
- The full-scaffold golden (`golden/new_full_test.go:110`) calls
  `goldentest.SeedProjectTools` before `verify`. That run must stay green. If it turns red, the
  seeding is incomplete; fix the seeding, not the check.

### Acceptance

- [x] `verify` without templ or narsilc on `PATH` exits 9 with a `missing-tool` violation whose
      fix is `andurel tool sync`.
- [x] `verify --skip-generated` exits 0, and the step reports `status: "skipped"`.
- [x] New `cli/verify_test.go` covers four cases: ok, fail, missing-tool and
      explicitly-skipped. These tests use a tiny temp project; they do not scaffold.
- [x] `verify --help` and `skills/andurel/references/cli-catalog.md` mention `missing-tool`, and
      `just commands-check` passes.

---

## W3. Upgrade gate for pre-rebuild v2 projects — P0

### Problem

`validateMajorVersion` (`layout/upgrade/orchestrator.go:219`) only refuses major-version
upgrades. An `alpha`–`alpha.3` project (Echo, Fx, `controllers/`, `models/`, `router/`) that
upgrades past `alpha.4` is not stopped. It then gets the `package-api-v2.0.0-alpha.5` manual
action (`upgrade_notes.go:16`, `:64`) about `anduin.NewServer` arguments, an API it has never
used.

RFC §15 says such apps are migrated by hand. The CLI should say so instead of attempting it.

### Decisions (settled)

- Add `const goNativeRebuildVersion = "v2.0.0-alpha.4"` next to `packageAPIVersion`.
- In `(*Upgrader).validatePreconditions` (`orchestrator.go:188`), refuse when
  `from < goNativeRebuildVersion <= to`. Use the same semver comparison as
  `validateMajorVersion`, with prerelease ordering.
- The refusal is a typed error:
  - `code`: `unsupported_upgrade`
  - `hint`: the app must be ported by hand to `domains/`, `services/`, `handlers/` and
    `pkg/anduin`, guided by `andurel patterns show composition|domain|handler`
  - `data`: `{from, to, rebuild_version, legacy_markers: [...]}`
- **Do** detect the legacy shape. `legacy_markers` lists whichever of these exist:
  - the directories `controllers/`, `router/` and `models/`;
  - a `go.uber.org/fx` require in `go.mod`.

  This is informational only. The gate is driven by the version.
- The check runs before any file, git or tool change, so no alpha.5 notes are produced.

### Acceptance

- [x] An `alpha.3 → alpha.5` upgrade fails before any file or tool change, with
      `unsupported_upgrade` and populated `legacy_markers`.
- [x] `alpha.4 → alpha.5` behaves exactly as today.
- [x] Upgrade orchestrator tests cover both cases plus `alpha → alpha.4`. The last
      one is refused, because the boundary is inclusive on `to`.

---

## W4. Layer lints on `go/types` + close known gaps — P1

### Problem

`cli/verify_layers.go` is purely syntactic. In a probe against a fresh scaffold, only the
direct `p.users` use inside an `AtomicOps` callback was caught. Everything else passed
`verify`.

| # | Shape | Today | Expected rule |
|---|-------|-------|---------------|
| 1 | Alias escape: `users := p.users` before `AtomicOps`, `users.Find` inside | passes | `tx-escape` |
| 2 | Method value callback: `storage.AtomicOps(ctx, p.transactor, p.work)` | passes | `tx-escape` |
| 3 | Raw SQL via `*storage.Postgres.Exec` in `services/` | passes | `db-handle` |
| 4 | Raw SQL via `p.pg.Pool().Exec` in `services/` | passes | `db-handle` |
| 5 | Raw SQL via `database/sql` in `services/` | passes | `db-handle` |
| 6 | Free function (no receiver) using an unbound captured value in `AtomicOps` | passes | `tx-escape` |
| 7 | Handler holding a `storage.Transactor` and running `AtomicOps` (skips the service layer) | passes | `tx-layer` |
| 8 | `domains/` importing `pkg/anduin` (HTTP leaking into domains) | passes | `domain-transport` |
| 9 | Control: `p.users.Find` directly inside the callback | **caught** | `tx-escape` (unchanged) |

Root causes:

- `forbiddenStorageIdents` (`cli/verify_layers.go:24`) only lists `Connection` and
  `Transaction`. `*storage.Postgres`, `.Pool()` and `database/sql` are not covered.
- `txCallbackViolations` only inspects methods with a named receiver and inline `FuncLit`
  callbacks, matching on `<recv>.<field>` selectors.
- No rule constrains imports *into* `domains/`, or `AtomicOps` *outside* `services/`/`domains/`.

### Probe code (turn into fixtures)

Written against a fresh scaffold (module `app`):

```go
// services/probe.go
package services

import (
	"context"
	"database/sql"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
	pg         *storage.Postgres
	sqlDB      *sql.DB
}

// Shape 1: alias escape.
func (p Probe) AliasEscape(ctx context.Context) error {
	users := p.users
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

// Shape 2: method value callback.
func (p Probe) MethodValue(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, p.work)
}

func (p Probe) work(ctx context.Context, tx storage.Tx) error {
	_, err := p.users.Find(ctx, [16]byte{})
	return err
}

// Shape 3: *storage.Postgres outside domains.
func (p Probe) RawSQL(ctx context.Context) error {
	_, err := p.pg.Exec(ctx, "delete from users")
	return err
}

// Shape 4: pool escape hatch.
func (p Probe) Pool(ctx context.Context) error {
	_, err := p.pg.Pool().Exec(ctx, "delete from users")
	return err
}

// Shape 5: database/sql outside domains.
func (p Probe) StdlibSQL(ctx context.Context) error {
	_, err := p.sqlDB.ExecContext(ctx, "delete from users")
	return err
}

// Shape 6: free function with a captured unbound dependency.
func PlainFunc(ctx context.Context, t storage.Transactor, users domains.Users) error {
	return storage.AtomicOps(ctx, t, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

// Shape 9 (control): already caught today.
func (p Probe) Control(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := p.users.Find(ctx, [16]byte{})
		return err
	})
}
```

```go
// handlers/probe.go — shape 7: transaction orchestration in a handler.
package handlers

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type ProbeHandler struct {
	transactor storage.Transactor
	users      domains.Users
}

func (h ProbeHandler) Show(ctx context.Context) error {
	return storage.AtomicOps(ctx, h.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := h.users.InTx(tx).Find(ctx, [16]byte{})
		return err
	})
}
```

```go
// domains/probe_http.go — shape 8: transport in the domain layer.
package domains

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

func ProbeHTTP(ctx context.Context) string { return anduin.Query(ctx, "x") }
```

### Decisions (settled)

- **Loader.** Use `golang.org/x/tools/go/packages` with
  `NeedSyntax|NeedTypes|NeedTypesInfo|NeedImports`, run from the project root, so the analysis
  is type-based. Write each rule as a `go/analysis` `Analyzer`, but run the analyzers in-process
  through a small driver. Keep the existing `{rule, file, line, message, fix}` output.
  `go vet -vettool` support is out of scope.
- **Bindable types.** These are identified by `types.Object` identity, not by name:
  - any named type in `app/domains` with an `InTx(storage.Tx) <same type>` method;
  - `storage.Transactor`;
  - `storage.InsertQueue`;
  - `*queries.Queries`.

  A value of a bindable type that is declared outside the `AtomicOps` callback and used inside
  it without going through `.InTx(tx)` is a `tx-escape`. For method values (shape 2), analyse
  the referenced method's body as the callback, one level deep.
- **`db-handle` (widened).** Applies outside `domains/` and `domains/internal/queries/`:
  - any use of `*storage.Postgres` methods that execute SQL or expose a handle (`Exec`,
    `Query*`, `Pool`, `BeginTransaction`; derive the list from the type's method set at
    implementation time, not from memory);
  - imports of `database/sql`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgxpool`
    and `github.com/jackc/pgx/v5/pgconn`.

  `internal/runtime/` may *construct* `*storage.Postgres`, but it must not call SQL methods on
  it.
- **`tx-layer`.** `storage.AtomicOps` and `storage.Transactor` are referenced only in
  `services/`, `domains/` and `internal/runtime/` (construction and wiring). The fix points to
  `andurel patterns show service`.
- **`domain-transport`.** `domains/` imports none of:
  - `pkg/anduin`, `pkg/inertia`, `pkg/kiks`, `pkg/routing`;
  - `net/http`;
  - `app/handlers`, `app/middleware`, `app/routes`.
- **Fixtures.**
  - **Unit tests.** Use `golang.org/x/tools/go/analysis/analysistest` with GOPATH-style
    testdata at `cli/internal/layers/testdata/src/`. That tree holds minimal stub packages for
    `github.com/mbvlabs/andurel/pkg/storage`, `…/pkg/anduin`, `app/domains` and
    `app/domains/internal/queries`, plus one directory per shape using `// want "<rule>"`
    comments.
  - **Real code.** One full-scaffold golden test applies the three probe files above to a
    fresh scaffold and asserts the exact rule set from `verify --json`.
- **Rule catalog.** One Go table (`id`, `summary`, `fix`, `layer`) in the new layers package is
  the single source for `verify --help`, `commands --json`, and later W9 and W10e.
- **Performance budget.** Today a warm `verify --json` on a fresh scaffold takes ~0.55 s wall
  time (measured at `71510391`). Target: ≤ 1.5 s warm.

### Acceptance

- [x] analysistest covers shapes 1–9. Shapes 1–8 report the expected rule from the table, and
      shape 9 still reports `tx-escape`.
- [x] The full-scaffold golden with the probe files reports exactly those rules. A fresh
      scaffold without them passes `verify`.
- [x] Warm `verify` on a fresh scaffold is ≤ 1.5 s. Put the measured number in the commit body.
- [x] `verify --help` lists rules from the catalog, and `just commands-check` passes.

---

## W5. Entity ↔ query column check (`entity-columns`) — P1

### Problem

narsilc maps result columns to entity fields **at runtime** by `andurel:` tag. An entity whose
tags drift from the query compiles, passes `verify`, and shows up in `inspect domains` without
complaint. For example, `Emial string \`andurel:"emial"\`` only fails when the query runs. The
scaffold has no domain tests yet (W7), so nothing catches it before production.

v1 and the alphas had a migration DDL catalog (`generator/internal/ddl` and
`generator/internal/catalog` in `v2.0.0-alpha.3`). It was deleted with the generators.

### How narsilc actually maps (v0.6.2, `scan.go` / `transform.go`)

- Generated query methods are generic. For example
  `func (q *Queries) CreateUser[T any](…) (T, error)` ends in
  `narsilc.Read[T, UserRow](row, []string{"id", "created_at", …})`. The column list is a
  **string literal in the generated method body**.
- `Read` checks whether `T` implements `narsilc.Transformer[Row]`:
  - If yes, it scans into `Row` and calls `FromRow`. The mapping is then user code.
  - If no, it calls `Scan[T]`.
- `Scan`/`buildLayout` behave as follows:
  - **Every returned column must have an exported field tagged `andurel:"<col>"`** on `T`;
    otherwise it returns `no field with andurel:"<col>"`.
  - A duplicate tag is an error.
  - Extra tagged fields with no returned column are silently left at their zero value.

### Decisions (settled)

- **Where to look.** Use the W4 loader. Find every instantiation of a generated generic query
  method from `domains/internal/queries` (via `types.Info.Instances`) where `T` is a struct
  type from `app/domains`. Skip it if `T` implements `narsilc.Transformer[Row]`.
- **Column source.** Read the `[]string{…}` literal from the `narsilc.Read`/`narsilc.Scan` call
  in the instantiated method's generated body.
- **Violations** (`entity-columns`, reported at the entity field or type position):
  - A returned column with no matching tag on `T`. This is a hard runtime failure.
    Fix: add a field tagged `andurel:"<col>"`, or narrow the query's columns and run
    `andurel sync queries`.
  - A duplicate tag on `T`.
- **Notes, not violations.** Report these in `inspect domains --json` only:
  - a tagged field that no query used with `T` ever returns (`unpopulated_tags[]`);
  - type differences against `<X>Row`. pgx legitimately scans across types, for example
    `pgtype.Timestamptz` → `time.Time`, so these are not failures.
- `inspect domains --json` gains `columns_ok` and `mismatches[]` per entity.
- Phase 2 (restoring a DDL catalog from migrations) is **out of scope for this alpha**.
  Delete `docs/ddl-model-generation.md` in this workstream.

### Acceptance

- [x] The `emial` drift (rename `Email`'s tag on `domains.User`) fails `verify` with
      `entity-columns` at the field's file and line.
- [x] An entity implementing `Transformer[Row]` is not checked.
- [x] A fresh scaffold passes, and the rule is in the W4 catalog.
- [x] `docs/ddl-model-generation.md` is removed, and nothing links to it.

---

## W6. `patterns show domain --entity` emits real code — P2

### Problem

RFC §11 says that, given `--entity/--table`, the `domain` pattern emits "table + columns +
types + nullability + constraints + indexes, the colocated `.sql`, the Go entity with
`Validate()` and its receiver". `cli/patterns/manifest.go:31` (`ManifestFor`) emits only an
ordered list of paths with one-line summaries.

### Decisions (settled)

- Source of truth: `<X>Row` in `domains/internal/queries/models.go`. No migration parsing (that
  was W5 phase 2, which is out of scope).
- When `<Singular>Row` exists, render the following file contents in the manifest, still
  read-only (stdout / `--json`):
  - the entity struct with `andurel:` tags, mapping `pgtype.*` to the Go types the scaffold's
    `domains/user.go` uses for the same pgtype;
  - a CRUD `.sql` skeleton with narsilc annotations;
  - the receiver with `New<Plural>` and `InTx`;
  - a `Validate()` stub with a TODO per non-pointer `string` field.
- When it does not exist, keep today's manifest and add a migration skeleton as file content.

### Depends on

Nothing hard. W5 makes the column data reusable; share the code if W5 has landed.

### Acceptance

- [x] `patterns show domain --entity User --table users --json` on a fresh scaffold returns an
      entity struct whose fields, tags and types match `domains/user.go`.
- [x] Golden test for the rendered output, covering both the table-exists and table-missing
      cases.

---

## W7. Scaffold ships one test per layer — P1

### Problem

The RFC calls the scaffold "the executable pattern book", but a fresh app ships only three
tests:

- `middleware/auth_test.go`
- `middleware/middleware_test.go`
- `services/authentication_test.go`

`test-domain`, `test-handler` and `test-integration` have no real file to copy, so
`verify --with-tests` exercises almost nothing.

### Decisions (settled)

- **Scaffold files.** Add templates in `layout/templates` for each of the files below and wire
  them into the layout:

  | File | What it does |
  |------|--------------|
  | `domains/main_test.go` | `TestMain` owns one `storagetest` cluster and skips cleanly without Docker (W1 shape). |
  | `domains/user_test.go` | Covers `Validate`, and `Create` then `Find` over a test DB. |
  | `handlers/main_test.go` | A real `testRenderer(t)` helper. |
  | `handlers/auth_test.go` | Uses `anduintest`: the login page renders `Auth/Login`, and bad credentials re-render with errors. |
  | `internal/runtime/main_test.go` | A real `testConfig(t)` helper. |
  | `internal/runtime/runtime_test.go` | Boots `New` and hits `/health`. |
- **Test config (USER decision).** `testConfig(t)` takes its `storage.Config` from
  `storagetest.Cluster.NewTestDBWithConfig`, which returns the config for the same migrated,
  isolated database as the test connection. W7 keeps the settled whole-package `TestMain` skip
  when Docker is unavailable; do not replace it with per-test skip logic or change that behavior.
- **Host registry: no `pkg/routing` change.** `routing.ConfigureHosts` writes a package-level
  registry, but it is guarded by `hostMu`, so there is no data race. `FullURL` reads that
  registry without a parameter, so making it an instance would ripple through every route.
  Instead, tests that call `runtime.New` use the same hosts config and do not use
  `t.Parallel()`. State this in the `test-integration` pattern rules. An instance registry is
  parked; see Out of scope.
- After this lands, the `test-*` patterns reference these files ("see `domains/user_test.go`")
  instead of defining the helpers inline (W1).

### Depends on

W1 (pattern text) and W8 (tagged and pinned, so scaffold tests import `storagetest`).
W8 Part 1 already provides the `NewTestDBWithConfig` helper W7's runtime test needs; the
remaining W7 blocker is the release and pin.

### Acceptance

- [x] On a fresh scaffold with Docker, `andurel verify --with-tests` runs the domain, handler and
      integration tests, and they pass.
- [x] Without Docker, the same command passes, with the DB-backed tests reported as skipped.
      `verify --with-tests` lists them under the go test step's `skipped_packages`.
- [x] Each `test-*` pattern names its scaffold counterpart.
- [x] Full-scaffold goldens are refreshed for all three UI variants.

---

## W8. Move test containers out of `pkg/storage` — P2

**Progress:** Part 1 moved the container helpers to `pkg/storage/storagetest` and added the
USER-approved test-only `NewTestDBWithConfig` helper, which returns the `storage.Config` for
the same isolated, migrated database. Part 2 is implemented against the tagged
`pkg/storage/v0.11.0`: the scaffold pin is bumped, the pattern, README and skill call sites use
`storagetest`, the upgrade note is covered by `upgrade_notes_test.go`, and the full-scaffold
goldens are refreshed.

### Problem

`TestCluster` and `NewTestCluster` live in `pkg/storage/psql.go`, so every production app links
`testing` plus testcontainers and its Docker client dependencies (readiness review C5, RFC §14).

### Decisions (settled)

- New package `pkg/storage/storagetest` in the same module, mirroring
  `pkg/anduin/anduintest`:
  - `storage.NewTestCluster` becomes `storagetest.NewCluster`;
  - `storage.TestCluster` becomes `storagetest.Cluster`;
  - methods are unchanged.
- No alias, per the hard-cut policy. Add a `### Changed` (breaking) entry to
  `pkg/storage/CHANGELOG.md`.
- **Part 1** (this branch): the `pkg/storage` code and changelog. Stop with
  "needs release: pkg/storage".
- **Part 2** (after the human tags), as a follow-up commit:
  - bump `layout/versions/versions.go` `Storage` and the root `go.mod`;
  - rename the call sites in the patterns from W1;
  - add a manual upgrade note in `layout/upgrade/upgrade_notes.go` for apps that call
    `storage.NewTestCluster`;
  - refresh goldens.

### Acceptance

- [x] `go list -deps ./cmd/app` in a fresh scaffold does not include `testcontainers`.
- [x] `pkg/storage` changelog entry. After part 2: the pin is bumped and the upgrade note is
      tested in `upgrade_notes_test.go`.

---

## W9. One source of truth for agent docs + drift fixes — P2

### Problem

Rules are restated in five places, and they have already drifted:

- the scaffold's `AGENTS.md` (`layout/templates/agents.tmpl`);
- `skills/andurel/SKILL.md`;
- `skills/andurel/references/layer-placement.md`;
- each pattern's `## Rules`;
- `README.md`.

Known drift:

- `skills/andurel/SKILL.md:20` and `:57` say `verify` enforces "the domain/service import
  boundary". It does not, and the skill omits the generated-code and transaction rules.
- `docs/inertia-v3.md:60` still documents `*echo.Context` callbacks.
- `docs/ddl-model-generation.md` documents the deleted model generator. W5 deletes it; if W9
  lands first, delete it here.
- `README.md:46` links `pkg.go.dev/github.com/mbvlabs/andurel/v2/pkg/anduin`, but the module
  path is `github.com/mbvlabs/andurel/pkg/anduin`.

### Proposal

1. Generate the "Verify" section of `AGENTS.md`, `SKILL.md` and `cli-catalog.md` from W4's rule
   catalog. Add a test, like `commands --check`, that fails when the embedded docs disagree with
   the catalog.
2. Keep layer-placement prose in `layer-placement.md` only. `AGENTS.md` and `SKILL.md` link to
   it instead of restating it.
3. **Drift fixes (wave 1, no dependency; completed):** fix the four items above now. For `SKILL.md`, list
   the rules that exist today: `queries-import`, `db-handle`, `tx-unwrap`, `tx-escape`,
   `tx-public-call` and `generated`.

### Depends on

W4 (rule catalog), for items 1–2 only.

### Acceptance

- [x] A test fails if a `verify` rule ID is missing from the skill or `AGENTS.md`, or if the docs
      name a rule that does not exist.
- [x] `grep -rn "echo\.Context\|models/\|controllers/" docs skills layout/templates` finds only
      intentional historical references. List them in the commit body.

---

## W10. Small agent-surface fixes — P2

Each item is its own small branch.

| # | Issue | Evidence | Fix |
|---|-------|----------|-----|
| a | `inspect domains` lists generated `domains/internal/queries/*` files as `kind: "domain"` | `cli/introspection.go:183` lists all of `domains/`; `isDomainEntityFile` only gates entity parsing | Filter `domains/internal/**` from the item list. Wave 1. |
| b | `inspect handlers` is a bare file list | `readOnlyListCommand("handlers", …)` | Report each handler's type, constructor and constructor params, plus the routes registered in `Register` (route name → method → path). Use `go/ast`; no type loading needed. |
| c | Inconsistent handler constructors agents will copy | `handlers.NewPages`, `handlers.NewAssets` vs `NewAuthHandler`, `NewAPIHandler` | Rename to `PagesHandler`/`NewPagesHandler` and `AssetsHandler`/`NewAssetsHandler` (RFC §4.3). Update templates, patterns and goldens. |
| d | `andurel new` breadcrumbs skip the agent loop | `printNewProjectNextSteps` (`cli/new_project.go:199`): `tool sync`, `.env`, `pnpm install`, `migrate up`, `run` | Add `andurel skill install` after `tool sync`, and `andurel verify` last. Update `TestPrintNewProjectNextSteps`. |
| e | `verify` long help lists rules by hand | `cli/verify.go` `Long` | Render from the W4 rule catalog. |

- [x] W10a: `inspect domains` excludes `domains/internal/**`, with regression coverage for retained entity metadata.
- [x] W10b: `inspect handlers` reports each handler's type, constructor, constructor params and the routes its `Register` mounts (route variable, name, method, path, action), parsed with `go/ast` only.
- [x] W10c: scaffold handlers are `PagesHandler`/`NewPagesHandler` and `AssetsHandler`/`NewAssetsHandler`, matching `NewAuthHandler`/`NewAPIHandler`; templates, the composition pattern and goldens updated. No upgrade note: `handlers/` and `internal/runtime/` are app-owned, so upgrades never rewrite them and existing apps keep compiling.
- [x] W10d: `andurel new` next steps now run `andurel skill install` right after `tool sync` and end with `andurel verify`; `TestPrintNewProjectNextSteps` asserts both and their order.
- [x] W10e: `verify` long help renders its rules and fixes from the W4 catalog through the same helper as the human help; the hand-written `missing-tool` paragraph and rule mentions in the agent notes and when-to-use metadata are gone, and a test fails if that prose names a catalog rule again.

---

## Out of scope for this alpha

- Replacing River or adding a queue facade (parked per rev-5 delta §7).
- Auto-migrating alpha.1–alpha.3 apps. W3 only refuses, with guidance.
- Bringing back code-writing generators. W6 stays read-only.
- W5 phase 2: a DDL catalog from migrations.
- An instance-based `routing` host registry (see W7).
- Running the layer analyzers through `go vet -vettool`.

---
