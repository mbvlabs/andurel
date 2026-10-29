# Andurel CLI catalog

This file is the agent-oriented command playbook. The live source of truth is
the command metadata on the `andurel` binary.

Discover commands:

```bash
andurel commands --json
andurel commands --markdown
andurel commands --check
andurel --agent --help
```

## Workflow

1. `andurel inspect project --json` — andurel.toml manifest (version, adapter), andurel.lock digests, tools.
2. `andurel patterns show <kind>` — read the reference shape for a layer.
   `andurel skill show <reference>` prints the skill's `http-and-routing`,
   `layer-placement`, or `cli-catalog` reference without installing it.
3. `andurel generate migration NAME` — create a migration (the only generated file).
4. Write domains, services, handlers, routes, and middleware by hand.
5. `andurel sync queries` / `sync routes` / `sync payloads` / `sync email` — refresh derived files.
6. `andurel verify [packages...]` — formatting, `go fix`, `go vet`, layer boundaries, the frontend typecheck, and generated-code drift.
7. `andurel doctor --json` — health and generated-file drift.

`andurel verify` reports each step as `ok`, `fail`, or `skipped`. A missing
`templ`, `tailwindcli`, or `narsilc` binary fails the `generated` step with the
`missing-tool` rule; run `andurel tool sync` to install it. `--skip-generated`
marks that step `skipped` and does not fail the run.

The `frontend` step runs the project package manager's `typecheck` script when
`package.json` defines one; it fails with an install hint when `node_modules`
is missing and is `skipped` for projects without the script. `--no-frontend`
skips it explicitly.

Pass Go package patterns to scope `gofmt`, `go fix`, `go vet`, and `layers` to
those packages (`andurel verify ./internal/docs/...`). The `generated` and
`frontend` steps always check the whole project; each generated-drift finding
names the authored source file that owns it.

`andurel verify --with-tests` also runs `go test ./...`. A test package that has
tests but ran none (the scaffold's DB-backed packages exit their `TestMain`
early when Docker is unavailable) keeps the `go test` step `ok` and the run
passing, but is listed under the step's `skipped_packages` as
`{package, reason}`; the human output prints `ok (N packages skipped: …)`.
Tests that call `t.Skip` are listed under the step's `skipped_tests` as
`{package, test, reason}`; they keep the step `ok` too, and the human output
prints `N tests skipped` followed by each `package: TestName (reason)`.

`andurel sync payloads` writes `resources/js/types/payloads.ts` from handler
`inertia.FromStruct` page props and `anduin.DecodeJSON`/`DecodeForm` payloads.
It resolves named composite literals, typed variables, function parameters,
and closure parameters. Pass `--strict` to exit non-zero when a `FromStruct`
argument cannot be resolved statically; it still writes `payloads.ts` before
returning the failure, and the error lists every `file:line` while structured
output keeps the same report as error data. Without `--strict` the
human output lists each unresolved root with the hint "pass a named composite
literal or a typed variable". `andurel verify` reports unresolved roots under
the generated step's `warnings` and does not fail on them.

The typed layer checks load the active build target, like `go vet`: files
excluded by GOOS, GOARCH, or build constraints are not analyzed by that step.

`andurel sync` refreshes derived app files. `andurel tool sync` downloads pinned
binaries. They are different commands.

Tool versions live in `andurel.toml`; `andurel.lock` stores only per-platform
SHA-256 digests for those downloads.

## Verify rules

Each `andurel verify` violation's `rule` is one of these (`andurel verify --help`
also prints each rule's fix):

