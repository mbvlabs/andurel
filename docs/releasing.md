# Releasing Andurel

Release tags must identify a commit that has already passed the canonical readiness gate on `master`.

1. Merge the release changes to `master`.
2. Wait for the `Release Readiness` check to pass on that exact `master` commit.
3. Create and push the new version tag only after that check is green.

The tag-triggered release workflow verifies tag identity and confirms that the tagged commit has a successful post-merge `Release Readiness` run on `master`. It does not create artifacts until that proof passes. It then creates a signed private draft, smoke-tests every supported archive, attests the release artifacts and SBOMs, and publishes only the verified draft.

## Module resolution

CI, `just` recipes, and release builds run with `GOWORK=off` so the CLI
resolves `pkg/*` from `go.mod` the same way `go install
github.com/mbvlabs/andurel/v2@…` does. The repo `go.work` is only for local
ad-hoc development.

If the CLI imports a symbol that exists only in a newer local `pkg/*` tree,
tests fail until that package is tagged and the root `go.mod` require is
bumped. That red build is intentional.

## Standalone modules

Modules under `pkg/` have independent semantic versions. Use Go's subdirectory
tag format, for example `pkg/inertia/v0.1.0`. Before tagging, verify the module
with `GOWORK=off`, update its `CHANGELOG.md`, and update the framework's verified
package version set and the root `go.mod` require when the CLI depends on it.
Generated applications must resolve the tag without a workspace or `replace`
directive.
