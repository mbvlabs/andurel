# Generated files and upgrade behavior

Andurel distinguishes framework-owned files from application files. Normal upgrades own framework internals, currently centered on `internal/*`. Controllers, models, views, jobs, routes, migrations, application entrypoints, configuration, and other application code remain user-owned unless a command explicitly documents a narrower generated declaration boundary.

Factory synchronization is one such narrow boundary. It regenerates the Andurel-owned factory types, `Build<Name>`, `Create<Name>`, `Create<Name>s`, and generated `WithX` option functions. Custom helpers whose names do not collide with generated declarations are preserved.

The Inertia root embedding migration is another narrow boundary. It moves `views/root.go.html` to `assets/inertia/root.go.html` and replaces only the exact legacy `inertia.Init("views/root.go.html")` call in `cmd/app/main.go`. The upgrade stops without writing if that call is not present.

## Manual session-cookie recovery migration

The `router/*` tree is application-owned, so `andurel upgrade` does not install session-cookie decode recovery into an existing project. New projects include the recovery automatically. Existing projects should reconcile only these declarations against a fresh scaffold created with the target Andurel version:

When an upgrade crosses from a version before `v1.5.4` to `v1.5.4` or later, `andurel upgrade` prints this complete migration with the project's module path already rendered. The same manual action is included in dry-run and structured JSON output. Projects starting on `v1.5.4` or later do not receive the note.

1. Add `router/cookies/session.go` from [`router_cookies_session.tmpl`](../layout/templates/router_cookies_session.tmpl), replacing the template module import with the project's module path.
2. In `router/cookies/cookies.go` and `router/cookies/flash.go`, replace calls to Echo's `session.Get` with the shared `getSession` helper and remove the now-unused Echo session imports.
3. In `router/middleware/middleware.go`, call `cookies.RecoverInvalidSessions(c)` inside `ValidateSession`, after the assets and API bypass and before calling the next handler.
4. Add `github.com/gorilla/securecookie v1.1.2` as a direct dependency, then run `gofmt`, `go fix ./...`, and `go vet ./...`.

Do not replace the whole `router/*` tree. These changes preserve valid sessions, replace only cookies that fail secure-cookie decoding, and continue to return configuration, usage, internal, or save errors.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final lock version last. Any replacement or validation failure rolls every changed file and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The automated upgrader requires an explicit `schemaVersion` and does not accept release-candidate locks. Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 must use the [RC-to-v1 manual upgrade guide](upgrade-rc-base-scaffold-prompt.md).
