#!/usr/bin/env bash
# shellcheck disable=SC2016,SC2030,SC2031
# Beta.3 F40 (hygiene/frozen install) and F06 (live doctor negatives) probes.
#
# This is a durable, repeatable companion to scripts/beta3-fix-acceptance.sh.
# It creates its own mktemp tree, builds the candidate CLI, generates a brand-new
# React/pnpm application with the selected agent harness, points the generated
# module at this checkout's pkg/* modules with temporary go.mod replaces, and
# records the exact command, stdout, stderr, and exit status per probe.
#
# It proves, against a real generated app:
#   F40  selected harness skill install + git tracking, no lockfile shipped by
#        `andurel new`, pnpm lockfile generation, frozen reinstall, frozen
#        drift rejection, ignored build output/secrets, and the setup
#        negative -> positive transition (tool sync + pnpm install).
#   F06  doctor missing configuration, malformed configuration (secret-safe),
#        exported-only configuration, and an unreachable database
#        (secret-safe diagnostics).
#   Also scoped/changed/content verify acceptance where it is cheap and useful.
#
# Development proof only: the generated app uses temporary go.mod replaces for
# the unpublished candidate pkg/inertia pin. This is not release-shaped proof
# and never publishes, tags, or pushes anything.
#
# Usage:
#   scripts/acceptance/beta3-hygiene-doctor.sh [--evidence-dir DIR]
#                                              [--require-all] [--keep-scratch]
#
# Exit status: 0 when every executed probe passed. A blocked probe (missing
# prerequisite or unavailable module/tool download) makes the result PARTIAL,
# never a pass. Any failed probe exits 1. With --require-all, a blocked probe
# also exits 1.
set -Eeuo pipefail
IFS=$'\n\t'
umask 022

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CANDIDATE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
EVIDENCE=""
REQUIRE_ALL=0
KEEP_SCRATCH=0
SCRATCH=""
A=""
APP=""
PROJECT="hygiene-app"
RESULT_ROWS=()
FAILURES=0
BLOCKED=0
PASSED=0

usage() {
	sed -n '2,32p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--evidence-dir)
		EVIDENCE="${2:-}"
		shift 2
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

if [[ -z "$EVIDENCE" ]]; then
	EVIDENCE="$ROOT/plans/evidence/beta3-finish-hygiene/${CANDIDATE_SHA:0:12}-hygiene-doctor"
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
	if [[ -n "$SCRATCH" && -d "$SCRATCH" && "$KEEP_SCRATCH" -eq 0 ]]; then
		rm -rf "$SCRATCH"
	fi
}
trap cleanup EXIT INT TERM

mkdir -p "$EVIDENCE"
: >"$EVIDENCE/probes.tsv"

# capture runs one command with a timeout, recording stdout/stderr/status.
# It never lets set -e abort the probe; the caller asserts the expected status.
capture() {
	local prefix="$1"
	shift
	{
		printf 'cwd=%q\ncommand=' "$PWD"
		local arg
		for arg in "$@"; do
			case "$arg" in
			*PASSWORD=* | *KEY=* | PEPPER=* | *TOKEN=*) arg="${arg%%=*}=<redacted>" ;;
			esac
			printf '%q ' "$arg"
		done
		printf '\n'
	} >"$prefix.command"
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
		head -80 "$prefix.stdout" >&2 || true
		printf -- '--- stderr ---\n' >&2
		head -80 "$prefix.stderr" >&2 || true
		return 1
	fi
}

assert_nonzero() {
	local prefix="$1"
	local got
	got="$(cat "$prefix.status")"
	if [[ "$got" == "0" ]]; then
		printf 'expected a nonzero exit, got 0\n' >&2
		printf -- '--- stdout ---\n' >&2
		head -80 "$prefix.stdout" >&2 || true
		printf -- '--- stderr ---\n' >&2
		head -80 "$prefix.stderr" >&2 || true
		return 1
	fi
}

