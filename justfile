# Build aliases
alias b := build

# Test aliases
alias t := test
alias tc := test-critical
alias ta := test-all

# Default recipe - show available commands
default:
    @just --list

# ============================================================================
# Build / local helpers
# ============================================================================

# Build a local snapshot using GoReleaser (requires goreleaser installed)
release-snapshot:
    goreleaser release --snapshot --clean

# Build the andurel binary
build:
    go build -ldflags "-X main.version=$(git rev-parse HEAD)" -o dev-andurel main.go

move:
    mv dev-andurel ~/.local/bin

bmo:
    just build
    just move

# ============================================================================
# Testing / CI
#
# GitHub Actions calls these recipes instead of inlining go test / go vet.
# PR: just ci-pr. Nightly / release-readiness: just ci-nightly / just ci-readiness.
# ============================================================================

# Run go vet on the module and standalone pkg/* modules
vet:
    #!/usr/bin/env bash
    set -euo pipefail
    go vet ./...
    repo_root="$(pwd)"
    while IFS= read -r module_file; do
    	module_dir="$(dirname "$module_file")"
    	echo "vetting ${module_dir#"$repo_root"/}"
    	(
    		cd "$module_dir"
    		GOWORK=off go mod download
    		GOWORK=off go vet ./...
    	)
    done < <(find "$repo_root/pkg" -mindepth 2 -maxdepth 2 -name go.mod -print | sort)

# Run unit tests (excludes ./golden CLI goldens — use just test-golden)
test: install-dev-tools
    go test $(go list ./... | grep -v '/golden$$') -count=1

# Run unit tests with race detection (PR and nightly CI)
test-race: install-dev-tools
    go test $(go list ./... | grep -v '/golden$$') -race -count=1

