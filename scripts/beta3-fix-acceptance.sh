#!/usr/bin/env bash
# Beta.3 complete-fix acceptance harness.
#
# This script is the durable, repeatable form of the handoff's fresh-scaffold
# protocol. It never reuses an existing application, lease directory, port, or
# build output: every run creates its own mktemp tree and records the candidate
# commit, tool versions, exact command, stdout, stderr, and exit status per
# probe.
#
# Usage:
#   scripts/beta3-fix-acceptance.sh [--mode dev|release] [--evidence-dir DIR]
#                                   [--full] [--require-all] [--keep-scratch]
#
# Modes:
#   dev      Development integration. A generated app is pointed at this
#            checkout's pkg/* modules through temporary go.mod replaces, so
#            unreleased candidate APIs compile. This is not release-shaped
#            proof.
#   release  Release-shaped. No replace directives and no candidate module
#            proxy. If the candidate pkg/inertia pin is unpublished, the
#            release gate is recorded blocked, never passed.
#
# Exit status: 0 when every executed probe passed. Blocked probes are reported
# and make the result PARTIAL, never a pass. With --require-all (full
# acceptance), any blocked probe exits 1. Any failed probe exits 1.
set -Eeuo pipefail
IFS=$'\n\t'
umask 022

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE=dev
FULL=0
REQUIRE_ALL=0
KEEP_SCRATCH=0
EVIDENCE=""
CANDIDATE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
SCRATCH=""
A=""
HEALTHCHILD=""
OWN_PIDS=()
RESULT_ROWS=()
FAILURES=0
BLOCKED=0
PASSED=0

usage() {
	sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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
	--full)
		FULL=1
		shift
		;;
	--require-all)
		REQUIRE_ALL=1
		shift
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
*)
	echo "--mode must be dev or release" >&2
	exit 2
	;;
esac

if [[ -z "$EVIDENCE" ]]; then
	EVIDENCE="$ROOT/plans/evidence/beta3-complete-fix/${CANDIDATE_SHA:0:12}-$MODE"
fi

log() { printf '== %s\n' "$*" >&2; }
blocked() {
	printf 'BLOCKED: %s\n' "$*" >&2
	return 77
}
fail() {
	printf 'FAIL: %s\n' "$*" >&2
	return 1
}

cleanup() {
	local pid
	for pid in "${OWN_PIDS[@]:-}"; do
		if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
			kill -TERM "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
		fi
	done
	sleep 0.2
	for pid in "${OWN_PIDS[@]:-}"; do
		if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
			kill -KILL "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
		fi
	done
	if [[ -n "$SCRATCH" && -d "$SCRATCH" && "$KEEP_SCRATCH" -eq 0 ]]; then
		rm -rf "$SCRATCH"
	fi
}
trap cleanup EXIT INT TERM

mkdir -p "$EVIDENCE"

# capture runs one command with a timeout, recording stdout/stderr/status.
# It never lets set -e abort the probe; the caller asserts the expected status.
capture() {
	local prefix="$1"
	shift
	set +e
	timeout "${PROBE_TIMEOUT:-180}" "$@" >"$prefix.stdout" 2>"$prefix.stderr"
	local code=$?
	set -e
	printf '%s\n' "$code" >"$prefix.status"
	printf '%s' "$code"
}

capture_in() {
	local dir="$1"
	local prefix="$2"
	shift 2
	(
		cd "$dir"
		capture "$prefix" "$@"
	)
}

assert_status() {
	local prefix="$1"
	local want="$2"
	local got
	got="$(cat "$prefix.status")"
	if [[ "$got" != "$want" ]]; then
		printf 'expected exit %s, got %s\n' "$want" "$got" >&2
		printf -- '--- stdout ---\n' >&2
		head -80 "$prefix.stdout" >&2
		printf -- '--- stderr ---\n' >&2
		head -80 "$prefix.stderr" >&2
		return 1
	fi
}

