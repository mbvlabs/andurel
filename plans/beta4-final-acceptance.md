# Beta.4 final acceptance

Branch: `beta4/integration`
Worktree: `/home/mbv/work/worktrees/andurel-b4-integration`
Integration head tested: `0486cce0` (this evidence document is committed after
that code head).
Build temp and evidence root: `/home/mbv/work/build-tmp/b4-integration`

## Delivered range

- `c20a938d` merge `beta4/cli`, with all four conflicts resolved and no conflict
  markers.
- `fa6795ac` queue telemetry context, routing subtree accessor, local-pkg golden
  harness.
- `71ee12ee` payload fixture and slim/full golden refresh, including `app/`,
  `payloads/`, and `cmd/configcheck/` trees.
- `7943b03f` testing skill reference.
- `0a3760d4`, `4ac7385c`, `3b793cb3` beta.4 release notes, upgrade docs, and
  verify example corrections.
- Review fixes: `04dcd7e3` telemetry initial redaction; `2289833c`
  transactional queue inserts through `FailingTransactor`; `17bbe380`
  capability-token prefix validation; `d7d30e28` nil options on bound
  `AtomicOps`; `00bac7cb` changelog correction.
- Terra review fixes: `f59aa2e8` payload JSON contracts; `2fa561ee`
  `FromStruct` embedded-field semantics; `8aa8f98a` ambiguous host bindings;
  `f33029e9` nullable-slice TypeScript compile test; `78fd67f1` omitempty only
  on the outer slice; `0486cce0` multiassignment host targets.

## Final validation

Environment: Go 1.27.3, Node 26.12.1, pnpm 12.9.1, Docker 29.8.2.
`TMPDIR=GOTMPDIR=/home/mbv/work/build-tmp/b4-integration`.

Root:

```text
gofmt -l .                                      -> empty
go fix ./...                                    -> no output
go vet ./...                                    -> no output
go test -count=1 ./...                          -> pass
```

Root result: 20 packages `ok`, 6 packages `no test files`, 0 `FAIL`, 0
package-level test skips in `root-tests-final-acceptance.log`.

Nested modules:

```text
for each pkg/*/go.mod:
  gofmt -l . && go fix ./... && go vet ./... && go test -count=1 ./...
```

Result: all modules passed (`pkg-modules-terra-fixes.log`). The Docker-backed
`pkg/storage/storagetest` suite ran 32.0s; the verbose run recorded 26 PASS and
0 SKIP (`storagetest-verbose.log`), including the real-Postgres domain + River
rollback test.

Golden suite:

```text
ANDUREL_GOLDEN_LOCAL_PKG=1 ANDUREL_GOLDEN_FULL=1 \
  go test -count=1 ./golden -timeout 45m       -> pass in 102.813s
```

This includes all three full-tree scaffolds (React, Vue, Svelte) and
`TestPatternsCompile`. No uncommitted golden drift after the run
(`golden-final-acceptance.log`).

Fresh React/pnpm scaffold:
`/home/mbv/work/build-tmp/b4-integration/scratch-followup/app`.

```text
go build ./...                                  -> pass
go vet ./...                                    -> pass
go test -count=1 ./...                          -> pass
pnpm typecheck                                  -> pass (tsc --noEmit)
andurel verify                                  -> pass
```

`andurel verify` steps: setup/gofmt/go fix/go vet/layers/frontend/generated
`ok`; content `SKIP` because there is no `andurel.content.toml`; go test
`SKIP` because `--with-tests` was not set. Logs:
`go-test-terra-fixes.log`, `pnpm-typecheck-final-acceptance.log`,
`verify-final-acceptance.log`.

Generated payload TypeScript was compiled with the scratch app's real
`tsc` via `ANDUREL_TSC`, covering quoted inline JSON names and nested nullable
slices (`TestRenderPayloadsTSAppliesOmitemptyOnlyToOuterSlice`,
`TestRenderPayloadsTSQuotesInlineJSONNames`).

## Review findings closed

- Telemetry starts spans at `/[redacted]`, protecting direct `WrapHandler`,
  pre-telemetry rejection, and `OnStart` processors; matched routes still
  replace it and are covered by OTLP tests.
- `TransactionUnwrapper` restores transactional River inserts through the
  fault-injecting transactor; a real-Postgres test proves the domain row and
  queued job both roll back.
- Capability-token prefixes are limited to ASCII letters, digits, `-`, `_`.
- Nil `AtomicOps` options are ignored on bound and unbound transactors.
- Non-omitempty slices/bytes are nullable; `omitempty` applies only to the
  outer slice; nested slice elements stay nullable.
- Embedded payload fields follow `encoding/json` flattening/nesting, pointer
  nil omission, and conflict dominance in both generator and `FromStruct`;
  lazy `Prop`/function values are preserved.
- `database/sql` `Null*` payload fields are rejected with pointer guidance.
- Inline JSON names are quoted identically to top-level names.
- Host-local inspection omits ambiguous bindings, including multiassignment
  reassignment and `:=` shadowing.
- The testing skill documents race detection, clocks, DB versioning, fault
  injection, read-only/repeatable-read transactions, and capability tokens.

## Skips and blocked gates

- Published-pin release-shaped proof is **blocked**: the standalone module tags
  for the new routing/storage/auth/validation/telemetry/anduin APIs are not
  published. The scratch scaffold's last published pins fail compilation with
  `routes.ViteBuild.InSubtree undefined`; this is recorded in
  `published-pin-build-failure.log`. No pins were bumped to nonexistent
  versions and no release-shaped claim is made.
- Local package replacements are development-only and explicitly recorded in
  `scratch-followup/local-replacements.txt`.
- Upgrade E2E was not run; it requires released CLI artifacts.
- Vue/Svelte were compile/build/typecheck validated earlier in
  `scratch`; the final full-tree Go golden compile ran for all adapters.
- Content verification is `SKIP` on the scratch app because it has no content
  configuration. `verify --with-tests` was not part of the final fresh-app run;
  uncached DB tests were run directly.

## narsilc beta.4 work

Separate, validated, pending integration/publication:
`/home/mbv/work/worktrees/narsilc-b4`, commits `12509c1a0` (`AS` aliases,
duplicate-tag rejection) and `c4e5232f4` (`NOT NULL` parameter typing). They
are not part of this branch and no narsilc version was bumped or published.

## Release policy

No tags, pushes, or module publications were performed. No app Dockerfiles or
app-only fixes were added.
