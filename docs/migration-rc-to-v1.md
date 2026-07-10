# Migrating an RC project to Andurel v1

The v1 upgrader recognizes projects created by v1.0.0-rc.1, v1.0.0-rc.2, and v1.0.0-rc.3. It migrates a missing RC lock schema to schema 1, applies only recognized scaffold corrections, updates framework-owned files and tool metadata, and preserves application-owned code.

## 1. Prepare the project

Commit the entire project or create a backup, then confirm that Git is clean:

```bash
git status --short
```

Install the stable CLI after v1 is released:

```bash
go install github.com/mbvlabs/andurel@v1.0.0
andurel --version
```

During the RC.4 validation period, use `@v1.0.0-rc.4` instead. Archive installations must be verified using the [release verification guide](release-verification.md).

## 2. Preview the complete migration

```bash
andurel upgrade --dry-run --diff --json
```

Normal structured output keeps the v1 success envelope. Inspect `data.upgrade.ReplacedFiles`, `data.upgrade.RemovedFiles`, `data.upgrade.LockMigrations`, `data.upgrade.ToolMetadataChanges`, `data.upgrade.Conflicts`, and `data.upgrade.Diffs`. The dry run does not alter files, Git state, tools, or the lock.

If `data.upgrade.Conflicts` is non-empty, no real upgrade should be attempted. Compare the reported structure with your application changes and resolve it manually. Andurel will not overwrite ambiguous or unrecognized content.

## 3. Apply and verify

With a clean worktree and a conflict-free dry run:

```bash
andurel upgrade --json
andurel tool sync
andurel doctor --verbose
andurel fmt --check
git diff
```

Run the project's normal CI suite before committing. The final `andurel.lock` has `schemaVersion: 1`; its `version` records the installed framework release independently of the lock schema.

If the upgrade fails while writing or validating, Andurel restores every changed file and the lock. Keep the worktree unchanged until the failure has been inspected so the original state remains easy to compare.

## 4. Move from RC.4 to v1.0.0

The stable v1 tag is promoted from the exact validated RC.4 commit after the required soak. Install `@v1.0.0`, repeat the dry run, and confirm that no unexpected project migration remains. If code or generated artifacts changed after RC.4, the project will receive another release candidate instead of an in-place promotion.