assert_contains() {
	local path="$1"
	local needle="$2"
	if ! grep -Fq -- "$needle" "$path"; then
		printf 'expected %s to contain %q\n' "$path" "$needle" >&2
		head -60 "$path" >&2
		return 1
	fi
}

assert_not_contains() {
	local path="$1"
	local needle="$2"
	if grep -Fq -- "$needle" "$path"; then
		printf 'expected %s not to contain %q\n' "$path" "$needle" >&2
		return 1
	fi
}

run_probe() {
	local name="$1"
	local fn="$2"
	local prefix="$EVIDENCE/$name"
	log "$name"
	set +e
	(
		set -e
		PROBE_PREFIX="$prefix"
		"$fn"
	) >"$prefix.probe.stdout" 2>"$prefix.probe.stderr"
	local code=$?
	set -e
	local status
	case "$code" in
	0)
		status=pass
		PASSED=$((PASSED + 1))
		;;
	77)
		status=blocked
		BLOCKED=$((BLOCKED + 1))
		;;
	*)
		status=fail
		FAILURES=$((FAILURES + 1))
		;;
	esac
	RESULT_ROWS+=("$(printf '%s\t%s\t%s' "$name" "$status" "$code")")
	printf '%s\t%s\t%s\n' "$name" "$status" "$code" >>"$EVIDENCE/probes.tsv"
	printf '%-42s %s\n' "$name" "$status" >&2
}