assert_contains() {
	local path="$1"
	local needle="$2"
	if ! grep -Fq -- "$needle" "$path"; then
		printf 'expected %s to contain %q\n' "$path" "$needle" >&2
		head -60 "$path" >&2 || true
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

assert_file() {
	local path="$1"
	if [[ ! -s "$path" ]]; then
		fail "expected a non-empty file: $path"
	fi
}

assert_no_file() {
	local path="$1"
	if [[ -e "$path" ]]; then
		fail "expected no file: $path"
	fi
}

assert_jq() {
	local path="$1"
	local expression="$2"
	if ! jq -e "$expression" "$path" >/dev/null 2>&1; then
		printf 'expected jq expression in %s: %s\n' "$path" "$expression" >&2
		head -80 "$path" >&2 || true
		return 1
	fi
}

assert_tracked() {
	local path="$1"
	if ! git -C "$APP" ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
		fail "expected git to track $path"
	fi
}

assert_untracked() {
	local path="$1"
	if [[ -n "$(git -C "$APP" ls-files -- "$path")" ]]; then
		fail "expected git not to track $path"
	fi
}

assert_ignored() {
	local path="$1"
	if ! git -C "$APP" check-ignore -q -- "$path"; then
		fail "expected .gitignore to ignore $path"
	fi
}

assert_not_ignored() {
	local path="$1"
	if git -C "$APP" check-ignore -q -- "$path"; then
		fail "expected .gitignore not to ignore $path"
	fi
}

assert_secret_absent() {
	local prefix="$1"
	local secret="$2"
	assert_not_contains "$prefix.stdout" "$secret"
	assert_not_contains "$prefix.stderr" "$secret"
}

# doctor_clean runs the candidate doctor with only the supplied environment, so
# the host's own variables and .env cannot make a negative case pass.
doctor_clean() {
	local prefix="$1"
	shift
	local -a clean=(env -i "PATH=$PATH" "HOME=$HOME" "GOWORK=off")
	clean+=("TMPDIR=${TMPDIR:-/tmp/opencode}" "GOTMPDIR=${GOTMPDIR:-${TMPDIR:-/tmp/opencode}}")
	if [[ -n "${GOCACHE:-}" ]]; then
		clean+=("GOCACHE=$GOCACHE")
	fi
	if [[ -n "${GOMODCACHE:-}" ]]; then
		clean+=("GOMODCACHE=$GOMODCACHE")
	fi
	capture_in "$APP" "$prefix" "${clean[@]}" "$@" "$A" doctor --json
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
		printf 'candidate_dirty=%s\n' "$(git -C "$ROOT" status --porcelain | wc -l)"
		printf 'go=%s\n' "$(go version)"
		printf 'node=%s\n' "$(node --version 2>/dev/null || echo unavailable)"
		printf 'pnpm=%s\n' "$(pnpm --version 2>/dev/null || echo unavailable)"
		printf 'npm=%s\n' "$(npm --version 2>/dev/null || echo unavailable)"
		printf 'git=%s\n' "$(git --version)"
		printf 'jq=%s\n' "$(jq --version 2>/dev/null || echo unavailable)"
		printf 'os=%s/%s\n' "$(uname -s)" "$(uname -m)"
		printf 'dates=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	} >"$EVIDENCE/environment.txt"
	git -C "$ROOT" log -1 --format='%H %s' >"$EVIDENCE/candidate-commit.txt"
	git -C "$ROOT" status --porcelain >"$EVIDENCE/candidate-status.txt"
	grep -E 'Inertia = |Anduin = |Storage = ' "$ROOT/layout/versions/versions.go" \
		>"$EVIDENCE/candidate-pins.txt" 2>/dev/null || true
}

prepare() {
	local scratch_parent="${TMPDIR:-/tmp/opencode}"
	mkdir -p "$scratch_parent"
	SCRATCH="$(mktemp -d "$scratch_parent/andurel-hygiene-XXXXXX")"
	mkdir -p "$SCRATCH/bin" "$SCRATCH/apps"
	log "scratch: $SCRATCH"
	log "evidence: $EVIDENCE"
	(
		cd "$ROOT"
		GOWORK=off go build -o "$SCRATCH/bin/andurel" .
	)
	A="$SCRATCH/bin/andurel"
	"$A" --version >"$EVIDENCE/cli-version.txt" 2>&1
	APP="$SCRATCH/apps/$PROJECT"
}

# --- Probes ---------------------------------------------------------------

probe_new_react_pnpm_harness() {
	local p="$PROBE_PREFIX"
	local code
	code="$(cd "$SCRATCH/apps" && capture "$p.new" "$A" new "$PROJECT" --harness opencode)"
	assert_status "$p.new" 0
	assert_contains "$p.new.stdout" "installed for OpenCode under ./.opencode/skills/andurel"
	assert_contains "$p.new.stdout" "Commit it with the project."

	# The selected harness is installed with every embedded file.
	assert_file "$APP/.opencode/skills/andurel/SKILL.md"
	assert_file "$APP/.opencode/skills/andurel/agents/openai.yaml"
	assert_file "$APP/.opencode/skills/andurel/references/cli-catalog.md"
	assert_file "$APP/.opencode/skills/andurel/references/http-and-routing.md"
	assert_file "$APP/.opencode/skills/andurel/references/layer-placement.md"
	find "$APP/.opencode/skills/andurel" -type f -print | sort >"$p.skill-files.txt"

	# No unselected harness directory is installed.
	local unselected
	for unselected in .codex .claude .pi .crush; do
		if [[ -e "$APP/$unselected" ]]; then
			fail "unselected harness directory exists: $unselected"
		fi
	done

	# The default UI is React/pnpm and `andurel new` ships no lockfile.
	assert_contains "$APP/andurel.toml" "inertia = 'react'"
	assert_contains "$APP/andurel.toml" "javascriptPackageManager = 'pnpm'"
	assert_no_file "$APP/pnpm-lock.yaml"

	# The generated docs carry the frozen-install policy this probe enforces.
	assert_contains "$APP/README.md" "pnpm install --frozen-lockfile"
	assert_contains "$APP/AGENTS.md" "pnpm install --frozen-lockfile"
	assert_contains "$APP/AGENTS.md" "pnpm-lock.yaml"
	assert_contains "$APP/README.md" "andurel skill install --harness"

	# Harness selection is explicit in non-interactive output: a bare install
	# is a usage error and must not silently install every harness.
	code="$(capture_in "$APP" "$p.skill-requires-harness" "$A" skill install --json)"
	assert_status "$p.skill-requires-harness" 1
	assert_contains "$p.skill-requires-harness.stderr" '"code": "usage_error"'
	assert_contains "$p.skill-requires-harness.stderr" "--harness"
	if [[ -e "$APP/.claude" ]]; then
		fail "a failed non-interactive skill install created .claude"
	fi
}

probe_local_replacements() {
	local p="$PROBE_PREFIX"
	local module_dir module code
	for module_dir in "$ROOT"/pkg/*/; do
		[[ -f "${module_dir}go.mod" ]] || continue
		module="$(basename "$module_dir")"
		(
			cd "$APP"
			go mod edit -replace "github.com/mbvlabs/andurel/pkg/${module}=${module_dir%/}"
		)
	done
	code="$(cd "$APP" && PROBE_TIMEOUT=300 capture "$p.tidy" env GOWORK=off go mod tidy)"
	if [[ "$code" != "0" ]]; then
		blocked "go mod tidy needs module downloads for this candidate; network or module cache unavailable (see $p.tidy.stderr)"
	fi
	assert_status "$p.tidy" 0
	code="$(cd "$APP" && capture "$p.vet" env GOWORK=off go vet ./...)"
	assert_status "$p.vet" 0
	# A development replace must never be committed back to the framework.
	if git -C "$ROOT" grep -Fq -- "$SCRATCH"; then
		fail "machine-specific scratch path leaked into the repository"
	fi

	# Setup is incomplete before tool sync and pnpm install, and the exact
	# commands are named. This is the negative half of the setup transition.
	code="$(capture_in "$APP" "$p.setup-before" "$A" verify --only setup --json)"
	assert_status "$p.setup-before" 9
	assert_contains "$p.setup-before.stderr" '"name": "setup"'
	assert_contains "$p.setup-before.stderr" "andurel tool sync"
	assert_contains "$p.setup-before.stderr" "pnpm install"
}

probe_lockfile_frozen_install() {
	local p="$PROBE_PREFIX"
	local code before after
	assert_no_file "$APP/pnpm-lock.yaml"
	code="$(cd "$APP" && PROBE_TIMEOUT=600 capture "$p.install" pnpm install)"
	if [[ "$code" != "0" ]]; then
		blocked "pnpm install could not complete (network or registry unavailable; see $p.install.stderr)"
	fi
	assert_status "$p.install" 0
	assert_file "$APP/pnpm-lock.yaml"
	if [[ ! -d "$APP/node_modules" ]]; then
		fail "pnpm install did not create node_modules"
	fi

	# The lockfile is project state to commit, not ignored output.
	assert_not_ignored "pnpm-lock.yaml"
	code="$(capture "$p.add-lockfile" git -C "$APP" add pnpm-lock.yaml)"
	assert_status "$p.add-lockfile" 0
	assert_tracked "pnpm-lock.yaml"

	before="$(sha256sum "$APP/pnpm-lock.yaml" | cut -d' ' -f1)"
	printf '%s\n' "$before" >"$p.lock-before.sha256"
	code="$(cd "$APP" && capture "$p.frozen" pnpm install --frozen-lockfile)"
	assert_status "$p.frozen" 0
	after="$(sha256sum "$APP/pnpm-lock.yaml" | cut -d' ' -f1)"
	printf '%s\n' "$after" >"$p.lock-after.sha256"
	if [[ "$before" != "$after" ]]; then
		fail "pnpm install --frozen-lockfile rewrote pnpm-lock.yaml"
	fi

	# Frozen mode must reject a manifest that drifted from the lockfile.
	cp "$APP/package.json" "$SCRATCH/package.json.acceptance"
	jq '.dependencies["left-pad"] = "^1.3.0"' "$SCRATCH/package.json.acceptance" \
		>"$APP/package.json"
	code="$(cd "$APP" && capture "$p.drift" pnpm install --frozen-lockfile)"
	assert_nonzero "$p.drift"
	assert_contains "$p.drift.stderr" "ERR_PNPM_OUTDATED_LOCKFILE"
	assert_contains "$p.drift.stderr" "left-pad"
	cp "$SCRATCH/package.json.acceptance" "$APP/package.json"

	code="$(cd "$APP" && capture "$p.frozen-restored" pnpm install --frozen-lockfile)"
	assert_status "$p.frozen-restored" 0
	after="$(sha256sum "$APP/pnpm-lock.yaml" | cut -d' ' -f1)"
	if [[ "$before" != "$after" ]]; then
		fail "restored frozen install rewrote pnpm-lock.yaml"
	fi
}

probe_setup_positive() {
	local p="$PROBE_PREFIX"
	local code tool
	code="$(cd "$APP" && PROBE_TIMEOUT=900 capture "$p.tool-sync" "$A" tool sync)"
	if [[ "$code" != "0" ]]; then
		blocked "andurel tool sync could not download the pinned tools (network unavailable; see $p.tool-sync.stderr)"
	fi
	assert_status "$p.tool-sync" 0

	# Every tool recorded in andurel.toml has a project-local binary.
	local tool_count=0
	while IFS= read -r tool; do
		[[ -n "$tool" ]] || continue
		tool_count=$((tool_count + 1))
		assert_file "$APP/bin/$tool"
	done < <(
		sed -n "/^\[tools\]/,/^\[/p" "$APP/andurel.toml" |
			grep -E '^  [a-z][a-z0-9_-]* = ' |
			sed -E 's/^  ([a-z][a-z0-9_-]*) = .*/\1/'
	)
	if [[ "$tool_count" -eq 0 ]]; then
		fail "andurel.toml lists no managed tools to verify"
	fi

	# The documented setup now passes, including the frontend dependencies.
	code="$(capture_in "$APP" "$p.verify-setup" "$A" verify --only setup --json)"
	assert_status "$p.verify-setup" 0
	assert_jq "$p.verify-setup.stdout" \
		'.data.steps[] | select(.name == "setup") | .status == "ok" and .ok == true'

	# Doctor sees the same setup as complete; runtime configuration still fails
	# because no .env exists and nothing was exported. Setup green must not
	# mask an unverified runtime.
	code="$(doctor_clean "$p.doctor")"
	assert_status "$p.doctor" 1
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "Project setup") | .status == "pass"'
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "tool versions") | .status == "pass"'
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "Runtime config") | .status == "fail"'
}

