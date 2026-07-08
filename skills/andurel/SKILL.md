---
name: andurel
description: Use this skill for Andurel framework projects when deciding where code belongs, adding or changing resources, controllers, models, services, routes, templ views, Inertia/Vue screens, background jobs, migrations, config, clients, or framework-adjacent internals. Focuses on project structure, layer placement, command discovery, generator workflows, and agent-safe CLI usage.
---

# Andurel

Use this skill when working in an Andurel project or generating Andurel code. It helps place code in the right layer and use the `andurel` CLI safely.

## Agent Invariants

- Prefer `andurel --agent --help` and `andurel commands --json` for discovery.
- Run `andurel project info --json` before generation.
- Use `--json` or `--jq` when extracting data.
- Use `--dry-run --json` before mutating commands when intent is uncertain.
- Inspect returned artifact arrays before assuming which files changed.
- After adding or changing Inertia routes, run `andurel generate routes --json` so frontend pages can import `resources/js/routes.ts`.
- Follow the repository rules for verification.
- Prefer the local project pattern over a generic Rails, Echo, Bun, Templ, or Vue convention.
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
- Put admin Inertia pages and reusable Vue components in `resources/js/`.
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
andurel project info --json
andurel routes --json
andurel models --json
andurel migrations --json
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
andurel routes --json
andurel generate routes --json
```

`andurel generate routes` reads `router/routes/*.go` as the source of truth and writes `resources/js/routes.ts`. Import helpers from that file in Vue or React pages instead of hard-coding URLs.

Check or sync factories:

```bash
andurel generate factory Product --check --json
andurel generate factory Product --sync --json
andurel generate factories --check --json
andurel generate factories --sync --json
```

Factory guidance:

1. Treat model `Entity` structs as the source of truth for generated factory fields.
2. Keep reusable test data builders in `models/factories/`.
3. Prefer `andurel generate factory NAME --check --json` before editing factory files by hand.
4. Use `--sync` to update Andurel generated regions and preserve custom helpers outside those regions.
5. Pass `--skip-factory` only when a generated model or scaffold should intentionally omit a factory.

Generate a named database seed:

1. Inspect the relevant models and existing factories in `models/factories`.
2. Add a seed function to `database/seeds`, using only exported model/factory/storage primitives.
3. Register it in `seeds.Registry` with a stable lowercase name.
4. Keep the seed idempotence expectations explicit in code comments when it may be re-run.
5. Verify the seed is discoverable:

```bash
andurel database seed --list
andurel database seed development
andurel database seed test
```

Check project health:

```bash
andurel doctor --json
```

## Validation

Use the repository's allowed validation commands and project guidance. In this repo, do not run `go test`, `go build`, or `npm run`; use `go vet`, `go fix`, and `gofmt`.
