# Layout Module

This package provides the project scaffolding logic used by the `andurel` CLI. It renders the base application templates, loads optional extensions, and manages post-processing steps like `templ` and `sqlc` generation.

## Extension Slots

Extensions contribute behaviour after the base project finishes rendering by populating named slots. A slot identifier follows the `<scope>:<region>` convention:

- `scope` maps to the logical template or generated file (e.g. `controllers`, `cmd/app`, `models`, `routes`).
- `region` describes the injection point inside that template (e.g. `imports`, `structFields`, `build`).

### Available Helper Functions

Templates can access slot data through helper functions registered on the template's `FuncMap`:

- `slot "controllers:imports"` returns a slice of snippets.
- `slotJoined "models:functions" "\n\n"` joins snippets with a separator.
- `slotData "config:values"` returns structured slot payloads.

The base templates typically consume slots by iterating over the slices, for example:

```gotemplate
{{/* slot controllers:imports */}}
{{range .Slot "controllers:imports"}}{{printf "\t%s\n" .}}{{end}}
```

Slots that receive no contributions render nothing, so base scaffolds without extensions continue to produce valid code.

### Writing Extensions

Extensions interact with slot data through the `extensions.Context` helpers:

```go
ctx.AddSlotSnippet("routes:build", "r = append(r, authRoutes...)")
ctx.AddSlotData("router:middleware", MiddlewareConfig{Priority: 10, Name: "Auth"})
```

After slot snippets are registered, the scaffold re-renders only the templates that expose the corresponding scopes. Scope to template mappings live in `slotScopeTemplates` within `layout.go`. Adding a new slot typically means updating the template with a `{{/* slot ... */}}` comment and adding the scope name to that mapping.

Extensions render their own files by calling `ctx.ProcessTemplate`, the same helper used by the base recipe. Post-install steps such as `sqlc generate` can be registered through `ctx.AddPostStep`.

### Ordering Guarantees

Extensions are resolved alphabetically by name, ensuring a deterministic order when multiple extensions target the same slot. Within a single extension, snippets keep the order in which `AddSlotSnippet` is called.

Attempting to write to a slot whose scope is unknown results in an error during the re-render phase. This fails fast and prevents silently skipping contributions due to typos.

## Tests

Run `go test ./...` from the repository root to execute layout and extension tests. Golden files under `layout/testdata` capture full scaffold snapshots for both SQLite and PostgreSQL configurations.

