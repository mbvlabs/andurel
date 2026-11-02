# rc.1 publication handoff (owner-controlled)

Status: **DRAFT — local implementation and postmerge gates accepted, and
updated-runtime site live/image/docs parity independently accepted; framework
candidate not yet release-ready.** This is a freeze snapshot for the
parent/orchestrator, not framework release approval. The anduin and inertia
packages are owner-published and publicly verified; kiks remains `v0.6.1`. All
local gates on source HEAD `41657d5d` / tree `f2f56478` are independently
accepted. Final updated-runtime site live/image/docs parity is independently
accepted in `ses_e624cd00bffe4dosR1x3y1tC7z` (220-file manifest
`708790e5`; published pins/image/auth 403 both document and Inertia; Chrome;
docs parity 53 URLs/47 pages/0 findings/1 intentional entry difference; own
cleanup preserved 7 containers/4 networks; no secret hits). Exact final-master
push/CI and release readiness including artifact preflight, plus the owner rc.1
go/no-go/tag, remain pending. The framework rc.1 tag, pushes and any publication
of the framework candidate remain unauthorized.

Prepared: 2026-11-02. Framework worktree:
`/home/mbv/work/andurel-rc1-work`, branch `fix/rc1-final-readiness`, HEAD
`41657d5dee3f043916c2cb12a3bc4a28ee19082b`, tree
`f2f56478c84bb8fe53d2530e16c186a13576ea43`, which merges accepted
`origin/master` `ee84032c` (narsilc `v0.6.4` scaffold change #857). Reviewed
source/tooling/pin work is committed in `edc738c0`; kiks test metadata is
committed in `d6f062f9`. The tracked baseline and pre-existing untracked files
were preserved; only `plans/` and the draft release notes are untracked here.

Read-only inputs audited for this handoff:

- `/home/mbv/work/andurel/specs/rc1-final-readiness.md`
- `plans/rc1-final-readiness/overview.md`
- `plans/rc1-final-readiness/t02-coordinated.md`
- `plans/rc1-final-readiness/t02-inertia.md`
- `plans/rc1-final-readiness/t02-kiks.md`
- `plans/rc1-final-readiness/t02-integration.md`
- `pkg/anduin/go.mod`, `pkg/inertia/go.mod`, `pkg/kiks/go.mod`
- `layout/versions/versions.go`, package changelogs, existing tags and release
  notes style

## Current gate status

- **Published runtime dependencies: DONE.**
  - `pkg/anduin v0.8.0` was owner-published at tag `718076ed`; public proxy and
    checksum verification passed.
  - `pkg/inertia v0.14.1` was owner-published at tag `c6c724fa`; public proxy
    and checksum verification passed.
  - `pkg/kiks` stays `v0.6.1`; no new kiks release was created.
- **Local/postmerge gates: independently ACCEPTED.** The exact postmerge source
  is HEAD `41657d5d` / tree `f2f56478`. Merged pins were reviewed in
  `ses_e621993d0ffeeBhVhYcas763fU`; gate evidence was reviewed in
  `ses_e62033d55ffeGY5dRfO1pE62rm`. Verified evidence:
  - full release golden `133.537s`, all 439 hashes stable;
  - public pins `12/12`;
  - `check-fix` PASS, not waived;
  - root race across 36 packages plus 3 standalone public read-only checks;
  - three actual upgrade cases, including beta.5 and beta.7 -> rc.1;
  - pinned lint 0 issues; pinned vuln root + 12 packages with no called
    vulnerabilities; vet root + 12 packages pass.
- **T02 final reviews: closed.** All T02 final reviews are closed, including the
  last barrier. Full root and affected-module race/vet/lint/vuln checks with
  public dependencies pass.
- **Framework pins/goldens: ACCEPTED for the exact postmerge tree.**
  `layout/versions/versions.go` and the three Inertia scaffold `go.mod.golden`
  files contain anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`, and narsilc
  tool/runtime `v0.6.4`. The full release golden evidence above is accepted.
- **Standalone published integration: ACCEPTED.** The published-module
  integration test passed 10x and under race; the 3 standalone public read-only
  checks also pass.
- **Candidate E2E: ACCEPTED under independent review.** The three actual upgrade
  cases, including beta.5 -> rc.1 and beta.7 -> rc.1, are green. Historical
  beta.4 -> beta.5 evidence is preserved and retained.
- **Site pins/regressions and live/image/docs parity: independently ACCEPTED.**
  Full site verify passed 375 tests and 516 subtests with 0 skips. The final
  updated-runtime live/image/docs parity is accepted in
  `ses_e624cd00bffe4dosR1x3y1tC7z`: all 220 manifest `708790e5` hashes,
  published pins and image identity, authenticated 403 props in both document
  and Inertia, Chrome gate, docs parity 53 URLs/47 pages/0 findings/1
  intentional entry difference, own cleanup preserved 7 containers/4 networks,
  and no secret hits. The premerge verifier was adequate for project config
  only, not final CLI proof; parity is not historical-source parity.
- **Normal merge: DONE.** HEAD `41657d5d` merges accepted `origin/master`
  `ee84032c` (#857). The combined constants are narsilc tool/runtime `v0.6.4`,
  anduin `v0.8.0`, inertia `v0.14.1`, and kiks `v0.6.1`. Published package tags
  `718076ed`/`c6c724fa` remain immutable.
- **Framework candidate: NOT RELEASE READY.** Updated-runtime site
  live/image/docs parity is accepted. Exact final-master push/CI and release
  readiness including artifact preflight have not run, and no owner rc.1
  go/no-go/tag decision has been recorded. No rc.1 tag exists, and the candidate
  is not eligible to tag pending CI.
- **Authorization: packages/pins only.** Package and pin work is owner-authorized;
  the final rc.1 framework tag, push and publication are not authorized.

## Published dependency record

| Module | Version | Tag/commit | Public verification | State |
| --- | --- | --- | --- | --- |
| `pkg/anduin` | `v0.8.0` | `pkg/anduin/v0.8.0` / `718076ed` | Public proxy and checksum confirmed | Published |
| `pkg/inertia` | `v0.14.1` | `pkg/inertia/v0.14.1` / `c6c724fa` | Public proxy and checksum confirmed | Published |
| `pkg/kiks` | `v0.6.1` | `pkg/kiks/v0.6.1` | Unchanged published baseline | No new release |
| `pkg/routing` | `v0.9.1` | `pkg/routing/v0.9.1` | Existing published baseline | Unchanged |
| framework | `v2.0.0-beta.7` | `v2.0.0-beta.7` | Existing framework tag | rc.1 not tagged |

Public-only module downloads used for verification (no workspace or replacement):

```sh
GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  go mod download -json github.com/mbvlabs/andurel/pkg/anduin@v0.8.0
GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  go mod download -json github.com/mbvlabs/andurel/pkg/inertia@v0.14.1
```

The published anduin checksum is
`h1:jWpOUeyy2a7DblnBebgg3Hs3S8jC9Af3eaJztzwL8SU=`; the published inertia checksum
is `h1:TT54ERsQ+IVGEz4/7ZmXVmWcYIM61+YZPjQRdDTRrN8=`. These are recorded from the
completed public verification, not inferred or selected by this document.

## Narsilc v0.6.4 integration (completed via accepted master)

`origin/master` merged the accepted narsilc `v0.6.4` scaffold change (#857) as
`ee84032c`. The normal merge into this branch is complete at HEAD `41657d5d`,
and the combined constants are now narsilc tool/runtime `v0.6.4`, anduin
`v0.8.0`, inertia `v0.14.1`, and kiks `v0.6.1`. The scaffold `andurel.toml`,
`andurel.lock` and generated `go.mod` goldens reflect the merged narsilc value.

Narsilc `v0.6.4` entered through the accepted master change, **not** because the
site manually aligned itself to `v0.6.4`. The published package tags
`718076ed`/`c6c724fa` were not moved. Postmerge local pin/golden and release
gates are independently accepted for this exact tree; see the gate evidence
above.

## Worktree freeze target

HEAD `41657d5d` contains the reviewed root/tooling/pin work (`edc738c0`) and the
kiks test metadata (`d6f062f9`). Only the notes/plans files are untracked at
this freeze point. The parent will commit all root notes/evidence with the
reviewed DRAFT notes; historical evidence claims are preserved. No commit,
push, tag, package publication or framework release is performed by this
handoff.

Current untracked documentation/evidence:

- `docs/release-notes/v2.0.0-rc.1.md` — still draft, not ready;
- `plans/rc1-final-readiness/` — handoff and evidence notes, including this
  file.

The already-published package-release commits are `718076ed` and `c6c724fa`.
Do not rewrite or move those published tags.

## Next action and remaining sequence

1. Parent commits all root notes/evidence and the reviewed DRAFT notes at the
   frozen source. **Owner then pushes the branch** and proceeds with the PR to
   master; master readiness must pass.
2. Complete the exact final-master push/CI and release readiness including
   artifact preflight for the candidate SHA.
3. Replace the draft banner only after those gates pass, then make the owner
   rc.1 go/no-go decision.
4. Stop at eligible to tag. rc.1 tagging and pushing remain separate owner
   actions and are not authorized here.

The package versions are settled: anduin `v0.8.0` and inertia `v0.14.1` are
published; kiks stays at its published `v0.6.1` baseline.

## Current pins and files

| Path | Current state | Remaining action |
| --- | --- | --- |
| `layout/versions/versions.go` | Anduin `v0.8.0`, Inertia `v0.14.1`, Kiks `v0.6.1`, Narsilc tool/runtime `v0.6.4` | Committed at merged HEAD; pin gate accepted |
| `pkg/inertia/go.mod` / `go.sum` | Published in `pkg/inertia/v0.14.1` with anduin `v0.8.0` | Immutable published tag; no further edit |
| `pkg/kiks/go.mod` / `go.sum` | Committed in `d6f062f9` for published anduin `v0.8.0`; Kiks pin stays `v0.6.1` | No kiks tag; local gate accepted |
| `testdata/golden/new/postgresql-inertia-react/go.mod.golden` | anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`, narsilc `v0.6.4` | Committed at merged HEAD; golden gate accepted |
| `testdata/golden/new/postgresql-inertia-svelte/go.mod.golden` | anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`, narsilc `v0.6.4` | Committed at merged HEAD; golden gate accepted |
| `testdata/golden/new/postgresql-inertia-vue/go.mod.golden` | anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`, narsilc `v0.6.4` | Committed at merged HEAD; golden gate accepted |
| `docs/release-notes/v2.0.0-rc.1.md` | Draft, not ready | Finalize after final-master CI gates pass and owner go/no-go |
| Site `go.mod` / `go.sum` | Site pins/regressions accepted; full verify 375 tests/516 subtests/0 skips | Live/image/docs parity accepted in `ses_e624cd00bffe4dosR1x3y1tC7z` |
| Site `andurel.toml` / `andurel.lock` | Accepted with the site pin update; framework version stays `v2.0.0-beta.7` | Do not hand-edit; use the supported upgrade process; point at rc.1 only once a CLI is published |
| Root `go.mod` / `go.work` | No change expected; release proof uses `GOWORK=off` | Do not add local replacement proof |

No lock digest or checksum is prescribed by this document. Package checksums
come only from the published module graph and the Go checksum database.

## Compatibility impact for the release notes

### Global and host middleware errors render earlier

- A global or host middleware error now renders at the failing boundary, before
  enclosing middleware unwinds. `ErrorHandler` receives the context actually
  passed to the failing layer, so middleware values such as the kiks bag/session
  and enclosing resources are available, and kiks-style persistence sees the
  final status (including 409 flashes, 303 redirects and 403 clears).
- **This is the intentional compatibility change:** global/host middleware that
  consumes, translates or retries an error must wrap its layer with
  `CatchErrors`. Without it, the inner guard has already rendered before the
  outer middleware sees the error. `CatchErrors` propagates the original error,
  renders once at scope exit with the latest failing pass, and supports nested
  scopes where the outer owner consumes/translates/retries before the single
  render.
- Route and handler errors keep the existing delayed render point after route
  middleware unwinds. Existing route consumers such as `APIDocumentationErrors`
  continue to return their own response and prevent a router render; route
  middleware needs no `CatchErrors` change.
- Route scopes retain the deepest middleware values from the latest failing
  pass, but not resources owned by middleware that has already returned.
  Delayed contexts merge those values with live cancellation and freeze the
  first classification: an expired deadline is `DeadlineExceeded` with its
  custom cause, explicit cancellation or cleanup is `Canceled`, cleanup before
  a later deadline stays `Canceled`, and derived `WithCancel`/`WithCancelCause`
  children inherit it; no deadline timer is recreated. A context created and
  discarded entirely inside a middleware that never passes it to `next` remains
  unrecoverable.
- Rendering is once-only. After an early render, retries return
  `ErrResponseCommitted` through `CommittedError`, which still unwraps the
  rendered error. `RenderedError(ctx)` lets a coordinated capture writer decide
  to commit the captured error page.
- `ResponseBuffer`/`DiscardBuffered` lets a coordinating writer discard a
  buffered success before the error page. An old inertia capture without the
  interface is detected as mixed versions and gets one safe plain 500, not a
  themed error, and may not persist kiks error status. Pins must move together;
  do not claim end-to-end themed host/global errors with old inertia.
- ServeMux 404/405 errors now travel through the host/global boundary, so
  `CatchErrors` can consume or translate them. `Allow` is preserved across
  capture discard and the response contract/fallback behavior is unchanged.
  Global/host middleware now observe these mux errors after the early render
  where they previously saw nil; this is intentional.
- B7-007 ordering is unchanged: host middleware runs before the host mux,
  including mux-generated `Isolate` 404/405 responses; fallback-to-primary
  requests bypass host middleware.

Known middleware limits to keep in the notes:

- Early host/global renders guarantee enclosing resources; delayed
  route/`CatchErrors` renders guarantee values, not resources already released.
- A partial response already delivered cannot be replaced after an error; the
  router logs it and avoids a second render.
- Inertia capture header restoration is a narrow metadata policy (`Allow`,
  `Vary`, security/CORS policy and standard authentication/retry metadata), not
  a general header transaction.
- Kiks persists the bag exactly once, on the first response write:
  `persistWriter` saves in `WriteHeader`, `Write` or `Flush`/`FlushError`
  before forwarding that operation, using the status observed at that write.
  If the response never writes, the middleware's `next` return path saves once
  at status 200. Because that single save already happened at the first write,
  an `AddFlash` (or other bag mutation) after the response starts is never
  persisted; add flashes before writing. A `CatchErrors` scope that needs the
  kiks bag must be installed inside `Jar.Middleware`; an outer scope renders
  after kiks has already saved (status 200 when nothing wrote first).
- `ResponseBuffer` support is required for themed early host/global error pages;
  old captures fall back as described above.

### Tool/runtime diagnostics

- `andurel tool set-version narsilc` remains scoped to tool configuration,
  installation and the lock file. It never edits `go.mod` or `go.sum`.
- After a successful tool update it inspects the `github.com/mbvlabs/narsilc`
  requirement with the Go module parser and semver helpers. A mismatch prints
  the exact `go get github.com/mbvlabs/narsilc@<version>` and `go mod tidy`
  remediation; structured output carries the same state in `narsilc_runtime`,
  warnings and next steps.
- `andurel doctor` reports the same alignment on human and structured surfaces.
  Replaced, unreadable or malformed module data is reported as unverifiable,
  never aligned. A missing runtime requirement is reported as not required and
  is not injected.

### Upgrade JSON and skipped stages

- Structured `andurel upgrade` never runs `go mod tidy` or `andurel tool sync`.
  `commands_run` contains only subprocesses actually attempted, including
  read-only `git status --porcelain` safety checks; failed attempts carry a
  `(failed)` suffix. The fabricated follow-up commands were removed.
- Required tidy/tool-sync work is returned as conditional warnings and
  breadcrumbs. Dry runs and pending repairs phrase it as proposed work that was
  not run. Structured error data preserves the failed report and actual command
  provenance.
- `ok: true` means the upgrade mutation completed, not that the project builds,
  was tidied or had tool binaries installed.
- A verify stage may be `status: "skipped"`, `ok: false`,
  `not_applicable: true` while the command-level envelope remains `ok: true`.
  Consumers must inspect the stage status; B7-003 is a documented contract, not
  a schema change.

### Synthetic test-main filtering

- `isSyntheticTestMain` no longer relies on project-root containment. It uses a
  conservative composite of go/packages identity, the actual Go build-cache
  data-artifact layout (`<cache>/<2 hex>/<64 hex>-d`) and the exact generated
  `go test` header.
- `GOCACHE` is resolved lazily only when a candidate exists. Empty, `off`,
  failed or unavailable lookups do not filter source. Symlink escapes are
  resolved before containment.
- Real `.test` main packages with generated-looking headers, ordinary generated
  project files and genuine `_test.go` violations remain violations. There is
  no blanket cache-directory or header filter. The pinned Go/x/tools metadata
  test makes a future cache-layout change visible.

## Scaffold and site migration checks

### Fresh scaffold

The exact postmerge full-release golden gate is **ACCEPTED**: `133.537s`, all
439 hashes stable, with anduin `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`, and
narsilc `v0.6.4`. The merged scaffold outputs retain the published package pins
and no `replace` directives, and the narsilc-related `andurel.toml.golden`,
`andurel.lock.golden` and generated module values match the merged `v0.6.4`
defaults. No regeneration or local rerun is required unless source changes.

### Existing application and site

- Site pins/regressions and updated-runtime live/image/docs parity are
  **ACCEPTED**: full site verify passed 375 tests and 516 subtests with 0
  skips; live/image/docs parity was independently accepted in
  `ses_e624cd00bffe4dosR1x3y1tC7z` (scope above).
- The site's `sharedAuth` `ErrNoBag` fallback may remain as an explicit
  workaround. Remove it only with separate test coverage. Validate that a
  signed-in host error sees a real bag/authenticated session, not merely the
  themed component and 403.
- Validate Inertia capture commit for global/host errors, buffered-success
  replacement, 405 `Allow`, rate-limit `Retry-After`, authentication challenges,
  CORS/CSP/security headers, kiks 409/303/403 persistence, AuthOnly-style
  redirect cookies, and route API JSON consumers such as
  `APIDocumentationErrors`.
- Rerun site generation twice and confirm empty generated-contract/module diffs.
  Then run the site's established checks (see
  `plans/beta7-port/verification-run.md`):

