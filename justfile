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

# Run pkg unit tests (CLI/generator goldens live under ./golden)
test:
	go test ./pkg/... -v

# Run unit tests with coverage (pkg only; CLI goldens use just test-golden)
test-coverage:
	./scripts/coverage.sh

# Run golden CLI tests (builds andurel with -tags andurel_golden)
test-golden: install-dev-tools
	go test ./golden/... -v -timeout 15m

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
	@echo "\nRunning pkg tests..."
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

# Update golden files under testdata/golden (smoke + generate + sync + new)
update-golden: install-dev-tools
	go clean -testcache
	go test ./golden/... -v -timeout 15m -update

# Alias for update-golden
update-golden-all: update-golden

# Clean test artifacts and cache
clean-test:
	go clean -testcache
	rm -f coverage.out coverage-summary.out coverage.txt
	rm -rf /tmp/andurel-e2e-* /tmp/andurel-golden-*
