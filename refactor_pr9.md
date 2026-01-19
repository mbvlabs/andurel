# PR9: Documentation + Examples + E2E Expectations (Echo v5)

Purpose
- Update remaining docs and examples to reflect Echo v5 APIs.
- Ensure scaffold README and any code snippets compile conceptually.

Scope
- Templates:
  - `layout/templates/readme.tmpl`
  - any docs templates that import echo (including extension templates)
- Repo files:
  - any hard-coded snippets in this repo referencing echo/v4

Detailed changes

1) README + docs
- Replace `echo/v4` imports with `echo/v5`.
- Update handler signatures to `func(c *echo.Context) error`.
- Remove references to v4-only APIs (e.g. `e.URI` / `e.URL` / `e.Reverse` were removed in v5).
- When discussing reverse routing:
  - Prefer framework typed routes (`router/routes`) and avoid `...any`.
  - Mention Echo v5 `Routes.Reverse` and `AddRoute` naming as implementation detail.

2) CSRF docs
- Ensure README documents:
  - `Sec-Fetch-Site` strictness (header-only mode)
  - how to fix test clients by setting `Sec-Fetch-Site`
  - how to disable CSRF (and risks)

3) Observability docs
- Replace mention of `otelecho` with `otelhttp`.

4) Final cleanup
- Ensure no remaining `echo/v4` in repo.

Acceptance criteria
- Docs align with the implemented scaffold behavior.
- `ripgrep 'echo/v4'` in repo returns no results.