```sh
pnpm install --frozen-lockfile
pnpm build
pnpm build:ssr
andurel verify --with-tests --json
go test -p 1 -count=1 ./...
```

- The historical beta.7 T07 evidence is preserved as historical provenance and
  must not be presented as proof of the final changed runtime. Final post-pin
  site/runtime T09 live/image/docs parity is accepted in
  `ses_e624cd00bffe4dosR1x3y1tC7z`; it is current-content parity, not
  historical-source parity, and it does not itself prove the final rc.1 CLI.

## Accepted local gates and remaining final gates

The exact postmerge local gate set is **ACCEPTED** for source HEAD `41657d5d` /
tree `f2f56478`, with merged pins reviewed in
`ses_e621993d0ffeeBhVhYcas763fU` and gate evidence reviewed in
`ses_e62033d55ffeGY5dRfO1pE62rm`:

- full release golden `133.537s`, all 439 hashes stable;
- public pins `12/12`;
- `check-fix` PASS, not waived;
- root race 36 packages plus 3 standalone public read-only checks;
- 3 actual upgrade cases, including beta.5 and beta.7 -> rc.1;
- pinned lint 0 issues; pinned vuln root + 12 with no called vulnerabilities;
  vet root + 12 pass;
- published standalone integration accepted.

No local rerun is required unless source changes. The remaining externally
gated checks are:

