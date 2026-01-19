# PR7: Rails-Style CSRF Strategy (Sec-Fetch-Site) + Strictness + Off Switch + Docs

Purpose
Implement Rails-like CSRF verification inspired by rails/rails#56350.

Requirements (from discussion)
- Provide two strategies:
  - `header_only` (default for new apps, Rails 8.2 direction)
  - `header_or_legacy_token`
- Legacy token field name must be `_csrf`.
- Strictness: keep Rails-like strictness in dev:
  - in `header_only` mode, unsafe requests missing `Sec-Fetch-Site` must be rejected.
- Provide a setting to turn CSRF off (documented).
- Continue skipping CSRF for API + asset routes.

Why Echo v5 makes this feasible
- Echo v5 CSRF middleware already uses Fetch Metadata:
  - `TrustedOrigins []string`
  - `AllowSecFetchSiteFunc func(c *echo.Context) (bool, error)`
  - `TokenLookup` supports multiple sources (header + form)
- Echo v5 default cookie name is `_csrf`.

Scope
- Templates:
  - `layout/templates/router_router.tmpl` (global middleware)
  - `layout/templates/config_app.tmpl` (add config flags / env)
  - `layout/templates/env.tmpl` (document env vars)
  - `layout/templates/readme.tmpl` (document behavior)
- Potentially adjust frontend forms / JS to send token if needed.

Design

1) Add CSRF settings to app config
- Add fields to app config struct:
  - `CSRFEnabled bool`
  - `CSRFStrategy string` ("header_only" or "header_or_legacy_token")
  - optional: `CSRFTrustedOrigins []string` (can start empty; derive from BaseURL in prod)

2) Pass settings into router.New
- `router.New(..., opts RouterOptions)` or explicit args.
- Router should decide whether to include CSRF middleware.

3) Implement strategy behavior

A) `header_only`
- Must reject unsafe requests that are missing `Sec-Fetch-Site`.
- Echo v5 CSRF middleware currently falls back to legacy token check when header is missing.
- Therefore add a small middleware BEFORE CSRF that:
  - if request method is unsafe (not GET/HEAD/OPTIONS/TRACE)
  - and `Sec-Fetch-Site` header is missing or invalid ("" or "none")
  - then return `403` with a clear error message.
- Then configure `echomw.CSRFWithConfig` primarily to:
  - set `_csrf` cookie and context value
  - enforce cross-site/same-site policies
  - set `TrustedOrigins` (prod) and `AllowSecFetchSiteFunc` to allow `same-site` if desired later.

B) `header_or_legacy_token`
- Allow unsafe requests when:
  - `Sec-Fetch-Site` is `same-origin` or `same-site` (same-site allowed like Rails)
  - `Sec-Fetch-Site` is `cross-site` only when origin is trusted
  - header is missing/none: fall back to token
- Configure CSRF middleware `TokenLookup` to allow legacy token:
  - `TokenLookup = "header:X-CSRF-Token,form:_csrf"`
- Add optional log warning when falling back to token (Rails logs).

4) Trusted origins + environments
- Production:
  - require `config.BaseURL` to be set
  - set `TrustedOrigins` to include the origin derived from BaseURL.
- Development:
  - keep strictness for header_only.
  - allow dev-friendly CORS (handled in other PRs) and document how to set `Sec-Fetch-Site` in tests.

5) Vary header
- Add middleware (or in CSRF pre-check) to append:
  - `Vary: Sec-Fetch-Site`
- This mirrors Rails and avoids cache mixing.

6) Documentation
- Update `README.md` template:
  - explain strategies
  - explain strict header-only behavior and how to fix tests/clients:
    - set `Sec-Fetch-Site: same-origin`
  - explain how to turn CSRF off (and risks)

Acceptance criteria
- Default scaffold runs with CSRF enabled and strategy `header_only`.
- Unsafe requests without `Sec-Fetch-Site` are rejected in header-only mode.
- `header_or_legacy_token` mode accepts legacy token in form field `_csrf`.
- CSRF can be disabled entirely via config.

Notes
- Keep API routes stateless; CSRF skipper should skip `routes.APIPrefix` and `routes.AssetsPrefix`.
