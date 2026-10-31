# rc.1 port-feedback implementation worklist

Source: `../andurel-site/plans/beta4-port/`. Baseline: `fe808b61`.
Implementation workers: `openai/gpt-6.1-terra#medium`. No tags, pushes, releases,
production access, or published-pin changes are authorized by this worklist.

## Implementation lanes

| Lane | Items | Deliverable | Status |
| --- | --- | --- | --- |
| A: verifier | B4-012; positional scope reporting | Robustly exclude only synthetic Go test mains, retain enforcement on real tests/generated source, regression-test whole/scoped loads and report package scope accurately. | Running: rc1/verifier |
| B: domain timestamps | B4-009 | Plain Go timestamps at scaffold domain/service boundary; domain-only SQL conversion; explicit set/preserve/clear nullable updates; align domain pattern and real-Postgres tests. | Running: rc1/timestamps |
| C: contracts and port guidance | B4-013/014; B4-002/003; typed-nil guidance | Register routes with Pattern(), preserve Path() for URLs; support heterogeneous JSON maps if safely representable, otherwise clearly document supported workaround; document nested DTO mapping, recursive nullability, omission/raw JSON and FromStruct typed nil. | Running: rc1/contracts |
| D: generation safety | B4-005 | Inspect query generation manifest mutation; expose safe preview/control and clear machine/human disclosure using existing CLI conventions; deterministic regression coverage, no unsupported blame for historical duplicate requirements. | Running: rc1/generation |
| Integration/security | Preview capability review; regression acceptance | Review existing telemetry/log sinks and test coverage, fix newly reproduced framework leaks only; regenerate shared goldens once; full checks and fresh-site candidate validation. | Running; initial anduin/inertia/telemetry uncached suites pass |

## Classification of every remaining ledger item

- B4-001: existing `andurel --version` works outside projects; document it alongside inspect discovery rather than introduce a redundant command.
- B4-002/003: intentional conventions; retain app/ and independent payloads/, improve guidance in lane C, do not weaken rules.
- B4-004: site Tiptap overrides/lockfile already resolved; no framework dependency change.
- B4-006: site domain-owned test fixtures already resolved; preserve SQL boundary.
- B4-007: demo credentials are development-only; review scaffold warnings, no production provisioning or credentials.
- B4-008: site product/operational follow-ups, not framework release defects: version metadata publication policy, renderer/request/search/diff budgets, password-reset/API-token policy. Keep explicitly open for site owner; do not silently change application behavior.
- B4-010: site acceptance harness already fixed; no framework work.
- B4-011: site bundle budgets/splitting deferred; no warning suppression.
- B4-015: local C compiler prerequisite resolved; preserve race-toolchain guidance.
- B4-016: latest site docs refreshed; historical 1.5.2 docs remain untouched. Framework guidance updated only where relevant.
- Preview URL safety: review fresh evidence and framework sinks; upstream proxy logs require operator configuration and cannot be guaranteed by app code.

## Worker contract

Use isolated worktrees and respect repository AGENTS.md. Use patch/edit tools;
never Python/Perl. Write a failing regression first for defects. Run gofmt,
`go fix ./...`, `go vet ./...`, and relevant tests; each changed pkg module is
independent and requires the same checks plus an Unreleased changelog entry.
Do not regenerate shared scaffold goldens in worker lanes. Do not alter module
pins or unrelated user files. Commit lane changes and report exact commands,
results and blockers. Integration owns shared goldens and final validation.

## Acceptance / release boundary

- Root gofmt, go fix, go vet and uncached tests; changed nested modules checked independently.
- Updated golden/pattern/CLI metadata tests; actual PostgreSQL coverage for nullable timestamp semantics.
- Reproduce B4-012 against minimal same-package and external tests, scoped/all-project selections and warm/fresh cache; forbidden source imports must still fail.
- Candidate CLI against the completed beta.4 site: full verify with tests, inspect skips, keep application checkout changes intentional and bounded.
- Fresh external scaffold with published dependency pins; build/typecheck/generated verification where tooling permits. Do not call local candidate validation published rc.1 validation.
- Record commands/results and unresolved environment/product/operational limits. Another project port remains a separate pre-rc.1 decision.
