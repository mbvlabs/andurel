<img width="2256" height="408" alt="andurel-wordmark-dark" src="https://github.com/user-attachments/assets/fe351b07-15e8-41e1-9be1-23feb894acf5" />

# Andurel, Space-grade Go framework for humans and agents

[![Go Version](https://img.shields.io/badge/go-1.27.1%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbvlabs/andurel.svg)](https://pkg.go.dev/github.com/mbvlabs/andurel)
[![Go Report Card](https://goreportcard.com/badge/github.com/mbvlabs/andurel)](https://goreportcard.com/report/github.com/mbvlabs/andurel)
[![codecov](https://codecov.io/gh/mbvlabs/andurel/branch/master/graph/badge.svg)](https://app.codecov.io/gh/mbvlabs/andurel)

---

Everything you and your agent(s) need to build robust and performant applications that will scale to the far-side of the moon.

Andurel is the fullstack framework for the agentic era: one-time generation that writes Go you own, explicit wiring, and an agent-ready CLI.

**Docs:** [andurel.com](https://andurel.com) · **Discord:** [join here](https://discord.gg/TnTBZHvat3)

> **This README tracks `master` (head / nightly).** It may describe features not yet in the latest semver release. Stable documentation matches `go install …@latest` at [andurel.com/docs](https://andurel.com/docs/latest/introduction).

## Platform Support

Andurel supports **Linux** and **macOS** on **amd64** and **arm64**. Official releases contain and smoke-test all four combinations.

See [release verification](docs/release-verification.md) for archive installation, checksums, SBOMs, keyless signatures, and provenance.

## Why Andurel?

Andurel is built for humans and agents on the same pad:

- **Airframe**: Generated models, factories, controllers, routes, and pages that belong to the application
- **GNC**: Fx for dependency injection and lifecycle. Explicit wiring, not a hidden runtime
- **Propellant**: PostgreSQL with narsilc-generated queries into application-owned model structs
- **Payload**: Inertia v3 with React, Vue, or Svelte. Same Go backend, official Inertia adapters, optional SSR. Templ + Datastar remains available for hypermedia pages
- **Range ops**: River jobs, email, and queues on the same stack that serves the request path
- **Ground support**: Agent-ready CLI and JSON discovery so machines can operate the project without a second stack
- **Live reload**: `andurel run` with [Shadowfax](https://github.com/mbvlabs/shadowfax) for Go, templates, CSS, and Vite
- **Production build**: `andurel build` compiles Templ, Tailwind CSS, Vite assets, and the Go binary

**Humans.** One-time generation. The scaffold writes Go and Inertia pages you can read, edit, and ship. After that, the code is yours.

**Agents.** An agent-ready CLI with JSON discovery. Machines get the same pad as humans, without inventing a second stack.

## Core Technologies

- **[Echo](https://echo.labstack.com/)**: HTTP framework
- **[PostgreSQL](https://www.postgresql.org/)** + **[pgx](https://github.com/jackc/pgx)**: Database
- **[narsilc](https://github.com/mbvlabs/narsilc)**: Typed SQL with caller-owned result structs
- **[Templ](https://templ.guide/)** + **[Datastar](https://data-star.dev/)**: Default hypermedia UI
- **[Inertia v3](https://andurel.com/docs/latest/frontend-options)**: Default React / Vue / Svelte + Vite
- **[River](https://riverqueue.com/)**: Background jobs
- **[OpenTelemetry](https://opentelemetry.io/)**: Observability
- **[Tailwind CSS](https://tailwindcss.com/)**: CSS
- **[Shadowfax](https://github.com/mbvlabs/shadowfax)**: Dev runner
- **[Fx](https://uber-go.github.io/fx/)**: Dependency injection

## Install

Match the install channel to the docs you follow.

### Latest stable (semver)

```bash
go install github.com/mbvlabs/andurel@latest
andurel --version
```

### Specific version

```bash
go install github.com/mbvlabs/andurel@v1.2.3
andurel --version
```

### Nightly (matches `master` / this README)

Nightly ships as prebuilt binaries on the [`nightly` GitHub release](https://github.com/mbvlabs/andurel/releases/tag/nightly). Do not use `go install` for nightly; it builds from source without the embedded nightly version metadata.

```bash
# Linux amd64 (also available: andurel-darwin-arm64, andurel-darwin-amd64)
curl -L https://github.com/mbvlabs/andurel/releases/download/nightly/andurel-linux-amd64 -o andurel
chmod +x andurel
mv andurel "$(go env GOBIN)/"
andurel --version
```

Verified release archives (checksums, attestations) are documented in [release verification](docs/release-verification.md).

Full install guide: [andurel.com/docs](https://andurel.com/docs/latest/installation).

## Quick Start

```bash
# Default: PostgreSQL + Inertia React + pnpm
andurel new myapp

# Other UI combinations
andurel new myapp --ui vue/bun
andurel new myapp --ui svelte/npm
andurel new myapp --ui templ/datastar

cd myapp
andurel tool sync
andurel skill install

cp .env.example .env
# Edit .env with your database credentials

# If using Inertia, install JS deps with the package manager from --ui
andurel db create          # optional: if the local database does not exist yet
andurel db migrate up
andurel run
```

App: `http://localhost:8080`

Generate a first resource after writing a migration:

```bash
andurel generate migration create_products_table
# Edit the SQL, then:
andurel generate scaffold Product
```

See what generators can do:

```bash
andurel generate --help
andurel db --help
```

## Project Layout & Options

A new app is ordinary Go you own. Top-level shape:

| Path | Role |
|------|------|
| `cmd/app` | Entry point and Fx wiring |
| `config/` | App, DB, auth, email, telemetry config |
| `controllers/` | HTTP handlers |
| `models/` | Domain types + narsilc queries |
| `router/` | Routes and middleware |
| `views/` | Templ pages for Datastar UI; Inertia root document when using Inertia |
| `resources/js/` | Inertia pages (default UI) |
| `database/` | Migrations and seeds |
| `queue/` | River jobs and workers |
| `css/` / `assets/` | Tailwind source and compiled assets |

**Default**: Inertia React + pnpm (`--ui react/pnpm`).

**`--ui react/pnpm|vue/bun|svelte/npm|templ/datastar`**: Same Go backend; choose Inertia + JS package manager (`pnpm`, `bun`, or `npm`), or Templ + Datastar. Generate follows the project UI; pass `--api` for JSON responses instead.

Details: [directory structure](https://andurel.com/docs/latest/directory-structure) · [frontend options](https://andurel.com/docs/latest/frontend-options).

## Core Commands

| Command | Purpose |
|---------|---------|
| `andurel new` (`n`) | Scaffold a project |
| `andurel generate` | Create model, migration, controller, scaffold, job, email, query |
| `andurel sync` | Refresh derived files (factories, views, queries, …) |
| `andurel db` | Database lifecycle: create, drop, nuke, rebuild, migrate, seed, console |
| `andurel run` (`r`) | Dev server with live reload |
| `andurel build` | Production build (Templ, CSS, Vite, Go binary) |
| `andurel tool sync` | Download pinned project binaries |
| `andurel upgrade` | Bring an app forward to a newer Andurel |
| `andurel doctor` | Project diagnostics |
| `andurel skill` | Show / install the embedded agent skill |
| `andurel inspect` | Read-only project shape (routes, models, …) |
| `andurel commands` | Structured command discovery |

Use `andurel <command> --help` for flags. Full CLI reference: [andurel.com/docs/latest/cli](https://andurel.com/docs/latest/cli).

## Agents

The CLI is meant for humans and machines. Prefer discovery over scraping this README:

```bash
andurel commands --json
andurel inspect project --json
andurel skill install
```

Many mutating commands support `--dry-run` and `--json`. Agent workflows: [andurel.com/docs/latest/agent-workflows](https://andurel.com/docs/latest/agent-workflows).

## Contributing

1. Fork and branch
2. Make changes
3. Run `go fix ./...`, `gofmt -w` on changed Go files, and `go vet ./...`
4. Open a pull request

```bash
git clone https://github.com/mbvlabs/andurel
cd andurel
go mod download
go fix ./...
go vet ./...
```

## License

MIT. See [LICENSE](LICENSE).

## Sites built with Andurel

- [DeployCrate](https://deploycrate.com): Remote boxes for your agents and teams
- [MBV Blog](https://mortenvistisen.com): Personal blog
- [Master Golang](https://mastergolang.com): Course platform
- [Palantir](https://github.com/mbvlabs/palantir): Open sourced analytics platform (WIP)

Built something cool? Let me know or open a PR.

## Author

Created by [Morten Vistisen](https://mortenvistisen.com)

- [Twitter/X](https://x.com/mbvlabs)
- [Mail](mailto:andurel@mbvlabs.com)