write_environment() {
	{
		printf 'candidate_sha=%s\n' "$CANDIDATE_SHA"
		printf 'mode=%s\n' "$MODE"
		printf 'candidate_dirty=%s\n' "$(git -C "$ROOT" status --porcelain | wc -l)"
		printf 'go=%s\n' "$(go version)"
		printf 'node=%s\n' "$(node --version 2>/dev/null || echo unavailable)"
		printf 'pnpm=%s\n' "$(pnpm --version 2>/dev/null || echo unavailable)"
		printf 'npm=%s\n' "$(npm --version 2>/dev/null || echo unavailable)"
		printf 'docker=%s\n' "$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unavailable)"
		printf 'psql=%s\n' "$(psql --version 2>/dev/null || echo unavailable)"
		printf 'dates=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	} >"$EVIDENCE/environment.txt"
	git -C "$ROOT" log -1 --format='%H %s' >"$EVIDENCE/candidate-commit.txt"
	git -C "$ROOT" status --porcelain >"$EVIDENCE/candidate-status.txt"
	grep -E 'Inertia = |Anduin = |Storage = ' "$ROOT/layout/versions/versions.go" \
		>"$EVIDENCE/candidate-pins.txt" 2>/dev/null || true
}

prepare() {
	SCRATCH="$(mktemp -d "${TMPDIR:-/tmp}/andurel-acceptance-XXXXXX")"
	mkdir -p "$SCRATCH/bin" "$SCRATCH/apps" "$SCRATCH/leases" "$SCRATCH/evidence"
	log "scratch: $SCRATCH"
	log "evidence: $EVIDENCE"
	(
		cd "$ROOT"
		GOWORK=off go build -o "$SCRATCH/bin/andurel" .
	)
	A="$SCRATCH/bin/andurel"
	"$A" --version >"$EVIDENCE/cli-version.txt" 2>&1
	(
		cd "$ROOT"
		GOWORK=off go build -o "$SCRATCH/bin/healthchild" ./scripts/acceptance/healthchild
	)
	HEALTHCHILD="$SCRATCH/bin/healthchild"
}

# --- Probes ---------------------------------------------------------------

probe_cli_surface() {
	local p="$PROBE_PREFIX"
	local code
	code="$(capture "$p.commands-check" "$A" commands --check)"
	assert_status "$p.commands-check" 0
	code="$(capture "$p.commands-json" "$A" commands --json)"
	assert_status "$p.commands-json" 0
	assert_contains "$p.commands-json.stdout" '"path": "andurel ports launch"'
	assert_contains "$p.commands-json.stdout" '"supports_jq": true'
	assert_contains "$p.commands-json.stdout" '"path": "andurel build"'

	code="$(capture "$p.ports-help" "$A" ports --help)"
	assert_status "$p.ports-help" 0
	assert_contains "$p.ports-help.stdout" "ports launch"

	code="$(capture "$p.inspect-invalid" "$A" inspect definitely-not-a-command --json)"
	assert_status "$p.inspect-invalid" 1
	assert_contains "$p.inspect-invalid.stderr" '"code": "usage_error"'

	code="$(capture "$p.db-console-json" "$A" db console --json)"
	assert_status "$p.db-console-json" 1
	assert_contains "$p.db-console-json.stderr" '"code": "usage_error"'
}

probe_new_quickstart() {
	local p="$PROBE_PREFIX"
	local apps="$SCRATCH/apps"
	local code
	code="$(cd "$apps" && capture "$p.new" "$A" new fresh-app)"
	assert_status "$p.new" 0
	for needle in "andurel tool sync" "cp .env.example .env" "andurel db create" "andurel db migrate up" "andurel doctor" "andurel run" "andurel verify"; do
		assert_contains "$p.new.stdout" "$needle"
	done
	code="$(capture "$p.help" "$A" new --help)"
	assert_status "$p.help" 0
	assert_contains "$p.help.stdout" "--harness"
	local project="$apps/fresh-app"
	for needle in "andurel tool sync" "pnpm install" "cp .env.example .env" "andurel db create" "andurel db migrate up" "andurel doctor" "andurel run" "andurel verify"; do
		assert_contains "$project/README.md" "$needle"
	done
	cp -a "$project" "$SCRATCH/apps/pristine-app"
	chmod -R u+w "$SCRATCH/apps/pristine-app"
}

probe_untouched_verify() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/pristine-app"
	local code
	code="$(capture_in "$app" "$p.verify" "$A" verify --json)"
	assert_status "$p.verify" 9
	assert_contains "$p.verify.stderr" '"name": "setup"'
	assert_contains "$p.verify.stderr" '"status": "fail"'
	assert_contains "$p.verify.stderr" '"name": "generated"'
	assert_contains "$p.verify.stderr" '"status": "blocked"'
	assert_not_contains "$p.verify.stdout" "generated drift"
	code="$(capture_in "$app" "$p.content-only" "$A" verify --only content --json)"
	assert_status "$p.content-only" 0
	assert_contains "$p.content-only.stdout" '"not_applicable": true'
}

probe_untouched_doctor() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/pristine-app"
	local code
	code="$(capture_in "$app" "$p.doctor" "$A" doctor --json)"
	if [[ "$code" == "0" ]]; then
		fail "doctor must not pass a project without runtime configuration"
	fi
	assert_contains "$p.doctor.stderr" '"name": "Runtime config"'
	assert_contains "$p.doctor.stderr" '"status": "fail"'
	assert_contains "$p.doctor.stderr" '"blocking": true'
}

probe_migration_creation() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/pristine-app"
	local code
	code="$(capture_in "$app" "$p.generate" "$A" generate migration acceptance_probe)"
	assert_status "$p.generate" 0
	local found
	found="$(find "$app/migrations" -maxdepth 1 -name '*_acceptance_probe.sql' -print -quit)"
	if [[ -z "$found" ]]; then
		fail "migration file was not created"
	fi
	if [[ ! -s "$found" ]]; then
		fail "migration file is empty: $found"
	fi
	assert_not_contains "$p.generate.stderr" "andurel.lock"
}

probe_config_projection() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/pristine-app"
	local code
	code="$(capture_in "$app" "$p.json" "$A" inspect config --json)"
	assert_status "$p.json" 0
	assert_contains "$p.json.stdout" '"keys"'
	code="$(capture_in "$app" "$p.jq" "$A" inspect config --jq .keys)"
	assert_status "$p.jq" 0
	if [[ ! -s "$p.jq.stdout" ]]; then
		fail "inspect config --jq .keys printed nothing"
	fi
}