# Run unit tests with coverage (excludes golden; CLI goldens use just test-golden)
test-coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    package_list="$(go list ./...)"
    packages=()
    while IFS= read -r package; do
    	if [[ -z "${package}" ]]; then
    		continue
    	fi
    	case "${package}" in
    		*/e2e | */e2e/* | */golden | */golden/*) ;;
    		*) packages+=("${package}") ;;
    	esac
    done <<< "${package_list}"
    if (( ${#packages[@]} == 0 )); then
    	echo "no coverable Go packages found" >&2
    	exit 1
    fi
    go test "${packages[@]}" \
    	-race \
    	-covermode=atomic \
    	-coverpkg=./... \
    	-coverprofile=coverage.out
    go tool cover -func=coverage.out -o coverage-summary.out
    awk '/^total:/ {printf "total statement coverage: %s\n", $3; found=1} END {exit !found}' coverage-summary.out

# Run golden CLI tests (PR track: generate + sync + slim new MVC)
test-golden: install-dev-tools
    env -u ANDUREL_GOLDEN_FULL go test ./golden/... -v -timeout 15m

# Run golden CLI tests including nightly full-tree `andurel new`
test-golden-full: install-dev-tools
    ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m

# Fail if gofmt would change any file
check-fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [[ -n "$unformatted" ]]; then
    	echo "gofmt would change the following files:"
    	echo "$unformatted"
    	exit 1
    fi

# Fail if go fix would change any file
check-fix:
    #!/usr/bin/env bash
    set -euo pipefail
    go fix -diff ./... | tee /tmp/go-fix.diff
    test ! -s /tmp/go-fix.diff
    git diff --exit-code -- .

# Verify module download, checksums, and tidy drift
check-mod:
    #!/usr/bin/env bash
    set -euo pipefail
    go mod download
    go mod verify
    go mod tidy -diff

# Run golangci-lint via the pinned wrapper
lint *args:
    #!/usr/bin/env bash
    set -euo pipefail
    readonly expected_version="2.13.2"
    installed_version="$(golangci-lint version 2>&1)"
    if [[ "${installed_version}" != *"has version ${expected_version} "* ]]; then
    	echo "golangci-lint ${expected_version} is required" >&2
    	echo "found: ${installed_version}" >&2
    	exit 1
    fi
    lint_cache="$(mktemp -d "${TMPDIR:-/tmp}/andurel-golangci-lint.XXXXXX")"
    go_cache="$(mktemp -d "${TMPDIR:-/tmp}/andurel-go-build.XXXXXX")"
    cleanup() {
    	rm -rf "${lint_cache}" "${go_cache}"
    }
    trap cleanup EXIT INT TERM
    GOLANGCI_LINT_CACHE="${lint_cache}" GOCACHE="${go_cache}" \
    	golangci-lint run --config .golangci.yml {{ args }}

# Run govulncheck on the module
vulncheck:
    govulncheck ./...

# Run critical tests (vet + golden)
test-critical:
    @echo "Running go vet..."
    @just vet
    @echo "\nRunning golden tests..."
    @just test-golden

# Run all currently available checks
test-all:
    @just test-critical
    @echo "\nRunning unit tests..."
    @just test

# Run quick check (vet)
check:
    @just vet

# PR CI suite (Test workflow). Coverage stays a separate best-effort step.
ci-pr:
    @just vet
    @just test-race
    @just test-golden

# Nightly / release-readiness suite (full goldens + race)
ci-nightly:
    @just vet
    @just test-golden-full
    @just test-race

# Local alias for the full release-readiness validation suite
ci-readiness:
    @just check-mod
    @just check-fmt
    @just check-fix
    @just vet
    @just lint
    @just vulncheck
    @just test-race
    @just test-golden-full

# Local alias for the PR CI suite
ci:
    @just ci-pr

# Install formatter/codegen tools used by golden updates (pinned; match layout/versions)
install-dev-tools:
    #!/usr/bin/env bash
    # Versions must match layout/versions. Golden TestMain also installs these
    # into a temp GOBIN, and downloads narsilc/tailwindcli/goose from andurel.lock.
    set -euo pipefail
    readonly TEMPL_VERSION="${ANDUREL_TEMPL_VERSION:-v0.3.1020}"
    readonly GOLINES_VERSION="${ANDUREL_GOLINES_VERSION:-v0.13.0}"
    readonly GOIMPORTS_VERSION="${ANDUREL_GOIMPORTS_VERSION:-v0.44.0}"
    go install "github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}"
    go install "github.com/segmentio/golines@${GOLINES_VERSION}"
    go install "golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION}"

# Update PR golden files under testdata/golden (generate + sync + slim new)
update-golden: install-dev-tools
    go clean -testcache
    env -u ANDUREL_GOLDEN_FULL go test ./golden/... -v -timeout 15m -update

# Update nightly full-tree `new/` goldens (also refreshes generate + sync)
update-golden-full: install-dev-tools
    go clean -testcache
    ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m -update

# Clean test artifacts and cache
clean-test:
    go clean -testcache
    rm -f coverage.out coverage-summary.out coverage.txt
    rm -rf /tmp/andurel-e2e-* /tmp/andurel-golden-*

# ============================================================================
# Release
# ============================================================================

# Verify a release tag peels to the expected commit and matches HEAD
verify-release-tag tag sha:
    #!/usr/bin/env bash
    set -euo pipefail
    tag="{{ tag }}"
    expected_sha="{{ sha }}"
    if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    	echo "release tag ${tag} does not match the required semantic tag pattern" >&2
    	exit 1
    fi
    if [[ ! "${expected_sha}" =~ ^[0-9a-f]{40}$ ]]; then
    	echo "expected SHA must be a full lowercase 40-character commit ID" >&2
    	exit 1
    fi
    release_ref="refs/tags/${tag}^{commit}"
    if ! release_commit="$(git rev-parse --verify "${release_ref}" 2>/dev/null)"; then
    	echo "release tag ${tag} does not exist or does not peel to a commit" >&2
    	exit 1
    fi
    head_commit="$(git rev-parse --verify HEAD)"
    if [[ "${release_commit}" != "${expected_sha}" ]]; then
    	echo "release tag ${tag} peels to ${release_commit}, expected ${expected_sha}" >&2
    	exit 1
    fi
    if [[ "${head_commit}" != "${expected_sha}" ]]; then
    	echo "HEAD is ${head_commit}, expected release commit ${expected_sha}" >&2
    	exit 1
    fi

# Smoke-test a signed release archive (checksums, SBOM, binary --version)
smoke-release-archive archive tag asset_dir signing_identity="":
    #!/usr/bin/env bash
    set -euo pipefail
    archive="$(cd "$(dirname "{{ archive }}")" && pwd)/$(basename "{{ archive }}")"
    tag="{{ tag }}"
    asset_dir="$(cd "{{ asset_dir }}" && pwd)"
    version="${tag#v}"
    archive_name="$(basename "${archive}")"
    checksum_file="${asset_dir}/checksums.txt"
    sbom_checksum_file="${asset_dir}/sbom-checksums.txt"
    sbom="${asset_dir}/${archive_name}.sbom.json"
    signing_identity="{{ signing_identity }}"
    if [[ -z "${signing_identity}" ]]; then
    	signing_identity="https://github.com/${GITHUB_REPOSITORY:-mbvlabs/andurel}/.github/workflows/release.yml@refs/tags/${tag}"
    fi
    case "$(uname -s)" in
    	Linux) expected_os=linux ;;
    	Darwin) expected_os=darwin ;;
    	*) echo "unsupported smoke-test operating system: $(uname -s)" >&2; exit 1 ;;
    esac
    case "$(uname -m)" in
    	x86_64|amd64) expected_arch=amd64 ;;
    	arm64|aarch64) expected_arch=arm64 ;;
    	*) echo "unsupported smoke-test architecture: $(uname -m)" >&2; exit 1 ;;
    esac
    expected_name="andurel_${version}_${expected_os}_${expected_arch}.tar.gz"
    test "${archive_name}" = "${expected_name}"
    test -f "${archive}"
    test -f "${sbom}"
    test -f "${checksum_file}"
    test -f "${checksum_file}.sigstore.json"
    test -f "${sbom_checksum_file}"
    test -f "${sbom_checksum_file}.sigstore.json"
    cosign verify-blob \
    	--bundle "${checksum_file}.sigstore.json" \
    	--certificate-identity "${signing_identity}" \
    	--certificate-oidc-issuer https://token.actions.githubusercontent.com \
    	"${checksum_file}"
    cosign verify-blob \
    	--bundle "${sbom_checksum_file}.sigstore.json" \
    	--certificate-identity "${signing_identity}" \
    	--certificate-oidc-issuer https://token.actions.githubusercontent.com \
    	"${sbom_checksum_file}"
    verify_manifest_entry() {
    	local manifest="$1"
    	local file="$2"
    	local name
    	local expected
    	local actual
    	name="$(basename "${file}")"
    	expected="$(awk -v name="${name}" '$2 == name {print $1}' "${manifest}")"
    	test -n "${expected}"
    	if command -v sha256sum >/dev/null 2>&1; then
    		actual="$(sha256sum "${file}" | awk '{print $1}')"
    	else
    		actual="$(shasum -a 256 "${file}" | awk '{print $1}')"
    	fi
    	test "${actual}" = "${expected}"
    }
    verify_manifest_entry "${checksum_file}" "${archive}"
    verify_manifest_entry "${sbom_checksum_file}" "${sbom}"
    jq -e '.spdxVersion | startswith("SPDX-")' "${sbom}" >/dev/null
    tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/andurel-release-smoke.XXXXXX")"
    cleanup() {
    	rm -rf "${tmp_dir}"
    }
    trap cleanup EXIT INT TERM
    install_dir="${tmp_dir}/install"
    mkdir -p "${install_dir}"
    tar -xzf "${archive}" -C "${install_dir}"
    binary="${install_dir}/andurel"
    test -x "${binary}"
    "${binary}" --version | grep -F "${version}" >/dev/null
    "${binary}" commands --json > "${tmp_dir}/commands.json"
    jq -e '.ok == true and (.data.commands | type == "array")' "${tmp_dir}/commands.json" >/dev/null
