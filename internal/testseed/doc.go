// Package testseed provides deterministic clocks and entropy for golden CLI
// builds. Production binaries (without the andurel_golden build tag) keep
// real randomness and wall-clock time. Golden binaries built with
// -tags andurel_golden lock scaffold secrets and timestamps so raw file
// assertions need no scrubbers. The CLI version string itself is not handled
// here: golden TestMain injects -ldflags -X main.version=latest at build time.
//
// # Golden-tag behavior
//
// When Enabled() is true (binary built with -tags andurel_golden):
//
//   - RandomReader / Now: fixed secret stream and 2025-01-01T00:00:00Z clock
//     (scaffold .env.example secrets, migration filenames, CLI date banner).
//   - layout.Scaffold: after writing templates + andurel.lock, skips goose fix,
//     templ generate, narsilc generate, go mod tidy, and go fmt (network /
//     tool-version churn). Capture allowlisted template outputs only.
//   - cli generateNarsilcIfNeeded: skips invoking the narsilc binary.
//   - generator.Coordinator: uses NopPrimaryKeyResolver (no interactive prompts).
//   - generator.ModelManager: skips packages.Load factory type-checks.
package testseed
