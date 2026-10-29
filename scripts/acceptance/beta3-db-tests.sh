#!/usr/bin/env bash
# shellcheck disable=SC2016,SC2329
# Beta.3 F20 database-test gate.
#
# Proves that a freshly generated React/pnpm application, built from this
# checkout's CLI and pointed at this checkout's pkg/* modules through temporary
# go.mod replaces (development proof only), builds its frontend assets and then
# actually executes the generated named database tests. Docker must be
# available: the generated TestMain helpers start disposable PostgreSQL
# containers. When Docker is unavailable the gate records a genuine blocker and
# exits 77; it never reports a pass.
#
# The direct test run uses -count=1 and -p 1. -count=1 forbids Go's test cache;
# -p 1 is the documented Ryuk workaround for testcontainers' shared reaper
# session. The follow-up verify run uses a scratch GOCACHE and disables Ryuk
# (TESTCONTAINERS_RYUK_DISABLED=true) because verify runs packages in parallel
# and does not expose -p.
#
# Usage:
#   scripts/acceptance/beta3-db-tests.sh [--mode dev|release]
#                                        [--evidence-dir DIR]
#                                        [--keep-scratch]
#
# Modes:
#   dev      Development proof. Temporary go.mod replaces point the generated
#            app at this checkout's pkg/* modules. This is not release-shaped
#            proof.
#   release  No replaces. If the candidate pkg pin is unpublished, the gate
#            records the release blocker instead of a pass.
#
# Exit status: 0 pass, 1 failure, 77 blocked (missing tool, no Docker, or an
# unavailable module/frontend dependency). Evidence is written even when the
# gate is blocked or fails.
set -Eeuo pipefail
IFS=$'\n\t'
umask 022

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE=dev
EVIDENCE=""
KEEP_SCRATCH=0
SCRATCH=""
A=""
APP=""
APP_NAME=""
CANDIDATE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
CANDIDATE_VERSION="${ANDUREL_ACCEPTANCE_VERSION:-v2.0.0-beta.4}"

# Generated database tests. Each one calls testCluster.NewTestDB (or
# NewTestDBWithConfig through testConfig), so a pass proves a real migrated
# PostgreSQL database was created and used.
NAMED_TESTS=(
	TestUsers_CreateThenFind
	TestUsers_CreateRejectsDuplicateEmail
	TestUsers_BumpSessionVersion
	TestUsers_ChangePasswordBumpsSessionVersion
	TestUsers_ConcurrentChangePasswordAndValidationBothPersist
	TestTokens_DestroyExpired
	TestSessionNewRendersLogin
	TestSessionCreateRejectsBadCredentials
	TestRegistrationDuplicateEmailBehavesLikeSuccess
	TestPasswordUpdateRejectsInvalidTokenGenerically
	TestAuthOnlyRejectsStaleSessionVersion
	TestPasswordResetInvalidatesExistingSession
	TestVerifyEmailRejectsMissingPendingUser
	TestVerificationAttemptsAreBoundedByTheLimit
	TestHomePageIsPublic
)
declare -A TEST_PACKAGE=(
	[TestUsers_CreateThenFind]=domains
	[TestUsers_CreateRejectsDuplicateEmail]=domains
	[TestUsers_BumpSessionVersion]=domains
	[TestUsers_ChangePasswordBumpsSessionVersion]=domains
	[TestUsers_ConcurrentChangePasswordAndValidationBothPersist]=domains
	[TestTokens_DestroyExpired]=domains
	[TestSessionNewRendersLogin]=handlers
	[TestSessionCreateRejectsBadCredentials]=handlers
	[TestRegistrationDuplicateEmailBehavesLikeSuccess]=handlers
	[TestPasswordUpdateRejectsInvalidTokenGenerically]=handlers
	[TestAuthOnlyRejectsStaleSessionVersion]=handlers
	[TestPasswordResetInvalidatesExistingSession]=handlers
	[TestVerifyEmailRejectsMissingPendingUser]=services
	[TestVerificationAttemptsAreBoundedByTheLimit]=services
	[TestHomePageIsPublic]=internal/runtime
)

