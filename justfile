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
# ============================================================================

# Run go vet
vet:
	go vet ./...

# Run unit tests (excludes ./golden CLI goldens — use just test-golden)
test:
	go test $(go list ./... | grep -v '/golden$$') -count=1

# Run unit tests with coverage (excludes golden; CLI goldens use just test-golden)
test-coverage:
	./scripts/coverage.sh

# Run golden CLI tests (PR track: generate + sync)
test-golden: install-dev-tools
	go test ./golden/... -v -timeout 15m

# Run golden CLI tests including nightly full-tree `andurel new`
test-golden-full: install-dev-tools
	ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m

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

# Run full CI check (vet + contracts + golden)
ci:
	@echo "Running go vet..."
	@just vet
	@echo "\nChecking contracts..."
	@just check-contracts
	@echo "\nRunning golden tests..."
	@just test-golden
	@echo "\n✅ All CI checks passed!"

# Install formatter/codegen tools used by golden updates (pinned; match layout/versions)
install-dev-tools:
	./scripts/install-dev-tools.sh

# Update PR golden files under testdata/golden (generate + sync)
update-golden: install-dev-tools
	go clean -testcache
	go test ./golden/... -v -timeout 15m -update

# Update nightly full-tree `new/` goldens (also refreshes generate + sync)
update-golden-full: install-dev-tools
	go clean -testcache
	ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m -update

# Clean test artifacts and cache
clean-test:
	go clean -testcache
	rm -f coverage.out coverage-summary.out coverage.txt
	rm -rf /tmp/andurel-e2e-* /tmp/andurel-golden-*