probe_doctor_missing_env() {
	local p="$PROBE_PREFIX"
	local code
	assert_no_file "$APP/.env"
	code="$(doctor_clean "$p.doctor")"
	assert_status "$p.doctor" 1
	assert_jq "$p.doctor.stderr" \
		'.data.summary.status == "fail" and (.data.summary.failed > 0)'
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "Runtime config") | .status == "fail" and .blocking == true'
	assert_contains "$p.doctor.stderr" "HOST_PRIMARY is required"
	assert_not_contains "$p.doctor.stderr" "config check did not run"
	# The check is read-only: it must not create the missing .env.
	assert_no_file "$APP/.env"
}

probe_doctor_malformed_env() {
	local p="$PROBE_PREFIX"
	local secret="not-hexadecimal-session-key"
	local code
	code="$(doctor_clean "$p.doctor" \
		HOST_PRIMARY=localhost:8080 \
		TOKEN_SIGNING_KEY=acceptance-token-signing-key \
		PEPPER=acceptance-pepper \
		SESSION_KEY="$secret" \
		SESSION_ENCRYPTION_KEY="$(printf 'cd%.0s' {1..32})")"
	assert_status "$p.doctor" 1
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "Runtime config") | .status == "fail"'
	assert_contains "$p.doctor.stderr" "SESSION_KEY"
	# A rejected secret value must never be echoed back.
	assert_secret_absent "$p.doctor" "$secret"
	assert_no_file "$APP/.env"
}