probe_content_invariants() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/dev-app"
	[[ -d "$app" ]] || blocked "dev-app was not prepared (local-replace/tidy blocked)"
	cat >"$app/andurel.content.toml" <<'EOF'
schemaVersion = 1
include = ["content/**/*.md"]
required = ["content/index.md"]

[[catalog]]
slug = "welcome"
file = "content/welcome.md"

[links]
anchors = "ignore"
external = "ignore"
EOF
	mkdir -p "$app/content"
	printf '# Welcome\n\n[home](index.md)\n' >"$app/content/welcome.md"
	printf '# Index\n\n[welcome](welcome.md)\n' >"$app/content/index.md"
	cp "$app/andurel.content.toml" "$SCRATCH/evidence/content-manifest.toml"

	local code
	code="$(capture_in "$app" "$p.valid" "$A" verify --only content --json)"
	assert_status "$p.valid" 0

	mv "$app/content/welcome.md" "$app/content/welcome.md.bak"
	code="$(capture_in "$app" "$p.missing" "$A" verify --only content --json)"
	assert_status "$p.missing" 9
	assert_contains "$p.missing.stderr" '"content-catalog"'
	mv "$app/content/welcome.md.bak" "$app/content/welcome.md"

	cat >>"$app/andurel.content.toml" <<'EOF'

[[catalog]]
slug = "welcome"
file = "content/index.md"
EOF
	code="$(capture_in "$app" "$p.duplicate" "$A" verify --only content --json)"
	assert_status "$p.duplicate" 9
	assert_contains "$p.duplicate.stderr" '"content-slug"'
	cp "$SCRATCH/evidence/content-manifest.toml" "$app/andurel.content.toml"

	printf '# Welcome\n\n[missing](missing.md)\n' >"$app/content/welcome.md"
	code="$(capture_in "$app" "$p.broken-link" "$A" verify --only content --json)"
	assert_status "$p.broken-link" 9
	assert_contains "$p.broken-link.stderr" '"content-link"'
}

probe_test_accounting() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/dev-app"
	[[ -d "$app" ]] || blocked "dev-app was not prepared (local-replace/tidy blocked)"
	mkdir -p "$app/acceptanceunit"
	cat >"$app/acceptanceunit/pass_test.go" <<'EOF'
package acceptanceunit

import "testing"

func TestAcceptancePasses(t *testing.T) {}
EOF
	cat >"$app/acceptanceunit/skip_test.go" <<'EOF'
package acceptanceunit

import "testing"

func TestAcceptanceSkips(t *testing.T) {
	t.Skip("acceptance fixture skip")
}
EOF
	local code
	code="$(capture_in "$app" "$p.with-tests" "$A" verify --only go-test --with-tests --json ./acceptanceunit/...)"
	assert_status "$p.with-tests" 0
	assert_contains "$p.with-tests.stdout" '"skipped_tests"'
	assert_contains "$p.with-tests.stdout" "acceptance fixture skip"
	assert_contains "$p.with-tests.stdout" '"totals"'

	printf 'package acceptanceunit\n\nimport "testing"\n\nfunc TestAcceptanceFails(t *testing.T) { t.Fatal("expected failure") }\n' \
		>"$app/acceptanceunit/fail_test.go"
	code="$(capture_in "$app" "$p.failing" "$A" verify --only go-test --with-tests --json ./acceptanceunit/...)"
	assert_status "$p.failing" 9
	rm -f "$app/acceptanceunit/fail_test.go"
}

probe_go_test_opt_in() {
	local p="$PROBE_PREFIX"
	local code
	code="$(capture "$p.cli" go test ./cli -run 'TestVerifyDoesNotRunTestsWithoutWithTests' -count=1)"
	assert_status "$p.cli" 0
}

