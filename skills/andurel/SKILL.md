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
- Run `andurel inspect project --json` before changing code.
- Use `--json` or `--jq` when extracting data.
- `andurel generate` does not write application code. Read a pattern with `andurel patterns show <kind>` and write the files yourself. Only `andurel generate migration` creates a file.
- Run `andurel verify` after a change: it checks formatting, `go fix`, `go vet`, the typed persistence and transaction boundary rules (`queries-import`, `db-handle`, `tx-unwrap`, `tx-escape`, `tx-public-call`, `tx-layer`, `domain-transport`), and generated-code drift (`generated`; a missing templ or narsilc binary reports `missing-tool` — run `andurel tool sync`). Add `--with-tests` to also run the test suite.
- Prefer the local project pattern over a generic Rails, Echo, Bun, Templ, or frontend convention.
- Keep handlers as thin HTTP adapters: parse input, call a domain or service, map errors, render a response.
- Create a service only when there is real cross-domain orchestration, not just because code exists.

## Read When Placing Code

Read [references/layer-placement.md](references/layer-placement.md) before adding or moving behavior across domains, services, handlers, routes, middleware, queue jobs, config, clients, or internal packages.

## First Pass

1. Inspect the existing resource closest to the requested change.
2. Identify the delivery surface: Inertia page, API endpoint, background job, email, or CLI/command.
3. Identify the domain object or workflow being changed.
4. Keep changes in the smallest layer that can own the behavior honestly.
5. Use `andurel patterns show <kind>` and `andurel inspect … --json` before writing.

## Layer Placement

- Put invariant business rules, input validation (`Validate()` with `pkg/validation`), persistence methods, and finder/query methods in `domains/` (one file per domain).
- Colocate each domain's SQL at `domains/<entity>.sql`; narsilc output goes to `domains/internal/queries`.
- Put transactions, cross-domain coordination, external side effects, and multi-step application workflows in `services/`.
- Put HTTP adapters in the single flat `handlers/` package (`AuthHandler`, `APIHandler`, `NewXHandler` constructors, no sub-packages).
- Put route names, paths, and URL builders in `routes/`.
- Put app HTTP policy (auth, rate limiting) in `middleware/`; the session definition lives there too, but jar construction lives in `internal/runtime`.
- Put Inertia pages and reusable frontend components in `resources/js/`; the Inertia root document is `resources/root.templ`.
- Put River job argument types in `queue/jobs/` and workers/registration in `queue/`.
- Put provider adapters in a package under `internal/` named for its job (for example `internal/clients/` or `internal/payment/`), email templates/helpers in `email/`, and config/environment loading in `config/`. Adapters wrap external APIs only; business rules stay in `domains/` or `services/`.
- Put composition (constructors and the dependency graph) in `internal/runtime`. `cmd/app` and `cmd/queue` only load env and call `runtime.New` / the queue runtime.
- `cmd/migrate` applies pending SQL with `storage.RunMigrations` on the embedded migration FS. It is a one-shot process. Development uses `andurel db migrate up`.
- Wire new dependencies explicitly in `internal/runtime`, and add them to the `App` `closers` list when they need shutdown.

Hard rules:

- One file per domain; keep cross-domain rules in services. If a rule spans two entities, a service coordinates them.
- Only `domains` imports `domains/internal/queries` and issues SQL.
- Handlers are `func(ctx context.Context) error` and use `pkg/anduin` request/response helpers.
- `andurel verify` enforces the persistence and transaction boundary through `queries-import`, `db-handle`, `tx-unwrap`, `tx-escape`, `tx-public-call`, `tx-layer`, and `domain-transport`, and checks generated-code drift with `generated` (`missing-tool` when a generation tool is absent; run `andurel tool sync`).

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
```

Add a domain (type-driven: change the Go type first, then let SQL and queries follow):

1. `andurel patterns show domain` — read the placement rules and checklist.
2. Write `domains/<entity>.go` (entity + receiver + rules) and `domains/<entity>.sql`.
3. Run `andurel sync queries --json` to regenerate `domains/internal/queries`.
4. Write the migration with `andurel generate migration <name>`.

Keep hand-written SQL in `domains/<entity>.sql` and generated code in `domains/internal/queries/`. Only `domains` should import that internal package. Each receiver has `New<Plural>(db)` (constructed in `internal/runtime`) and `InTx(tx storage.Tx)`. Services take a `storage.Transactor`, their receivers, and the queue individually, and run multi-domain writes in `storage.AtomicOps`, binding receivers with `InTx(tx)` at the top of the callback.

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

`andurel sync routes` reads `routes/*.go` as the source of truth and writes `resources/js/routes.ts`. Import helpers from that file in Inertia pages instead of hard-coding URLs. Helpers include the route host and return absolute URLs once `configureRouteHosts` has run; call `routes.name.path()` when a relative path is required.

Generate Inertia payload types:

```bash
andurel sync payloads --json
```

`andurel sync payloads` scans handler `inertia.FromStruct` props and the named `*Payload` structs passed to `anduin.DecodeJSON`/`DecodeForm` and writes `resources/js/types/payloads.ts`. Import page and form types from that file instead of hand-written TypeScript. Run it after editing those Go structs.

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
