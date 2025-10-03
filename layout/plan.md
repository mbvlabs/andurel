# Extension Slot Refactor Plan

Status: COMPLETE

Use this plan as a guide for refactoring the layout scaffolding system to support a more modular and maintainable extension mechanism. After each file change commit your changes.

If you need to have any long-term notes or ideas, consider adding them to a separate `notes.md` file.

Once done, mark this plan as complete and exit.

## Goal
Refactor the layout scaffold so extensions apply their changes after the base project is generated, using well-defined slots instead of boolean conditionals scattered across templates. The updated system should let future extensions inject files, code fragments, configuration data, and post-steps without duplicating templates or modifying unrelated core files.

## High-Level Approach
1. Render the base project once using core templates that expose named slots for extension contributions.
2. Collect extension metadata describing which slots to populate, which additional files to render, and what post-processing steps to run.
3. Re-render only the affected templates with the combined slot data, and then execute extension post-steps (e.g., `sqlc generate`).
4. Provide utilities for adding slots, merging contributions, and resolving ordering.

## Detailed Tasks

### 1. Extend Template Data Model
- Update `extensions.TemplateData` to include a slot registry, e.g., `map[string][]string` for text snippets and possibly `map[string]any` for structured data.
- Document the naming convention for slots (e.g., `controllers:imports`, `controllers:structFields`, `routes:authRoutes`).
- Ensure template execution can access slots easily (`{{range .Slot "controllers:imports"}}...{{end}}`).

### 2. Core Template Updates
- Identify files that currently rely on `ExtensionFlags` conditionals (`controllers_controller.tmpl`, `cmd_app_main.tmpl`, `models_model.tmpl`, `router_routes_routes.tmpl`).
- Replace conditional blocks with slot placeholders. Example: replace auth import block with `{{block "controllers:imports" .}}` or a helper that iterates over slot content.
- Add comments in templates indicating slot purpose to reduce regressions.
- Verify that base rendering without extensions still produces valid Go code (slot placeholders should render nothing when empty).

### 3. Template Rendering Helpers
- Introduce helper functions in `layout.go` (or new package) to manage slots:
  - `AddSlotSnippet(data *TemplateData, slot string, snippet string)` to append text.
  - `RenderTemplateWithSlots(targetDir, templateFile, targetPath string, data *TemplateData)` that reuses existing logic but ensures slots are available to templates (possibly via `template.FuncMap`).
- Optionally define `template.FuncMap` entries such as `slot` (returns joined snippet) and `slotlines` (renders each snippet on a newline).

### 4. Extension Interface Enhancements
- Expand `extensions.Context` with APIs to contribute to slots:
  - `AddSlotSnippet(slot string, snippet string)`
  - Optional structured APIs for common patterns (e.g., `AddImport(file, importPath)` that maps to relevant slot).
- Track which templates an extension needs re-rendered by returning metadata or by using slot naming convention that implies target templates.
- Allow extensions to register new template files (already supported via `ProcessTemplate`).

### 5. Apply To Simple Auth Extension
- Replace current template duplication logic with slot injections:
  - Register snippets for imports, struct fields, middleware wiring, routes, etc.
  - Render new files that have no base counterpart (e.g., `router/middleware/auth.go`) via standard `ProcessTemplate` calls.
  - Schedule post-step for `sqlc generate` as currently done.
- Update or remove templates that become redundant once slot system is in place.

### 6. Update Layout Flow
- Modify `Scaffold`:
  - After base templates render, iterate over extensions to collect slot snippets.
  - Re-render only templates that expose slots (maintain a list or detect via slot usage).
  - Ensure idempotency when multiple extensions contribute to the same slot; define ordering rules (e.g., alphabetical by extension name or explicit weight).
- Consider writing intermediate outputs to a temporary location before overwriting final files to avoid partial writes on failure.

### 7. Testing Strategy
- Expand unit/integration tests:
  - Add minimal “dummy extension” in tests to verify slot injection renders expected content.
  - Update golden files to reflect new output structure for simple-auth (after applying slot-based changes and bug fixes).
  - Add tests for duplicate slot contributions and nil/empty slots.
- Run `go test ./...`, `go vet ./...`, and ensure scaffolding passes for both sqlite/postgres with and without extensions (with network disabled as necessary by using cached `go mod tidy`).

### 8. Migration/Cleanup
- Remove `ExtensionFlags` references from code and templates once slots are in place.
- Delete obsolete templates that were previously rendered only under conditional blocks.
- Update documentation (README or inline comments) describing how to build new extensions under the slot system.

## Open Questions / Decisions
- Slot naming granularity: decide whether to use file-based namespaces (`controllers/auth.go` slots) or functional names (`auth:routes`).
- Ordering semantics for slot content: default alphabetical vs explicit priority field in API.
- Error handling when an extension requests a non-existent slot—should it fail fast or warn and skip?

## Implementation Notes
- Maintain ASCII output; keep template helper names consistent.
- Prefer `path` over `filepath` for embedded template paths.
- Continue to use repo-local `GOCACHE` when running tests in sandboxed environments.
