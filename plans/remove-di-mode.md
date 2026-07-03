# Remove DI Mode Concept

After committing to uberfx as the only DI mode, clean up all leftover manual DI code paths, templates, and test artifacts.

## Status Key
- [ ] = not started
- [x] = done

## Phase 1: Delete deprecated template files

```
layout/templates/
  deprecated_cmd_app_main.tmpl
  deprecated_router_router.tmpl
  deprecated_controllers_api.tmpl
  deprecated_controllers_assets.tmpl
  deprecated_controllers_controller.tmpl
  deprecated_controllers_pages.tmpl
  deprecated_controllers_pages_inertia.tmpl
  deprecated_controllers_sessions.tmpl
  deprecated_controllers_registrations.tmpl
  deprecated_controllers_confirmations.tmpl
  deprecated_controllers_reset_passwords.tmpl
  deprecated_router_connect_api_routes.tmpl
  deprecated_router_connect_assets_routes.tmpl
  deprecated_router_connect_pages_routes.tmpl
  deprecated_router_connect_sessions_routes.tmpl
  deprecated_router_connect_registrations_routes.tmpl
  deprecated_router_connect_confirmations_routes.tmpl
  deprecated_router_connect_reset_passwords_routes.tmpl

generator/templates/
  deprecated_resource_controller.tmpl
  deprecated_api_resource_controller.tmpl
  deprecated_inertia_vue_resource_controller.tmpl
```

`rm` each file (or batch them with a glob).

## Phase 2: Remove manual blueprint code in layout/layout.go

### 2a. Delete `initializeManualBlueprint` function and `initializeBaseBlueprint` switch

- Remove the function `initializeManualBlueprint` entirely (lines ~1094-1174)
- Simplify `initializeBaseBlueprint` to just call `initializeUberFxBlueprint` directly — no switch needed

### 2b. Flatten `baseTemplateMappings`

Remove entries that were manual-DI-only and are now unused (their `deprecated_` variants + the fx overrides):

Remove these keys:
- `deprecated_cmd_app_main.tmpl`
- `deprecated_controllers_api.tmpl`
- `deprecated_controllers_assets.tmpl`
- `deprecated_controllers_controller.tmpl`
- `deprecated_controllers_pages.tmpl`
- `deprecated_controllers_pages_inertia.tmpl`
- `deprecated_controllers_sessions.tmpl`
- `deprecated_controllers_registrations.tmpl`
- `deprecated_controllers_confirmations.tmpl`
- `deprecated_controllers_reset_passwords.tmpl`
- `deprecated_router_router.tmpl`
- `deprecated_router_connect_api_routes.tmpl`
- `deprecated_router_connect_assets_routes.tmpl`
- `deprecated_router_connect_pages_routes.tmpl`
- `deprecated_router_connect_sessions_routes.tmpl`
- `deprecated_router_connect_registrations_routes.tmpl`
- `deprecated_router_connect_confirmations_routes.tmpl`
- `deprecated_router_connect_reset_passwords_routes.tmpl`

Then move `fxTemplateOverrides` entries into `baseTemplateMappings`:
- `cmd_app_main_fx.tmpl` → `cmd/app/main.go`
- `router_router_fx.tmpl` → `router/router.go`
- `controllers_api_fx.tmpl` → `controllers/api.go`
- `controllers_assets_fx.tmpl` → `controllers/assets.go`
- `controllers_controller_fx.tmpl` → `controllers/controller.go`
- `controllers_pages_fx.tmpl` → `controllers/pages.go`
- `controllers_sessions_fx.tmpl` → `controllers/sessions.go`
- `controllers_registrations_fx.tmpl` → `controllers/registrations.go`
- `controllers_confirmations_fx.tmpl` → `controllers/confirmations.go`
- `controllers_reset_passwords_fx.tmpl` → `controllers/reset_passwords.go`
- `services_service_fx.tmpl` → `services/service.go`
- `services_identity_fx.tmpl` → `services/identity.go`

Add the inertia fx template:
- `controllers_pages_inertia_fx.tmpl` → `controllers/pages.go`

### 2c. Delete `fxTemplateOverrides` map

No longer needed — all entries moved into `baseTemplateMappings`.

### 2d. Delete `fxSkippedTemplates` map

No longer needed — no templates to skip.

### 2e. Flatten `inertiaTemplateOverrides`

- Remove the non-fx entry `deprecated_controllers_pages_inertia.tmpl`
- Keep only `controllers_pages_inertia_fx.tmpl`

