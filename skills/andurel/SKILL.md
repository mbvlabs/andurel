---
name: andurel
description: Use this skill for Andurel framework projects when deciding where code belongs, adding or changing resources, controllers, models, services, routes, templ views, Inertia screens, background jobs, migrations, config, clients, or framework-adjacent internals. Focuses on project structure, layer placement, command discovery, generator workflows, and agent-safe CLI usage.
---

# Andurel

Use this skill when working in an Andurel project or generating Andurel code. It helps place code in the right layer and use the `andurel` CLI safely.

## Agent Invariants

- Prefer `andurel --agent --help`, `andurel commands --json`, and `andurel commands --check` for discovery.
- Read [references/cli-catalog.md](references/cli-catalog.md) for group layout and workflows.
- Run `andurel inspect project --json` before generation.
- Use `--json` or `--jq` when extracting data.
- Use `--dry-run --json` before mutating commands when intent is uncertain.
- Inspect returned artifact arrays before assuming which files changed.
- Treat `andurel inspect project --json` as the source of truth for the configured Inertia adapter and JavaScript package manager.
- Persist through narsilc-generated queries. Keep generated `models/internal/queries` types behind the owning model package.
- After adding or changing Inertia routes, run `andurel sync routes --json` so frontend pages can import `resources/js/routes.ts`.
- After adding or changing Inertia controller payload or Bind structs, run `andurel sync payloads --json` so frontend pages can import `resources/js/types/payloads.ts`.
- Follow the repository rules for verification.
- Prefer the local project pattern over a generic Rails, Echo, Bun, Templ, or frontend framework convention.
- Keep controllers as HTTP adapters: parse input, call models or services, map errors, and render a response.
- Create a service only when there is real application orchestration, not just because code exists.

## Read When Placing Code

Read [references/layer-placement.md](references/layer-placement.md) before adding or moving behavior across models, services, controllers, routes, views, queue jobs, config, clients, or internal packages.

## First Pass

1. Inspect the existing resource closest to the requested change.
2. Identify the delivery surface: public hypermedia page, admin Inertia page, API endpoint, background job, email, or CLI/command.
3. Identify the domain object or workflow being changed.
4. Keep changes in the smallest layer that can own the behavior honestly.
5. Use the CLI discovery commands before generating or mutating project files.

## Layer Placement

- Put invariant business rules, domain validation, entity construction, persistence methods, and finder/query methods in `models/`.
- Put test factory definitions and factory helpers in `models/factories/`.
- Put transactions, cross-model coordination, external side effects, and multi-step application workflows in `services/`.
- Put HTTP-specific concerns in `controllers/`, `controllers/admin/`, or `controllers/api/`.
- Put route names, route paths, and URL builders in `router/routes/`.
- Put templ rendering helpers and presentation-specific adapters in `views/`.
- Put admin Inertia pages and reusable frontend components in `resources/js/`.
- Put River job argument types in `queue/jobs/` and worker implementations or registration in `queue/`.
- Put provider adapters in `clients/`, email templates/helpers in `email/`, and config/environment loading in `config/`.
- Put reusable framework-like support that is independent of one resource in `internal/`.
- Register new constructors in the existing `fx` modules for the package that owns them.

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
andurel inspect models --json
andurel inspect migrations --json
andurel commands --json
```

Preview scaffold generation:

```bash
andurel generate scaffold Product --dry-run --json
```

Generate and review artifacts:

```bash
andurel generate scaffold Product --json
```

Generate Inertia route helpers:

```bash
andurel inspect routes --json
andurel sync routes --json
```

`andurel sync routes` reads `router/routes/*.go` as the source of truth and writes `resources/js/routes.ts`. It only runs when `andurel.lock` has `scaffoldConfig.inertia` set to `vue`, `react`, or `svelte`. Import helpers from that file in Inertia pages instead of hard-coding URLs.

Generate Inertia payload types:

```bash
andurel sync payloads --json
```

`andurel sync payloads` scans controller `inertia.FromStruct` and named Bind structs and writes `resources/js/types/payloads.ts`. Import page and form types from that file instead of hand-written or catalog-derived TypeScript. Run it after editing those Go structs. `generate controller --inertia` and `generate scaffold --inertia` refresh the same file.

Generate an Inertia resource:

```bash
andurel inspect project --jq .scaffold_config.inertia
andurel generate scaffold Product --inertia --dry-run --json
andurel generate scaffold Product --inertia --json
```

Controller and scaffold generation defaults to Templ even in an Inertia project. Pass `--inertia` explicitly when the generated views should be frontend pages. Do not combine `--inertia` with `--api`; choose either Inertia pages or JSON responses.

Add a narsilc query:

```bash
andurel generate query UserReport --table users --dry-run --json
andurel generate query UserReport --table users --json
# Edit models/queries/user_report.sql, then generate typed code.
andurel sync queries --json
```

Keep hand-written SQL in `models/queries/` and generated code in `models/internal/queries/`. Only the owning `models` package should import that internal package. Construct clients with `queries.New(db)` where `db` is `storage.Connection`, and inside shared transactions use `queries.New(tx)` where `tx` is `storage.Transaction`.

Check or sync factories:

```bash
andurel sync factory Product --check --json
andurel sync factory Product --sync --json
andurel sync factories --check --json
andurel sync factories --sync --json
```

Factory guidance:

1. Treat model `Entity` structs as the source of truth for generated factory fields.
2. Keep reusable test data builders in `models/factories/`.
3. Prefer `andurel sync factory NAME --check --json` before editing factory files by hand.
4. Use `--sync` to update Andurel generated regions and preserve custom helpers outside those regions.
5. Pass `--skip-factory` only when a generated model or scaffold should intentionally omit a factory.

Generate a named database seed:

1. Inspect the relevant models and existing factories in `models/factories`.
2. Add a seed function to `seeds/`, using only exported model/factory/storage primitives.
3. Register it in `seeds.Registry` with a stable lowercase name.
4. Keep the seed idempotence expectations explicit in code comments when it may be re-run.
5. Verify the seed is discoverable:

```bash
andurel db seed --list
andurel db seed development
andurel db seed test
```

Check project health:

```bash
andurel doctor --json
```

In Inertia projects, `doctor` checks whether `resources/js/routes.ts` matches the current `router/routes/*.go` manifest and whether `resources/js/types/payloads.ts` matches controller payload structs. If the `routes.ts` check fails, run `andurel sync routes --json`. If the `payloads.ts` check fails, run `andurel sync payloads --json`. Doctor does not delete leftover per-resource files under `resources/js/types/`.

When annotated narsilc queries exist, `doctor` also checks generated code for drift. If the `narsilc generate` check fails, run `andurel sync queries --json`.

Update standalone Andurel packages in `go.mod` (`github.com/mbvlabs/andurel/pkg/*`):

```bash
andurel packages --json
andurel packages update --dry-run --json
andurel packages update --json
```

This updates required package versions to the latest published releases. It does not change framework-owned files. `andurel upgrade` pins required packages to the versions verified with the installed CLI.

## Validation

Follow the target repository's `AGENTS.md` and local validation guidance. Do not assume that a command is permitted merely because it is common in another Andurel project.