probe_doctor_export_only_unreachable_db() {
	local p="$PROBE_PREFIX"
	local password="acceptance-super-secret-password"
	local code
	assert_no_file "$APP/.env"
	# No .env at all: configuration comes only from exported variables. Port 1
	# on loopback refuses connections, so the database probe is deterministic
	# and bounded without starting a database.
	code="$(doctor_clean "$p.doctor" \
		HOST_PRIMARY=localhost:8080 \
		TOKEN_SIGNING_KEY=acceptance-token-signing-key \
		PEPPER=acceptance-pepper \
		SESSION_KEY="$(printf 'ab%.0s' {1..32})" \
		SESSION_ENCRYPTION_KEY="$(printf 'cd%.0s' {1..32})" \
		DB_HOST=127.0.0.1 \
		DB_PORT=1 \
		DB_NAME=hygiene \
		DB_USER=postgres \
		DB_PASSWORD="$password")"
	assert_status "$p.doctor" 1
	# Exported-only configuration is valid; only the database is unreachable.
	assert_jq "$p.doctor.stderr" \
		'.data.checks[] | select(.name == "Runtime config") | .status == "fail" and .message == "database is unreachable"'
	assert_contains "$p.doctor.stderr" "connection refused"
	assert_secret_absent "$p.doctor" "$password"
	assert_no_file "$APP/.env"
}

