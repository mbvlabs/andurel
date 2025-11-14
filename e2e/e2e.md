# E2E Test Suite

## Overview

End-to-end tests that build the andurel binary and test scaffolding and code generation commands.

## Running Tests

```bash
# Run all critical tests (subset for CI)
E2E_CRITICAL_ONLY=true go test ./e2e/... -v -timeout 10m

# Run all tests
go test ./e2e/... -v -timeout 10m
```

## Test Coverage

### ✅ Scaffold Matrix (6/6 passing)
Tests project scaffolding in all database/CSS/extension combinations:
- PostgreSQL + Tailwind
- PostgreSQL + Vanilla CSS
- SQLite + Tailwind
- SQLite + Vanilla CSS
- SQLite + Docker extension
- PostgreSQL + Auth extension

Validates: Files created correctly, `go vet ./...` passes

### ✅ Migration Workflow (2/2 passing)
Tests migration creation and model generation workflow:
- PostgreSQL: Create migration → Generate model
- SQLite: Create migration → Generate model

Validates: Migration files created, model/query files generated

### ✅ Generate Commands (16/16 passing)
Tests code generation commands:
- ✅ `generate model` (postgresql & sqlite)
- ✅ `generate controller --with-views` (postgresql & sqlite)
- ✅ `generate view` (postgresql & sqlite)
- ✅ `generate resource` (postgresql & sqlite)

## Implementation Notes

- Binary built once in `TestMain` and reused across all tests
- Migration files use single-file goose format (not split .up/.down files)
- Database-specific SQL: UUID for PostgreSQL, TEXT for SQLite
- Generated code intentionally incomplete (users must fill in params)
- No `go vet` checks on generated code (expected to have compilation errors)