- exact final-master push/CI and release readiness including artifact preflight;
- the owner rc.1 go/no-go/tag.

Site T09 (disposable copy, candidate CLI; final updated-runtime live/image/docs
parity independently accepted in `ses_e624cd00bffe4dosR1x3y1tC7z`):

```sh
cd /tmp/opencode/site-rc1-copy
/tmp/opencode/andurel-rc1 upgrade --dry-run --json > /tmp/opencode/site-upgrade-dry-run.json
/tmp/opencode/andurel-rc1 upgrade --json > /tmp/opencode/site-upgrade.json
# Inspect commands_run and warnings, then run the reported follow-ups:
go mod tidy
/tmp/opencode/andurel-rc1 tool sync
pnpm install --frozen-lockfile
pnpm build
pnpm build:ssr
/tmp/opencode/andurel-rc1 verify --with-tests --json
go test -p 1 -count=1 ./...
```

T07 candidate live/image rerun (fresh evidence path outside `TMPDIR`, no reused
or skipped mandatory cases; see site `t07.md` for the exact harness contract):

```sh
scripts/site-acceptance.sh --live \
  --candidate-cli <exact-candidate-cli> \
  --evidence /tmp/opencode/t07-rc1-<fresh-id>
PATH=<candidate-bin>:$PATH VERSION=<fresh-id> \
  scripts/image-build-local.sh andurel-site:<fresh-id>
scripts/image-smoke-local.sh andurel-site:<fresh-id>
```

