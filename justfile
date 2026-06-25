# Build aliases
alias b := build

# Test aliases
alias t := test
alias tc := test-critical
alias ta := test-all

# Default recipe - show available commands
default:
	@just --list

# Build the andurel binary
build:
	go build -o andurel-dev main.go

# Build a local snapshot using GoReleaser (requires goreleaser installed)
release-snapshot:
	goreleaser release --snapshot --clean

move:
	mv andurel-dev ../

# Scaffolding recipes
scaf-psql:
	cd ../ && ./andurel-dev new myp-psql && mv ./andurel-dev ./myp-psql && cd ./myp-psql && cp .env.example .env && just new-migration users

full-psql:
	just build
	just move
	just scaf-psql

full:
	just build
	just move

# ============================================================================
# Testing Commands
# ============================================================================

# Run go vet
vet:
	go vet ./...

# Run unit tests (excludes e2e, fast)
test:
	go list ./... | grep -v /e2e | xargs go test -v

# Run unit tests with coverage
test-cover:
	go list ./... | grep -v /e2e | xargs go test -v -race -coverprofile=coverage.txt -covermode=atomic

# Run critical e2e tests only
test-e2e-critical:
	go clean -testcache
	E2E_CRITICAL_ONLY=true go test ./e2e/... -v -timeout 25m

# Run full e2e test suite
test-e2e-full:
	go clean -testcache
	go test ./e2e/... -v -timeout 55m

# Run scaffold golden e2e tests
test-e2e-scaffold:
	go clean -testcache
	go test ./e2e -run TestScaffoldGoldens -v -timeout 30m

# Run critical tests (unit + critical e2e, recommended for PRs)
test-critical:
	@echo "Running unit tests..."
	@just test
	@echo "\nRunning critical e2e tests..."
	@just test-e2e-critical

# Run all tests (unit + full e2e suite)
test-all:
	@echo "Running unit tests..."
	@just test
	@echo "\nRunning full e2e test suite..."
	@just test-e2e-full

# Run quick check (vet + unit tests, very fast)
check:
	@echo "Running go vet..."
	@just vet
	@echo "\nRunning unit tests..."
	@just test

# Run full CI check (vet + unit tests + critical e2e, matches PR workflow)
ci:
	@echo "Running go vet..."
	@just vet
	@echo "\nRunning unit tests with coverage..."
	@just test-cover
	@echo "\nRunning critical e2e tests..."
	@just test-e2e-critical
	@echo "\n✅ All CI checks passed!"

# Update scaffold golden files
update-golden:
	go clean -testcache
	go test ./e2e -run TestScaffoldGoldens -v -timeout 30m -update -clean

# Clean test artifacts and cache
clean-test:
	go clean -testcache
	rm -f coverage.txt
	rm -rf /tmp/andurel-e2e-*