probe_route_usage_regressions() {
	local p="$PROBE_PREFIX"
	local code
	code="$(capture "$p.cli" go test ./cli -run 'TestRouteUsage|TestLoadTsconfig' -count=1)"
	assert_status "$p.cli" 0
}

probe_doctor_regressions() {
	local p="$PROBE_PREFIX"
	local code
	code="$(capture "$p.cli" go test ./cli -run 'TestCheckRuntimeConfiguration|TestVerifySetup' -count=1)"
	assert_status "$p.cli" 0
}

probe_build_recovery_regressions() {
	local p="$PROBE_PREFIX"
	local code
	code="$(capture "$p.cli" go test ./cli -run 'TestRecoverInterrupted|TestPublishBuildArtifactsRepairs' -count=1)"
	assert_status "$p.cli" 0
}

start_port_launcher() {
	local pid_var="$1"
	local lease_dir="$2"
	local ready_file="$3"
	local log_prefix="$4"
	local port
	port="${5:-0}"
	(
		cd "$ROOT"
		exec "$A" ports launch \
			--json \
			--lease-dir "$lease_dir" \
			--ready-file "$ready_file" \
			--startup-timeout 30s \
			--port "$port" \
			-- "$HEALTHCHILD"
	) >"$log_prefix.stdout" 2>"$log_prefix.stderr" &
	printf -v "$pid_var" '%s' "$!"
	OWN_PIDS+=("$!")
}

wait_ready_file() {
	local ready_file="$1"
	local dead_pid="$2"
	local prefix="$3"
	local deadline=$((SECONDS + 45))
	while ((SECONDS < deadline)); do
		if [[ -s "$ready_file" ]]; then
			return 0
		fi
		if ! kill -0 "$dead_pid" 2>/dev/null; then
			head -60 "$prefix.stderr" >&2 || true
			return 1
		fi
		sleep 0.1
	done
	return 1
}

stop_pid() {
	local pid="$1"
	local deadline=$((SECONDS + 20))
	kill -TERM "$pid" 2>/dev/null || true
	while kill -0 "$pid" 2>/dev/null && ((SECONDS < deadline)); do
		sleep 0.1
	done
	if kill -0 "$pid" 2>/dev/null; then
		return 1
	fi
	set +e
	wait "$pid" 2>/dev/null
	local code=$?
	set -e
	[[ "$code" == "0" ]]
}

probe_port_lease() {
	local p="$PROBE_PREFIX"
	local lease_dir="$SCRATCH/leases/one"
	local ready="$SCRATCH/evidence/lease-one.json"
	mkdir -p "$lease_dir"
	local pid
	start_port_launcher pid "$lease_dir" "$ready" "$p.launcher"
	if ! wait_ready_file "$ready" "$pid" "$p.launcher"; then
		fail "launcher never published a ready file"
	fi
	assert_contains "$ready" '"port"'
	assert_contains "$ready" '"listen_url"'
	assert_contains "$ready" '"child_pid"'
	if ! stop_pid "$pid"; then
		fail "launcher did not exit cleanly after SIGTERM"
	fi
	if [[ -e "$ready" ]]; then
		fail "ready file still exists after clean shutdown"
	fi
	assert_contains "$p.launcher.stdout" '"ok": true'
}

