<p align="center">
  <img width="400" height="300" alt="andurel-logo" src="https://github.com/user-attachments/assets/8261d514-c070-44c0-a96a-4132045855fc" />
</p>

# Andurel - Rails-like Web Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.26.0%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbvlabs/andurel.svg)](https://pkg.go.dev/github.com/mbvlabs/andurel)
[![Go Report Card](https://goreportcard.com/badge/github.com/mbvlabs/andurel)](https://goreportcard.com/report/github.com/mbvlabs/andurel)
![Coverage](https://img.shields.io/badge/Coverage-48.5%25-yellow)

---

**Andurel is a comprehensive web development framework for Go.
It prioritizes development speed.** Inspired by Ruby on Rails, it uses just enough conventions to let you build full-stack web applications incredibly fast.

Join the discord [here](https://discord.gg/TnTBZHvat3)

## Platform Support

Andurel currently supports **Linux** and **macOS** only. Windows is not supported at this time.

If you'd like to help bring Windows support to Andurel, please see [issue #382](https://github.com/mbvlabs/andurel/issues/382) - contributions are welcome!

---

## Why Andurel?

Development speed is everything. Andurel eliminates boilerplate and lets you focus on building features:

- **Instant Scaffolding** - Generate complete CRUD resources with one command
- **Live Reload** - Hot reloading for Go, templates, and CSS with `andurel run` powered by [Shadowfax](https://github.com/mbvlabs/shadowfax)
- **Type Safety Everywhere** - Bun for SQL, Templ/Vue for HTML, Go for logic
- **Batteries Included** — Echo, Datastar, background jobs, sessions, CSRF protection, telemetry, email support, authentication, optional extensions (docker, aws-ses, css-components)
- **Dependency Injection** — Declarative application wiring with `go.uber.org/fx`
- **Two Frontend Options** — Server-rendered HTML with **Templ + Datastar** for hypermedia interactivity, or **Inertia SPA with Vue 3 or React + Vite** for a reactive single-page app
- **Production Build** — One command (`andurel build`) to compile everything: Templ, Tailwind CSS, Vite assets, and Go binary
- **Just enough Convention** - Convention over configuration is great to a certain point. Andurel provides just enough sensible defaults that just work and get out of your way.
- **PostgreSQL-Backed** - Built on PostgreSQL with River job queues, pgx driver, and UUID support

The core philosophy around resource generation in andurel, is that it should be a one-time operation that creates everything you need for a fully functional CRUD interface. After that, you can modify and extend the generated code as needed but it's yours to manage going forward.

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
- **[gonertia](https://github.com/romsar/gonertia)** - Inertia.js Go adapter (optional, `--inertia vue` or `--inertia react`; append `/npm`, `/pnpm`, `/bun`, or `/yarn` to set JS runtime)
- **[Vue.js](https://vuejs.org/) / [React](https://react.dev/)** - JavaScript UI adapters (optional, via Inertia)
- **[Vite](https://vitejs.dev/)** - Next-generation frontend build tool (optional, via Inertia)

## Quick Start

### Installation

```bash
go install github.com/mbvlabs/andurel@v1.0.0-rc.1
```

### Create Your First Project

Andurel gives you choices when creating a new project:

```bash
# Create a new project (PostgreSQL + Tailwind CSS)
andurel new myapp

# Add extensions for additional features:
andurel new myapp -e docker              # Add Dockerfile for containerization
andurel new myapp -e aws-ses             # Add AWS SES email integration

# Choose your frontend approach:
andurel new myapp --inertia vue           # Inertia SPA with Vue 3 + Vite (JS runtime: npm)
andurel new myapp --inertia react/pnpm    # Inertia SPA with React + Vite (JS runtime: pnpm)
andurel new myapp --inertia vue/bun       # Inertia SPA with Vue 3 + Vite (JS runtime: bun)

# Combine options:
andurel new myapp --inertia vue -e docker

cd myapp

# Sync tools
andurel tool sync

# Configure environment
cp .env.example .env

# Note: you need to edit .env with your database details

# Install JS dependencies (only if using --inertia vue/react)
# andurel new prints the correct package manager command based on the configured runtime (npm/pnpm/bun/yarn)

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
| `--jq '.field.path'` | Apply a built-in simple field-path filter to structured data |
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

The embedded agent skill is available from the binary:

```bash
andurel skill show
andurel skill show --json
andurel skill install
```

`skill install` writes the Andurel skill into the current project at `.codex/skills/andurel/`, including the framework-specific layer-placement reference.

### Mutation previews

Mutating commands that support `--dry-run` report artifact changes before writing files:

```bash
andurel new myapp --dry-run --json
andurel generate scaffold Product --dry-run --json
andurel generate controller Dashboard overview --dry-run --json
andurel extension add docker --dry-run --json
andurel upgrade --dry-run --json
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
| `--inertia` | Frontend adapter: `vue` or `react`. Optionally append `/npm`, `/pnpm`, `/bun`, or `/yarn` to set JS runtime (default: `npm`). Example: `--inertia vue/pnpm` |

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
```

**`generate model`** — Creates a model from a database migration, or updates an existing one. Fields, types, and timestamps are read from the migration automatically.

| Flag | Description |
|------|-------------|
| `--skip-factory` | Skip generating a factory file |
| `--table-name`   | Override the default table name (e.g. `--table-name=people_data`) |
| `--update`       | Update an existing model from migration changes |
| `--yes`          | Apply changes without prompting for confirmation (use with `--update`) |
| `--primary-key`  | Specify the primary key column (skips interactive detection) |
| `--dry-run`      | Preview file changes without applying them |
| `--diff`         | Include a text diff preview in structured output |

**`generate factory`** — Generates or syncs one model factory from the model entity. Use `--check --json` in CI or agent workflows to detect drift without writing files, and `--sync --json` to update generated factory regions.

**`generate factories`** — Checks or syncs every model factory in the project. Use `--check --json` for a structured drift report across all models.

**`generate controller`** — Creates a controller for a resource. With no actions, it generates the full standard CRUD controller, views, and routes. With one or more standard CRUD actions (`index`, `show`, `new`, `create`, `edit`, `update`, `destroy`), it generates only those resource actions; partial CRUD views are self-contained and only link to companion actions that are also present. Generated resource/controller views default to Templ in every project; pass `--inertia` to generate Inertia pages (uses the adapter from `andurel.lock`).

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

In this mode, controller/UI artifacts use `Dashboard` (`controllers/dashboards.go`, `views/dashboards_resource.templ`, `/dashboards` routes), while model calls and entity types use `User` (`models.User.Paginate`, `models.User.Find`, `models.UserEntity`, `models.CreateUserData`, `models.UpdateUserData`). This is only for `generate controller`; `generate scaffold` keeps the existing one-resource-name behavior.

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

**`generate scaffold`** — Convenience command that runs `generate model` + `generate controller` with full CRUD actions (index, show, new, create, edit, update, destroy). By default generates Templ views, including in projects created with Inertia; pass `--inertia` for Inertia views (reads the adapter from `andurel.lock`).

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

The command is always visible in CLI discovery, but only runs in projects whose `andurel.lock` has `scaffoldConfig.inertia` set to `vue` or `react`. It reads the same `router/routes/*.go` route package used by `andurel routes --json` and writes `resources/js/routes.ts`. Route variables become lower-camel-case helper names, and typed route params become function arguments:

```ts
// resources/js/routes.ts
export const routes = {
  passwordEdit: (token: string) => `/users/password/${token}/edit`,
  sessionCreate: () => '/users/sign-in',
}
```

Use this after adding or changing routes for an Inertia project so Vue or React pages can import route helpers instead of hard-coding URL strings. Non-Inertia projects receive a structured `invalid_inertia_adapter` error. `--json` reports the generated file, helper count, skipped count, and any skipped manifest entries.

### `andurel routes` — Route manifest

Lists route metadata extracted from `router/routes/*.go`.

```bash
andurel routes
andurel routes --json
```

The default output is a table with route variables, route names, actual URL paths, parameters, and source locations. In this command, `path` means the URL path for the route. The Go file where the route variable is declared is reported separately as `source_file` in JSON output.

`andurel routes --json` is the stable machine-readable route manifest. `andurel generate routes` uses this same source of truth to generate Inertia `resources/js/routes.ts` helpers.

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

Runs Templ generation, minifies Tailwind CSS, installs JavaScript dependencies and builds Vite assets with the runtime stored in `andurel.lock` (if using Inertia), downloads Go dependencies, and compiles a static Linux binary.

For Inertia projects, `andurel build` reads `scaffoldConfig.javascriptRuntime` from `andurel.lock`. Existing locks without that field default to `npm`.

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

> Commit or create a branch before upgrading — this command modifies files in place.

### `andurel doctor` — Project diagnostics

Run comprehensive diagnostic checks (Go version, config, code quality, code generation).

```bash
andurel doctor (alias: doc) [--verbose]
```

For Inertia projects, the Code Generation checks also compare `resources/js/routes.ts` against the current `router/routes/*.go` manifest and fail when the file is missing or stale. Run `andurel generate routes` to update it.

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

### `andurel skill` — Embedded agent skill

Shows or installs the Andurel agent skill with CLI recipes, invariants, and framework layer-placement guidance.

```bash
andurel skill show
andurel skill show --json
andurel skill install
```

`andurel skill install` copies the embedded skill into `.codex/skills/andurel/` for the current project.

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
│   └── app/
│       └── main.go          # Application entry point with fx wiring
├── config/
│   ├── config.go            # Main config aggregator
│   ├── app.go               # Sessions, tokens, security
│   ├── auth.go              # Authentication config
│   ├── database.go          # Database connection config
│   ├── email.go             # Email configuration
│   └── telemetry.go         # Logging, tracing, metrics
├── controllers/
│   ├── controller.go        # Controller module setup
│   ├── api.go
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

### Inertia Mode (`--inertia vue` or `--inertia react`)

When using the Inertia SPA frontend, these files are **added**:

```
myapp/
├── resources/
│   └── js/
│       ├── app.ts/app.tsx       # Vue/React + Inertia app entry point
│       ├── Layouts/
│       │   └── Layout.vue/tsx    # Shared Inertia page layout
│       └── Pages/
│           ├── Auth/             # Login, registration, email confirmation, password reset
│           └── Errors/           # Bad request, not found, internal error
├── views/
│   ├── root.go.html             # Inertia root HTML shell
│   └── welcome.templ            # Server-rendered welcome page
├── internal/
│   └── inertia/
│       ├── render.go            # Inertia response helpers
│       └── vite.go              # Vite dev/prod manifest resolver
├── vite.config.ts
├── package.json
├── tsconfig.json
```

The auth and default error pages use Inertia, while `controllers/pages.go` keeps the welcome page server-rendered with Templ. `cmd/app/main.go` initializes `internal/inertia`. Run the configured package manager's install command after scaffolding (the `andurel new` output shows the right command based on the configured runtime). Later resource/controller generation still defaults to Templ; pass `--inertia` to `andurel generate controller` or `andurel generate scaffold` for Inertia resource pages (reads the adapter from `andurel.lock`).

When using `--inertia vue` or `--inertia react`, controllers can render Inertia pages alongside Templ.

You can specify the JS runtime by appending `/npm`, `/pnpm`, `/bun`, or `/yarn` to the adapter name:
- `--inertia vue` — uses `npm` (default)
- `--inertia vue/pnpm` — uses `pnpm`
- `--inertia react/bun` — uses `bun`
- `--inertia react/yarn` — uses `yarn`

The runtime is stored in `andurel.lock` as `scaffoldConfig.javascriptRuntime`. `andurel build` uses this value for both dependency installation and Vite asset compilation, so a project created with `--inertia react/pnpm` uses `pnpm`, not `npm`.


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
import Layout from '../../Layouts/Layout.vue'
import { routes } from '../../routes'

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

Flash messages set via `cookies.AddFlash()` in the controller are automatically injected into Inertia props as `flash` and displayed as toast notifications in the Vue app entry point.

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

Inspired by Ruby on Rails and its philosophy that developer happiness and productivity matter. Built for developers who want to move fast without sacrificing type safety or code quality.

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
