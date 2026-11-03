# Agent-first Andurel

Andurel is optimized for coding agents. That is a constraint on the framework
and CLI, not a requirement on how you work. Every command remains usable by a
person; the application embeds no agent or hosted AI service.

## What the optimization means

- **One local, machine-readable interface.** The `andurel` binary handles
  discovery, inspection, patterns, sync, verification, and builds;
  `andurel commands --json` is the authoritative catalog.
- **Read-only patterns.** Application code is ordinary Go written from
  `andurel patterns`, which prints code, rules, and a checklist and never
  writes files.
- **Explicit ownership.** After scaffolding, generators write only derived
  artifacts; upgrades never rewrite application source.
- **Actionable verification.** `andurel verify` reports each violation with its
  rule, file, line, message, and fix. The agent applies the fix.

## The working loop

1. **Discover.** Use `andurel commands --json` and `andurel inspect project --json`.
2. **Plan and place.** Read the nearest existing resource and the matching
   pattern, then choose the layer that owns the behavior.
3. **Edit.** Make the smallest ordinary-Go change.
4. **Sync.** Refresh only the derived artifacts affected by that change.
5. **Verify.** Run scoped checks, then the full project gate.
6. **Prove.** Run tests and a build. Verification is not proof of business
   correctness.

Inspect narrowly by default. `inspect handlers --json` or `inspect domains
--json` returns the relevant manifest; open the files it names.

## A focused workflow

A domain starts from the Go type and its SQL, not a CRUD generator:

```bash
andurel inspect domains --json
andurel patterns show domain
andurel patterns show migration

# Write domains/product.go and domains/product.sql.
andurel generate migration create_products
andurel sync queries --dry-run --json
andurel sync queries --json
andurel verify ./domains/...
```

`sync queries --dry-run` is a preview, not a lock. Query generation can update
shared `go.mod` and `go.sum`; review and coordinate those changes.

For HTTP, run `andurel inspect handlers --json`, read `patterns show handler` and
`patterns show route`, edit the handler, route, and `app/` wiring, then run
`andurel sync routes --json`, `andurel sync payloads --json`, and `andurel
verify --json`.

Scoped `verify` still runs the whole-project frontend typecheck. Use
`--no-frontend` only when you deliberately skip it, or `--only` to select stages
when generated drift cannot run in a scoped invocation.

## Output and diagnostics

`andurel commands --json` is the source of truth for flags and capabilities.
Global flags are `--json`, `--agent`, `--md`, `--jq`, `--quiet`, and
`--verbose`. Success uses `{ok, data, summary, breadcrumbs}`; failure uses
`{ok: false, code, error, hint, exit_code}`. `--jq` accepts a simple field path
such as `.routes[0].name`, not full jq.

A failed `andurel verify` with `--json` or `--agent` writes its report to stderr
and leaves stdout empty; each violation includes the rule, location, message,
and fix. `andurel doctor --json` is a read-only health and drift check.

`andurel inspect config --json` reports environment keys, defaults, and source
lines from `config/*.go`; it does not read `.env` values. Keep credentials out
of context output.

## Layers and patterns

Read `andurel patterns show <kind>` before adding a primitive.

| Layer | Owns |
| --- | --- |
| `domains/` | Entities, validation, persistence receivers, and adjacent SQL |
| `services/` | Cross-domain orchestration |
| `handlers/` | Thin HTTP adapters |
| `routes/` | Route names, paths, hosts, and URL builders |
| `middleware/` | Application HTTP policy |
| `app/` | Explicit constructors and process composition |
| `payloads/` | Inertia page props and request bodies |
| `queue/` | River job arguments, workers, and registration |
| `emails/` | Go email structs, React Email sources, and templates |

Only `domains/**` imports `domains/internal/queries`. SQL methods and drivers
stay out of handlers, services, and app code; domain packages import no HTTP or
Inertia package; transaction callbacks use dependencies bound inside the
transaction. See
[the layer-placement reference](../skills/andurel/references/layer-placement.md).

## Generated files

These committed artifacts are owned by their generators:

- `domains/internal/queries/` from `andurel sync queries`
- `resources/js/routes.ts` from `andurel sync routes`
- `resources/js/types/payloads.ts` from `andurel sync payloads`
- `emails/generated/` and `emails/templates/` from `andurel sync emails`

Edit the source of truth, run the matching `sync`, and commit the result. Do not
hand-edit generated output. `andurel generate migration` is the only `generate`
command that writes into the application; `andurel new` creates the reference
application. See [generated files and upgrade behavior](generated-files-and-upgrades.md).

## Skills and harnesses

```bash
andurel skill install --harness codex
andurel skill install --harness claude
andurel skill install --harness pi
andurel skill install --harness opencode
andurel skill install --harness crush
```

The flag accepts comma-separated or repeated names (for example `claude,pi`);
`andurel new myapp --harness claude` installs it during scaffolding. Read
references with `andurel skill show layer-placement`.

## Verification and tests

`andurel verify` runs formatting, `go fix`, `go vet`, the typed layer rules, the
frontend typecheck, and generated-drift checks. It does not certify business
behavior, performance, or production readiness. Use `andurel verify
--with-tests` to run the suite; skipped packages and `t.Skip` calls are reported
and are not a pass. Tests remain the application's responsibility.
`andurel build` compiles every process and asset set for production.

## Further reading

- [Andurel 2.0 release candidate](release-notes/v2.0.0-rc.1.md)
- [Generated files and upgrade behavior](generated-files-and-upgrades.md)
- [CLI reference](https://andurel.com/docs/latest/cli)
- [Installation guide](https://andurel.com/docs/latest/installation)
