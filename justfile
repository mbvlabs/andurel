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
	go build -o dev-andurel main.go

# Build a local snapshot using GoReleaser (requires goreleaser installed)
release-snapshot:
	goreleaser release --snapshot --clean

move:
	mv dev-andurel ../

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
	go test $(go list ./... | grep -v /e2e) -v -race -cover -coverprofile coverage.out -coverpkg ./...
	go tool cover -func coverage.out -o coverage.out
	tail -1 coverage.out

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

# Update golden files for generator model tests
update-golden-generator-models:
	go clean -testcache
	go test ./generator -run TestModelGenerationGoldens -v -update

# Update golden files for generator controller/view tests
update-golden-generator-controller-views:
	go clean -testcache
	go test ./generator -run TestControllerViewGenerationGoldens -v -update

# Update golden files for generator scaffold tests
update-golden-generator-scaffold:
	go clean -testcache
	go test ./generator -run TestScaffoldGenerationGoldens -v -update

# Update all golden files
update-golden-all:
	go clean -testcache
	go test ./generator -run TestModelGenerationGoldens -v -update
	go test ./generator -run TestControllerViewGenerationGoldens -v -update
	go test ./generator -run TestScaffoldGenerationGoldens -v -update
	go test ./e2e -run TestScaffoldGoldens -v -timeout 30m -update -clean

# Clean test artifacts and cache
clean-test:
	go clean -testcache
	rm -f coverage.txt
	rm -rf /tmp/andurel-e2e-*