probe_hygiene_ignored_outputs_secrets() {
	local p="$PROBE_PREFIX"
	local marker="hygiene-secret-marker"
	local code
	printf 'ACCEPTANCE_SECRET=%s\n' "$marker" >"$APP/.env"
	mkdir -p "$APP/dist" "$APP/tmp" "$APP/assets/dist/ssr"
	printf 'console.log("build output")\n' >"$APP/dist/artifact.js"
	printf 'ssr bundle\n' >"$APP/assets/dist/ssr/ssr.js"
	printf 'cache\n' >"$APP/tmp/cache"
	printf 'sqlite\n' >"$APP/$PROJECT.db"

	# Stage everything; ignored output and secrets must stay out of the index.
	code="$(capture "$p.add" git -C "$APP" add -A)"
	assert_status "$p.add" 0

	assert_untracked ".env"
	assert_untracked "node_modules"
	assert_untracked "bin"
	assert_untracked "dist"
	assert_untracked "assets/dist"
	assert_untracked "tmp"
	assert_untracked "$PROJECT.db"
	code="$(capture "$p.secret-grep" git -C "$APP" grep --cached -F -- "$marker")"
	assert_nonzero "$p.secret-grep"
	if [[ -s "$p.secret-grep.stdout" ]]; then
		fail "the .env secret marker was staged in git"
	fi

	assert_ignored ".env"
	assert_ignored "node_modules/.modules.yaml"
	assert_ignored "bin/templ"
	assert_ignored "dist/artifact.js"
	assert_ignored "assets/dist/ssr/ssr.js"
	assert_ignored "tmp/cache"
	assert_ignored "$PROJECT.db"
	code="$(capture "$p.check-ignore" git -C "$APP" check-ignore -v -- \
		.env node_modules/.modules.yaml bin/templ dist/artifact.js \
		assets/dist/ssr/ssr.js tmp/cache "$PROJECT.db")"
	assert_status "$p.check-ignore" 0
	code="$(capture "$p.ignored-status" git -C "$APP" status --porcelain --ignored)"
	assert_status "$p.ignored-status" 0
	code="$(capture "$p.tracked-list" git -C "$APP" ls-files -- \
		.opencode pnpm-lock.yaml andurel.toml andurel.lock .env.example .gitignore)"
	assert_status "$p.tracked-list" 0
	assert_contains "$p.tracked-list.stdout" ".opencode/skills/andurel/SKILL.md"
	assert_contains "$p.tracked-list.stdout" "pnpm-lock.yaml"
	code="$(capture "$p.check-ignore-tracked" git -C "$APP" check-ignore -v -- \
		.opencode/skills/andurel/SKILL.md pnpm-lock.yaml)"
	assert_nonzero "$p.check-ignore-tracked"

	# Project state that must be committed stays tracked and unignored.
	assert_tracked ".env.example"
	assert_tracked ".gitignore"
	assert_tracked "andurel.toml"
	assert_tracked "andurel.lock"
	assert_tracked "pnpm-lock.yaml"
	assert_tracked ".opencode/skills/andurel/SKILL.md"
	assert_tracked ".opencode/skills/andurel/references/cli-catalog.md"
	assert_not_ignored ".opencode/skills/andurel/SKILL.md"
	assert_not_ignored "pnpm-lock.yaml"

	# Establish the baseline commit used by the changed-scope probe.
	code="$(capture "$p.commit" git -C "$APP" \
		-c user.email=acceptance@example.invalid \
		-c user.name=Acceptance commit -qm baseline)"
	assert_status "$p.commit" 0
	assert_tracked ".opencode/skills/andurel/SKILL.md"
	assert_tracked "pnpm-lock.yaml"
}

