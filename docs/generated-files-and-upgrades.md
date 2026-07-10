# Generated files and upgrade behavior

Andurel distinguishes framework-owned files from application files. Normal upgrades own framework internals, currently centered on `internal/*`. Controllers, models, views, jobs, routes, migrations, application entrypoints, configuration, and other application code remain user-owned unless a command explicitly documents a narrower generated declaration boundary.

Factory synchronization is one such narrow boundary. It regenerates the Andurel-owned factory types, `Build<Name>`, `Create<Name>`, `Create<Name>s`, and generated `WithX` option functions. Custom helpers whose names do not collide with generated declarations are preserved.

## Release-candidate corrections

The v1 upgrader contains explicit migrations for known RC.1, RC.2, and RC.3 scaffold defects outside the normal ownership boundary. Each correction must match an exact known file or a unique expected syntax structure. Andurel refuses missing, duplicated, ambiguous, or unrecognized structures instead of guessing.

These migrations do not expand permanent ownership of application files. Later upgrades cannot overwrite the same files merely because an RC migration once inspected them.

## Planning and conflicts

Run a structured dry run before applying an upgrade:

```bash
andurel upgrade --dry-run --diff --json
```

The response reports replacements, deletions, lock migrations, tool metadata changes, conflicts, unified diffs, and dirty-worktree state. Dry-run is read-only and deterministic, including on a dirty worktree.

A conflict stops a real upgrade before any file is written. Review `data.upgrade.Conflicts`, resolve the application edit deliberately, and run the dry run again. Do not treat a conflict as permission to delete or overwrite user code.

## Transaction guarantees

A real upgrade plans and renders the complete result before writing. It validates staged content, creates recoverable backups, replaces files atomically where the filesystem permits, and writes the final lock version last. Any replacement or validation failure rolls every changed file and `andurel.lock` back to their byte-identical originals.

Real upgrades require a clean Git worktree. Re-running a successful upgrade is idempotent. See the [RC-to-v1 migration guide](migration-rc-to-v1.md) for the supported operator procedure.