Or better, move the fx inertia entry into `baseTemplateMappings` (see 2b) and delete `inertiaTemplateOverrides` entirely.

### 2f. Simplify `processTemplatedFiles`

- Remove the `diMode == "uberfx"` branch (the fxSkippedTemplates deletion + fx overrides addition)
- Remove `diMode` branching in the inertia block — always use fx inertia templates
- Result: a single flat mapping loop without mode checks

### 2g. Simplify `rerenderBlueprintTemplates`

- Remove the `if td.DIMode == "uberfx"` / `else` branching
- Always use the uberfx set of blueprint templates
- The else branch templates (`deprecated_cmd_app_main.tmpl`, etc.) can be dropped

### 2h. Remove `DIMode` field from `TemplateData`

- Remove `DIMode string` from the struct
- Remove `DIMode:` assignment in `Scaffold()` call
- Remove `DIMode` from `ScaffoldConfig`
- Remove `diMode string` parameter from `Scaffold()` function
- Remove `diMode` parameter from `initializeBaseBlueprint()`
- Remove `diMode` parameter from `initializeUberFxBlueprint()` (rename to `initializeBlueprint`)
- Remove `diMode` from `extensions.Context` struct
- Remove `diMode` from lock file writing and reading

## Phase 3: Clean up generator/controllers/template_renderer.go

### 3a. Simplify `RenderControllerFile`

- Remove `diMode string` parameter
- Remove all non-fx branches:
  - `api_resource_controller.tmpl` branch → always use `api_resource_controller_fx.tmpl`
  - `resource_controller.tmpl` branch → always use `resource_controller_fx.tmpl`
  - `inertia_vue_resource_controller.tmpl` branch → always use `inertia_vue_resource_controller_fx.tmpl`
- Result: a simple `switch` on `controller.Type` with no mode branching

### 3b. Update all callers

Search for `RenderControllerFile` calls and remove the `diMode` argument.

## Phase 4: Clean up test files

### 4a. `cli/command_behavior_test.go`

- Remove the two `"manual"` lock file writes that were kept to test manual DI behavior
- Those tests should now exercise the same paths (the generated code will be the same)

### 4b. `cli/generate_job_email_test.go`

- Remove `writeGenerateFileTestLock(t, rootDir, "manual")` line
- The setup function no longer needs a diMode parameter

### 4c. `e2e/scaffold_matrix_test.go`

- Remove `diModes` slice — it's now a single hardcoded `"uberfx"` (or just remove the loop)
- Remove `diMode` from `ScaffoldConfig` struct
- Remove `diMode` from `scaffoldConfigName()` — it's always uberfx
- Remove `diMode` from config struct construction

### 4d. `e2e/scaffold_matrix_test.go` — `isCriticalScaffoldConfig` / `TestScaffoldCriticalConfigs`

Already updated in the previous session. No further changes needed if config names don't change.

### 4e. `layout/extension_add_test.go`

- Remove `DIMode` field from any test scaffold config structs

## Phase 5: Audit other Go files

Search the entire codebase for `diMode`, `DIMode`, `DiMode`, `"manual"` (in DI context), `fxSkipped`, `fxOverride`:

```
rg -l "diMode|DIMode|DiMode|fxSkipped|fxOverride|fxTemplateOverrides" --type go
```

Check:
- `cli/new_project.go` — already done (previous session)
- `cli/command_behavior_test.go`
- `cli/generate_job_email_test.go`
- `layout/layout.go`
- `layout/extension_add.go`
- `layout/extension_add_test.go`
- `layout/upgrade/` — check for DIMode references
- `generator/controllers/template_renderer.go`
- `generator/controller_manager.go`
- `e2e/scaffold_matrix_test.go`
- Any extension `.go` files that reference DIMode

## Phase 6: Build & test

```bash
go build ./...
go vet ./...
go test ./...
```

Expect: green across the board.

## Phase 7 (optional): Clean up lock file

If desired, remove `di_mode` from `AndurelLock` / `ScaffoldConfig` struct in `layout/layout.go` so the lock file no longer writes it.

## Ordering Dependencies

```
Phase 1 (delete templates) → no code dependency
Phase 2 (layout.go) → after Phase 1
Phase 3 (template_renderer.go) → after/parallel with Phase 2
Phase 4 (tests) → after Phase 2 & 3
Phase 5 (audit) → after Phase 2 & 3
Phase 6 (build/test) → after all code changes
```
