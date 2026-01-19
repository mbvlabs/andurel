# PR1: Add Echo v5 Refactor Plan Docs

Purpose
- Land a docs-only plan that breaks the Echo v5 migration into small, sequential PRs.
- Capture decisions from the discussion so subsequent PRs can be executed with minimal back-and-forth.

Scope
- Add the following docs at repo root:
  - `refactor_overview.md`
  - `refactor_pr1.md` (this file)
  - `refactor_pr2.md` .. `refactor_pr9.md`
- No code changes.

Why this PR exists
- Echo v5 migration touches many templates in `layout/templates/` and `generator/templates/`.
- Keeping PRs atomic reduces merge conflicts, makes reviews easier, and lets us bisect problems.

Key decisions already made (must be reflected in later PRs)
- Observability: replace `otelecho` with `net/http` instrumentation via `otelhttp`.
- Route naming: keep typed `router/routes` package; use Echo v5 `AddRoute(echo.Route{ Name: ... })` to keep stable route names.
- Route registration: prefer returning aggregated errors (Option A) vs panicking.
- Cookies: remove embedded Echo context from cookie/session structs; keep session-backed storage (`echo-contrib/session`) and use v5 signatures.
- Streaming/SSE: migrate to `http.NewResponseController` rather than Echo v4 response wrappers.
- CSRF: Rails-like approach inspired by rails/rails#56350:
  - Support strategy `header_only` and `header_or_legacy_token`.
  - Keep strictness in dev: header-only mode rejects missing/invalid `Sec-Fetch-Site`.
  - Use `_csrf` as legacy token field name.
  - Provide a switch to turn CSRF off (documented).

Acceptance criteria
- Docs exist and are linked correctly:
  - `refactor_overview.md` lists all PR docs.
  - Each `refactor_prX.md` is self-contained.

Notes for implementers
- Subsequent PRs must avoid large "drive-by" edits.
- Each PR should update only what it claims, and end with `go test ./...` where possible.
