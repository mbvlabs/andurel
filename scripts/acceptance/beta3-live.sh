#!/usr/bin/env bash
# shellcheck disable=SC2016
# Beta.3 live fresh-scaffold acceptance gate (harness probe 17).
#
# This script is the durable form of the live runtime gate. It never reuses an
# existing application, database container, port, build output, or lease:
# every run creates its own scratch tree, disposable PostgreSQL container, and
# isolated build output directories.
#
# It proves, against a freshly generated React/pnpm scaffold with temporary
# local replacements for the candidate pkg/* modules:
#   * anonymous GET / is public and server-rendered (no sign-in redirect)
#   * a seeded login still works and the authenticated session is preserved
#     through a server-rendered home request
#   * /api/ready reports ready, loses readiness when the SSR sidecar is stopped,
#     answers the themed 500 page for the failed SSR document, recovers without
#     an application restart, and reports database_unavailable while PostgreSQL
#     is stopped and recovers after it returns
#   * the compiled cmd/ssr binary exits nonzero on a duplicate listener and when
#     its JavaScript child crashes after startup
#   * two concurrent isolated builds publish independent outputs and leave the
#     source tree and the in-place asset/dist trees untouched
#
# Usage:
#   scripts/acceptance/beta3-live.sh [--mode dev|release] [--evidence-dir DIR] [--scratch-root DIR]
#                                    [--andurel PATH] [--keep-scratch]
#
# Exit status: 0 when every check passed, 1 on a failed check, 77 when a
# prerequisite (tool, Docker daemon, PostgreSQL image, network/cache for tools
# or frontend dependencies) is unavailable. A blocked result is never a pass.
set -Eeuo pipefail
IFS=$'\n\t'
umask 022

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CANDIDATE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
CANDIDATE_VERSION="${ANDUREL_ACCEPTANCE_VERSION:-v2.0.0-beta.4}"
MODE=dev
EVIDENCE=""
KEEP_SCRATCH=0
SCRATCH_ROOT="${ANDUREL_LIVE_SCRATCH_ROOT:-${TMPDIR:-/tmp/opencode}}"
ANDUREL_BIN=""
APP_NAME="live-acceptance"
PG_IMAGE="${ANDUREL_LIVE_POSTGRES_IMAGE:-postgres:17-alpine}"

SCRATCH=""
APP=""
APP_URL=""
SSR_URL=""
APP_PORT=""
SSR_PORT=""
PG_PORT=""
PG_NAME=""
PG_ID=""
PG_PASSWORD="postgres"
DB_NAME="liveapp"
APP_PID=""
SSR_PID=""
CRASH_PID=""
PROC_PID=""

usage() {
	sed -n '2,32p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--mode)
		MODE="${2:-}"
		shift 2
		;;
	--evidence-dir)
		EVIDENCE="${2:-}"
		shift 2
		;;
	--scratch-root)
		SCRATCH_ROOT="${2:-}"
		shift 2
		;;
	--andurel)
		ANDUREL_BIN="${2:-}"
		shift 2
		;;
	--app-name)
		APP_NAME="${2:-}"
		shift 2
		;;
	--keep-scratch)
		KEEP_SCRATCH=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

case "$MODE" in
dev | release) ;;
*) echo "--mode must be dev or release" >&2; exit 2 ;;
esac

if [[ -z "$EVIDENCE" ]]; then
	EVIDENCE="$ROOT/plans/evidence/beta3-complete-fix/${CANDIDATE_SHA:0:12}-live"
fi

log() { printf '== %s\n' "$*" >&2; }

pass() {
	printf 'pass\t%s\n' "$*" >>"$EVIDENCE/checks.tsv"
}

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	printf 'fail\n' >"$EVIDENCE/live-state.txt"
	exit 1
}

blocked() {
	printf 'BLOCKED: %s\n' "$*" >&2
	printf 'blocked\n' >"$EVIDENCE/live-state.txt"
	exit 77
}

record_cmd() {
	local first=1
	local arg
	for arg in "$@"; do
		if ((first == 1)); then
			printf '%s' "$arg" >>"$EVIDENCE/commands.log"
			first=0
		else
			printf ' %s' "$arg" >>"$EVIDENCE/commands.log"
		fi
	done
	printf '\n' >>"$EVIDENCE/commands.log"
}

stop_process() {
	local pid="$1"
	local deadline
	if [[ -z "$pid" ]]; then
		return 0
	fi
	kill -TERM "$pid" 2>/dev/null || true
	deadline=$((SECONDS + 20))
	while kill -0 "$pid" 2>/dev/null && ((SECONDS < deadline)); do
		sleep 0.1
	done
	if kill -0 "$pid" 2>/dev/null; then
		pkill -TERM -P "$pid" 2>/dev/null || true
		deadline=$((SECONDS + 5))
		while kill -0 "$pid" 2>/dev/null && ((SECONDS < deadline)); do
			sleep 0.1
		done
	fi
	if kill -0 "$pid" 2>/dev/null; then
		pkill -KILL -P "$pid" 2>/dev/null || true
		kill -KILL "$pid" 2>/dev/null || true
	fi
	set +e
	wait "$pid" 2>/dev/null
	set -e
}

scrub_evidence() {
	local file
	if [[ -z "$EVIDENCE" || ! -d "$EVIDENCE" ]]; then
		return 0
	fi
	while IFS= read -r -d '' file; do
		sed -i -E \
			-e 's/(Set-Cookie: [^=[:space:]]+=)[^;]*/\1REDACTED/g' \
			-e 's/(Cookie: [^=[:space:]]+=)[^;]*/\1REDACTED/g' \
			-e 's/(X-CSRF-Token: )[^[:space:]]+/\1REDACTED/g' \
			"$file"
	done < <(find "$EVIDENCE" -type f -print0)
}

