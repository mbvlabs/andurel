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

1. `andurel inspect project --json` — lockfile, Inertia adapter, tools.
2. `andurel patterns show <kind>` — read the reference shape for a layer.
3. `andurel generate migration NAME` — create a migration (the only generated file).
4. Write domains, services, handlers, routes, and middleware by hand.
5. `andurel sync queries` / `sync routes` / `sync payloads` / `sync email` — refresh derived files.
6. `andurel verify` — formatting, `go fix`, `go vet`, layer boundaries, and generated-code drift.
7. `andurel doctor --json` — health and generated-file drift.

`andurel verify` reports each step as `ok`, `fail`, or `skipped`. A missing
`templ` or `narsilc` binary fails the `generated` step with the `missing-tool`
rule; run `andurel tool sync` to install it. `--skip-generated` marks that step
`skipped` and does not fail the run.

The typed layer checks load the active build target, like `go vet`: files
excluded by GOOS, GOARCH, or build constraints are not analyzed by that step.

`andurel sync` refreshes derived app files. `andurel tool sync` downloads pinned
binaries. They are different commands.

## Patterns

`andurel patterns` never writes files. It prints reference code, the placement
rules, and a "definition of done" checklist you self-verify against.

Pattern ids: `domain`, `service`, `handler`, `route`, `job`, `email`,
`migration`, `test-domain`, `test-handler`, `test-integration`, `composition`.

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
andurel generate migration create_posts
andurel db migrate up
andurel inspect project --json
andurel inspect domains --json
andurel inspect handlers --json
andurel inspect routes --json
andurel sync queries --json
andurel sync routes --json
andurel sync payloads --json
andurel verify
andurel doctor --json
andurel run
andurel run --tools mailpit
```

## Create vs refresh vs inspect

- Read a pattern: `andurel patterns show <kind>` (add `--json` for the parsed form; add `--entity/--table` with `domain` for the per-entity file manifest).
- Create SQL: `andurel generate migration NAME` then `andurel db migrate up`. A one-shot process that applies embedded SQL is `cmd/migrate`.
- Add a domain: hand-write `domains/<entity>.go` and its colocated `domains/<entity>.sql`, construct the receiver with `domains.NewX(db)` in `internal/runtime`, then run `andurel sync queries`.
- Add a service, handler, route, middleware, or composition: read the matching pattern and write the files yourself. `andurel new` ships a complete reference app for each.
- Compile SQL: `andurel sync queries` writes `domains/internal/queries`. Only `domains/**` may import it.
- List domains: `andurel inspect domains --json`.
- List handlers: `andurel inspect handlers --json`.
- List routes: `andurel inspect routes --json` (includes host when set).
- Write `resources/js/routes.ts`: `andurel sync routes --json` (Inertia only; helpers are host-aware full URLs).
- Bind a resource to a named host: declare `routing.HostName` in `config/hosts.go`, load `HOST_*` into `App.Hosts`, then bind routes with `rt.Host(name)`.
- Refresh derived artifacts: `andurel sync email` compiles email templates.

`--json` and `--agent` never wait on a TTY. Pass `--force`, `--harness`, or `--yes` instead of answering a prompt.

## Command table

| Route | Summary |
| --- | --- |
| `andurel` | Andurel CLI for humans and agents |
| `andurel build` | Build the application for production |
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
| `andurel inspect domains` | List domain files |
| `andurel inspect handlers` | List handler files |
| `andurel inspect jobs` | List job and worker files |
| `andurel inspect migrations` | List migration files |
| `andurel inspect project` | Inspect project metadata |
| `andurel inspect project info` | Show project metadata |
| `andurel inspect routes` | List the route manifest |
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
| `andurel skill show` | Print the embedded Andurel skill |
| `andurel sync` | Refresh derived files from a source of truth |
| `andurel sync email` | Compile Tailwind classes in email templates |
| `andurel sync payloads` | Write TypeScript payload types for Inertia |
| `andurel sync queries` | Compile narsilc SQL into Go in `domains/internal/queries` |
| `andurel sync routes` | Write TypeScript route helpers for Inertia |
| `andurel tool` | Manage project tools and binaries |
| `andurel tool dblab` | Open the dblab database UI |
| `andurel tool list` | List project tool status |
| `andurel tool mailpit` | Start Mailpit for local email capture |
| `andurel tool set-version` | Pin a tool version in andurel.lock and sync it |
| `andurel tool sync` | Download pinned binaries from andurel.lock |
| `andurel upgrade` | Upgrade framework-owned files to this CLI version |
| `andurel verify` | Check formatting, vet, layer boundaries, and generated code |
