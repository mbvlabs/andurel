# Build aliases
alias b := build

# Test aliases
alias t := test
alias tc := test-critical
alias ta := test-all

# Default recipe - show available commands
default:
	@just --list

# Regenerate public API, CLI, and lock contract fixtures
update-contracts:
	./scripts/update-contracts.sh

# Verify committed contract fixtures match the source tree
check-contracts:
	./scripts/check-contracts.sh

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
# Testing Commands
#
# GitHub Actions calls these recipes instead of inlining go test / go vet.
# PR: just ci-pr. Nightly / release-readiness: just ci-nightly.
# ============================================================================

# Run go vet on the module and standalone pkg/* modules
vet:
	go vet ./...
	./scripts/vet-standalone-modules.sh

# Run unit tests (excludes ./golden CLI goldens — use just test-golden)
test: install-dev-tools
	go test $(go list ./... | grep -v '/golden$$') -count=1

# Run unit tests with race detection (PR and nightly CI)
test-race: install-dev-tools
	go test $(go list ./... | grep -v '/golden$$') -race -count=1

# Run unit tests with coverage (excludes golden; CLI goldens use just test-golden)
test-coverage:
	./scripts/coverage.sh

# Run golden CLI tests (PR track: generate + sync + slim new MVC)
test-golden: install-dev-tools
	env -u ANDUREL_GOLDEN_FULL go test ./golden/... -v -timeout 15m

# Run golden CLI tests including nightly full-tree `andurel new`
test-golden-full: install-dev-tools
	ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m

# Fail if gofmt would change any file
check-fmt:
	test -z "$(gofmt -l .)"

# Run golangci-lint via the pinned wrapper
lint:
	./scripts/lint.sh

# Run critical tests (vet + contracts + golden)
test-critical:
	@echo "Running go vet..."
	@just vet
	@echo "\nChecking contracts..."
	@just check-contracts
	@echo "\nRunning golden tests..."
	@just test-golden

# Run all currently available checks
test-all:
	@just test-critical
	@echo "\nRunning unit tests..."
	@just test

# Run quick check (vet + contracts)
check:
	@echo "Running go vet..."
	@just vet
	@echo "\nChecking contracts..."
	@just check-contracts

# PR CI suite (Test workflow). Coverage stays a separate best-effort step.
ci-pr:
	@just check-contracts
	@just vet
	@just test-race
	@just test-golden

# Nightly / release-readiness suite (full goldens + race)
ci-nightly:
	@just check-contracts
	@just vet
	@just test-golden-full
	@just test-race

# Local alias for the PR CI suite
ci:
	@just ci-pr

# Install formatter/codegen tools used by golden updates (pinned; match layout/versions)
install-dev-tools:
	./scripts/install-dev-tools.sh

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
