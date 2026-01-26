# Task: Switch Andurel "run" tool to Shadowfax repo

## Context
- The `run` dev server tool is being moved out of this repo into a new repo, `mbvlabs/shadowfax`.
- This branch will not be merged; the goal is to update the **scaffold** and **tool upgrade/sync** behavior in this repo so that generated projects no longer include `cmd/run/*` and `run` is downloaded like other tools.
- First Shadowfax release: `v0.1.0` at `https://github.com/mbvlabs/shadowfax/releases/tag/v0.1.0`.

## Why
- Currently, `cmd/run` is templated and lives in the scaffold, which forces overwriting `cmd/run` on upgrade to get new functionality.
- Moving to a released tool lets projects upgrade the dev server via `andurel tool sync` without touching scaffolded code.

## Goal
- Remove `cmd/run` from scaffold and upgrade templates.
- Treat `run` as a downloadable tool (release asset) from Shadowfax.
- Keep `andurel run` behavior intact, but it should use the downloaded `bin/run`.

## Expected behavioral changes
- New projects: no `cmd/run/*` files.
- `andurel upgrade`: no longer overwrites `cmd/run/main.go`.
- `andurel tool sync`: downloads `run` from Shadowfax release assets instead of building from `cmd/run`.

## Implementation checklist (what + where)

### 1) Remove `cmd/run` from scaffold output
- Files:
  - `layout/layout.go` (template map)
  - `layout/templates/readme.tmpl` (project structure section)
- What to change:
  - Remove all `cmd_run_*.tmpl` mappings in `layout/layout.go` so `cmd/run` is no longer generated.
  - Update `layout/templates/readme.tmpl` project tree to remove `cmd/run/` entry.

### 2) Stop upgrading `cmd/run` during `andurel upgrade`
- File: `layout/upgrade/generator.go`
- What to change:
  - Remove `"cmd_run_main.tmpl", "cmd/run/main.go"` from `GetFrameworkTemplates`.

### 3) Convert `run` from "built" tool to downloaded tool
- Files:
  - `layout/layout.go` (`GetExpectedTools`, `generateLockFile`)
  - `layout/lock.go` (tool sync)
  - `layout/versions/versions.go` (RunTool version)
  - `layout/upgrade/orchestrator.go` (framework-managed tools)
  - `cli/sync.go` (sync flow)
  - `layout/cmds/download.go` (download URL mapping)
- What to change:
  - Replace `NewBuiltTool("cmd/run/main.go", versions.RunTool)` with a download tool.
  - Use `NewGoTool` with module repo `github.com/mbvlabs/shadowfax` and version `v0.1.0`, then add download logic for a new tool name `run`.
  - Remove "built" tool special cases for `run` in:
    - `layout/lock.go` (built tool switch)
    - `cli/sync.go` (built tool switch)
    - `layout/upgrade/orchestrator.go` (`isFrameworkManagedTool`, `getBuiltToolNameFromPath`, built-tool updates)
  - Add `run` to the download mapping in `layout/cmds/download.go`:
    - Use Shadowfax release assets.
    - Match the asset naming convention used in Shadowfax release.
    - If naming is unknown, inspect the release assets and align the URL format (e.g., `run-<os>-<arch>` or `shadowfax-<os>-<arch>`).
  - Update `layout/versions/versions.go` to set `RunTool` to `v0.1.0`.

### 4) `andurel tool set-version` should allow `run` (optional but recommended)
- File: `cli/lock.go`
- What to change:
  - Add `run` to `goTools` with module `github.com/mbvlabs/shadowfax` so users can bump versions via CLI.

### 5) Remove build path helpers for `cmd/run`
- File: `layout/cmds/cmd.go`
- What to change:
  - Remove `RunGoRunBin` and its usage if it's no longer needed.
  - Ensure no callers are left (search for `RunGoRunBin`).

### 6) Update tool list in scaffold blueprint if needed
- File: `layout/layout.go` (default tools list)
- What to change:
  - If `run` was included as a tool in the generated project’s tool list, ensure it still appears but as a downloaded tool (not a built tool).

## Notes & constraints
- Keep ASCII-only edits.
- Avoid touching unrelated files.
- This repo may have other uncommitted changes; do not revert them.

## Verification steps
- `rg -n "cmd/run" layout cli` -> should only reference docs/legacy text if any remains.
- `rg -n "RunGoRunBin|built.*run" -S` -> should be removed or no longer used.
- `andurel tool sync` in a generated project should download `bin/run` from Shadowfax.
- `andurel upgrade` should not overwrite `cmd/run`.

## Open items to clarify before coding
- Confirm Shadowfax release asset naming (must match download URL in `layout/cmds/download.go`).
- Confirm if Shadowfax should be treated as "go tool" (downloaded from GitHub release assets like templ/sqlc/etc.).