probe_verify_scoped_changed_content() {
	local p="$PROBE_PREFIX"
	local code
	# A project without content configuration reports the content stage as
	# not_applicable, never as a pass.
	code="$(capture_in "$APP" "$p.content-none" "$A" verify --only content --json)"
	assert_status "$p.content-none" 0
	assert_jq "$p.content-none.stdout" \
		'.data.steps[] | select(.name == "content") | .not_applicable == true'

	cat >"$APP/andurel.content.toml" <<'EOF'
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
	mkdir -p "$APP/content"
	printf '# Welcome\n\n[home](index.md)\n' >"$APP/content/welcome.md"
	printf '# Index\n\n[welcome](welcome.md)\n' >"$APP/content/index.md"

	code="$(capture_in "$APP" "$p.content-pass" "$A" verify --only content --json)"
	assert_status "$p.content-pass" 0
	assert_jq "$p.content-pass.stdout" \
		'.data.steps[] | select(.name == "content") | .status == "ok"'

	printf '# Welcome\n\n[missing](missing.md)\n' >"$APP/content/welcome.md"
	code="$(capture_in "$APP" "$p.content-link" "$A" verify --only content --json)"
	assert_status "$p.content-link" 9
	assert_jq "$p.content-link.stderr" \
		'.data.steps[] | select(.name == "content") | .violations[0].rule == "content-link"'
	printf '# Welcome\n\n[home](index.md)\n' >"$APP/content/welcome.md"

	# --only scopes the stages, and --changed scopes the Go packages to the
	# changed file. An unformatted new file fails exactly that file.
	mkdir -p "$APP/scopecheck"
	printf 'package scopecheck\n\nfunc  Broken( ) string {\nreturn "x"\n}\n' \
		>"$APP/scopecheck/scope.go"
	git -C "$APP" add scopecheck/scope.go
	code="$(capture_in "$APP" "$p.changed-fail" "$A" verify --only gofmt --changed --json)"
	assert_status "$p.changed-fail" 9
	assert_jq "$p.changed-fail.stderr" \
		'.data.scope.mode == "changed" and (.data.scope.changed_files | index("scopecheck/scope.go"))'
	assert_jq "$p.changed-fail.stderr" \
		'.data.scope.stages == ["gofmt"]'
	assert_jq "$p.changed-fail.stderr" \
		'.data.steps[] | select(.name == "gofmt") | .violations[0].file == "scopecheck/scope.go"'

	gofmt -w "$APP/scopecheck/scope.go"
	code="$(capture_in "$APP" "$p.changed-pass" "$A" verify --only gofmt --changed --json)"
	assert_status "$p.changed-pass" 0
	assert_jq "$p.changed-pass.stdout" \
		'.data.scope.mode == "changed" and (.data.scope.changed_files | index("scopecheck/scope.go"))'
	assert_jq "$p.changed-pass.stdout" \
		'.data.steps[] | select(.name == "gofmt") | .status == "ok"'
}