usage() {
	cat <<'EOF'
Usage: scripts/acceptance/beta3-db-tests.sh [options]

Options:
  --mode dev|release   dev (default) uses temporary pkg/* replaces; release
                       uses no replaces and records the unpublished-pin blocker.
  --evidence-dir DIR   write evidence to DIR instead of
                       plans/evidence/beta3-db-tests/<commit>-<mode>
  --keep-scratch       keep the temporary scaffold, module cache and binaries
  -h, --help           show this help

Exit status: 0 pass, 1 failure, 77 blocked.
EOF
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
	--keep-scratch)
		KEEP_SCRATCH=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
done

case "$MODE" in
dev | release) ;;
*)
	printf -- '--mode must be dev or release\n' >&2
	exit 2
	;;
esac

if [[ -z "$EVIDENCE" ]]; then
	EVIDENCE="$ROOT/plans/evidence/beta3-db-tests/${CANDIDATE_SHA:0:12}-$MODE"
fi

log() { printf '== %s\n' "$*" >&2; }

cleanup() {
	local status=$?
	if [[ -n "$SCRATCH" && -d "$SCRATCH" && "$KEEP_SCRATCH" -eq 0 ]]; then
		rm -rf "$SCRATCH"
	fi
	return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

step_status() {
	cat "$EVIDENCE/$1.status" 2>/dev/null || printf 'missing\n'
}

append_step() {
	printf '%s\t%s\n' "$1" "$2" >>"$EVIDENCE/steps.tsv"
}

# run_recorded runs one command with a fresh evidence prefix and never lets
# set -e abort the caller before the status is recorded. Callers that need a
# controlled environment prefix the command with `env KEY=VALUE`; the recorded
# command is then exact and paths with spaces stay intact.
run_recorded() {
	local name="$1"
	local dir="$2"
	shift 2
	local prefix="$EVIDENCE/$name"
	{
		printf 'cwd=%s\n' "$dir"
		printf 'cmd='
		printf '%q ' timeout --kill-after=15 "${PROBE_TIMEOUT:-900}" "$@"
		printf '\n'
	} >"$prefix.command"

	set +e
	(
		cd "$dir"
		timeout --kill-after=15 "${PROBE_TIMEOUT:-900}" "$@"
	) >"$prefix.stdout" 2>"$prefix.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$prefix.status"
	case "$name" in
	*.info) ;;
	*) append_step "$name" "$code" ;;
	esac
}

assert_status() {
	local name="$1"
	local want="$2"
	local got
	got="$(step_status "$name")"
	if [[ "$got" != "$want" ]]; then
		printf 'expected %s exit %s, got %s\n' "$name" "$want" "$got" >&2
		printf -- '--- stdout ---\n' >&2
		head -80 "$EVIDENCE/$name.stdout" >&2 || true
		printf -- '--- stderr ---\n' >&2
		head -80 "$EVIDENCE/$name.stderr" >&2 || true
		return 1
	fi
}

finish() {
	local result="$1"
	local code="$2"
	local reason="${3:-}"
	printf '%s\n' "$result" >"$EVIDENCE/result.txt"
	{
		printf '# Beta.3 F20 database-test gate\n\n'
		printf -- '- result: **%s**\n' "$result"
		printf -- '- mode: %s\n' "$MODE"
		if [[ "$MODE" == dev ]]; then
			printf -- '- proof shape: development only; temporary `pkg/*` replaces are not release-shaped proof\n'
		fi
		printf -- '- candidate: %s\n' "$CANDIDATE_SHA"
		printf -- '- evidence: %s\n' "$EVIDENCE"
		if [[ -n "$reason" ]]; then
			printf -- '- reason: %s\n' "$reason"
		fi
		printf '\n## Steps\n\n| Step | Exit |\n| --- | --- |\n'
		if [[ -f "$EVIDENCE/steps.tsv" ]]; then
			while IFS=$'\t' read -r name status; do
				[[ -n "$name" ]] || continue
				printf '| %s | %s |\n' "$name" "$status"
			done <"$EVIDENCE/steps.tsv"
		fi
		printf '\nEvidence in this directory:\n\n'
		for artifact in environment.txt candidate-commit.txt candidate-status.txt \
			app-name.txt steps.tsv named-db-passes.tsv named-db-pass-events.txt \
			verify-go-test.json verify-package-totals.txt verify-totals.txt; do
			if [[ -f "$EVIDENCE/$artifact" ]]; then
				printf -- '- `%s`\n' "$artifact"
			fi
		done
		printf -- '- `*.command`, `*.stdout`, `*.stderr`, `*.status` for every recorded step\n'
	} >"$EVIDENCE/summary.md"

	if [[ "$result" == pass ]]; then
		log "PASS: $reason"
	else
		log "${result^^}: $reason"
	fi
	exit "$code"
}

command_value() {
	local fallback="$1"
	shift
	local out
	out="$("$@" 2>/dev/null || true)"
	out="${out//$'\n'/ }"
	out="${out#"${out%%[![:space:]]*}"}"
	out="${out%"${out##*[![:space:]]}"}"
	if [[ -z "$out" ]]; then
		out="$fallback"
	fi
	printf '%s' "$out"
}

write_environment() {
	{
		printf 'candidate_sha=%s\n' "$CANDIDATE_SHA"
		printf 'candidate_version=%s (unpublished acceptance build)\n' "$CANDIDATE_VERSION"
		printf 'branch=%s\n' "$(command_value unavailable git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
		printf 'candidate_dirty_files=%s\n' "$(git -C "$ROOT" status --porcelain | wc -l)"
		printf 'mode=%s\n' "$MODE"
		printf 'go=%s\n' "$(command_value unavailable go version)"
		printf 'node=%s\n' "$(command_value unavailable node --version)"
		printf 'pnpm=%s\n' "$(command_value unavailable pnpm --version)"
		printf 'docker_client=%s\n' "$(command_value unavailable docker version --format '{{.Client.Version}}')"
		printf 'docker_server=%s\n' "$(command_value unavailable docker version --format '{{.Server.Version}}')"
		printf 'jq=%s\n' "$(command_value unavailable jq --version)"
		printf 'os=%s\n' "$(command_value unavailable uname -srm)"
		printf 'date=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	} >"$EVIDENCE/environment.txt"
	git -C "$ROOT" log -1 --format='%H %s' >"$EVIDENCE/candidate-commit.txt"
	git -C "$ROOT" status --porcelain >"$EVIDENCE/candidate-status.txt"
}

prepare() {
	local base="${TMPDIR:-/tmp/opencode}"
	mkdir -p "$base"
	SCRATCH="$(mktemp -d "$base/andurel-db-tests-XXXXXX")"
	mkdir -p "$SCRATCH/bin" "$SCRATCH/apps" "$SCRATCH/gocache" "$SCRATCH/gotmp"
	log "scratch: $SCRATCH"
	log "evidence: $EVIDENCE"

	run_recorded 01-build-cli "$ROOT" env GOWORK=off go build -ldflags "-X main.version=$CANDIDATE_VERSION" -o "$SCRATCH/bin/andurel" .
	if ! assert_status 01-build-cli 0; then
		finish fail 1 "building the candidate CLI failed"
	fi
	A="$SCRATCH/bin/andurel"
	run_recorded 01b-cli-version "$ROOT" "$A" --version
	if ! assert_status 01b-cli-version 0; then
		finish fail 1 "candidate CLI --version failed"
	fi
}

preflight_tools() {
	local missing=()
	local cmd
	for cmd in go node pnpm docker jq; do
		command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
	done
	if [[ "${#missing[@]}" -gt 0 ]]; then
		local joined
		joined="$(printf '%s ' "${missing[@]}")"
		finish blocked 77 "missing required tools: ${joined% }"
	fi
}

docker_preflight() {
	run_recorded 02-docker-server "$ROOT" docker version --format '{{.Server.Version}}'
	if ! assert_status 02-docker-server 0; then
		finish blocked 77 "Docker server is unavailable; generated database tests cannot run (see 02-docker-server.stderr)"
	fi
	if [[ ! -s "$EVIDENCE/02-docker-server.stdout" ]]; then
		finish blocked 77 "Docker server version is empty; cannot prove database tests ran"
	fi
	# A missing image is not a blocker: testcontainers may pull it. Record the
	# local image identity when it exists, and let a failed pull surface below.
	run_recorded 02b-postgres-image.info "$ROOT" docker image inspect postgres:17-alpine --format '{{.Id}}'
}

generate_app() {
	APP_NAME="dbtests$(date -u +%Y%m%d%H%M%S)$$"
	APP="$SCRATCH/apps/$APP_NAME"
	run_recorded 03-new "$SCRATCH/apps" "$A" new "$APP_NAME"
	if ! assert_status 03-new 0; then
		finish fail 1 "andurel new failed (see 03-new.stdout/stderr)"
	fi
	if [[ ! -f "$APP/go.mod" ]]; then
		finish fail 1 "andurel new did not create $APP/go.mod"
	fi
	if ! grep -Fq "inertia = 'react'" "$APP/andurel.toml"; then
		finish fail 1 "generated app is not the expected React scaffold"
	fi
	if ! grep -Fq "javascriptPackageManager = 'pnpm'" "$APP/andurel.toml"; then
		finish fail 1 "generated app is not the expected pnpm scaffold"
	fi
	if ! grep -Fq '"@inertiajs/react"' "$APP/package.json"; then
		finish fail 1 "generated package.json does not use @inertiajs/react"
	fi
	printf '%s\n' "$APP_NAME" >"$EVIDENCE/app-name.txt"
	printf '%s\n' "$APP" >"$EVIDENCE/app-path.txt"
}

apply_local_replaces() {
	if [[ "$MODE" == release ]]; then
		local inertia_version
		inertia_version="$(awk -F '"' '/^[[:space:]]*Inertia = / { print $2 }' "$ROOT/layout/versions/versions.go")"
		if [[ -z "$inertia_version" ]]; then
			finish fail 1 "cannot determine candidate Inertia version"
		fi
		if ! git -C "$ROOT" show-ref --verify --quiet "refs/tags/pkg/inertia/$inertia_version"; then
			finish blocked 77 "release gate: pkg/inertia/$inertia_version tag is absent; do not fetch an unpublished candidate"
		fi
		run_recorded 04-go-mod-tidy "$APP" env GOWORK=off go mod tidy
		if [[ "$(step_status 04-go-mod-tidy)" != 0 ]]; then
			finish blocked 77 "release-shaped go mod tidy failed; the candidate pkg/inertia pin is likely unpublished (see 04-go-mod-tidy.stderr)"
		fi
		return
	fi

	local prefix="$EVIDENCE/04-local-replace"
	{
		printf 'cwd=%s\n' "$APP"
		printf '# temporary development replaces; not release-shaped proof\n'
	} >"$prefix.command"
	: >"$prefix.stdout"
	: >"$prefix.stderr"
	local module_dir module rc=0
	for module_dir in "$ROOT"/pkg/*/; do
		[[ -f "${module_dir}go.mod" ]] || continue
		module="$(basename "$module_dir")"
		printf 'go mod edit -replace %q\n' "github.com/mbvlabs/andurel/pkg/${module}=${module_dir%/}" >>"$prefix.command"
		if ! (
			cd "$APP"
			go mod edit -replace "github.com/mbvlabs/andurel/pkg/${module}=${module_dir%/}"
		) >>"$prefix.stdout" 2>>"$prefix.stderr"; then
			rc=1
		fi
	done
	printf '%s\n' "$rc" >"$prefix.status"
	append_step 04-local-replace "$rc"
	if [[ "$rc" != 0 ]]; then
		finish fail 1 "adding temporary local module replaces failed"
	fi
	# A development replace must never be committed back to the framework.
	if git -C "$ROOT" grep -Fq -- "$SCRATCH"; then
		finish fail 1 "machine-specific scratch path leaked into the repository"
	fi

	run_recorded 04b-go-mod-tidy "$APP" env GOWORK=off go mod tidy
	if [[ "$(step_status 04b-go-mod-tidy)" != 0 ]]; then
		finish blocked 77 "go mod tidy needs module downloads for this candidate; network or module cache unavailable (see 04b-go-mod-tidy.stderr)"
	fi
}

