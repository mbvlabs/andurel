# Frontend staging

Build an Inertia page before its handler by declaring its route and payload in
Go first. Generated TypeScript helpers come from existing Go declarations.

## Files

### routes/products.go (fragment)
```go
var ProductIndex = routing.NewSimpleRoute("", "products.index", "/products", routing.TypeScriptRoute())
```

### payloads/products.go (fragment)
```go
type ProductIndexProps struct {
	Title string `json:"title"`
}
```

### resources/js/Pages/Products/Index.tsx (fragment)
```tsx
// Import the generated route helper and payload types after syncing them.
```

## Rules

- Define route values in `routes/` and exported payload structs in `payloads/` without importing handlers; follow the existing scaffold declarations.
- Run `andurel sync routes --json` and `andurel sync payloads --json` before using the generated `resources/js/routes.ts` and `resources/js/types/payloads.ts`.
- Build the page and run the frontend stage of `andurel verify`; then implement the handler, register it in `app/`, and run verify again.
- Re-run the sync commands whenever routes or payloads change; do not hand-edit generated TypeScript.

## Checklist

- [ ] Route and payload declarations exist and both sync commands succeed.
- [ ] The page typechecks against generated helpers and types.
- [ ] The finished handler is registered and `andurel verify` passes.