probe_port_concurrent_claims() {
	local p="$PROBE_PREFIX"
	local lease_a="$SCRATCH/leases/a"
	local lease_b="$SCRATCH/leases/b"
	mkdir -p "$lease_a" "$lease_b"
	local ready_a="$SCRATCH/evidence/lease-a.json"
	local ready_b="$SCRATCH/evidence/lease-b.json"
	local pid_a pid_b
	start_port_launcher pid_a "$lease_a" "$ready_a" "$p.a"
	start_port_launcher pid_b "$lease_b" "$ready_b" "$p.b"
	if ! wait_ready_file "$ready_a" "$pid_a" "$p.a"; then fail "claim a never became ready"; fi
	if ! wait_ready_file "$ready_b" "$pid_b" "$p.b"; then fail "claim b never became ready"; fi
	local port_a port_b
	port_a="$(grep -o '"port":[0-9]*' "$ready_a" | head -1 | grep -o '[0-9]*' || true)"
	port_b="$(grep -o '"port":[0-9]*' "$ready_b" | head -1 | grep -o '[0-9]*' || true)"
	if [[ -z "$port_a" || -z "$port_b" || "$port_a" == "$port_b" ]]; then
		fail "concurrent claims got the same or no port (a=$port_a b=$port_b)"
	fi
	if ! stop_pid "$pid_a"; then fail "claim a did not exit cleanly"; fi
	if ! stop_pid "$pid_b"; then fail "claim b did not exit cleanly"; fi
}