cleanup() {
	local pid
	for pid in "$APP_PID" "$SSR_PID" "$CRASH_PID"; do
		if [[ -n "$pid" ]]; then
			stop_process "$pid"
		fi
	done
	if [[ -n "$PG_ID" ]]; then
		docker rm -f "$PG_ID" >/dev/null 2>&1 || true
	fi
	scrub_evidence
	if [[ -n "$SCRATCH" && -d "$SCRATCH" && "$KEEP_SCRATCH" -eq 0 ]]; then
		rm -rf "$SCRATCH"
	fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$EVIDENCE"
: >"$EVIDENCE/checks.tsv"
: >"$EVIDENCE/commands.log"

write_environment() {
	{
		printf 'candidate_sha=%s\n' "$CANDIDATE_SHA"
		printf 'candidate_version=%s (unpublished acceptance build)\n' "$CANDIDATE_VERSION"
		printf 'mode=%s\n' "$MODE"
		printf 'candidate_dirty=%s\n' "$(git -C "$ROOT" status --porcelain | wc -l)"
		printf 'go=%s\n' "$(go version)"
		printf 'node=%s\n' "$(node --version 2>/dev/null || echo unavailable)"
		printf 'pnpm=%s\n' "$(pnpm --version 2>/dev/null || echo unavailable)"
		printf 'docker=%s\n' "$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unavailable)"
		printf 'postgres_image=%s\n' "$PG_IMAGE"
		printf 'dates=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf 'secrets=redacted\n'
	} >"$EVIDENCE/environment.txt"
	git -C "$ROOT" log -1 --format='%H %s' >"$EVIDENCE/candidate-commit.txt"
	git -C "$ROOT" status --porcelain >"$EVIDENCE/candidate-status.txt"
	grep -E 'Inertia = |Anduin = |Storage = ' "$ROOT/layout/versions/versions.go" \
		>"$EVIDENCE/candidate-pins.txt" 2>/dev/null || true
}

check_prereqs() {
	local missing=()
	local tool
	for tool in go node pnpm docker curl jq openssl timeout git awk grep sed find; do
		command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
	done
	if ((${#missing[@]} > 0)); then
		blocked "live gate needs: ${missing[*]}"
	fi
	if ! docker info >/dev/null 2>&1; then
		blocked "docker daemon is not reachable"
	fi
	record_cmd "docker image inspect $PG_IMAGE"
	if ! docker image inspect "$PG_IMAGE" >"$EVIDENCE/00-postgres-image.stdout" 2>"$EVIDENCE/00-postgres-image.stderr"; then
		record_cmd "timeout 300 docker pull $PG_IMAGE"
		if ! timeout 300 docker pull "$PG_IMAGE" >"$EVIDENCE/00-postgres-pull.stdout" 2>"$EVIDENCE/00-postgres-pull.stderr"; then
			blocked "PostgreSQL image $PG_IMAGE is unavailable and could not be pulled"
		fi
	fi
	pass "prerequisites"
}

prepare_scratch() {
	mkdir -p "$SCRATCH_ROOT"
	SCRATCH="$(mktemp -d "$SCRATCH_ROOT/andurel-beta3-live-XXXXXX")"
	mkdir -p "$SCRATCH/bin" "$SCRATCH/apps" "$SCRATCH/builds"
	log "scratch: $SCRATCH"
	log "evidence: $EVIDENCE"
	printf '%s\n' "$SCRATCH" >"$EVIDENCE/scratch-path.txt"
}

run_step() {
	local name="$1"
	shift
	record_cmd "$@"
	set +e
	timeout --kill-after=15 "${PROBE_TIMEOUT:-900}" "$@" >"$EVIDENCE/$name.stdout" 2>"$EVIDENCE/$name.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/$name.status"
	if ((code != 0)); then
		printf 'step %s failed (exit %s): %s\n' "$name" "$code" "$*" >&2
		tail -40 "$EVIDENCE/$name.stderr" >&2 || true
	fi
	return "$code"
}

run_step_in() {
	local dir="$1"
	local name="$2"
	shift 2
	(
		cd "$dir"
		run_step "$name" "$@"
	)
}

build_cli() {
	if [[ -z "$ANDUREL_BIN" ]]; then
		if ! run_step_in "$ROOT" 01-build-cli env GOWORK=off go build -ldflags "-X main.version=$CANDIDATE_VERSION" -o "$SCRATCH/bin/andurel" .; then
			fail "candidate CLI did not build"
		fi
		ANDUREL_BIN="$SCRATCH/bin/andurel"
	fi
	if [[ ! -x "$ANDUREL_BIN" ]]; then
		fail "andurel CLI is not executable: $ANDUREL_BIN"
	fi
	if ! run_step 01-cli-version "$ANDUREL_BIN" --version; then
		fail "candidate CLI did not report a version"
	fi
	pass "candidate CLI"
}

generate_app() {
	if ! run_step_in "$SCRATCH/apps" 02-new "$ANDUREL_BIN" new "$APP_NAME"; then
		fail "andurel new failed"
	fi
	APP="$SCRATCH/apps/$APP_NAME"
	if [[ ! -f "$APP/go.mod" || ! -f "$APP/andurel.lock" ]]; then
		fail "generated scaffold is missing go.mod or andurel.lock"
	fi
	check_contains 02-new "Successfully created project"
	pass "fresh scaffold generated"
}

replace_modules() {
	if [[ "$MODE" == release ]]; then
		local inertia_version
		inertia_version="$(awk -F '"' '/^[[:space:]]*Inertia = / { print $2 }' "$ROOT/layout/versions/versions.go")"
		if [[ -z "$inertia_version" ]]; then
			fail "cannot determine candidate Inertia version"
		fi
		if ! git -C "$ROOT" show-ref --verify --quiet "refs/tags/pkg/inertia/$inertia_version"; then
			blocked "release gate: pkg/inertia/$inertia_version tag is absent; do not fetch an unpublished candidate"
		fi
		pass "release mode uses published pins without local replacements"
		return
	fi
	local module_dir
	local module
	for module_dir in "$ROOT"/pkg/*/; do
		[[ -f "${module_dir}go.mod" ]] || continue
		module="$(basename "$module_dir")"
		record_cmd "go mod edit -replace github.com/mbvlabs/andurel/pkg/$module=${module_dir%/}"
		set +e
		(
			cd "$APP"
			go mod edit -replace "github.com/mbvlabs/andurel/pkg/${module}=${module_dir%/}"
		) >"$EVIDENCE/03-replace-$module.stdout" 2>"$EVIDENCE/03-replace-$module.stderr"
		local code=$?
		set -e
		printf '%s\n' "$code" >"$EVIDENCE/03-replace-$module.status"
		if ((code != 0)); then
			fail "local replacement for pkg/$module failed"
		fi
	done
	pass "candidate pkg modules replaced locally"
}

tidy_app() {
	if ! run_step_in "$APP" 04-go-mod-tidy env GOWORK=off go mod tidy; then
		blocked "go mod tidy needs network/module cache for the candidate replacements; see $EVIDENCE/04-go-mod-tidy.stderr"
	fi
	if [[ ! -s "$APP/go.sum" ]]; then
		fail "go mod tidy did not produce go.sum"
	fi
	pass "candidate module graph tidied"
}

sync_tools() {
	if ! run_step_in "$APP" 05-tool-sync "$ANDUREL_BIN" tool sync; then
		blocked "tool sync needs network access for pinned binaries; see $EVIDENCE/05-tool-sync.stderr"
	fi
	pass "project tools synced"
}

install_frontend() {
	if ! run_step_in "$APP" 06-pnpm-install pnpm install; then
		blocked "pnpm install needs network/cache for frontend dependencies; see $EVIDENCE/06-pnpm-install.stderr"
	fi
	if [[ ! -x "$APP/node_modules/.bin/vite" ]]; then
		fail "pnpm install did not install vite"
	fi
	pass "frontend dependencies installed"
}

verify_after_setup() {
	if ! run_step_in "$APP" 06-verify-after-setup "$ANDUREL_BIN" verify --json; then
		fail "untouched scaffold verification failed after documented tool/frontend setup"
	fi
	pass "plain verify passes after setup without source edits"
}

build_app() {
	if ! run_step_in "$APP" 07-build "$ANDUREL_BIN" build --version "live-${CANDIDATE_SHA:0:8}"; then
		fail "production build failed"
	fi
	local artifact
	for artifact in dist/app dist/ssr dist/migrate dist/queue assets/dist/ssr/ssr.js; do
		if [[ ! -s "$APP/$artifact" ]]; then
			fail "production build is missing $artifact"
		fi
	done
	if [[ ! -x "$APP/dist/app" || ! -x "$APP/dist/ssr" ]]; then
		fail "production app or SSR binary is not executable"
	fi
	pass "production app, migrate, queue, and SSR binaries built"
}

free_port() {
	local port
	for _ in $(seq 1 200); do
		port=$(((RANDOM % 20000) + 20000))
		if command -v ss >/dev/null 2>&1; then
			if ! ss -ltnH "sport = :$port" 2>/dev/null | grep -q .; then
				printf '%s' "$port"
				return 0
			fi
		else
			if ! nc -z 127.0.0.1 "$port" 2>/dev/null; then
				printf '%s' "$port"
				return 0
			fi
		fi
	done
	return 1
}

wait_pg() {
	local timeout="${1:-90}"
	local deadline=$((SECONDS + timeout))
	while ((SECONDS < deadline)); do
		if docker exec "$PG_NAME" psql -U postgres -d "$DB_NAME" -c 'select 1' \
			>/dev/null 2>&1; then
			return 0
		fi
		sleep 0.5
	done
	return 1
}

start_database() {
	PG_PORT="$(free_port)" || fail "no free PostgreSQL host port"
	PG_NAME="andurel-live-$(date -u +%s)-$$-$RANDOM"
	record_cmd "docker run -d --name $PG_NAME --label andurel.acceptance=beta3-live -p 127.0.0.1:$PG_PORT:5432 $PG_IMAGE"
	set +e
	docker run -d \
		--name "$PG_NAME" \
		--label "andurel.acceptance=beta3-live" \
		-e "POSTGRES_PASSWORD=$PG_PASSWORD" \
		-e "POSTGRES_DB=$DB_NAME" \
		-p "127.0.0.1:${PG_PORT}:5432" \
		"$PG_IMAGE" >"$EVIDENCE/08-postgres-run.stdout" 2>"$EVIDENCE/08-postgres-run.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/08-postgres-run.status"
	if ((code != 0)); then
		fail "disposable PostgreSQL container did not start"
	fi
	PG_ID="$(head -1 "$EVIDENCE/08-postgres-run.stdout")"
	if [[ ! "$PG_ID" =~ ^[a-f0-9]{64}$ ]]; then
		PG_ID=""
		fail "disposable PostgreSQL did not return a container ID"
	fi
	if ! wait_pg 90; then
		docker logs "$PG_NAME" >"$EVIDENCE/postgres.log" 2>&1 || true
		fail "disposable PostgreSQL did not become ready"
	fi
	pass "disposable PostgreSQL ready on 127.0.0.1:$PG_PORT"
}

configure_env() {
	APP_PORT="$(free_port)" || fail "no free application port"
	SSR_PORT="$(free_port)" || fail "no free SSR port"
	while [[ "$SSR_PORT" == "$APP_PORT" ]]; do
		SSR_PORT="$(free_port)" || fail "no distinct SSR port"
	done
	APP_URL="http://127.0.0.1:$APP_PORT"
	SSR_URL="http://127.0.0.1:$SSR_PORT"
	local session_key
	local encryption_key
	local token_key
	local pepper
	session_key="$(openssl rand -hex 32)"
	encryption_key="$(openssl rand -hex 32)"
	token_key="$(openssl rand -hex 32)"
	pepper="$(openssl rand -hex 16)"
	record_cmd "write $APP/.env (production, disposable values, secrets redacted)"
	cat >"$APP/.env" <<EOF
ENVIRONMENT=production
PROJECT_NAME=$APP_NAME
SITE_NAME=Live Acceptance
HOST_PRIMARY=127.0.0.1:$APP_PORT
PROTOCOL=http

TOKEN_SIGNING_KEY=$token_key
PEPPER=$pepper

CACHE_DRIVER=memory
CACHE_NAMESPACE=live-acceptance

DB_KIND=postgres
DB_HOST=127.0.0.1
DB_PORT=$PG_PORT
DB_NAME=$DB_NAME
DB_USER=postgres
DB_PASSWORD=$PG_PASSWORD
DB_SSL_MODE=disable

EMAIL_PROVIDER=log

HOST=127.0.0.1
PORT=$APP_PORT
CSRF_STRATEGY=header_only

INERTIA_SSR_RUNTIME=node
INERTIA_SSR_BUNDLE=assets/dist/ssr/ssr.js
INERTIA_SSR_LISTEN=$SSR_URL
INERTIA_SSR_URL=$SSR_URL
INERTIA_SSR_STARTUP_TIMEOUT=10s
INERTIA_SSR_REQUEST_TIMEOUT=2s
INERTIA_SSR_FAIL_FAST=true

SESSION_KEY=$session_key
SESSION_ENCRYPTION_KEY=$encryption_key
LOG_LEVEL=INFO
EOF
	chmod 600 "$APP/.env"
	printf 'app_url=%s\nssr_url=%s\ndb_port=%s\nsecrets=redacted\n' \
		"$APP_URL" "$SSR_URL" "$PG_PORT" >"$EVIDENCE/09-environment.txt"
	pass "production runtime configuration written"
}

migrate() {
	if ! run_step_in "$APP" 10-migrate ./dist/migrate; then
		fail "migrations failed"
	fi
	if ! run_step_in "$APP" 10-db-check docker exec "$PG_NAME" psql -U postgres -d "$DB_NAME" -tAc \
		"select count(*) from users"; then
		fail "migrated database did not expose the users table"
	fi
	pass "migrations applied"
}

seed() {
	if ! run_step_in "$APP" 11-build-seeds env GOWORK=off go build -o "$SCRATCH/bin/seeds" ./cmd/seeds; then
		fail "seeds binary did not build"
	fi
	if ! run_step_in "$APP" 11-seeds "$SCRATCH/bin/seeds"; then
		fail "seeds failed"
	fi
	check_contains 11-seeds "Created admin user: admin@example.com"
	if ! run_step_in "$APP" 11-user-check docker exec "$PG_NAME" psql -U postgres -d "$DB_NAME" -tAc \
		"select count(*) from users where email = 'admin@example.com'"; then
		fail "seeded admin user is missing"
	fi
	check_contains 11-user-check "1"
	pass "verified seed user created"
}

start_process_bg() {
	local dir="$1"
	local logfile="$2"
	shift 2
	(
		cd "$dir"
		exec "$@" >"$logfile" 2>&1
	) &
	PROC_PID=$!
}

wait_ssr_health() {
	local port="$1"
	local timeout="${2:-30}"
	local deadline=$((SECONDS + timeout))
	while ((SECONDS < deadline)); do
		if curl -fsS --max-time 2 "http://127.0.0.1:$port/health" >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.1
	done
	return 1
}

wait_http_code() {
	local url="$1"
	local want="$2"
	local timeout="${3:-30}"
	local deadline=$((SECONDS + timeout))
	local code
	while ((SECONDS < deadline)); do
		code="$(curl -sS --max-time 3 -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
		if [[ "$code" == "$want" ]]; then
			return 0
		fi
		sleep 0.2
	done
	return 1
}

http_get() {
	local name="$1"
	local url="$2"
	shift 2
	record_cmd "curl GET $url $*"
	set +e
	curl -sS --max-time 20 -D "$EVIDENCE/$name.headers" -o "$EVIDENCE/$name.body" \
		-w '%{http_code}' "$@" "$url" >"$EVIDENCE/$name.code" 2>"$EVIDENCE/$name.curl.stderr"
	local code=$?
	set -e
	if ((code != 0)); then
		fail "$name: curl failed (exit $code); see $EVIDENCE/$name.curl.stderr"
	fi
}

http_post_json() {
	local name="$1"
	local url="$2"
	local body="$3"
	local csrf="$4"
	record_cmd "curl POST $url (JSON body and CSRF token redacted)"
	set +e
	curl -sS --max-time 20 -D "$EVIDENCE/$name.headers" -o "$EVIDENCE/$name.body" \
		-w '%{http_code}' \
		-H "Content-Type: application/json" \
		-H "X-CSRF-Token: $csrf" \
		-H "Cookie: _csrf=$csrf" \
		--data "$body" "$url" >"$EVIDENCE/$name.code" 2>"$EVIDENCE/$name.curl.stderr"
	local code=$?
	set -e
	if ((code != 0)); then
		fail "$name: curl failed (exit $code); see $EVIDENCE/$name.curl.stderr"
	fi
}

header_value() {
	local file="$1"
	local name="$2"
	awk -v want="$name" '
		BEGIN { IGNORECASE = 1 }
		tolower($1) == tolower(want ":") {
			v = $2
			sub(/\r$/, "", v)
			print v
			exit
		}
	' "$file"
}

cookie_pair_prefix() {
	local file="$1"
	local prefix="$2"
	awk -v want="$prefix" '
		BEGIN { IGNORECASE = 1 }
		tolower($1) == "set-cookie:" {
			pair = $2
			sub(/\r$/, "", pair)
			eq = index(pair, "=")
			if (eq == 0) {
				next
			}
			name = substr(pair, 1, eq - 1)
			if (index(tolower(name), tolower(want)) == 1) {
				print pair
				exit
			}
		}
	' "$file"
}

cookie_value() {
	local file="$1"
	local name="$2"
	local pair
	pair="$(cookie_pair_prefix "$file" "$name")"
	if [[ -z "$pair" ]]; then
		return 0
	fi
	pair="${pair#*=}"
	printf '%s' "${pair%%;*}"
}

check_code() {
	local name="$1"
	local want="$2"
	local got
	got="$(cat "$EVIDENCE/$name.code")"
	if [[ "$got" != "$want" ]]; then
		fail "$name: HTTP status $got, want $want"
	fi
	pass "$name status $want"
}

check_contains() {
	local name="$1"
	local needle="$2"
	local file="$EVIDENCE/$name.body"
	if [[ ! -f "$file" ]]; then
		file="$EVIDENCE/$name.stdout"
	fi
	if ! grep -Fq -- "$needle" "$file"; then
		fail "$name: expected $file to contain $needle"
	fi
	pass "$name contains $needle"
}

check_not_contains() {
	local name="$1"
	local needle="$2"
	local file="$EVIDENCE/$name.body"
	if grep -Fq -- "$needle" "$file"; then
		fail "$name: expected $file not to contain $needle"
	fi
	pass "$name excludes $needle"
}

check_header() {
	local name="$1"
	local header="$2"
	local want="$3"
	local got
	got="$(header_value "$EVIDENCE/$name.headers" "$header")"
	if [[ "$got" != "$want" ]]; then
		fail "$name: header $header is $got, want $want"
	fi
	pass "$name header $header"
}

check_header_absent() {
	local name="$1"
	local header="$2"
	local got
	got="$(header_value "$EVIDENCE/$name.headers" "$header")"
	if [[ -n "$got" ]]; then
		fail "$name: header $header unexpectedly present: $got"
	fi
	pass "$name has no $header"
}

start_runtime() {
	record_cmd "start $APP/dist/ssr (cwd $APP)"
	start_process_bg "$APP" "$EVIDENCE/ssr.log" ./dist/ssr
	SSR_PID="$PROC_PID"
	if ! wait_ssr_health "$SSR_PORT" 30; then
		tail -40 "$EVIDENCE/ssr.log" >&2 || true
		fail "compiled cmd/ssr did not become healthy"
	fi
	record_cmd "start $APP/dist/app (cwd $APP)"
	start_process_bg "$APP" "$EVIDENCE/app.log" ./dist/app
	APP_PID="$PROC_PID"
	if ! wait_http_code "$APP_URL/api/health" 200 30; then
		tail -60 "$EVIDENCE/app.log" >&2 || true
		fail "compiled cmd/app did not become healthy"
	fi
	if ! wait_http_code "$APP_URL/api/ready" 200 30; then
		tail -60 "$EVIDENCE/app.log" >&2 || true
		fail "compiled cmd/app never reported ready"
	fi
	pass "production app and SSR sidecar running"
}

check_home() {
	http_get home-anonymous "$APP_URL/"
	check_code home-anonymous 200
	check_header_absent home-anonymous Location
	check_contains home-anonymous '"component":"Welcome"'
	check_contains home-anonymous 'data-server-rendered="true"'
	check_contains home-anonymous 'Space-grade Go, wired locally.'
	check_contains home-anonymous '"isAuthenticated":false'

	http_get signin-anonymous "$APP_URL/users/sign-in"
	check_code signin-anonymous 200
	check_header_absent signin-anonymous Location
	check_contains signin-anonymous '"component":"Auth\/Login"'
	check_contains signin-anonymous 'data-server-rendered="true"'
	pass "anonymous home and sign-in are public and server-rendered"
}

check_auth() {
	local csrf
	local session_pair
	csrf="$(cookie_value "$EVIDENCE/home-anonymous.headers" "_csrf")"
	if [[ -z "$csrf" ]]; then
		fail "anonymous home did not issue a CSRF cookie"
	fi
	http_post_json login "$APP_URL/users/sign-in" \
		'{"email":"admin@example.com","password":"password"}' "$csrf"
	check_code login 302
	check_header login Location "/"
	session_pair="$(cookie_pair_prefix "$EVIDENCE/login.headers" "app_sess_")"
	if [[ -z "$session_pair" ]]; then
		fail "login did not set an application session cookie"
	fi
	record_cmd "curl GET $APP_URL/ (seeded session cookie redacted)"
	set +e
	curl -sS --max-time 20 -D "$EVIDENCE/home-authenticated.headers" \
		-o "$EVIDENCE/home-authenticated.body" -w '%{http_code}' \
		-H "Cookie: $session_pair" "$APP_URL/" \
		>"$EVIDENCE/home-authenticated.code" 2>"$EVIDENCE/home-authenticated.curl.stderr"
	local code=$?
	set -e
	if ((code != 0)); then
		fail "authenticated home request failed (exit $code)"
	fi
	check_code home-authenticated 200
	check_contains home-authenticated 'data-server-rendered="true"'
	check_contains home-authenticated '"isAuthenticated":true'
	check_not_contains home-authenticated 'Create account'

	record_cmd "curl GET $APP_URL/users/sign-in (seeded session cookie redacted)"
	set +e
	curl -sS --max-time 20 -D "$EVIDENCE/signin-authenticated.headers" \
		-o "$EVIDENCE/signin-authenticated.body" -w '%{http_code}' \
		-H "Cookie: $session_pair" "$APP_URL/users/sign-in" \
		>"$EVIDENCE/signin-authenticated.code" 2>"$EVIDENCE/signin-authenticated.curl.stderr"
	code=$?
	set -e
	if ((code != 0)); then
		fail "authenticated sign-in request failed (exit $code)"
	fi
	check_code signin-authenticated 200
	check_contains signin-authenticated '"component":"Auth\/Login"'
	pass "seeded login and authenticated SSR session preserved"
}

check_readiness() {
	http_get ready-initial "$APP_URL/api/ready"
	check_code ready-initial 200
	check_contains ready-initial '"status":"ready"'

	stop_process "$SSR_PID"
	SSR_PID=""
	if ! wait_http_code "$APP_URL/api/ready" 503 30; then
		fail "readiness did not report unready after the SSR sidecar stopped"
	fi
	http_get ready-ssr-outage "$APP_URL/api/ready"
	check_code ready-ssr-outage 503
	check_contains ready-ssr-outage '"status":"unready"'
	check_contains ready-ssr-outage '"reason":"unavailable"'

	http_get home-ssr-outage "$APP_URL/"
	check_code home-ssr-outage 500
	check_contains home-ssr-outage '"component":"Errors\/InternalError"'
	check_contains home-ssr-outage 'data-page="app"'
	check_contains home-ssr-outage 'rel="stylesheet"'
	check_not_contains home-ssr-outage 'data-server-rendered="true"'
	check_not_contains home-ssr-outage 'Internal Server Error'
	# Explicit CSR fallback is a separate opt-in policy, never the production
	# default. Verify the compiled app with an environment override only.
	local fallback_port fallback_url
	fallback_port="$(free_port)" || fail "no free CSR fallback port"
	fallback_url="http://127.0.0.1:$fallback_port"
	start_process_bg "$APP" "$EVIDENCE/csr-fallback.log" \
		env PORT="$fallback_port" HOST_PRIMARY="127.0.0.1:$fallback_port" \
		INERTIA_SSR_FAIL_FAST=false ./dist/app
	CRASH_PID="$PROC_PID"
	wait_http_code "$fallback_url/api/ready" 200 30 || fail "explicit CSR fallback never became ready"
	http_get fallback-home "$fallback_url/"
	check_code fallback-home 200
	check_not_contains fallback-home 'data-server-rendered="true"'
	check_contains fallback-home '"component":"Welcome"'
	stop_process "$CRASH_PID"
	CRASH_PID=""

	record_cmd "start $APP/dist/ssr after outage (cwd $APP)"
	start_process_bg "$APP" "$EVIDENCE/ssr-recovered.log" ./dist/ssr
	SSR_PID="$PROC_PID"
	if ! wait_ssr_health "$SSR_PORT" 30; then
		tail -40 "$EVIDENCE/ssr-recovered.log" >&2 || true
		fail "SSR sidecar did not become healthy after restart"
	fi
	if ! wait_http_code "$APP_URL/api/ready" 200 30; then
		fail "readiness did not recover after the SSR sidecar restarted"
	fi
	http_get ready-ssr-recovered "$APP_URL/api/ready"
	check_code ready-ssr-recovered 200
	check_contains ready-ssr-recovered '"status":"ready"'
	http_get home-ssr-recovered "$APP_URL/"
	check_code home-ssr-recovered 200
	check_contains home-ssr-recovered 'data-server-rendered="true"'
	check_contains home-ssr-recovered 'Space-grade Go, wired locally.'
	# A healthy /health endpoint must not hide a render failure as HTTP 200.
	stop_process "$SSR_PID"
	SSR_PID=""
	start_process_bg "$SCRATCH" "$EVIDENCE/render-failure-sidecar.log" \
		node -e 'require("node:http").createServer((req,res)=>{if(req.url==="/health"){res.setHeader("Content-Type","application/json");res.end(JSON.stringify({status:"ok"}));}else{res.statusCode=500;res.end("intentional acceptance render failure");}}).listen(Number(process.argv[1]),"127.0.0.1")' "$SSR_PORT"
	SSR_PID="$PROC_PID"
	wait_ssr_health "$SSR_PORT" 30 || fail "render-failure fixture never became healthy"
	http_get render-failure-ready "$APP_URL/api/ready"
	check_code render-failure-ready 200
	http_get render-failure-home "$APP_URL/"
	check_code render-failure-home 500
	check_contains render-failure-home '"component":"Errors\/InternalError"'
	check_not_contains render-failure-home 'data-server-rendered="true"'
	stop_process "$SSR_PID"
	SSR_PID=""
	start_process_bg "$APP" "$EVIDENCE/ssr-after-render-failure.log" ./dist/ssr
	SSR_PID="$PROC_PID"
	wait_ssr_health "$SSR_PORT" 30 || fail "SSR failed to recover after render failure fixture"

	record_cmd "docker stop -t 5 $PG_NAME"
	set +e
	docker stop -t 5 "$PG_NAME" >"$EVIDENCE/12-postgres-stop.stdout" 2>"$EVIDENCE/12-postgres-stop.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/12-postgres-stop.status"
	if ((code != 0)); then
		fail "could not stop the disposable PostgreSQL container"
	fi
	if ! wait_http_code "$APP_URL/api/ready" 503 60; then
		fail "readiness did not report unready after PostgreSQL stopped"
	fi
	http_get ready-db-outage "$APP_URL/api/ready"
	check_code ready-db-outage 503
	check_contains ready-db-outage '"status":"unready"'
	check_contains ready-db-outage '"reason":"database_unavailable"'

	record_cmd "docker start $PG_NAME"
	set +e
	docker start "$PG_NAME" >"$EVIDENCE/13-postgres-start.stdout" 2>"$EVIDENCE/13-postgres-start.stderr"
	code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/13-postgres-start.status"
	if ((code != 0)); then
		fail "could not restart the disposable PostgreSQL container"
	fi
	if ! wait_pg 90; then
		fail "PostgreSQL did not become ready after restart"
	fi
	if ! wait_http_code "$APP_URL/api/ready" 200 60; then
		fail "readiness did not recover after PostgreSQL restarted"
	fi
	http_get ready-db-recovered "$APP_URL/api/ready"
	check_code ready-db-recovered 200
	check_contains ready-db-recovered '"status":"ready"'
	http_get home-db-recovered "$APP_URL/"
	check_code home-db-recovered 200
	check_contains home-db-recovered 'data-server-rendered="true"'
	pass "readiness outage and recovery for SSR and PostgreSQL"
}

check_ssr_failures() {
	record_cmd "timeout 30 ./dist/ssr with duplicate INERTIA_SSR_LISTEN=$SSR_URL"
	set +e
	(
		cd "$APP"
		env INERTIA_SSR_LISTEN="$SSR_URL" INERTIA_SSR_URL="$SSR_URL" \
			timeout 30 ./dist/ssr
	) >"$EVIDENCE/14-ssr-duplicate.stdout" 2>"$EVIDENCE/14-ssr-duplicate.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/14-ssr-duplicate.status"
	if ((code == 0)); then
		fail "compiled cmd/ssr accepted a duplicate listener"
	fi
	if ((code == 124)); then
		fail "compiled cmd/ssr hung on a duplicate listener"
	fi
	if ! grep -Eq 'EADDRINUSE|address already in use' "$EVIDENCE/14-ssr-duplicate.stdout" "$EVIDENCE/14-ssr-duplicate.stderr"; then
		fail "duplicate-listener failure did not report an address-in-use diagnostic"
	fi
	pass "compiled cmd/ssr exits nonzero on a duplicate listener"

	local crash_port
	crash_port="$(free_port)" || fail "no free SSR crash-test port"
	record_cmd "start $APP/dist/ssr crash test on 127.0.0.1:$crash_port (cwd $APP)"
	start_process_bg "$APP" "$EVIDENCE/15-ssr-crash.log" \
		env INERTIA_SSR_LISTEN="http://127.0.0.1:$crash_port" \
		INERTIA_SSR_URL="http://127.0.0.1:$crash_port" ./dist/ssr
	CRASH_PID="$PROC_PID"
	if ! wait_ssr_health "$crash_port" 30; then
		tail -40 "$EVIDENCE/15-ssr-crash.log" >&2 || true
		fail "crash-test SSR sidecar did not become healthy"
	fi
	local child
	child="$(pgrep -P "$CRASH_PID" | head -1 || true)"
	if [[ -z "$child" ]]; then
		fail "crash-test SSR sidecar did not start a JavaScript child"
	fi
	kill -KILL "$child"
	set +e
	wait "$CRASH_PID"
	code=$?
	set -e
	CRASH_PID=""
	printf '%s\n' "$code" >"$EVIDENCE/15-ssr-crash.status"
	if ((code == 0)); then
		fail "compiled cmd/ssr exited 0 after its JavaScript child crashed"
	fi
	pass "compiled cmd/ssr exits nonzero after its JavaScript child crashes"
}

manifest_tree() {
	local dir="$1"
	(
		cd "$dir"
		find assets/dist dist -type f -printf '%p %s %T@\n' 2>/dev/null | sort | sha256sum
	)
}

check_isolated_builds() {
	record_cmd "git -C $APP add -A && git commit (scratch baseline)"
	set +e
	(
		cd "$APP"
		git add -A
		git -c user.email=acceptance@example.invalid -c user.name=acceptance \
			commit -qm "acceptance baseline"
	) >"$EVIDENCE/16-baseline.stdout" 2>"$EVIDENCE/16-baseline.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$EVIDENCE/16-baseline.status"
	if ((code != 0)); then
		fail "could not commit the scratch source baseline"
	fi
	local before
	local after
	before="$(manifest_tree "$APP")"
	printf '%s\n' "$before" >"$EVIDENCE/16-before-manifest.txt"
	if [[ -n "$(git -C "$APP" status --porcelain)" ]]; then
		git -C "$APP" status --porcelain >"$EVIDENCE/16-before-status.txt"
		fail "scratch source tree is dirty before isolated builds"
	fi

	record_cmd "concurrent: andurel build --json --version live-a --output-dir $SCRATCH/builds/a"
	record_cmd "concurrent: andurel build --json --version live-b --output-dir $SCRATCH/builds/b"
	(
		cd "$APP"
		"$ANDUREL_BIN" build --json --version live-a --output-dir "$SCRATCH/builds/a" \
			>"$EVIDENCE/17-build-a.json" 2>"$EVIDENCE/17-build-a.stderr"
		printf '%s\n' "$?" >"$EVIDENCE/17-build-a.status"
	) &
	local pid_a=$!
	(
		cd "$APP"
		"$ANDUREL_BIN" build --json --version live-b --output-dir "$SCRATCH/builds/b" \
			>"$EVIDENCE/17-build-b.json" 2>"$EVIDENCE/17-build-b.stderr"
		printf '%s\n' "$?" >"$EVIDENCE/17-build-b.status"
	) &
	local pid_b=$!
	set +e
	wait "$pid_a"
	local code_a=$?
	wait "$pid_b"
	local code_b=$?
	set -e
	if ((code_a != 0)) || [[ "$(cat "$EVIDENCE/17-build-a.status")" != "0" ]]; then
		tail -40 "$EVIDENCE/17-build-a.stderr" >&2 || true
		fail "isolated build A failed"
	fi
	if ((code_b != 0)) || [[ "$(cat "$EVIDENCE/17-build-b.status")" != "0" ]]; then
		tail -40 "$EVIDENCE/17-build-b.stderr" >&2 || true
		fail "isolated build B failed"
	fi
	jq -e '.ok == true and .data.isolated == true and .data.version == "live-a" and
		([.data.artifacts[].process] | sort) == ["app", "migrate", "queue", "ssr"]' \
		"$EVIDENCE/17-build-a.json" >/dev/null ||
		fail "isolated build A manifest is not isolated or incomplete"
	jq -e '.ok == true and .data.isolated == true and .data.version == "live-b" and
		([.data.artifacts[].process] | sort) == ["app", "migrate", "queue", "ssr"]' \
		"$EVIDENCE/17-build-b.json" >/dev/null ||
		fail "isolated build B manifest is not isolated or incomplete"

	local dir
	local process
	for dir in a b; do
		for process in app migrate queue ssr; do
			if [[ ! -x "$SCRATCH/builds/$dir/$process" ]]; then
				fail "isolated build $dir is missing executable $process"
			fi
		done
	done
	if [[ "$(jq -r '.data.output_dir' "$EVIDENCE/17-build-a.json")" == \
		"$(jq -r '.data.output_dir' "$EVIDENCE/17-build-b.json")" ]]; then
		fail "concurrent isolated builds published to the same output directory"
	fi

	after="$(manifest_tree "$APP")"
	printf '%s\n' "$after" >"$EVIDENCE/17-after-manifest.txt"
	if [[ "$before" != "$after" ]]; then
		fail "isolated builds changed the in-place assets/dist or dist tree"
	fi
	if [[ -n "$(git -C "$APP" status --porcelain)" ]]; then
		git -C "$APP" status --porcelain >"$EVIDENCE/17-after-status.txt"
		fail "isolated builds changed the scratch source tree"
	fi
	pass "concurrent isolated builds are independent and leave the source tree untouched"
	# Execute both output pairs without source assets or node_modules present.
	# An executable bit and a successful build alone do not prove embedding.
	local runtime app_port ssr_port url
	for dir in a b; do
		runtime="$SCRATCH/run-$dir"
		mkdir -p "$runtime"
		app_port="$(free_port)" || fail "no free isolated app port"
		ssr_port="$(free_port)" || fail "no free isolated SSR port"
		while [[ "$ssr_port" == "$app_port" ]]; do
			ssr_port="$(free_port)" || fail "no distinct isolated SSR port"
		done
		sed -E \
			-e "s/^PORT=.*/PORT=$app_port/" \
			-e "s/^HOST_PRIMARY=.*/HOST_PRIMARY=127.0.0.1:$app_port/" \
			-e "s|^INERTIA_SSR_LISTEN=.*|INERTIA_SSR_LISTEN=http://127.0.0.1:$ssr_port|" \
			-e "s|^INERTIA_SSR_URL=.*|INERTIA_SSR_URL=http://127.0.0.1:$ssr_port|" \
			"$APP/.env" >"$runtime/.env"
		chmod 600 "$runtime/.env"
		start_process_bg "$runtime" "$EVIDENCE/18-isolated-$dir-ssr.log" "$SCRATCH/builds/$dir/ssr"
		SSR_PID="$PROC_PID"
		wait_ssr_health "$ssr_port" 30 || fail "isolated $dir SSR cannot run without source assets"
		start_process_bg "$runtime" "$EVIDENCE/18-isolated-$dir-app.log" "$SCRATCH/builds/$dir/app"
		APP_PID="$PROC_PID"
		url="http://127.0.0.1:$app_port"
		wait_http_code "$url/api/ready" 200 30 || fail "isolated $dir app cannot become ready"
		kill -0 "$APP_PID" "$SSR_PID" || fail "isolated $dir intended processes exited"
		http_get "isolated-$dir-home" "$url/"
		check_code "isolated-$dir-home" 200
		check_contains "isolated-$dir-home" 'data-server-rendered="true"'
		check_contains "isolated-$dir-home" 'Space-grade Go, wired locally.'
		stop_process "$APP_PID"
		APP_PID=""
		stop_process "$SSR_PID"
		SSR_PID=""
	done
	pass "both isolated output pairs run self-contained without source assets"
}

write_summary() {
	{
		printf '# Beta.3 live fresh-scaffold gate\n\n'
		printf 'status: pass\n\n'
		printf 'candidate: `%s`\n\n' "$CANDIDATE_SHA"
		printf 'Scratch: `%s` (removed unless --keep-scratch)\n\n' "$SCRATCH"
		printf '## Checks\n\n'
		printf '| Check | Result |\n| --- | --- |\n'
		while IFS=$'\t' read -r result name; do
			[[ -n "$result" ]] || continue
			printf '| %s | %s |\n' "$name" "$result"
		done <"$EVIDENCE/checks.tsv"
	} >"$EVIDENCE/live.md"
	printf 'pass\n' >"$EVIDENCE/live-state.txt"
	log "live gate passed; evidence: $EVIDENCE"
}

main() {
	cd "$ROOT"
	write_environment
	check_prereqs
	prepare_scratch
	build_cli
	generate_app
	replace_modules
	tidy_app
	sync_tools
	install_frontend
	verify_after_setup
	build_app
	start_database
	configure_env
	migrate
	seed
	start_runtime
	check_home
	check_auth
	check_readiness
	check_ssr_failures
	stop_process "$APP_PID"
	APP_PID=""
	stop_process "$SSR_PID"
	SSR_PID=""
	check_isolated_builds
	write_summary
}

main "$@"
