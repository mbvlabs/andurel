# Prompt: align an RC.2 or RC.3 base scaffold with the installed stable Andurel

Copy the prompt below into a coding agent that is running at the root of the Andurel application to upgrade. The target is the exact stable v1 release reported by the currently installed Andurel CLI.

```text
You are upgrading an existing Andurel application whose base scaffold was created with either v1.0.0-rc.2 or v1.0.0-rc.3. Bring its base scaffold behavior up to the exact stable v1 release reported by the currently installed Andurel CLI.

This is an implementation task. Inspect the application, make the required changes, validate them, and report the result. Do not stop after producing an audit or a plan.

Target selection

- Run `andurel --version`, record its full output, and extract the stable semantic version token into `TARGET_VERSION`.
- Require `TARGET_VERSION` to match `^v1\.[0-9]+\.[0-9]+$`. Reject `dev`, a release candidate, `latest`, `master`, or any other moving or non-version reference.
- Use the currently installed CLI only when its reported version exactly matches `TARGET_VERSION`. If an isolated binary is needed, install the exact `TARGET_VERSION` tag into a temporary `GOBIN` and verify its version before use.
- If no stable v1 CLI is currently installed or its exact tag is unavailable, stop and report that a valid target release is unavailable.
- The recognized source tags are v1.0.0-rc.2 at commit `5d78288` and v1.0.0-rc.3 at commit `70deaaa`.

Non-negotiable rules

1. Read and obey every applicable AGENTS.md or repository instruction before changing anything.
2. Preserve application behavior and user-authored code. A fresh target scaffold is a reference, not permission to replace the application wholesale.
3. Never overwrite controllers, models, services, routes, views, configuration, jobs, migrations, or entrypoints without reconciling local changes declaration by declaration.
4. Treat `internal/*` as framework-owned only when the fresh target scaffold contains the file and repository history shows that the application has not customized its behavior.
5. Do not edit generated Templ Go output by hand. Change the source `.templ` file and regenerate it with the project command.
6. Do not rotate or regenerate existing application secrets. Add missing configuration keys while preserving the current values.
7. Do not change database migrations or production data merely to resemble a fresh scaffold.
8. Do not commit, push, or open a pull request unless explicitly asked.
9. If the source version is neither rc.2 nor rc.3, stop and report that this prompt does not cover it.
10. If a technical choice would discard a local customization or has more than one valid product outcome, ask the user before choosing.
11. Upgrade production code and configuration only. Do not add, modify, or port test files.
12. Do not run `andurel upgrade`, including its dry-run mode. Reconcile the application directly against the fresh target scaffold.

Phase 1: establish provenance and a safe baseline

1. Inspect `git status --short`, `andurel.lock`, `go.mod`, the enabled extensions, the Inertia adapter, and the JavaScript runtime.
2. Read `andurel.lock.version` before changing it and record whether the source is rc.2 or rc.3.
3. Record all existing worktree changes before editing. Do not hide, stash, reset, or delete user changes.
4. Use the installed CLI that supplied `TARGET_VERSION`. If isolation is required, install that exact tag into a temporary `GOBIN`; never resolve a moving reference.
5. Inspect repository history for every base-scaffold file that differs from the target so generated code can be distinguished from application customization.

Phase 2: create the authoritative comparison scaffold

Create a fresh target-version project under a temporary directory, outside the application and outside any existing Andurel project. Use the same values from `andurel.lock.scaffoldConfig` and `andurel.lock.extensions`:

- the same project name
- PostgreSQL
- the same extension names
- the same Inertia adapter, if any
- the same JavaScript runtime, if any

Use the exact target CLI to run `andurel new`. Do not sync tools, create a database, or start the reference application. Keep the reference directory only long enough to compare files.

Normalize these expected sources of noise before judging a difference:

- module and project names
- generated secret values
- framework version strings in generated headers and `andurel.lock`
- extension timestamps
- generated or environment-specific absolute paths

The fresh target scaffold and its `andurel.lock` are the source of truth for the base scaffold. Do not rely on memory or infer target code from this checklist when an exact target file is available.

Phase 3: reconcile framework-owned files directly

Use the fresh target scaffold as the exact source for framework-owned changes. Apply edits directly and declaration by declaration. Do not run `andurel upgrade`.

The direct reconciliation must:

- migrate the lock to `schemaVersion: 1`
- update verified tool download metadata and target tool versions
- replace uncustomized framework-owned `internal/*` behavior with the target implementation
- reconcile target functions in `models/user.go`, `router/router.go`, and `cmd/app/main.go`
- preserve independent user declarations in those application files
- preserve or manually reapply application additions in customized framework-like files
- stop and ask before resolving ambiguous edits

Continue with the application-owned comparison below after the framework-owned files and lock metadata are reconciled.

Phase 4: reconcile the application-owned base scaffold

Diff the application against the fresh target scaffold. Focus on files that came from the base scaffold, but classify every difference before editing it:

- Functional or security correction: port it while preserving local behavior.
- Required target configuration or generated metadata: port it.
- New regression coverage for a ported behavior: skip it.
- Cosmetic base-scaffold change: preserve the application's intentional design unless the user explicitly wants the target scaffold's look.
- Application customization: preserve it.
- Feature absent because its extension or adapter is disabled: ignore it.

At minimum, verify and reconcile every checkpoint below.

A. Process lifecycle and HTTP server

- `cmd/app/main.go` starts the queue processor and HTTP server with the application context.
- Background components expose completion, are stopped once, and are awaited during shutdown.
- Shutdown errors from multiple components are joined instead of losing all but the first.
- `internal/server/server.go` leaves lifecycle orchestration to the application and uses the hardened server defaults from the target scaffold.

B. Session, CORS, CSRF, request paths, and rate limiting

- `config/app.go` includes `SessionMaxAge` and `CORSAllowedOrigins` with the target defaults and parsing behavior.
- `.env.example` documents `SESSION_MAX_AGE=604800` and `CORS_ALLOWED_ORIGINS=` without changing real secrets or deployment values.
- `router/router.go` configures the application session cookie with `Path=/`, `MaxAge`, `HttpOnly`, `SameSite=Lax`, and `Secure` in production.
- Credentialed CORS trusts the configured application origin plus explicit additional exact origins. Wildcards are rejected.
- API and asset detection matches path-segment boundaries. Paths such as `/apiary` are not treated as `/api`.
- Unsafe API requests bypass CSRF only when they have a non-empty Bearer token and no application session cookie. Cookie-authenticated API requests remain CSRF-protected.
- The IP rate limiter updates each IP count atomically and permits exactly the configured limit under concurrency.
- Preserve the target middleware order, including panic recovery placement.

C. Authentication secrets and pepper rotation

- `config/auth.go` and `.env.example` support `PREVIOUS_PEPPERS` as a comma-separated list.
- `services.Identity` keeps password peppering separate from token signing.
- Email verification and password-reset tokens use `TOKEN_SIGNING_KEY`, not the password pepper.
- Authentication tries the current pepper first, then configured previous peppers. A password accepted with an old pepper is rehashed and persisted with the current pepper.
- Empty entries and entries equal to the current pepper are filtered consistently with the target scaffold.
- Preserve deployed `PEPPER` and `TOKEN_SIGNING_KEY` values. Never silently swap or regenerate them.

D. Model correctness

- `models/user.go` preserves `CreatedAt` during updates, updates only intended columns, uses the primary key, returns the stored row, reports a missing row, and performs an actual delete in `Destroy`.
- User pagination and token pagination use Bun's `Count` behavior and retain the scaffold's public result types.
- `models/token.go` consistently treats the hashing input as the token signing secret.
- Reconcile local model fields and validation. Do not replace customized entities with the smaller reference entity.

E. Lock file, tools, Go metadata, and dependencies

- `andurel.lock` has `schemaVersion: 1`, the selected target framework version, current verified SHA-256 metadata for supported tool platforms, version-check metadata, and the expected tools for the enabled scaffold configuration.
- Preserve independent custom tool entries unless they conflict with a framework-owned tool name.
- Synchronize tools with `andurel tool sync` after reviewing the lock diff.
- Align the `go` directive and direct dependencies with the target scaffold when the application has not intentionally selected a newer compatible version.
- Reconcile dependency changes through normal Go module tooling. Do not copy an unrelated reference `go.sum` wholesale.

F. Production code only

Do not add or adapt the target scaffold's base regression test files. Existing tests may be run when repository instructions permit, but test source files must remain unchanged.

G. Inertia projects only

- Preserve or generate `resources/js/routes.ts` from the application's actual Go routes.
- Inertia auth and error pages use the configured `@/` import alias for layouts and route helpers.
- The Vite asset route and `controllers/assets.go` agree on the wildcard path and cache-key segment. Verify both development and built-asset paths conceptually against the target scaffold.
- Keep the target TypeScript compiler alias configuration.
- Preserve product-specific components, styling, and page composition.

H. Additional rc.2-only gap review

If the source was rc.2, compare directly from rc.2 to the target. Do not stop after making it resemble rc.3. In particular, verify:

- River periodic jobs are injected through the expected Fx group and passed to the queue client.
- typed Go route helpers and, for Inertia, TypeScript route helpers replace duplicated hard-coded scaffold URLs
- the Vite build asset route uses the target cache-key wildcard shape, and the controller does not retain the rc.3-only extra wildcard-segment stripping
- rc.3-era scaffold UI and branding changes are treated as cosmetic, not as justification to overwrite an application's design

Phase 5: regenerate and validate

1. Run the repository's required formatters and generators on changed source files. At minimum, when permitted by repository instructions:

   gofmt -w <changed Go files>
   go fix ./...
   andurel generate view
   andurel generate routes     # Inertia only

2. Run:

   andurel tool sync
   andurel doctor --verbose
   go vet ./...

3. Format changed production source files directly. Do not run a repo-wide formatter when it would modify test files or unrelated application code.
4. Run any additional validation allowed and required by the repository instructions. Never run a command that its AGENTS.md forbids.
5. Review `git diff` and `git status --short`.
6. Repeat the direct comparison against the fresh target scaffold and classify every remaining production-code difference.
7. Confirm that no real `.env` secret, database migration, application route, local feature, intentional UI customization, or test source file changed accidentally.

Completion report

Return a concise report containing:

- detected source version and selected target version
- reference scaffold configuration used for comparison
- framework-owned direct reconciliation result
- functional base-scaffold corrections ported manually
- application customizations deliberately preserved
- cosmetic reference differences deliberately skipped
- files added, changed, or removed
- validation commands and results
- remaining blockers or user decisions, if any

Do not claim completion if required target behavior is missing, validation fails, or a material difference remains unclassified.
```