<!-- andurel:verify-rules:begin: generated from the verify rule catalog by 'just docs-sync'; do not edit -->
| Rule | Checks |
| --- | --- |
| `gofmt` | every Go file is gofmt-clean |
| `go fix` | go fix reports no automatic rewrites |
| `queries-import` | only domains/** imports domains/internal/queries |
| `db-handle` | outside domains no SQL methods or SQL drivers; Connection/Transaction are persistence wiring |
| `tx-unwrap` | storage.Unwrap is referenced only under domains/** |
| `tx-escape` | a storage.AtomicOps callback uses only dependencies bound inside the transaction |
| `tx-public-call` | a storage.AtomicOps callback does not call an exported method on its own receiver |
| `tx-layer` | storage.AtomicOps and storage.Transactor are used only in services, domains, and internal/runtime |
| `domain-transport` | domains/** imports no HTTP, Inertia, routing, handler, middleware, or route package |
| `entity-columns` | every automatic generated query column maps to an exported andurel tag on its domain entity |
| `frontend` | the package.json typecheck script passes when the scaffold defines one |
| `generated` | generated templ, email, queries, routes, and payload output is up to date |
| `missing-tool` | a required generation binary is installed; run 'andurel tool sync' |
<!-- andurel:verify-rules:end -->

## Patterns

`andurel patterns` never writes files. It prints reference code, the placement
rules, and a "definition of done" checklist you self-verify against.

Pattern ids: `domain`, `service`, `handler`, `route`, `middleware`, `page`,
`job`, `email`, `migration`, `test-domain`, `test-handler`,
`test-integration`, `test-middleware`, `composition`, `deploy`.

## Groups

| Group | Use |
| --- | --- |
| `generate` | Create files with no derived source (SQL migrations) |
| `sync` | Refresh derived files (`queries`, `routes`, `payloads`, `email`) |
| `inspect` | Read-only project shape |
| `patterns` | Read-only reference patterns |
| `db` | Database create, migrate, seed, console |
| `tool` | Pin and download binaries |
| `packages` | Standalone `andurel/pkg` versions |
| `skill` | Install the Andurel agent skill |

## Common commands

```bash
andurel patterns list --json
andurel patterns show domain
andurel patterns show handler --json
andurel patterns show domain --entity User --table users --json
andurel skill show layer-placement
andurel generate migration create_posts
andurel db migrate up
andurel inspect project --json
andurel inspect config --json
andurel inspect pages --json
andurel inspect process --json
andurel inspect urls --json
andurel inspect domains --json
andurel inspect handlers --json
andurel inspect routes --json
andurel sync queries --json
andurel sync routes --json
andurel sync payloads --json
andurel build --only app,queue --json
andurel patterns show deploy
andurel verify
andurel doctor --json
andurel run
andurel run --tools mailpit
```

## Create vs refresh vs inspect

- Read a pattern: `andurel patterns show <kind>` (add `--json` for the parsed form; add `--entity/--table` with `domain` for the per-entity file manifest; once `queries.<Entity>Row` exists in `domains/internal/queries/models.go` it includes the entity struct, receiver, `Validate()` stub and CRUD `.sql` as file contents, otherwise a migration skeleton).
- Create SQL: `andurel generate migration NAME` writes `migrations/<timestamp>_<name>.sql` without needing database configuration; then apply it with `andurel db migrate up` (which reads `DB_*`). A one-shot process that applies embedded SQL is `cmd/migrate`.
- Add a domain: hand-write `domains/<entity>.go` and its colocated `domains/<entity>.sql`, construct the receiver with `domains.NewX(db)` in `internal/runtime`, then run `andurel sync queries`.
- Add a service, handler, route, middleware, or composition: read the matching pattern and write the files yourself. `andurel new` ships a complete reference app for each.
- Compile SQL: `andurel sync queries` writes `domains/internal/queries`. Only `domains/**` may import it.
- List domains: `andurel inspect domains --json`.
- List handlers with their constructor params and the routes each `Register` mounts (route name, method, path): `andurel inspect handlers --json`.
- List every env key read by `config/*.go`, with its default and source line: `andurel inspect config --json`.
- List frontend page components, their files, route associations, and statically known SSR policy: `andurel inspect pages --json`.
- List runnable `cmd/*` processes with responsibilities, config needs, startup order, and health endpoints: `andurel inspect process --json`.
- Resolve route hosts to origins and full URLs with configuration provenance; unresolved hosts stay explicit: `andurel inspect urls --json`.
- List routes: `andurel inspect routes --json` (every route reports `host`; unbound routes report `primary`).
- Write `resources/js/routes.ts`: `andurel sync routes --json` (Inertia frontend required; helpers cover routes marked `routing.TypeScriptRoute()` and are host-aware full URLs).
- Write `resources/js/types/payloads.ts`: `andurel sync payloads --json`; use `andurel sync payloads --strict` in CI so an unresolved `FromStruct` argument fails with its `file:line`.
- Bind a resource to a named host: declare `routing.HostName` in `config/hosts.go`, load `HOST_*` into `App.Hosts`, then bind routes with `rt.Host(name)`.
- Refresh derived artifacts: `andurel sync email` compiles `email/*.templ` into `email/*_templ.go` with Tailwind styles inlined; never run `templ generate` on email.
- Build and deploy: `andurel build` writes every buildable `cmd/*` process to `dist/`; `--only` narrows the set and `--json` emits the artifact manifest. `andurel patterns show deploy` covers the `migrate` → `ssr` → `app` + `queue` start order, environment, and health endpoints.

`--json` and `--agent` never wait on a TTY. Pass `--force`, `--harness`, or `--yes` instead of answering a prompt.

## Command table

| Route | Summary |
| --- | --- |
| `andurel` | Andurel CLI for humans and agents |
| `andurel build` | Build every application process for production |
| `andurel commands` | Show the command catalog |
| `andurel console` | Open an interactive database console |
| `andurel db` | Database lifecycle and migrations |
| `andurel db console` | Open an interactive database console |
| `andurel db create` | Create the configured database |
| `andurel db drop` | Drop the configured database |
| `andurel db migrate` | Apply, roll back, and inspect SQL migrations |
| `andurel db migrate down` | Roll back the most recent migration |
| `andurel db migrate down-to` | Roll back migrations down to a version |
| `andurel db migrate fix` | Re-number migrations to fix gaps |
| `andurel db migrate reset` | Roll back all migrations and re-apply them |
| `andurel db migrate status` | Show migration status |
| `andurel db migrate up` | Apply pending SQL migrations (dev CLI; production one-shot is `cmd/migrate`) |
| `andurel db migrate up-to` | Apply migrations up to a version |
| `andurel db nuke` | Drop and recreate the configured database |
| `andurel db rebuild` | Drop, recreate, migrate, and seed the database |
| `andurel db seed` | Run database seeds |
| `andurel doctor` | Run diagnostic checks on the project |
| `andurel fmt` | Format Go and Templ source files |
| `andurel generate` | Create files that have no derived source |
| `andurel generate migration` | Create a new SQL migration file |
| `andurel inspect` | Read-only project shape |
| `andurel inspect config` | List config env keys and defaults |
| `andurel inspect domains` | List domain files |
| `andurel inspect handlers` | List handlers with their constructors and routes |
| `andurel inspect jobs` | List queue job args and workers |
| `andurel inspect migrations` | List migration files |
| `andurel inspect pages` | List frontend page components and route associations |
| `andurel inspect process` | List runnable cmd processes, responsibilities, and health endpoints |
| `andurel inspect project` | Inspect project metadata |
| `andurel inspect project info` | Show project metadata |
| `andurel inspect routes` | List the route manifest |
| `andurel inspect upgrade-notes` | Show the manual actions of a past upgrade |
| `andurel inspect urls` | List routes with resolved host origins and full URLs |
| `andurel new` | Stand up a new Andurel project |
| `andurel packages` | List or update Andurel packages in go.mod |
| `andurel packages list` | List Andurel package versions in go.mod |
| `andurel packages update` | Update Andurel packages in go.mod to latest |
| `andurel patterns` | Read the reference patterns for every layer |
| `andurel patterns list` | List the available pattern ids |
| `andurel patterns show` | Print one reference pattern |
| `andurel run` | Start the development server (`--tools mailpit` for sidecars) |
| `andurel skill` | Show or install the embedded Andurel skill |
| `andurel skill install` | Install the embedded skill into an agent harness |
| `andurel skill show` | Print the embedded Andurel skill or one of its references |
| `andurel sync` | Refresh derived files from a source of truth |
| `andurel sync email` | Compile email templates into inline-styled Go |
| `andurel sync payloads` | Write TypeScript payload types for Inertia |
| `andurel sync queries` | Compile narsilc SQL into Go in `domains/internal/queries` |
| `andurel sync routes` | Write TypeScript route helpers for Inertia |
| `andurel tool` | Manage project tools and binaries |
| `andurel tool dblab` | Open the dblab database UI |
| `andurel tool list` | List project tool status |
| `andurel tool mailpit` | Start Mailpit for local email capture |
| `andurel tool set-version` | Pin a tool version in andurel.toml and sync it |
| `andurel tool sync` | Download tool versions pinned in andurel.toml |
| `andurel upgrade` | Upgrade the project to this CLI version |
| `andurel verify` | Check formatting, vet, layer boundaries, frontend, and generated code |
