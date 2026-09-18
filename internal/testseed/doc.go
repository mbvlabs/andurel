// Package testseed provides deterministic clocks and entropy for golden CLI
// builds. Production binaries (without the andurel_golden build tag) keep
// real randomness and wall-clock time. Golden binaries built with
// -tags andurel_golden lock scaffold secrets and timestamps so raw file
// assertions need no scrubbers.
package testseed