The acceptance record must show the exact source/image identity, zero mandatory
skips, safe cleanup and an independent reviewer. Do not use production services
or remove pre-existing containers. Do not claim T09 from a workspace/local-
replace run.

## Remaining owner/parent gates

- [x] Anduin `v0.8.0` owner-published; public proxy/checksum verified.
- [x] Inertia `v0.14.1` owner-published; public proxy/checksum verified.
- [x] Kiks remains `v0.6.1`; no new kiks release.
- [x] All T02 final reviews closed; root and affected-module race/vet/lint/vuln
      checks with public dependencies pass.
- [x] Framework pins/goldens **ACCEPTED** for the exact postmerge tree:
      full release golden `133.537s`, 439 hashes stable, public pins `12/12`.
- [x] `check-fix` PASS, not waived; root race 36 packages + 3 standalone public
      read-only checks; 3 actual upgrade cases including beta.5 and beta.7 ->
      rc.1; pinned lint 0 issues; pinned vuln root + 12 with no called
      vulnerabilities; vet root + 12 pass.
- [x] Standalone published integration accepted at 10x and under race.
- [x] Candidate E2E beta.5 -> rc.1 and beta.7 -> rc.1 green under independent
      review; historical beta.4 -> beta.5 evidence preserved.
- [x] Site pins/regressions accepted at source: full verify 375 tests / 516
      subtests / 0 skips.
