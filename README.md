<img width="2256" height="408" alt="andurel-wordmark-dark" src="https://github.com/user-attachments/assets/fe351b07-15e8-41e1-9be1-23feb894acf5" />

# Andurel, Space-grade Go framework for humans and agents

[![Go Version](https://img.shields.io/badge/go-1.26.0%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbvlabs/andurel.svg)](https://pkg.go.dev/github.com/mbvlabs/andurel)
[![Go Report Card](https://goreportcard.com/badge/github.com/mbvlabs/andurel)](https://goreportcard.com/report/github.com/mbvlabs/andurel)
[![codecov](https://codecov.io/gh/mbvlabs/andurel/branch/master/graph/badge.svg)](https://app.codecov.io/gh/mbvlabs/andurel)

---

Everything you and your agent(s) need to build robust and performant applications that will scale to the far-side of the moon.

Inspired by Ruby on Rails, Andurel is the fullstack framework for the agentic era: one-time generation that writes Go you own, explicit wiring, and an agent-ready CLI.

Join the discord [here](https://discord.gg/TnTBZHvat3)

## Platform Support

Andurel v1 supports **Linux** and **macOS** on **amd64** and **arm64**. Windows is not supported. Official releases contain and smoke-test all four operating-system and architecture combinations.

See [release verification](docs/release-verification.md) for archive installation, checksums, SBOMs, keyless signatures, and provenance. Maintainers must follow the [release procedure](docs/releasing.md) before creating a version tag.

The frozen v1 contracts are documented in the [public Go API policy](docs/contracts/public-api.md), [CLI and structured-output policy](docs/contracts/cli-v1.md), and [lock schema 1 specification](docs/contracts/lock-schema-v1.md). Model generation supports the conservative SQL subset in the [DDL parser contract](docs/ddl-model-generation.md), and upgrades follow the [generated-file ownership policy](docs/generated-files-and-upgrades.md).

If you'd like to help bring Windows support to Andurel, please see [issue #382](https://github.com/mbvlabs/andurel/issues/382) - contributions are welcome!

---

## Why Andurel?

Andurel is built for humans and agents on the same pad:

- **Airframe** — Generated models, factories, controllers, routes, and pages that belong to the application
- **GNC** — Fx for dependency injection and lifecycle. Explicit wiring, not a hidden runtime
- **Propellant** — PostgreSQL with Bun for ordinary persistence and sqlc for the queries that matter
- **Payload** — Inertia v3 with React, Vue, or Svelte. Same Go backend, official Inertia adapters, optional SSR. Templ + Datastar remains available for hypermedia pages
- **Range ops** — River jobs, email, and queues on the same stack that serves the request path
- **Ground support** — Agent-ready CLI, JSON discovery, and AGENTS.md so machines can operate the project without a second stack
- **Live reload** — `andurel run` with [Shadowfax](https://github.com/mbvlabs/shadowfax) for Go, templates, CSS, and Vite
- **Production build** — `andurel build` compiles Templ, Tailwind CSS, Vite assets, and the Go binary

**Humans.** One-time generation. The scaffold writes Go and Inertia pages you can read, edit, and ship. After that, the code is yours.

**Agents.** An agent-ready CLI with JSON discovery, plus AGENTS.md in the project. Machines get the same pad as humans, without inventing a second stack.

## Core Technologies

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP framework
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS tooling
- **[Bun](https://bun.uptrace.dev/)** - Type-safe SQL ORM and query builder
- **[Templ](https://templ.guide/)** - Type-safe HTML templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity
- **[River](https://riverqueue.com/)** - PostgreSQL-backed background jobs and workflows
- **[OpenTelemetry](https://opentelemetry.io/)** - Built-in observability
- **[PostgreSQL](https://www.postgresql.org/)** - Powerful open-source database with pgx driver and native UUID support
- **[Shadowfax](https://github.com/mbvlabs/shadowfax)** - Andurel-specific app runner
- **[go.uber.org/fx](https://uber-go.github.io/fx/)** - Dependency injection framework
- **[Andurel Inertia](./docs/inertia-v3.md)** - Echo-native Inertia.js v3 server adapter with a templ root document (optional, `--inertia vue`, `--inertia react`, or `--inertia svelte`; append `/npm`, `/pnpm`, `/bun`, or `/yarn` to set JavaScript package manager)
- **[Vue.js](https://vuejs.org/) / [React](https://react.dev/) / [Svelte](https://svelte.dev/)** - JavaScript UI adapters (optional, via Inertia)
- **[Vite](https://vitejs.dev/)** - Next-generation frontend build tool (optional, via Inertia)

## Quick Start

### Install the CLI

```bash
go install github.com/mbvlabs/andurel@latest

andurel --version
```

For reproducible automation, prefer an explicit stable v1 tag. Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 require manual reconciliation rather than `andurel upgrade`; follow the [RC-to-v1 manual upgrade guide](docs/upgrade-rc-base-scaffold-prompt.md).

### Stand up a vehicle

```bash
andurel new orbit --inertia react
```

Andurel gives you choices when creating a new project:

```bash
# Create a new project (PostgreSQL + Tailwind CSS)
andurel new myapp

# Add extensions for additional features:
andurel new myapp -e docker              # Add Dockerfile for containerization
andurel new myapp -e aws-ses             # Add AWS SES email integration

# Choose your frontend approach:
andurel new myapp --inertia vue           # Inertia SPA with Vue 3 + Vite (JS package manager: npm)
andurel new myapp --inertia react/pnpm    # Inertia SPA with React + Vite (JS package manager: pnpm)
andurel new myapp --inertia svelte        # Inertia SPA with Svelte 5 + Vite (JS package manager: npm)
andurel new myapp --inertia vue/bun       # Inertia SPA with Vue 3 + Vite (JS package manager: bun)

# Combine options:
andurel new myapp --inertia vue -e docker

cd myapp

# Sync tools
andurel tool sync

# Arm the agents
andurel skill install

# Configure environment
cp .env.example .env

# Note: you need to edit .env with your database details

# Install JS dependencies (only if using an Inertia adapter)
# andurel new prints the correct package manager command based on the configured package manager (npm/pnpm/bun/yarn)

# Apply database migrations
andurel database migrate up

# Run the development server (with live reload)
andurel run
```

Your app is now running on `http://localhost:8080`

### Database Lifecycle Commands

Andurel provides commands to manage your database lifecycle:

```bash
# Create the configured database
andurel database create                    # Requires .env to be filled out with DB credentials

# Drop the configured database (prompts for confirmation)
andurel database drop
andurel database drop --force              # Allow dropping system databases

# Drop and recreate the database
andurel database nuke
andurel database nuke --force              # Allow nuking system databases

# Full rebuild: drop, recreate, migrate, and seed
andurel database rebuild
andurel database rebuild --force           # Allow rebuilding system databases
andurel database rebuild --skip-seed       # Skip seeding after migrations
```

### Generate Your First Resource

```bash
# Create a migration and add the columns you need. Resource generation requires
# an `id` primary key (uuid/serial/bigserial/string-supported types). `created_at`
# and `updated_at` are optional but recommended.
andurel database migrate new create_products_table

# Create a complete resource with model, controller, views, and routes
andurel generate scaffold Product
```

This single command creates everything you need for a full CRUD interface: model, factory, controller, Templ views, and resource routes. Pass `--inertia` when you want the generated resource/controller views as Inertia pages instead (reads the adapter from `andurel.lock`).

## Agent-Ready CLI

Andurel's CLI is designed to be consumed by people and agents. Commands that support structured output use a stable response envelope:

```json
{
  "ok": true,
  "data": {},
  "summary": "Generated resource",
  "breadcrumbs": [{"cmd": "andurel routes --json", "description": "Inspect generated routes"}]
}
```

Structured errors use the same contract:

```json
{
  "ok": false,
  "code": "generation_failed",
  "error": "failed to generate resource",
  "hint": "Inspect the error details and generated files, then retry.",
  "exit_code": 5
}
```

### Output modes

| Flag | Purpose |
|------|---------|
| `--json` | Emit the full `{ok,data,summary,breadcrumbs}` envelope |
| `--agent` | Emit structured output for agents and suppress non-essential human progress output |
| `--md` | Emit Markdown where supported |
| `--quiet` | Suppress non-essential human output |
| `--jq '.field.path'` | Select from the command data payload and emit the selected JSON value directly |
| `--ids-only` | Emit only resource identifiers where supported |
| `--count` | Emit only resource counts where supported |
| `--verbose` | Emit additional diagnostics where supported |

### Discovery and project inspection

Agents should start with CLI discovery instead of scraping prose:

```bash
andurel --agent --help
andurel commands --json
andurel project info --json
andurel config show --json
```

Project-shape commands are read-only and return structured data:

```bash
andurel routes --json
andurel models --json
andurel migrations --json
andurel controllers --json
andurel views --json
andurel jobs --json
```

`andurel project info --json` reports the Inertia adapter and JavaScript package manager from `andurel.lock`. sqlc is always scaffolded but only active when `models/queries/` contains an annotated query; use `andurel generate queries --json` for a structured generation report.

The embedded agent skill is available from the binary:

```bash
andurel skill show
andurel skill show --json
andurel skill install
andurel skill install --harness claude,pi
```

In human mode, `skill install` prompts for one or more harnesses. Automation must pass one or more `--harness` values; the flag may be repeated or contain comma-separated values. Each selection receives the complete embedded skill, including the framework-specific layer-placement reference.

### Mutation previews

Mutating commands that support `--dry-run` report artifact changes before writing files:

```bash
andurel new myapp --dry-run --json
andurel generate scaffold Product --dry-run --json
andurel generate controller Dashboard overview --dry-run --json
andurel extension add docker --dry-run --json
andurel upgrade --dry-run --json
andurel packages update --dry-run --json
```

Add `--diff` with structured output when you need a text diff preview. Structured mutation reports include created, updated, and deleted files, route additions, commands run, warnings, and breadcrumbs.

## CLI Commands

### `andurel new` — Create a new project

Scaffolds a complete Andurel project with the given name.

```bash
andurel new (alias: n) [project-name] [flags]
```

| Flag | Description |
|------|-------------|
| `-e`, `--extensions` | Comma-separated extensions to enable (e.g. `docker,aws-ses,css-components`) |
| `--inertia` | Frontend adapter: `vue`, `react`, or `svelte`. Optionally append `/npm`, `/pnpm`, `/bun`, or `/yarn` to set JavaScript package manager (default: `npm`). Example: `--inertia vue/pnpm` |

### `andurel generate` — Code generation

Generate models, controllers, and scaffolds from your existing database migrations.

```bash
andurel generate (alias: g) model NAME [flags]
andurel generate factory NAME [flags]
andurel generate factories [flags]
andurel generate view (alias: v)
andurel generate controller (alias: c) NAME [action ...] [flags]
andurel generate scaffold (alias: s) NAME [flags]
andurel generate job (alias: j) NAME [flags]
andurel generate email (alias: e) NAME
andurel generate routes
andurel generate query NAME [flags]
andurel generate queries
```

**`generate model`** — Creates a model from a database migration, or updates an existing one. Fields, types, and timestamps are read from the migration automatically. When `--update` is applied, Andurel also syncs the matching factory unless `--skip-factory` is passed.

| Flag | Description |
|------|-------------|
| `--skip-factory` | Skip generating or updating the matching factory file |
| `--table-name`   | Override the default table name (e.g. `--table-name=people_data`) |
| `--update`       | Update an existing model from migration changes |
| `--yes`          | Apply changes without prompting for confirmation (use with `--update`) |
| `--primary-key`  | Specify the primary key column (skips interactive detection) |
| `--dry-run`      | Preview file changes without applying them |
| `--diff`         | Include a text diff preview in structured output |

**`generate factory`** — Generates or syncs one model factory from the model entity. With no flags, the singular command syncs by default. Use `--check --json` in CI or agent workflows to detect drift without writing files, and `--sync --json` to update the factory.

**`generate factories`** — Checks or syncs every model factory in the project. The plural command requires `--check` or `--sync` to avoid accidental repo-wide writes. Use `--check --json` for a structured drift report across all models.

Factory sync treats generated factory declarations as owned by Andurel. In practice, `Build<Name>`, `Create<Name>`, `Create<Name>s`, the factory types, and generated `WithX` option functions are regenerated from the current model entity. Custom helpers are preserved when they use names that do not collide with those generated declarations.

**`generate controller`** — Creates a controller for a resource. With no actions, it generates the full standard CRUD controller, views, and routes. With one or more standard CRUD actions (`index`, `show`, `new`, `create`, `edit`, `update`, `destroy`), it generates only those resource actions; partial CRUD views are self-contained and only link to companion actions that are also present. Generated resource/controller views default to Templ in every project; pass `--inertia` to generate Inertia pages (uses the adapter from `andurel.lock`). `--inertia` requires a configured Inertia project and cannot be combined with `--api`.

Non-CRUD actions create standalone/custom controller actions. They add empty controller methods, matching Templ components by default or Inertia pages with `--inertia`, and conventional `GET` routes:

```bash
andurel generate controller Dashboard overview
```

Generates:

| Artifact | Example |
|----------|---------|
| Controller method | `controllers/dashboards.go`: `Dashboards.Overview` |
| Templ view | `views/dashboards_resource.templ`: `DashboardOverview()` |
| Route variable | `router/routes/dashboards.go`: `DashboardOverview` |
| Route registration | `GET /dashboards/overview` named `dashboards.overview` |

Custom-only controller generation does not require a model or migration. If any CRUD action is requested, generation is model-backed and still requires an existing model/migration.

Use `--model-name` when the controller/resource name should differ from the model it is backed by:

```bash
andurel generate controller Dashboard --model-name User
andurel generate controller Dashboard index overview --model-name User
```

In this mode, controller/UI artifacts use `Dashboard` (`controllers/dashboards.go`, `views/dashboards_resource.templ`, `/dashboards` routes), while model calls and entity types use `User` (`users.Paginate`, `users.Find`, `models.User`, `models.CreateUserData`, `models.UpdateUserData`). This is only for `generate controller`; `generate scaffold` keeps the existing one-resource-name behavior.

Use `--api` to generate a JSON API controller instead. The controller is placed under `controllers/api` with `echo.JSON` responses and no views:

```bash
andurel generate controller Users --api
andurel generate controller admin/Widget export --api
```

When `--api` is set, any namespace segment is nested under `api`, and the default action set excludes `new` and `edit`. For example, `andurel generate controller v1/User create --api` writes `controllers/api/v1/users.go`. Custom actions create `etx.JSON(http.StatusOK, map[string]any{})` stubs.

| Flag | Description |
|------|-------------|
| `--model-name` | Use a different existing model for model-backed controller generation |
| `--inertia` | Generate Inertia views using the adapter configured in `andurel.lock` |
| `--api`       | Generate a JSON API controller under `controllers/api` without views |
| `--dry-run`   | Preview file changes without applying them |
| `--diff`      | Include a text diff preview in structured output |

**`generate view`** — Generates Go code from `.templ` template files (runs `templ generate`).

**`generate query`** — Creates an application-owned sqlc SQL file under `models/queries/`. Pass `--table` to include an active starter query for an existing table; without it, the file contains commented examples.

```bash
andurel generate query UserReport --table users --dry-run --json
andurel generate query UserReport --table users --json
```

**`generate queries`** — Runs the project-managed sqlc binary and formats generated Go code under `models/internal/queries/`. The command is a no-op when there are no SQL files containing a `-- name:` annotation, and structured modes still return a mutation report describing the skip.

```bash
andurel generate queries --json
```

sqlc is available in every newly scaffolded project but remains inactive until an annotated query is added. Bun remains the default for ordinary CRUD. Use sqlc for complex projections, reports, aggregates, bulk operations, or tuned SQL. Only the owning `models` package should import `models/internal/queries`; controllers and services should consume application-owned model types instead of sqlc-generated rows or parameters.

**`generate scaffold`** — Convenience command that runs `generate model` + `generate controller` with full CRUD actions (index, show, new, create, edit, update, destroy). By default generates Templ views, including in projects created with Inertia; pass `--inertia` for Inertia views (reads the adapter from `andurel.lock`). `--inertia` requires a configured Inertia project and cannot be combined with `--api`.

| Flag | Description |
|------|-------------|
| `--skip-factory` | Skip generating a factory file |
| `--table-name`   | Override the default table name |
| `--inertia`      | Generate Inertia views using the adapter configured in `andurel.lock` |
| `--api`          | Generate a JSON API controller under `controllers/api` without views |
| `--primary-key`  | Specify the primary key column (skips interactive detection) |
| `--dry-run`      | Preview file changes without applying them |
| `--diff`         | Include a text diff preview in structured output |

**`generate routes`** — Generates framework-neutral TypeScript helpers for Inertia frontends.

```bash
andurel generate routes
andurel generate routes --json
```

The command is always visible in CLI discovery, but only runs in projects whose `andurel.lock` has `scaffoldConfig.inertia` set to `vue`, `react`, or `svelte`. It reads the same `router/routes/*.go` route package used by `andurel routes --json` and writes `resources/js/routes.ts`. Route variables become lower-camel-case helper names, and typed route params become function arguments:

```ts
// resources/js/routes.ts
export const routes = {
  passwordEdit: (token: string) => `/users/password/${token}/edit`,
  sessionCreate: () => '/users/sign-in',
}
```

Use this after adding or changing routes for an Inertia project so Inertia pages can import route helpers instead of hard-coding URL strings. Non-Inertia projects receive a structured `invalid_inertia_adapter` error. `--json` reports the generated file, helper count, skipped count, and any skipped manifest entries.

### `andurel routes` — Route manifest

Lists route metadata extracted from `router/routes/*.go`.

```bash
andurel routes
andurel routes --json
andurel routes --jq .routes
```

The default output is a table with route variables, route names, actual URL paths, parameters, and source locations. In this command, `path` means the URL path for the route. The Go file where the route variable is declared is reported separately as `source_file` in JSON output.

`andurel routes --json` is the stable machine-readable route manifest. `andurel routes --jq .routes` emits the route array directly, without the normal success envelope. `andurel generate routes` uses this same source of truth to generate Inertia `resources/js/routes.ts` helpers.

Example JSON shape:

```json
{
  "ok": true,
  "data": {
    "routes": [
      {
        "variable": "SessionCreate",
        "name": "users.user_session",
        "path": "/users/sign-in",
        "constructor": "NewSimpleRoute",
        "kind": "simple",
        "source_file": "router/routes/users.go",
        "line": 12
      },
      {
        "variable": "PasswordEdit",
        "name": "users.edit_user_password",
        "path": "/users/password/:token/edit",
        "constructor": "NewRouteWithToken",
        "kind": "token",
        "params": [
          {
            "name": "token",
            "type": "string"
          }
        ],
        "source_file": "router/routes/users.go",
        "line": 39
      }
    ],
    "skipped": [
      {
        "variable": "Scripts",
        "constructor": "NewSimpleRoute",
        "source_file": "router/routes/assets.go",
        "line": 33,
        "reason": "route path is not a static string expression"
      }
    ]
  },
  "summary": "Listed 2 routes (1 skipped)"
}
```

`skipped` entries mean Andurel found a route constructor but could not statically evaluate its path, name, or prefix. This commonly happens for dynamic asset routes.

### `andurel fmt` — Format source files

Formats Go and Templ source files in the project.

```bash
andurel fmt (alias: f) [flags]
```

| Flag | Description |
|------|-------------|
| `--check`      | Check formatting without modifying files (CI-friendly) |
| `--skip-templ` | Skip Templ formatting |
| `--skip-go`    | Skip Go formatting (go fmt and golines) |

Runs `go fmt ./...`, `golines -w -m 100 .`, and `templ fmt` on `views/` and `email/` directories.

### `andurel database` — Database management

Manage the full database lifecycle.

```bash
andurel database (aliases: d, db)
andurel database create
andurel database drop [--force]
andurel database nuke [--force]
andurel database rebuild [--force] [--skip-seed]
andurel database seed
andurel database migrate (aliases: m, mig)
```

**`database migrate` subcommands:**

| Subcommand | Description |
|------------|-------------|
| `new [name]` (alias: `n`) | Create a new SQL migration file |
| `up` | Apply all pending migrations |
| `down` | Roll back the most recently applied migration |
| `status` (alias: `st`) | Show current migration version and status |
| `fix` | Re-number migrations to close gaps |
| `reset` (alias: `rs`) | Roll back all migrations, then re-apply them |
| `up-to [version]` (alias: `upto`) | Apply migrations up to a specific version |
| `down-to [version]` (alias: `downto`) | Roll back migrations down to a specific version |

### `andurel build` — Production build

Build the application binary and compile all assets for production deployment.

```bash
andurel build [--version]
```

Runs Templ generation, minifies Tailwind CSS, installs JavaScript dependencies and builds Vite assets with the package manager stored in `andurel.lock` (if using Inertia), downloads Go dependencies, and compiles a static Linux binary.

For Inertia projects, `andurel build` reads `scaffoldConfig.javascriptPackageManager` from `andurel.lock`. V1 locks migrate the legacy `javascriptRuntime` value as a package manager; missing values default to `npm`. The Node executable used by `cmd/ssr` comes from app config (`INERTIA_SSR_RUNTIME`), not the lock.

| Runtime | Install command used by `andurel build` | Vite build command used by `andurel build` |
|---------|------------------------------------------|---------------------------------------------|
| `npm`   | `npm ci`                                 | `npm run build`                             |
| `pnpm`  | `pnpm install --frozen-lockfile`         | `pnpm run build`                            |
| `bun`   | `bun install --frozen-lockfile`          | `bun run build`                             |
| `yarn`  | `yarn install --frozen-lockfile`         | `yarn build`                                |

| Flag | Description |
|------|-------------|
| `--version` | Set the application version (injected via ldflags) |

### `andurel run` — Development server

Starts the development server with live reload (powered by Shadowfax).

```bash
andurel run (alias: r)
```

For Inertia projects, `andurel run` passes an explicit flag contract to Shadowfax
(`--inertia`, package manager, SSR URL/bundle). Shadowfax starts Vite and the
project's `cmd/ssr` process (Laravel-style Node owner). The HTTP app (`cmd/app`)
is always an SSR HTTP client; pages opt in with `inertia.WithSSR()`.

### `andurel console` — Database console

Opens an interactive database console (usql) using connection details from `.env`.

```bash
andurel console (alias: c)
```

### `andurel tool` — Project tools and binaries

Manage CLI tools and binaries used by your project. Tools are defined in `andurel.lock` and downloaded to `bin/`.

```bash
andurel tool (alias: t)
andurel tool sync
andurel tool set-version <tool> <version>
andurel tool dblab (alias: d)
andurel tool mailpit (alias: m)
```

| Subcommand | Description |
|------------|-------------|
| `sync` (alias: `s`) | Download and validate binaries specified in `andurel.lock` |
| `set-version` (alias: `sv`) | Set a specific tool version (e.g. `templ 0.3.977`) |
| `dblab` (alias: `d`) | Open the dblab database UI in the browser |
| `mailpit` (alias: `m`) | Run the Mailpit email testing server (SMTP :1025, HTTP :8025) |

### `andurel extension` — Project extensions

Add and list optional framework features. Adding an extension to an existing
project generates its code files, updates framework-managed files (config.go,
.env.example, main.go, etc.), and records it in andurel.lock. Commit or create
a branch before adding an extension, as it modifies files in place.

```bash
andurel extension (aliases: ext, e)
andurel extension add (alias: a) [extension-name]
andurel extension list (alias: ls)
```

Available extensions: `docker`, `aws-ses`, `css-components`.

### `andurel upgrade` — Framework upgrade

Upgrade framework-managed files and tool versions to the latest.

```bash
andurel upgrade (alias: up) [--dry-run]
```

> Commit or create a branch before upgrading. A real upgrade requires a clean worktree and modifies files in place.

Before changing project files, `andurel upgrade` checks the latest stable Andurel release. If the installed CLI is outdated, it stops and prints the exact `go install github.com/mbvlabs/andurel@VERSION` command to run. Development builds and temporary network failures do not block an upgrade.

Run `andurel upgrade --dry-run --diff --json` first. Dry runs are read-only, and a failed transaction restores every changed file and `andurel.lock`. Upgrade ownership is limited to framework-owned files, currently centered on `internal/*`, plus verified `github.com/mbvlabs/andurel/pkg/*` pins already required in `go.mod` (Inertia is also added when the project uses an Inertia adapter). See [generated-file ownership and upgrade behavior](docs/generated-files-and-upgrades.md). To take the latest published package versions without a framework upgrade, use `andurel packages update`.

Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 must not use the automated upgrade command. Use the [RC-to-v1 manual upgrade guide](docs/upgrade-rc-base-scaffold-prompt.md) to reconcile the application against the stable scaffold for the currently installed Andurel version while preserving local changes.

### `andurel packages` — Andurel package versions

List or update standalone Andurel modules already required in the project's `go.mod` (`github.com/mbvlabs/andurel/pkg/*`). Latest versions are read from `proxy.golang.org`. Packages with a `replace` directive are reported and left unchanged. This does not upgrade framework-owned files. `andurel upgrade` pins required packages to the versions verified with the installed CLI.

```bash
andurel packages (aliases: package, pkg)
andurel packages list (alias: ls)
andurel packages update (alias: up) [--dry-run] [packages...]
```

`andurel packages update` runs `go get` and `go mod tidy` for outdated packages. Pass package names (`storage`, `email`, ...) or full module paths to limit the update.

### `andurel doctor` — Project diagnostics

Run comprehensive diagnostic checks (Go version, latest stable Andurel release, config, code quality, code generation).

```bash
andurel doctor (alias: doc) [--verbose]
```

For Inertia projects, the Code Generation checks also compare `resources/js/routes.ts` against the current `router/routes/*.go` manifest and fail when the file is missing or stale. Run `andurel generate routes` to update it.

If a newer stable CLI release exists, `andurel doctor` reports a nonblocking warning with the exact installation command. If the release lookup is unavailable, doctor warns without failing the project health check.

### `andurel commands` — Structured command discovery

Shows the full command tree, flags, descriptions, examples, and agent metadata.

```bash
andurel commands --json
andurel commands --agent
andurel generate --agent --help
```

Use this when an agent or script needs to discover the CLI surface without parsing human help text.

### `andurel project` — Project metadata

Reads project metadata from `go.mod`, `andurel.lock`, and Andurel config files.

```bash
andurel project info --json
```

The response includes the project root, Go module, Andurel version, scaffold config, database config, extensions, tools, and config/cache paths.

### `andurel config` — Agent configuration

Manages non-secret Andurel configuration across project, user, and cache scopes.

```bash
andurel config init [--scope project|user|cache]
andurel config show --json
andurel config set KEY VALUE [--scope project|user|cache]
andurel config unset KEY [--scope project|user|cache]
```

Project config is stored at `.andurel/config.json`. User config uses the OS config directory under `andurel/config.json`, and cache config uses the OS cache directory under `andurel/config.json`.

### `andurel skill` - Embedded agent skill

Shows or installs the Andurel agent skill with CLI recipes, invariants, and framework layer-placement guidance.

```bash
andurel skill show
andurel skill show --json
andurel skill install
andurel skill install --harness codex,claude
andurel skill install --harness pi --harness crush
```

Without `--harness`, human mode displays a numbered multi-select prompt with no default. JSON, agent, Markdown, and quiet modes require `--harness` so automation never waits for input.

| Harness | Project path |
|---|---|
| Codex | `.codex/skills/andurel/` |
| Claude | `.claude/skills/andurel/` |
| Pi | `.pi/skills/andurel/` |
| OpenCode | `.opencode/skills/andurel/` |
| Crush | `.crush/skills/andurel/` |

---

### Alias Reference

| Full Command | Alias(es) |
|---|---|
| `andurel new` | `n` |
| `andurel generate` | `g` |
| `andurel generate model` | `m` |
| `andurel generate factory` | none |
| `andurel generate factories` | none |
| `andurel generate view` | `v` |
| `andurel generate controller` | `c` |
| `andurel generate scaffold` | `s` |
| `andurel generate job` | `j` |
| `andurel generate email` | `e` |
| `andurel generate routes` | none |
| `andurel fmt` | `f` |
| `andurel database` | `d`, `db` |
| `andurel database create` | `crt` |
| `andurel database seed` | `s` |
| `andurel database rebuild` | `rb` |
| `andurel database migrate` | `m`, `mig` |
| `andurel database migrate new` | `n` |
| `andurel database migrate status` | `st` |
| `andurel database migrate reset` | `rs` |
| `andurel database migrate up-to` | `upto` |
| `andurel database migrate down-to` | `downto` |
| `andurel run` | `r` |
| `andurel console` | `c` |
| `andurel tool` | `t` |
| `andurel tool sync` | `s` |
| `andurel tool set-version` | `sv` |
| `andurel tool dblab` | `d` |
| `andurel tool mailpit` | `m` |
| `andurel extension` | `ext`, `e` |
| `andurel extension add` | `a` |
| `andurel extension list` | `ls` |
| `andurel upgrade` | `up` |
| `andurel packages` | `pkg`, `package` |
| `andurel packages list` | `ls` |
| `andurel packages update` | `up` |
| `andurel doctor` | `doc` |
| `andurel commands` | none |
| `andurel project info` | none |
| `andurel config` | none |
| `andurel routes` | none |
| `andurel skill` | none |

## Project Structure

Andurel generates a complete project based on your chosen options. Below is the default structure, followed by what changes with each option.

### Default Project

```
myapp/
├── assets/                  # Static assets (served at /assets/)
│   ├── assets.go
│   ├── css/
│   │   └── style.css       # Compiled Tailwind output
│   └── js/
│       ├── datastar_1-0-1.min.js
│       └── scripts.js
├── clients/
│   └── email/
│       └── mailpit.go       # Mailpit email client
├── cmd/
│   ├── app/
│   │   └── main.go          # Application entry point with fx wiring
│   └── ssr/
│       └── main.go          # Inertia SSR Node process owner
├── config/
│   ├── config.go            # Main config aggregator
│   ├── app.go               # Sessions, tokens, security
│   ├── auth.go              # Authentication config
│   ├── database.go          # Database connection config
│   ├── email.go             # Email configuration
│   └── telemetry.go         # Logging, tracing, metrics
├── controllers/
│   ├── controller.go        # Controller module setup
│   ├── api/
│   │   └── api.go
│   ├── assets.go
│   ├── cache.go             # Cache control utilities
│   ├── confirmations.go
│   ├── pages.go
│   ├── registrations.go
│   ├── reset_passwords.go
│   └── sessions.go
├── css/
│   ├── base.css             # Tailwind CSS source input
├── database/
│   ├── database.go          # Database connection helper
│   ├── test_helper.go       # Test database setup
│   ├── migrations/          # SQL migration files (goose)
│   └── seeds/
│       └── main.go          # Database seeder
├── email/
│   ├── email.go
│   ├── base_layout.templ
│   ├── components.templ
│   ├── reset_password.templ
│   └── verify_email.templ
├── internal/
│   ├── hypermedia/          # HTML-over-the-wire helpers
│   │   ├── broadcaster.go
│   │   ├── core.go
│   │   ├── helpers.go
│   │   ├── options.go
│   │   ├── render.go
│   │   ├── script.go
│   │   ├── signals.go
│   │   └── sse.go
│   ├── request/
│   │   ├── context.go
│   │   └── request.go
│   ├── routing/
│   │   ├── definitions.go
│   │   └── routes.go
│   ├── server/
│   │   └── server.go
│   └── storage/
│       ├── psql.go
│       └── queue.go
├── models/
│   ├── model.go
│   ├── errors.go
│   ├── token.go
│   ├── user.go
│   └── factories/           # Model factories for testing
│       ├── factories.go
│       ├── token.go
│       └── user.go
├── queue/
│   ├── queue.go
│   ├── jobs/
│   │   ├── send_marketing_email.go
│   │   └── send_transactional_email.go
│   └── workers/
│       ├── workers.go
│       ├── send_marketing_email.go
│       └── send_transactional_email.go
├── router/
│   ├── router.go            # Main router setup
│   ├── cookies/
│   │   ├── cookies.go
│   │   └── flash.go
│   ├── middleware/
│   │   ├── middleware.go
│   │   └── auth.go
│   └── routes/
│       ├── api.go
│       ├── assets.go
│       ├── pages.go
│       └── users.go
├── services/
│   ├── authentication.go
│   ├── registration.go
│   └── reset_password.go
├── telemetry/
│   ├── telemetry.go
│   ├── options.go
│   ├── logger.go
│   ├── log_exporters.go
│   ├── metrics.go
│   ├── metric_exporters.go
│   ├── tracer.go
│   ├── trace_exporters.go
│   └── helpers.go
├── views/                    # Templ templates
│   ├── layout.templ
│   ├── head.templ
│   ├── welcome.templ
│   ├── bad_request.templ
│   ├── confirm_email.templ
│   ├── internal_error.templ
│   ├── login.templ
│   ├── not_found.templ
│   ├── registration.templ
│   ├── reset_password.templ
│   └── components/
├── .env.example
├── .gitignore
├── andurel.lock              # Tool version lock file
├── go.mod
└── go.sum
```

### Inertia Mode (`--inertia vue`, `--inertia react`, or `--inertia svelte`)

When using the Inertia SPA frontend, these files are **added**:

```
myapp/
├── resources/
│   └── js/
│       ├── app.ts/app.tsx         # Inertia app entry point
│       ├── Components/            # Adapter-specific shared components
│       ├── Layouts/
│       │   └── Layout.vue/tsx/svelte # Shared Inertia page layout
│       └── Pages/                 # .vue, .tsx, or .svelte pages
│           ├── Auth/              # Login, registration, email confirmation, password reset
│           └── Errors/            # Bad request, not found, internal error
├── application/
│   └── metadata.go              # Shared non-sensitive application identity
├── views/
│   ├── root.templ               # Application-owned Inertia document
│   └── welcome.templ            # Server-rendered welcome page
├── config/
│   └── inertia.go               # Environment-backed Inertia settings
├── cmd/
│   └── ssr/
│       └── main.go              # Starts Node SSR from INERTIA_SSR_LISTEN
├── vite.config.ts
├── svelte.config.js            # Svelte projects only
├── package.json
├── tsconfig.json
```

The auth and default error pages use Inertia, while `controllers/pages.go` keeps the welcome page server-rendered with Templ. Controllers and the router import the reusable Echo/templ implementation directly from `github.com/mbvlabs/andurel/pkg/inertia`. The package supplies Vite integration, protocol behavior, and an optional managed SSR runtime used by `cmd/ssr`. The HTTP app owns its compiled templ document at `views/root.templ`; generated `config/inertia.go` supplies environment-backed settings, and `cmd/app` constructs an HTTP-client renderer with `views.Root`. Run the configured package manager's install command after scaffolding. Later resource/controller generation still defaults to Templ; pass `--inertia` to `andurel generate controller` or `andurel generate scaffold` for Inertia resource pages.

When using `--inertia vue`, `--inertia react`, or `--inertia svelte`, controllers can render Inertia pages alongside Templ.

You can specify the JavaScript package manager by appending `/npm`, `/pnpm`, `/bun`, or `/yarn` to the adapter name:
- `--inertia vue`: uses `npm` (default)
- `--inertia vue/pnpm`: uses `pnpm`
- `--inertia react/bun`: uses `bun`
- `--inertia react/yarn`: uses `yarn`
- `--inertia svelte`: uses `npm` (default)
- `--inertia svelte/pnpm`: uses `pnpm`

The package manager is stored in `andurel.lock` as `scaffoldConfig.javascriptPackageManager`. `andurel build` uses it for dependency installation and Vite scripts. The SSR Node executable is configured separately in `config/inertia.go` (`INERTIA_SSR_RUNTIME`) so choosing Bun as a package manager does not silently replace Node as the SSR runtime.

SSR uses per-response `inertia.WithSSR()`. Node process ownership belongs to `cmd/ssr` (started by Shadowfax under `andurel run`, or by a process manager in production). `cmd/ssr` binds Node using `INERTIA_SSR_LISTEN` (IP or localhost; `0.0.0.0` is allowed). The HTTP app only calls `INERTIA_SSR_URL` (any http(s) host, including service DNS) and keeps bounded fallback to client rendering.


### Real Example: Controller to Vue Component

Here's how an auth controller renders a Vue component via Inertia, from route definition to rendered page.

Use `andurel routes --json` when frontend tooling needs the same route metadata. The JSON manifest keeps `router/routes/*.go` as the source of truth while exposing URL paths, route names, params, and source locations to external generators. Use `andurel generate routes` to write those URLs as TypeScript helpers in `resources/js/routes.ts`.

#### Route Definition

```go
// router/routes/users.go
var SessionNew = routing.NewSimpleRoute("/sign-in", "users.new_user_session", UserPrefix)
var SessionCreate = routing.NewSimpleRoute("/sign-in", "users.user_session", UserPrefix)
```

#### Route Registration

```go
// controllers/sessions.go
func (s Sessions) RegisterRoutes(r *router.Router) error {
    _, err := r.AddRoute(echo.Route{
        Method:  http.MethodGet,
        Path:    routes.SessionNew.Path(),
        Name:    routes.SessionNew.Name(),
        Handler: s.New,
    })
    return err
}
```

#### Controller

```go
// controllers/sessions.go
func (s Sessions) New(etx *echo.Context) error {
    return inertia.Page(etx, "Auth/Login", inertia.Props{})
}
```

#### Vue Component

```vue
<!-- resources/js/Pages/Auth/Login.vue -->
<script setup lang="ts">
import { Head, useForm } from '@inertiajs/vue3'
import Layout from '@/Layouts/Layout.vue'
import { routes } from '@/routes'

const form = useForm({ email: '', password: '' })
</script>

<template>
  <Layout>
    <Head title="Login" />
    <form @submit.prevent="form.post(routes.sessionCreate())">
      <!-- login fields -->
    </form>
  </Layout>
</template>
```

#### CRUD Index Example

For paginated list views, the controller queries data and passes it as props:

```go
// controllers/widgets.go
func (w Widgets) Index(etx *echo.Context) error {
    page := int64(1)
    if p := etx.QueryParam("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = int64(parsed)
        }
    }
    perPage := int64(25)

    widgets, err := models.Widget.Paginate(
        etx.Request().Context(), w.db.Executor(), page, perPage,
    )
    if err != nil {
        return inertia.Page(etx, "Errors/InternalError", inertia.Props{})
    }

    return inertia.Page(etx, "Widget/Index", inertia.Props{
        "items": widgets.Widgets,
    })
}
```

```vue
<!-- resources/js/Pages/Widget/Index.vue -->
<script setup lang="ts">
import { Head, Link } from '@inertiajs/vue3'

defineProps<{
  items: Array<Record<string, unknown>>
}>()
</script>

<template>
  <Head title="Widgets" />
  <div>
    <h1>Widgets</h1>
    <Link :href="'/widgets/create'">New Widget</Link>
    <table>
      <tr v-for="item in items" :key="item.id">
        <td>{{ item.name }}</td>
        <td>
          <Link :href="`/widgets/${item.id}`">View</Link>
          <Link :href="`/widgets/${item.id}/edit`">Edit</Link>
        </td>
      </tr>
    </table>
  </div>
</template>
```

Flash messages set via `cookies.AddFlash()` in the controller are automatically injected into Inertia props as `flash` and displayed as toast notifications by the configured Inertia adapter.

## Contributing

Contributions are welcome! Here's how to get started:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run quality checks allowed by this repository's agent guidance: `go fix ./...`, `gofmt -w <changed-go-files>`, and `go vet ./...`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Setup

```bash
git clone https://github.com/mbvlabs/andurel
cd andurel
go mod download
go fix ./...
go vet ./...
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Tech Stack

Andurel is built on top of excellent open-source projects:

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP router and framework
- **[Bun](https://bun.uptrace.dev/)** - Type-safe SQL ORM and query builder
- **[Templ](https://templ.guide/)** - Type-safe Go templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity (RC6)
- **[River](https://riverqueue.com/)** - Fast PostgreSQL-backed job queue and workflows
- **[OpenTelemetry](https://opentelemetry.io/)** - Observability framework for logs, traces, and metrics
- **[pgx](https://github.com/jackc/pgx)** - PostgreSQL driver and toolkit
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS tooling
- **[Cobra](https://cobra.dev/)** - CLI framework

## Acknowledgments

Inspired by Ruby on Rails and its philosophy that developer happiness and productivity matter. Andurel is now its own thing: generated Go you own, explicit wiring, and an agent-ready CLI for humans and machines.

---

### Sites build with Andurel

Here is a collection of sites and projects, I've built with this framework:
- [DeployCrate](https://deploycrate.com)
- [MBV Blog](https://mortenvistisen.com) | personal blog
- [Master Golang](https://mastergolang.com) | course platform
- [Palantir](https://github.com/mbvlabs/palantir) | open sourced analytics platform (WIP)

If you build something cool with Andurel, let me know and I will add it to the list (or open a PR)!

---

## Author

Created by [Morten Vistisen](https://mortenvistisen.com)

Feel free to reach out to me on:
- [Twitter/X](https://x.com/mbvlabs)
- [Mail](mailto:andurel@mbvlabs.com)

If you have any questions!
