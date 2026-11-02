# Final root and scaffold golden verification

On 2026-11-02, Terra medium session `ses_e64a9eb62ffegL1xWaY5ykoUqM`
reported completion in the integration worktree after the auth, JSON-message,
README skip-list and runner proxy fixes had settled.

- Full local-package golden update and check passed with
  `ANDUREL_GOLDEN_LOCAL_PKG=1 ANDUREL_GOLDEN_FULL=1`, `GOWORK=off`,
  `GOFLAGS=-p=2`, `GOMAXPROCS=2`, and `-count=1`. The commands were
  `go test ./golden/... -v -timeout 45m -count=1 -update` and the same command
  without `-update`, using disk-backed temporary directories outside the repo.
- Full PostgreSQL React, Vue and Svelte scaffolds and slim React and Vue cases
  passed. Auth domain/service/test fixtures and the latest README were refreshed.
- Root `go test -timeout 20m ./...` and `go vet ./...` passed after the proxy fixes.
- `go fix -diff ./...`, `gofmt -l .`, and `git diff --check` produced no output.

Logs were retained at `/home/mbv/work/andurel-golden-final.aQjWPWpi/`.
This records the agent's completed validation report, not a new execution by the
coordinating session. Subsequent existing-database pattern guidance changes need
their own targeted checks. No source implementation was edited by this run.

These are local-package results. They do not verify unpublished versions or
replace the outstanding release-pinned, no-replacement scaffold run.

## Post shared-Docker safety fix global rerun

On 2026-11-02 the following commands completed in the integration worktree
after the Linux boot/PID namespace scope fix and public probe. The golden runs
used `ANDUREL_GOLDEN_LOCAL_PKG=1 ANDUREL_GOLDEN_FULL=1 GOWORK=off GOFLAGS=-p=2
GOMAXPROCS=2`, with both `TMPDIR` and `GOTMPDIR` set to the dedicated disk-backed
directory `/home/mbv/work/andurel-postfix-validation.g7VVSqVQ/tmp`:

- `go test ./golden/... -v -timeout 45m -count=1 -update`: PASS, 203.089s.
- `go test ./golden/... -v -timeout 45m -count=1`: PASS, 200.514s; no skips
  reported. Both runs exercised full PostgreSQL React, Vue and Svelte, and the
  configured slim React and Vue cases.
- `go test -timeout 20m ./...`: PASS; some package results were cached. This
  root run used the normal workspace configuration, `GOFLAGS=-p=2`,
  `GOMAXPROCS=2`, and the same temporary directories.
- `go vet ./...` and `go fix -diff ./...`: PASS, no output, using the same
  root-run environment. `gofmt -l .` and `git diff --check`: PASS, no output.

Golden and root-test logs are retained in
`/home/mbv/work/andurel-postfix-validation.g7VVSqVQ/`. The separate full storage
suite and public shared-daemon safety proof are documented in [CLI/storage](beta7-final-cli-storage-verification.md);
they were not repeated here. This local-package validation is not a substitute
for the blocked release-pinned, no-replacement scaffold run.