write_summary() {
	local acceptance=full
	if [[ "$BLOCKED" -gt 0 || "$FAILURES" -gt 0 ]]; then
		acceptance=partial
	fi
	{
		printf '# F40/F06 hygiene and doctor probes\n\n'
		printf 'acceptance: %s\n\n' "$acceptance"
		printf '| Probe | Status | Exit |\n| --- | --- | --- |\n'
		local row name status code
		for row in "${RESULT_ROWS[@]:-}"; do
			IFS=$'\t' read -r name status code <<<"$row"
			[[ -n "$name" ]] || continue
			printf '| %s | %s | %s |\n' "$name" "$status" "$code"
		done
		printf '\npassed=%d blocked=%d failed=%d\n' "$PASSED" "$BLOCKED" "$FAILURES"
		printf '\n## Coverage\n\n'
		printf -- '- F40: selected harness skill install and git tracking, no lockfile shipped by `andurel new`, generated pnpm lockfile, `pnpm install --frozen-lockfile` success without rewrite, frozen drift rejection, ignored `.env`/`node_modules`/`bin`/`dist`/`assets/dist`/`tmp`/database, and setup negative -> positive after `andurel tool sync` + `pnpm install`.\n'
		printf -- '- F06: doctor missing configuration, malformed configuration without echoing the rejected secret, exported-only configuration, and an unreachable database with a secret-safe diagnostic; each negative exits nonzero and names the blocking check.\n'
		printf -- '- Scoped/changed/content: `verify --only content` not_applicable/ok/fail, and `verify --only gofmt --changed` records the changed file and package scope.\n'
		if [[ "$acceptance" == partial ]]; then
			printf '\nPARTIAL ACCEPTANCE: %d probe(s) blocked and %d failed. This is not full acceptance.\n' "$BLOCKED" "$FAILURES"
		fi
	} >"$EVIDENCE/probes.md"
	log "passed=$PASSED blocked=$BLOCKED failed=$FAILURES acceptance=$acceptance"
	if [[ "$acceptance" == partial ]]; then
		log "PARTIAL ACCEPTANCE: blocked probes are not passes; see $EVIDENCE/probes.md"
	fi
	log "evidence: $EVIDENCE"
	printf '%s' "$acceptance" >"$EVIDENCE/acceptance-state.txt"
}

main() {
	cd "$ROOT"
	write_environment
	prepare
	run_probe 01-new-react-pnpm-harness probe_new_react_pnpm_harness
	run_probe 02-local-replacements probe_local_replacements
	run_probe 03-lockfile-frozen-install probe_lockfile_frozen_install
	run_probe 04-setup-positive probe_setup_positive
	run_probe 05-doctor-missing-env probe_doctor_missing_env
	run_probe 06-doctor-malformed-env probe_doctor_malformed_env
	run_probe 07-doctor-export-only-unreachable-db probe_doctor_export_only_unreachable_db
	run_probe 08-hygiene-ignored-outputs-secrets probe_hygiene_ignored_outputs_secrets
	run_probe 09-verify-scoped-changed-content probe_verify_scoped_changed_content
	write_summary
	if [[ "$FAILURES" -gt 0 ]]; then
		exit 1
	fi
	if [[ "$REQUIRE_ALL" -eq 1 && "$BLOCKED" -gt 0 ]]; then
		exit 1
	fi
}

main "$@"
