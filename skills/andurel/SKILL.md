---
name: andurel
description: Use this skill for Andurel framework projects when deciding where code belongs, adding or changing domains, services, handlers, routes, app middleware, background jobs, migrations, config, clients, or framework-adjacent internals. Focuses on project structure, layer placement, the read-only pattern library, and agent-safe CLI usage.
---

# Andurel

Use this skill when working in an Andurel project or adding Andurel code. It helps place code in the right layer and use the `andurel` CLI safely.

Andurel is Go-native: `net/http` through `pkg/anduin`, explicit constructor injection (no DI container), and Inertia for the UI. There is no Echo, no Fx, and no templ page layer.

## Agent Invariants

- Prefer `andurel --agent --help`, `andurel commands --json`, and `andurel commands --check` for discovery.
- Read [references/cli-catalog.md](references/cli-catalog.md) for group layout and workflows.
- Read [references/http-and-routing.md](references/http-and-routing.md) for the `pkg/anduin` HTTP model.
- `andurel skill show <reference>` prints `cli-catalog`, `http-and-routing`, or `layer-placement` without installing the skill; the same files ship in `<harness skills dir>/andurel/references/` after `andurel skill install`.
- Run `andurel inspect project --json` before changing code.
- Use `--json` or `--jq` when extracting data.
- `andurel generate` does not write application code. Read a pattern with `andurel patterns show <kind>` and write the files yourself. Only `andurel generate migration` creates a file.
- Run `andurel verify` after a change: it checks formatting, `go fix`, `go vet`, the typed layer rules, the `package.json` typecheck script, and generated-code drift. The rules are listed under [Verify](#verify). Pass Go package patterns to scope the Go checks, and add `--with-tests` to also run the test suite.
- Prefer the local project pattern over a generic Rails, Echo, Bun, Templ, or frontend convention.
- Keep handlers as thin HTTP adapters: parse input, call a domain or service, map errors, render a response.
- Create a service only when there is real cross-domain orchestration, not just because code exists.

## Read When Placing Code

Read [references/layer-placement.md](references/layer-placement.md) before adding or moving behavior across domains, services, handlers, routes, middleware, queue jobs, config, clients, or internal packages. It is the one place the placement table, the hard rules per layer, and the decision checklist are written down.

## First Pass

1. Inspect the existing resource closest to the requested change.
2. Identify the delivery surface: Inertia page, API endpoint, background job, email, or CLI/command.
3. Identify the domain object or workflow being changed.
4. Keep changes in the smallest layer that can own the behavior honestly.
5. Use `andurel patterns show <kind>` and `andurel inspect … --json` before writing.

## Verify

`andurel verify` runs `gofmt`, `go fix`, `go vet`, the typed layer rules, the package manager's `typecheck` script when `package.json` defines one, and generated-code drift. It reports each violation as `{rule, file, line, message, fix}`. Apply the `fix` and re-run. `andurel verify [packages...]` scopes the Go checks to package patterns (for example `./internal/docs/...`); generated drift and the frontend typecheck stay whole-project. `--no-frontend` skips the typecheck. With `--with-tests`, packages that ran no tests are listed under `skipped_packages` and `t.Skip` calls under `skipped_tests` with their reason. Unresolved `inertia.FromStruct` payload roots are listed in the generated step's `warnings` and do not fail the run. Each violation's `rule` is one of these (`andurel verify --help` also prints each rule's fix):

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
| `setup` | project setup is complete: managed tool binaries and frontend dependencies are installed |
| `frontend` | the package.json typecheck script passes when the scaffold defines one |
| `frontend-route-usage` | frontend modules do not call full-URL route helpers at module initialization; use .path() or call while rendering |
| `content-config` | andurel.content.toml parses, declares a supported schemaVersion, and its include/exclude globs are valid |
| `content-catalog` | every declared content catalog entry resolves to an included Markdown file |
| `content-slug` | content catalog slugs are non-empty, match the declared pattern, and are unique |
| `content-required` | every path listed in andurel.content.toml required exists |
| `content-link` | local Markdown links and images resolve inside the project; configured anchors exist |
| `content-checker` | each declared content checker exits 0 within its timeout from its declared working directory |
| `generated` | generated templ, email, queries, routes, and payload output is up to date |
| `missing-tool` | a required generation binary is installed; run 'andurel tool sync' |
<!-- andurel:verify-rules:end -->

## Output Modes

Use structured output by default when automating:

| Flag | Use |
|------|-----|
| `--json` | Full `{ok,data,summary,breadcrumbs}` envelope |
| `--agent` | Structured output with non-essential human progress suppressed |
| `--jq '.field.path'` | Built-in simple field-path extraction |
| `--quiet` | Suppress human-only output |
| `--md` | Markdown output where supported |

Structured failures include `ok:false`, a stable `code`, `error`, optional `hint`, and `exit_code`. Prefer the `hint` and `breadcrumbs` fields over guessing the next command.

## Common Workflows

Inspect a project:

```bash
andurel inspect project --json
andurel inspect config --json
andurel inspect routes --json
andurel inspect handlers --json
andurel inspect domains --json
andurel inspect migrations --json
andurel commands --json
```

Read a pattern before writing code:

```bash
andurel patterns list --json
andurel patterns show domain
andurel patterns show handler --json
andurel patterns show middleware
andurel patterns show page
```

Add a domain (type-driven: change the Go type first, then let SQL and queries follow):

1. `andurel patterns show domain` — read the placement rules and checklist.
2. Write `domains/<entity>.go` (entity + receiver + rules) and `domains/<entity>.sql`.
3. Run `andurel sync queries --json` to regenerate `domains/internal/queries`.
4. Write the migration with `andurel generate migration <name>`.

Keep hand-written SQL in `domains/<entity>.sql` and generated code in `domains/internal/queries/`. Only `domains` should import that internal package. Each receiver has `New<Plural>(db)` (constructed in `internal/runtime`) and `InTx(tx storage.Tx)`. A receiver whose own method writes more than once, or reads, modifies and writes a row, holds a `storage.Transactor`; that method locks the row before saving and joins a caller's transaction when bound. Services take a `storage.Transactor`, their receivers, and the queue individually, and run multi-domain writes in `storage.AtomicOps`, binding receivers with `InTx(tx)` at the top of the callback.

Add a background job:

1. `andurel patterns show job` — read the args, worker, and registration rules.
2. Put the args in `queue/jobs/` and the worker in `queue/`; call domain or service code, not SQL.
3. Register the worker in `queue/workers.go`. For recurring work, return a `river.NewPeriodicJob(...)` from `queue.PeriodicJobs()`; `runtime.NewQueue` passes it to River.

Add an HTTP resource:

1. `andurel patterns show handler` and `andurel patterns show route`.
2. Add route data to `routes/`.
3. Write the handler in `handlers/` using `pkg/anduin`.
4. Wire the handler in `internal/runtime`.
5. Run `andurel sync routes --json` so Inertia pages can import `resources/js/routes.ts`.

Generate Inertia route helpers:

```bash
andurel inspect routes --json
andurel sync routes --json
```

`andurel sync routes` reads `routes/*.go` as the source of truth and writes `resources/js/routes.ts` for routes marked `routing.TypeScriptRoute()` (`routing.InertiaRoute()` is a deprecated alias). Import helpers from that file in Inertia pages instead of hard-coding URLs. Helpers are flat and keyed by the route variable in lower camelCase (`UserIndex` → `routes.userIndex`), not by the route's declared name. Helpers include the route host and return absolute URLs once `configureRouteHosts` has run; call `routes.name.path()` when a relative path is required.

Call helpers at render time (`href={routes.widgetShow(id)}`), not in a module-scope constant such as `const url = routes.widgetShow(id)`: `configureRouteHosts` runs after the app boots, so a constant captured at import time keeps an empty origin (or the server's request-time value under SSR).

Generate Inertia payload types:

```bash
andurel sync payloads --json
andurel sync payloads --strict
```

`andurel sync payloads` scans handler `inertia.FromStruct` props (page props) and the named structs passed to `anduin.DecodeJSON`/`DecodeForm` (request payloads) and writes `resources/js/types/payloads.ts`. It follows named struct fields into other packages in the module, so a handler prop may reference a type such as `docs.PageProps`. It resolves named composite literals, typed variables, and function or closure parameters, including pointer parameters. Request payloads must be named with a `Payload` suffix; page props keep their struct name. `domains/` types are rejected, so define a handler-owned payload instead. Calls that pass a literal `inertia.Props{...}` map are skipped and need hand-written TypeScript, as is any `FromStruct` argument the scanner cannot trace. `--strict` still writes `payloads.ts` and then exits 5 listing every unresolved `file:line`; without it, the human output prints the same roots with the hint to pass a named composite literal or a typed variable. Import the generated page and form types from that file instead of hand-written TypeScript, and run it after editing those Go structs.

Generate a named database seed:

1. Inspect the relevant domains and existing seeds in `seeds/`.
2. Add a seed function to `seeds/`, using only exported domain/storage primitives.
3. Register it in `seeds.Registry` with a stable lowercase name.
4. Verify the seed is discoverable:

```bash
andurel db seed --list
andurel db seed development
andurel db seed test
```

Check project health and verify a change:

```bash
andurel doctor --json
andurel verify
```

In Inertia projects, `doctor` checks whether `resources/js/routes.ts` matches the current `routes/*.go` manifest and whether `resources/js/types/payloads.ts` matches handler payload structs. If the `routes.ts` check fails, run `andurel sync routes --json`. If the `payloads.ts` check fails, run `andurel sync payloads --json`.

When annotated narsilc queries exist, `doctor` also checks generated code for drift. If the `narsilc generate` check fails, run `andurel sync queries --json`.

Update standalone Andurel packages in `go.mod` (`github.com/mbvlabs/andurel/pkg/*`):

```bash
andurel packages list --json
andurel packages update --dry-run --json
andurel packages update --json
```

This updates required package versions to the latest published releases. It does not change framework-owned files. `andurel upgrade` pins required packages to the versions verified with the installed CLI.

## Validation

Follow the target repository's `AGENTS.md` and local validation guidance. Run `andurel verify` for the framework checks. Do not assume that a command is permitted merely because it is common in another Andurel project.