- [x] Normal merge completed at HEAD `41657d5d` / tree `f2f56478`, merging
      accepted `origin/master` `ee84032c` (#857).
- [x] Reviewed root/tooling/pin changes committed in `edc738c0`; kiks test
      metadata committed in `d6f062f9`.
- [x] Narsilc `v0.6.4` integrated through the accepted master change, not a
      site-only pin move; combined constants are narsilc `v0.6.4`, anduin
      `v0.8.0`, inertia `v0.14.1`, kiks `v0.6.1`.
- [x] Updated-runtime site live/image/docs parity independently accepted in
      `ses_e624cd00bffe4dosR1x3y1tC7z`; site pins/regressions remain accepted
      at source (full verify 375 tests / 516 subtests / 0 skips).
- [ ] Exact final-master push/CI and release readiness including artifact
      preflight completes for the candidate SHA.
- [ ] Parent commits all root notes/evidence and the reviewed DRAFT notes.
- [ ] Owner pushes the branch after the local docs commit and proceeds with the
      PR to master; master readiness must pass.
- [ ] Finalize release notes and make the owner rc.1 go/no-go; no rc.1 tag, push
      or publish is authorized by this handoff.

## Verification of this handoff

- Docs-only reconciliation (2026-11-02): this handoff, the framework overview
  and `docs/release-notes/v2.0.0-rc.1.md` were edited to record the
  independently accepted updated-runtime site live/image/docs parity and to
  remove the stale site-pending gate. No source, metadata, golden, package,
  pin or tag/push/publish state was changed.
- The freeze snapshot records the published package commits/tags and the
  independently accepted local gates for source HEAD `41657d5d` / tree
  `f2f56478`. Historical evidence claims are preserved.
- `just check-release-notes v2.0.0-rc.1` is re-run after the draft update; it
  validates the release-note path shape and does not tag or publish.