install_frontend() {
	# A fresh scaffold has no pnpm-lock.yaml yet; pnpm install creates it.
	run_recorded 05-pnpm-install "$APP" pnpm install
	if [[ "$(step_status 05-pnpm-install)" != 0 ]]; then
		finish blocked 77 "pnpm install failed; frontend assets cannot be built (see 05-pnpm-install.stderr)"
	fi
	if [[ ! -d "$APP/node_modules" ]]; then
		finish fail 1 "pnpm install succeeded but node_modules is missing"
	fi
}

build_frontend() {
	run_recorded 06-pnpm-build "$APP" pnpm build
	if [[ "$(step_status 06-pnpm-build)" != 0 ]]; then
		finish fail 1 "pnpm build failed; generated frontend assets are required before the DB tests (see 06-pnpm-build.stderr)"
	fi
	if [[ ! -f "$APP/assets/dist/vite/manifest.json" ]]; then
		finish fail 1 "pnpm build did not produce assets/dist/vite/manifest.json"
	fi
	if [[ -z "$(find "$APP/assets/dist" -type f -print -quit)" ]]; then
		finish fail 1 "pnpm build produced no frontend assets"
	fi
}

run_named_db_tests() {
	local regex="^("
	local test
	for test in "${NAMED_TESTS[@]}"; do
		regex+="$test|"
	done
	regex="${regex%|})$"

	run_recorded 07-named-db-tests "$APP" \
		env GOWORK=off GOCACHE="$SCRATCH/gocache" GOTMPDIR="$SCRATCH/gotmp" \
		go test -count=1 -p 1 -v -run "$regex" \
		./domains/... ./handlers/... ./services/... ./internal/runtime/...

	local status
	status="$(step_status 07-named-db-tests)"
	if [[ "$status" != 0 ]]; then
		if grep -Eq 'Cannot connect to the Docker daemon|error during connect|failed to resolve reference|pull access denied|no such host' \
			"$EVIDENCE/07-named-db-tests.stdout" "$EVIDENCE/07-named-db-tests.stderr" 2>/dev/null; then
			finish blocked 77 "generated database tests could not start PostgreSQL containers (see 07-named-db-tests.stdout/stderr)"
		fi
		finish fail 1 "generated named database tests failed (exit $status; see 07-named-db-tests.stdout/stderr)"
	fi

	# A Docker-less TestMain prints this and exits 0. Treat it as a failure,
	# never as a pass: the whole point of this gate is real database execution.
	if grep -Fq -- "skipping tests:" "$EVIDENCE/07-named-db-tests.stdout" "$EVIDENCE/07-named-db-tests.stderr"; then
		finish fail 1 "generated database tests reported a hidden skip instead of executing"
	fi
	if grep -Fq -- "--- SKIP:" "$EVIDENCE/07-named-db-tests.stdout"; then
		finish fail 1 "a selected database test reported SKIP instead of PASS"
	fi

	local missing=()
	for test in "${NAMED_TESTS[@]}"; do
		if ! grep -Fq -- "--- PASS: $test " "$EVIDENCE/07-named-db-tests.stdout"; then
			missing+=("$test")
		fi
	done
	if [[ "${#missing[@]}" -gt 0 ]]; then
		local joined
		joined="$(printf '%s ' "${missing[@]}")"
		finish fail 1 "named database tests did not report a pass event: ${joined% }"
	fi

	local pkg
	for pkg in domains handlers services internal/runtime; do
		if ! grep -Eq "^ok[[:space:]]+${APP_NAME}/${pkg}[[:space:]]" "$EVIDENCE/07-named-db-tests.stdout"; then
			finish fail 1 "go test did not report ok for ${APP_NAME}/${pkg}"
		fi
	done
	if ! grep -Fq "postgres:17-alpine" "$EVIDENCE/07-named-db-tests.stdout"; then
		finish fail 1 "go test output has no PostgreSQL container evidence"
	fi
	if ! grep -Fq "Container is ready" "$EVIDENCE/07-named-db-tests.stdout"; then
		finish fail 1 "go test output has no ready-container evidence"
	fi

	{
		printf 'package\ttest\tstatus\n'
		for test in "${NAMED_TESTS[@]}"; do
			printf '%s\t%s\tpass\n' "${TEST_PACKAGE[$test]}" "$test"
		done
	} >"$EVIDENCE/named-db-passes.tsv"
	grep -E -- '^--- PASS: ' "$EVIDENCE/07-named-db-tests.stdout" >"$EVIDENCE/named-db-pass-events.txt" || true
}

