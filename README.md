<img width="2256" height="408" alt="andurel-wordmark-dark" src="https://github.com/user-attachments/assets/fe351b07-15e8-41e1-9be1-23feb894acf5" />

# Andurel, Space-grade Go framework for humans and agents

[![Go Version](https://img.shields.io/badge/go-1.27.3%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbvlabs/andurel/v2.svg)](https://pkg.go.dev/github.com/mbvlabs/andurel/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/mbvlabs/andurel)](https://goreportcard.com/report/github.com/mbvlabs/andurel)
[![codecov](https://codecov.io/gh/mbvlabs/andurel/branch/master/graph/badge.svg)](https://app.codecov.io/gh/mbvlabs/andurel)

---

Everything you and your agent(s) need to build robust and performant applications that will scale to the far-side of the moon.

Andurel is the fullstack framework for the agentic era: one-time generation that writes Go you own, explicit wiring, and an agent-ready CLI. It is Go-native — `net/http`, no HTTP framework dialect, no dependency-injection container.

**Docs:** [andurel.com](https://andurel.com) · **Discord:** [join here](https://discord.gg/TnTBZHvat3)

> **This README tracks `master` (head / nightly).** It may describe features not yet in the latest semver release. Stable documentation matches `go install …@latest` at [andurel.com/docs](https://andurel.com/docs/latest/introduction).

## Platform Support

Andurel supports **Linux** and **macOS** on **amd64** and **arm64**. Official releases contain and smoke-test all four combinations.

See [release verification](docs/release-verification.md) for archive installation, checksums, SBOMs, keyless signatures, and provenance.

## Why Andurel?

Andurel is built for humans and agents on the same pad:

- **Airframe**: `domains/` (business logic + persistence), `services/` (application logic), `handlers/` (HTTP), `routes/`, and `middleware/` — all ordinary Go you own
- **GNC**: Explicit constructor injection in `internal/runtime`. The dependency graph is readable top-to-bottom; there is no container and no reflection
- **Propellant**: PostgreSQL with narsilc-generated queries from SQL colocated with the domain that owns it
- **Payload**: Inertia v3 with React, Vue, or Svelte. One Go backend, official Inertia adapters, optional SSR. Templ survives only for email and the Inertia root document
- **Range ops**: River jobs, email, and queues on the same stack that serves the request path
- **Ground support**: Agent-ready CLI with read-only patterns, JSON discovery, and AGENTS.md so machines can operate the project without a second stack
- **Live reload**: `andurel run` with [Shadowfax](https://github.com/mbvlabs/shadowfax) for Go, templates, CSS, and Vite (`--tools mailpit` for sidecars)
- **Production build**: `andurel build` compiles Templ, email Tailwind, Vite assets, and every `cmd/*` process (`app`, `queue`, `migrate`, `ssr`) into `dist/`

**Humans.** One-time generation. The scaffold writes Go and Inertia pages you can read, edit, and ship. After that, the code is yours, laid out in a shape you can navigate.

**Agents.** `andurel patterns show <kind>` publishes the reference code and a definition-of-done checklist for every layer. `andurel verify` proves a change respects the layer boundaries. Machines get the same pad as humans, without inventing a second stack.

## Core Technologies

- **[net/http](https://pkg.go.dev/net/http)** + **[`pkg/anduin`](https://pkg.go.dev/github.com/mbvlabs/andurel/pkg/anduin)**: The single HTTP package — routing, `func(ctx) error` handlers, middleware, request/response helpers, and the server lifecycle over `http.ServeMux`
- **[PostgreSQL](https://www.postgresql.org/)** + **[pgx](https://github.com/jackc/pgx)**: Database
- **[narsilc](https://github.com/mbvlabs/narsilc)**: Typed SQL with caller-owned result structs
- **[Inertia v3](https://andurel.com/docs/latest/frontend-options)**: React / Vue / Svelte + Vite
- **[templ](https://templ.guide/)**: Email templates and the Inertia root document only
- **[River](https://riverqueue.com/)**: Background jobs
- **[OpenTelemetry](https://opentelemetry.io/)**: Observability
- **[Tailwind CSS](https://tailwindcss.com/)**: CSS
- **[Shadowfax](https://github.com/mbvlabs/shadowfax)**: Dev runner

## Install

Match the install channel to the docs you follow.

### Latest v2 pre-release

V2 has not shipped a stable tag yet. Install the latest beta:

```bash
go install github.com/mbvlabs/andurel/v2@v2.0.0-beta.4
andurel --version
```

### Specific version

Pin any published tag, for example the previous pre-release:

```bash
go install github.com/mbvlabs/andurel/v2@v2.0.0-alpha.5
andurel --version
```

v1 releases stay on the unversioned module path (`go install github.com/mbvlabs/andurel@v1.x.y`). That path's `@latest` never jumps to v2.

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

# Other UI combinations (Inertia adapter + JS package manager)
andurel new myapp --ui vue/bun
andurel new myapp --ui svelte/npm

cd myapp
andurel tool sync
andurel skill install

cp .env.example .env
# Edit .env with your database credentials

andurel db create          # optional: if the local database does not exist yet
andurel db migrate up
andurel run
```

App: `http://localhost:8080`

Build a first resource. The generator creates migrations; the agent writes the code from the patterns:

```bash
andurel generate migration create_products_table
# Edit the SQL in migrations/, then apply it:
andurel db migrate up

# Read the pattern for each layer, then write the code yourself:
andurel patterns show domain
andurel patterns show handler
andurel patterns show route

andurel sync queries
andurel verify
```

Explore what the CLI knows:

```bash
andurel patterns list
andurel patterns --help
andurel db --help
```

## Project Layout & Options

A new app is ordinary Go you own. Top-level shape:

| Path | Role |
|------|------|
| `cmd/app`, `cmd/queue` | Process entry: load env, signal context, single `runtime.New` / `runtime.NewQueue` (the App owns its Server) |
| `cmd/migrate` | One-shot process: apply pending SQL via `storage.RunMigrations` |
| `cmd/seeds` | One-shot process: run named seed sets |
| `internal/runtime` | Composition root: explicit constructors, HTTP server + queue lifecycle |
| `internal/…` | Other app support code, such as third-party adapters (for example `internal/clients`, `internal/payment`) |
| `config/` | App, DB, auth, email, telemetry config |
| `domains/` | Business logic + persistence in one flat package; SQL colocated as `domains/<entity>.sql` |
| `domains/internal/queries/` | narsilc output; imported only by `domains/**` |
| `services/` | Application logic / cross-domain orchestration |
| `handlers/` | HTTP handlers (`func(ctx context.Context) error`); one flat package |
| `routes/` | Route data: names, paths, URL builders |
| `middleware/` | App HTTP middleware (auth, authorization); the cookie jar is built in `internal/runtime` |
| `resources/root.templ` | Inertia root document (the only page-shaped templ) |
| `resources/js/` | Inertia pages, layouts, components, and generated TS helpers |
| `email/` | templ email templates and send helpers |
| `queue/` | River jobs and workers |
| `migrations/` | SQL migrations |
| `seeds/` | Seed data |
| `css/` / `assets/` | Tailwind source and compiled/static assets |

**Default**: Inertia React + pnpm (`--ui react/pnpm`).

**`--ui react/pnpm|vue/bun|svelte/npm`**: Same Go backend; choose the Inertia adapter and JavaScript package manager. Inertia is the only UI; there are no templ/Datastar pages. JSON endpoints are ordinary routes that return `anduin.JSON`; there is no separate API mode.

Details: [directory structure](https://andurel.com/docs/latest/directory-structure) · [frontend options](https://andurel.com/docs/latest/frontend-options).

## Core Commands

| Command | Purpose |
|---------|---------|
| `andurel new` (`n`) | Scaffold a project |
| `andurel patterns` | Read-only reference patterns for every layer (`patterns list`, `patterns show <kind> --json`) |
| `andurel verify` | Format, `go fix`, `go vet`, the typed layer rules, and generated-code drift; `--with-tests` also runs tests. Rules: `andurel verify --help` or [the rule table](skills/andurel/references/cli-catalog.md#verify-rules) |
| `andurel generate migration` | Create a migration file (code-writing generators are gone) |
| `andurel sync` | Refresh derived files (queries, email, routes, payloads) |
| `andurel db` | Database lifecycle: create, drop, nuke, rebuild, migrate, seed, console |
| `andurel console` (`c`) | Open an interactive database console |
| `andurel run` (`r`) | Dev server with live reload |
| `andurel build` | Production build (Templ, email Tailwind, Vite, all `cmd/*` processes in `dist/`) |
| `andurel tool sync` | Download pinned project binaries |
| `andurel upgrade` | Bring an app forward to a newer Andurel |
| `andurel doctor` | Project diagnostics |
| `andurel skill` | Show / install the embedded agent skill |
| `andurel inspect` | Read-only project shape (`handlers`, `domains`, `routes`, …) |
| `andurel commands` | Structured command discovery |

Use `andurel <command> --help` for flags. Full CLI reference: [andurel.com/docs/latest/cli](https://andurel.com/docs/latest/cli).

## Agents

The CLI is meant for humans and machines. Prefer discovery over scraping this README:

```bash
andurel patterns show domain --json
andurel verify
andurel inspect handlers --json
andurel inspect domains --json
andurel inspect routes --json
andurel commands --json
andurel skill install
andurel skill show layer-placement
```

`andurel patterns` never writes files. It prints reference code, the placement rules, and a definition-of-done checklist the agent self-verifies against. Many commands support `--dry-run` and `--json`. Agent workflows: [andurel.com/docs/latest/agent-workflows](https://andurel.com/docs/latest/agent-workflows).

## Contributing

1. Fork and branch
2. Make changes
3. Run `just check-fmt`, `just check-fix`, and `just vet` (or `gofmt` / `go fix` / `go vet` with `GOWORK=off`)
4. Open a pull request

`just` recipes and CI set `GOWORK=off` so the CLI resolves `pkg/*` from `go.mod`, matching `go install`. The workspace file is for local ad-hoc work only.

```bash
git clone https://github.com/mbvlabs/andurel
cd andurel
GOWORK=off go mod download
just vet
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