probe_port_occupied() {
	local p="$PROBE_PREFIX"
	local lease_dir="$SCRATCH/leases/fixed"
	mkdir -p "$lease_dir"
	# Reserve a port by asking the launcher for one, then require a second fixed
	# claim on the held address to fail instead of reusing it.
	local ready="$SCRATCH/evidence/occupy.json"
	local holder_pid
	start_port_launcher holder_pid "$lease_dir" "$ready" "$p.holder"
	if ! wait_ready_file "$ready" "$holder_pid" "$p.holder"; then
		fail "holder claim never became ready"
	fi
	local url
	url="$(grep -o '"listen_url":"[^"]*"' "$ready" | head -1 | cut -d'"' -f4 || true)"
	if [[ -z "$url" ]]; then
		fail "holder did not publish a listen URL"
	fi
	local code
	code="$(cd "$ROOT" && capture "$p.second" "$A" ports launch --lease-dir "$SCRATCH/leases/other" --listen "$url" --startup-timeout 5s -- "$HEALTHCHILD")"
	if [[ "$code" == "0" ]]; then
		fail "a second claim on a leased address succeeded"
	fi
	if ! stop_pid "$holder_pid"; then fail "holder did not exit cleanly"; fi
}

probe_local_replacement() {
	local p="$PROBE_PREFIX"
	local app="$SCRATCH/apps/pristine-app"
	local dev="$SCRATCH/apps/dev-app"
	cp -a "$app" "$dev"
	chmod -R u+w "$dev"
	if [[ "$MODE" == "release" ]]; then
		local code
		code="$(cd "$dev" && GOWORK=off capture "$p.tidy" go mod tidy)"
		if [[ "$code" != "0" ]]; then
			blocked "release gate: candidate pkg/inertia pin is not published; go mod tidy failed (see $p.tidy.stderr)"
		fi
		assert_status "$p.tidy" 0
		return
	fi
	local module_dir
	for module_dir in "$ROOT"/pkg/*/; do
		[[ -f "${module_dir}go.mod" ]] || continue
		local module
		module="$(basename "$module_dir")"
		(cd "$dev" && go mod edit -replace "github.com/mbvlabs/andurel/pkg/${module}=${module_dir%/}")
	done
	local code
	code="$(cd "$dev" && GOWORK=off capture "$p.tidy" go mod tidy)"
	if [[ "$code" != "0" ]]; then
		blocked "go mod tidy needs module downloads for this candidate; network or module cache unavailable (see $p.tidy.stderr)"
	fi
	assert_status "$p.tidy" 0
	# A development replace must never be committed back to the framework.
	if git -C "$ROOT" grep -Fq -- "$SCRATCH"; then
		fail "machine-specific scratch path leaked into the repository"
	fi
	code="$(cd "$dev" && GOWORK=off capture "$p.vet" go vet ./...)"
	assert_status "$p.vet" 0
}

probe_runtime_gates() {
	local p="$PROBE_PREFIX"
	local missing=()
	command -v node >/dev/null 2>&1 || missing+=(node)
	command -v pnpm >/dev/null 2>&1 || missing+=(pnpm)
	command -v docker >/dev/null 2>&1 || missing+=(docker)
	[[ -n "${ANDUREL_ACCEPTANCE_DATABASE_URL:-}" ]] || missing+=(ANDUREL_ACCEPTANCE_DATABASE_URL)
	if [[ "${#missing[@]}" -gt 0 ]]; then
		blocked "live app/SSR/build gates need: ${missing[*]}; see plans/evidence/beta3-complete-fix/README.md"
	fi
	if [[ "$FULL" -ne 1 ]]; then
		blocked "live app/SSR/build gates are opt-in: rerun with --full after tool sync, pnpm install, migrations, and assets/dist"
	fi
	blocked "live app/SSR/build gate runner is not implemented in this harness revision; run the documented commands in plans/evidence/beta3-complete-fix/README.md"
}

probe_site_gate() {
	blocked "andurel-site adoption (F35/F32/F21/F25) is verified in the separate site worktree, not the framework scaffold"
}

write_summary() {
	local acceptance=full
	if [[ "$BLOCKED" -gt 0 || "$FAILURES" -gt 0 ]]; then
		acceptance=partial
	fi
	{
		printf '# Probes\n\n'
		printf 'acceptance: %s\n\n' "$acceptance"
		printf '| Probe | Status | Exit |\n| --- | --- | --- |\n'
		for row in "${RESULT_ROWS[@]:-}"; do
			IFS=$'\t' read -r name status code <<<"$row"
			[[ -n "$name" ]] || continue
			printf '| %s | %s | %s |\n' "$name" "$status" "$code"
		done
		printf '\npassed=%d blocked=%d failed=%d\n' "$PASSED" "$BLOCKED" "$FAILURES"
		if [[ "$acceptance" == partial ]]; then
			printf '\nPARTIAL ACCEPTANCE: %d probe(s) blocked and %d failed. This is not full acceptance.\n' "$BLOCKED" "$FAILURES"
		fi
	} >"$EVIDENCE/probes.md"
	log "passed=$PASSED blocked=$BLOCKED failed=$FAILURES acceptance=$acceptance"
	if [[ "$acceptance" == partial ]]; then
		log "PARTIAL ACCEPTANCE: blocked gates are not passes; see $EVIDENCE/probes.md"
	fi
	log "evidence: $EVIDENCE"
	printf '%s' "$acceptance" >"$EVIDENCE/acceptance-state.txt"
}

main() {
	cd "$ROOT"
	write_environment
	prepare
	run_probe 01-cli-surface probe_cli_surface
	run_probe 02-new-quickstart probe_new_quickstart
	run_probe 03-untouched-verify probe_untouched_verify
	run_probe 04-untouched-doctor probe_untouched_doctor
	run_probe 05-migration-creation probe_migration_creation
	run_probe 06-config-projection probe_config_projection
	run_probe 07-local-replacement probe_local_replacement
	run_probe 08-content-invariants probe_content_invariants
	run_probe 09-test-accounting probe_test_accounting
	run_probe 10-go-test-opt-in probe_go_test_opt_in
	run_probe 11-route-usage-regressions probe_route_usage_regressions
	run_probe 12-doctor-regressions probe_doctor_regressions
	run_probe 13-build-publication-recovery probe_build_recovery_regressions
	run_probe 14-port-lease probe_port_lease
	run_probe 15-port-concurrent-claims probe_port_concurrent_claims
	run_probe 16-port-occupied probe_port_occupied
	run_probe 17-live-runtime-gates probe_runtime_gates
	run_probe 18-site-gate probe_site_gate
	write_summary
	if [[ "$FAILURES" -gt 0 ]]; then
		exit 1
	fi
	if [[ "$REQUIRE_ALL" -eq 1 && "$BLOCKED" -gt 0 ]]; then
		exit 1
	fi
}

main "$@"
