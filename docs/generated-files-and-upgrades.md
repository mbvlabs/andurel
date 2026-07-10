# Generated files and upgrade behavior

Andurel distinguishes framework-owned files from application files. Normal upgrades own framework internals, currently centered on `internal/*`. Controllers, models, views, jobs, routes, migrations, application entrypoints, configuration, and other application code remain user-owned unless a command explicitly documents a narrower generated declaration boundary.

Factory synchronization is one such narrow boundary. It regenerates the Andurel-owned factory types, `Build<Name>`, `Create<Name>`, `Create<Name>s`, and generated `WithX` option functions. Custom helpers whose names do not collide with generated declarations are preserved.

## Planning and preview

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, tool metadata changes, unified diffs, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final lock version last. Any replacement or validation failure rolls every changed file and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. The automated upgrader requires an explicit `schemaVersion` and does not accept release-candidate locks. Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 must use the [RC-to-v1 manual upgrade guide](upgrade-rc-base-scaffold-prompt.md).