run_verify() {
	# verify runs packages in parallel and does not expose -p. Disable Ryuk for
	# this run so a finishing package cannot reap a sibling's PostgreSQL
	# container; Cluster.Close still terminates each package's own container.
	run_recorded 08-verify-with-tests "$APP" \
		env GOWORK=off GOCACHE="$SCRATCH/gocache" TESTCONTAINERS_RYUK_DISABLED=true \
		"$A" verify --only go-test --with-tests --json

	local status
	status="$(step_status 08-verify-with-tests)"
	if [[ "$status" != 0 ]]; then
		finish fail 1 "andurel verify --only go-test --with-tests failed (exit $status; see 08-verify-with-tests.stderr)"
	fi

	local step="$EVIDENCE/verify-go-test.json"
	if ! jq -e '.data.steps[] | select(.name == "go test")' \
		"$EVIDENCE/08-verify-with-tests.stdout" >"$step"; then
		finish fail 1 "verify JSON has no go test step"
	fi
	if ! jq -e '.status == "ok"' "$step" >/dev/null; then
		finish fail 1 "verify go test step is not ok"
	fi
	if ! jq -e '.totals.failed == 0' "$step" >/dev/null; then
		finish fail 1 "verify go test totals report failures"
	fi
	if ! jq -e ".totals.passed >= ${#NAMED_TESTS[@]}" "$step" >/dev/null; then
		finish fail 1 "verify go test totals report fewer passes than the named database tests"
	fi
	if ! jq -e '.totals.packages >= 4' "$step" >/dev/null; then
		finish fail 1 "verify go test totals report fewer than four packages"
	fi
	if ! jq -e '(.skipped_packages // []) | length == 0' "$step" >/dev/null; then
		finish fail 1 "verify reports skipped packages; generated DB tests may have hidden Docker skips"
	fi
	if ! jq -e '(.skipped_tests // []) | length == 0' "$step" >/dev/null; then
		finish fail 1 "verify reports skipped tests; the gate requires no hidden DB skips"
	fi
	if ! jq -e '.totals.skipped == 0' "$step" >/dev/null; then
		finish fail 1 "verify go test totals report skipped tests"
	fi

	local pkg full
	for pkg in domains handlers services internal/runtime; do
		full="$APP_NAME/$pkg"
		if ! jq -e --arg pkg "$full" '
			.packages[]
			| select(.package == $pkg)
			| .status == "ok"
				and .passed > 0
				and ((.no_tests_ran // false) == false)
				and ((.cached // false) == false)
		' "$step" >/dev/null; then
			finish fail 1 "verify package $full is not an uncached ok package with passing tests"
		fi
		if ! jq -e --arg pkg "$full" '
			[(.skipped_packages // [])[] | select(.package == $pkg)] | length == 0
		' "$step" >/dev/null; then
			finish fail 1 "verify lists $full as a skipped package"
		fi
	done

	jq -r '
		.packages[]
		| select(.package | test("/(domains|handlers|services|internal/runtime)$"))
		| "\(.package)\t\(.status)\tpassed=\(.passed)\tfailed=\(.failed)\tskipped=\(.skipped)\tcached=\(.cached // false)\tno_tests_ran=\(.no_tests_ran // false)"
	' "$step" >"$EVIDENCE/verify-package-totals.txt"
	jq -r '
		"packages=\(.totals.packages) passed=\(.totals.passed) failed=\(.totals.failed) skipped=\(.totals.skipped) subtests_passed=\(.totals.subtests_passed) no_test_files=\(.totals.no_test_files) duration_seconds=\(.totals.duration_seconds)"
	' "$step" >"$EVIDENCE/verify-totals.txt"
}

main() {
	cd "$ROOT"
	mkdir -p "$EVIDENCE"
	: >"$EVIDENCE/steps.tsv"
	write_environment
	preflight_tools
	prepare
	docker_preflight
	generate_app
	apply_local_replaces
	install_frontend
	build_frontend
	run_named_db_tests
	run_verify
	finish pass 0 "generated named database tests ran uncached and verify reported per-package totals with no hidden DB skips"
}

main "$@"
