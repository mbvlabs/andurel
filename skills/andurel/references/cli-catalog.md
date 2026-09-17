# Andurel CLI catalog

This file is the agent-oriented command playbook. The live source of truth is command metadata on the `andurel` binary.

Discover commands:

```bash
andurel commands --json
andurel commands --markdown
andurel commands --check
andurel --agent --help
```

## Workflow

1. `andurel inspect project --json` — lockfile, Inertia adapter, tools.
2. `andurel generate … --dry-run --json` — preview application-owned files.
3. `andurel generate … --json` — write application-owned files.
4. `andurel sync … --json` — refresh derived files from their source of truth.
5. `andurel doctor --json` — health and generated-file drift.

`andurel sync` refreshes derived app files. `andurel tool sync` downloads pinned binaries. They are different commands.

## Groups

| Group | Use |
| --- | --- |
| `generate` | Create application-owned code (migrations, models, controllers, scaffolds, jobs, emails, SQL query files) |
| `sync` | Refresh derived files (`views`, `queries`, `routes`, `payloads`, `email`, `factory`, `factories`) |
| `inspect` | Read-only project shape |
| `db` | Database create, migrate, seed, console |
| `tool` | Pin and download binaries |
| `packages` | Standalone `andurel/pkg` versions |
| `skill` | Install the Andurel agent skill |

## Common commands

```bash
andurel generate scaffold Product --dry-run --json
andurel generate migration create_posts
andurel generate model Post --dry-run --json
andurel db migrate up
andurel inspect project --json
andurel inspect routes --json
andurel sync queries --json
andurel sync routes --json
andurel sync payloads --json
andurel sync factories --check --json
andurel doctor --json
andurel run
```

## Create vs refresh vs inspect

- Create SQL: `andurel generate migration NAME` then `andurel db migrate up`.
- Create a model: `andurel generate model NAME`.
- Compile SQL: `andurel sync queries`.
- List models: `andurel inspect models --json`.
- List routes: `andurel inspect routes --json`.
- Write `resources/js/routes.ts`: `andurel sync routes --json` (Inertia only).
- Compile Templ: `andurel sync views`.
- List `.templ` files: `andurel inspect views --json`.

`--json` and `--agent` never wait on a TTY. Pass `--yes`, `--force`, `--harness`, or `--primary-key` instead of answering a prompt.

## Command table

| Route | Summary |
| --- | --- |
| `andurel` | Andurel CLI for humans and agents |
| `andurel build` | Build the application for production |
| `andurel commands` | Show the command catalog |
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
| `andurel db migrate up` | Apply pending SQL migrations |
| `andurel db migrate up-to` | Apply migrations up to a version |
| `andurel db nuke` | Drop and recreate the configured database |
| `andurel db rebuild` | Drop, recreate, migrate, and seed the database |
| `andurel db seed` | Run database seeds |
| `andurel doctor` | Run diagnostic checks on the project |
| `andurel fmt` | Format Go and Templ source files |
| `andurel generate` | Create application-owned code |
| `andurel generate controller` | Generate a controller, views, and routes |
| `andurel generate email` | Author a new email template |
| `andurel generate job` | Generate a background job and worker |
| `andurel generate migration` | Create a new SQL migration file |
| `andurel generate model` | Generate or update a model from a SQL migration |
| `andurel generate query` | Create a narsilc SQL query file |
| `andurel generate scaffold` | Generate a model, controller, views, and routes |
| `andurel inspect` | Read-only project shape |
| `andurel inspect controllers` | List controller files |
| `andurel inspect jobs` | List job and worker files |
| `andurel inspect migrations` | List migration files |
| `andurel inspect models` | List model files |
| `andurel inspect project` | Inspect project metadata |
| `andurel inspect project info` | Show project metadata |
| `andurel inspect routes` | List the route manifest |
| `andurel inspect views` | List Templ view files |
| `andurel new` | Stand up a new Andurel project |
| `andurel packages` | List standalone andurel/pkg versions in go.mod |
| `andurel packages list` | List Andurel package versions in go.mod |
| `andurel packages update` | Update Andurel packages in go.mod to latest |
| `andurel run` | Start the development server |
| `andurel skill` | Show or install the embedded Andurel skill |
| `andurel skill install` | Install the embedded skill into an agent harness |
| `andurel skill show` | Print the embedded Andurel skill |
| `andurel sync` | Refresh derived files from a source of truth |
| `andurel sync email` | Compile Tailwind classes in email templates |
| `andurel sync factories` | Check or sync every model factory |
| `andurel sync factory` | Sync one model factory from the model Entity |
| `andurel sync payloads` | Write TypeScript payload types for Inertia |
| `andurel sync queries` | Compile narsilc SQL into Go |
| `andurel sync routes` | Write TypeScript route helpers for Inertia |
| `andurel sync views` | Generate Go from Templ templates |
| `andurel tool` | Manage project tools and binaries |
| `andurel tool dblab` | Open the dblab database UI |
| `andurel tool list` | List project tool status |
| `andurel tool mailpit` | Start Mailpit for local email capture |
| `andurel tool set-version` | Pin a tool version in andurel.lock and sync it |
| `andurel tool sync` | Download pinned binaries from andurel.lock |
| `andurel upgrade` | Upgrade framework-owned files to this CLI version |

