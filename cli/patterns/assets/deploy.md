# Deploy

A production app runs four processes from the binaries `andurel build` writes:
`migrate` once, then the `ssr` sidecar, then `app` and `queue`. They share one
release and one environment; only the work each process does differs.

## Files

### deploy/processes.txt (fragment)
```text
migrate  one shot   applies the embedded SQL migrations, then exits
ssr      sidecar    runs the Inertia SSR bundle on INERTIA_SSR_LISTEN
app      web        serves HTTP, renders Inertia pages, embeds client assets
queue    worker     runs River jobs against PostgreSQL; no HTTP listener
```

### deploy/start.sh (fragment)
```sh
#!/bin/sh
set -eu

# 1. Schema first. The binaries carry the migrations; no external SQL files.
./dist/migrate

# 2. SSR sidecar. Wait for its health endpoint before serving pages.
./dist/ssr &
until curl -fsS "${INERTIA_SSR_LISTEN:-http://127.0.0.1:13714}/health" >/dev/null; do
	sleep 1
done

# 3. Web and workers. Both stay in the foreground of their supervisor.
./dist/app &
./dist/queue &

wait
```

## Rules

- Build with `andurel build`; it writes `dist/app`, `dist/queue`, `dist/migrate`, and `dist/ssr` (when Inertia is configured). `--only app,queue` narrows the set, and `--json` emits an artifact manifest with path, size, and version.
- `assets/assets.go` embeds `assets/dist`, including `assets/dist/ssr/ssr.js`, so the app and SSR binaries need no asset files beside them at runtime.
- Every process calls `env.Load()` and reads the same environment. Inject variables through the platform, not by shipping a `.env` file.
- Shared configuration: database `DB_*` for `migrate`, `app`, and `queue`; hosts `HOST_PRIMARY` and `PROTOCOL`; session `SESSION_KEY`, `SESSION_ENCRYPTION_KEY`, `TOKEN_SIGNING_KEY`, and `PEPPER`; email `EMAIL_PROVIDER` (use `smtp` in production) and its `SMTP_*` settings; `CACHE_*`, `QUEUE_*`, and `TELEMETRY_*`.
- `migrate` needs only the `DB_*` settings. It is a one-shot process: run it before `app`/`queue` and never from application startup.
- `ssr` needs only `INERTIA_*` settings plus a JavaScript runtime (`INERTIA_SSR_RUNTIME`, default `node`) with major version 22 or newer on `PATH`. It listens on `INERTIA_SSR_LISTEN` (default `http://127.0.0.1:13714`).
- `app` additionally needs the web settings (`HOST`, `PORT`, CSRF/CORS, Inertia, telemetry) and `INERTIA_SSR_URL` (default `http://127.0.0.1:13714`) pointing at the SSR sidecar.
- `queue` needs the database, `QUEUE_*`, email, cache, and telemetry settings used by its workers. If `cmd/queue` still builds the full `config.New()` graph, run it with the same environment as `app`.
- Set `INERTIA_SSR_FAIL_FAST=true` in production. It is the default; leave it on so an SSR failure returns an error instead of silently rendering client-side only. Turning it off is an explicit client-side-rendering fallback and makes readiness ignore the SSR sidecar.
- Health checks: `GET /api/health` on `app` is liveness and answers `"app is healthy and running"` (503 when the database is unavailable). `GET /api/ready` on `app` is readiness: 200 `{"status":"ready"}` only when the database answers and a required Inertia SSR sidecar is healthy; otherwise 503 `{"status":"unready","reason":"database_unavailable"|"unavailable"}` with a stable reason. `GET /health` on the SSR listen address answers `{"status":"ok"}`. `queue` has no HTTP health endpoint.

## Checklist

- [ ] `andurel build --version <release>` produced all four binaries in `dist/`.
- [ ] The release's environment defines the shared database, session, email, cache, queue, and telemetry settings.
- [ ] `dist/migrate` ran once and exited successfully.
- [ ] `dist/ssr` answers `GET /health` before `dist/app` starts.
- [ ] `dist/app` answers `GET /api/health` (liveness) and `GET /api/ready` (readiness).
- [ ] `dist/queue` is running with the same database and queue settings as `app`.
- [ ] `INERTIA_SSR_FAIL_FAST=true` in production.
