# Echo v5 Migration Refactor Plan (Atomic PRs)

Goal: Upgrade all scaffolding output (layout templates + generator templates) from Echo v4 to Echo v5 with minimal risk, keeping PRs small and sequential.

Guiding principles
- Keep each PR independently reviewable and mergeable.
- Prefer mechanical changes first, then behavior changes.
- Keep scaffolds Rails-like where discussed (CSRF strategy, strictness, etc.).

## PR List (apply in order)

- PR1: Add docs-only migration plan + invariants (`refactor_pr1.md`)
- PR2: Switch template imports + handler signatures to Echo v5 (`refactor_pr2.md`)
- PR3: Replace otelecho with otelhttp; plumb instrumentation flag through router.New (`refactor_pr3.md`)
- PR4: Refactor route registration to use `AddRoute` + collect errors (`refactor_pr4.md`)
- PR5: Response API migration: use `http.ResponseController`, unwrap response for metrics (`refactor_pr5.md`)
- PR6: Cookies + flash: remove embedded Echo context, update to echo-contrib/session v0.50.0 API (`refactor_pr6.md`)
- PR7: Rails-style CSRF strategy (header_only vs header_or_legacy_token) + strictness + "off" switch + docs (`refactor_pr7.md`)
- PR8: Fix generator/controller tests and any remaining v4 APIs (PathValues, etc.) (`refactor_pr8.md`)
- PR9: Update README snippets + examples + e2e expectations for v5 (`refactor_pr9.md`)

## Completion Criteria
- `go test ./...` passes in this repo.
- Scaffolding output compiles against `github.com/labstack/echo/v5`.
- No remaining `echo/v4` imports in templates.
- Router registration returns actionable errors (aggregated) rather than panicking.
- Observability uses `otelhttp` with route naming derived from Echo v5 route info.
- CSRF behavior matches the agreed Rails-like policy.
