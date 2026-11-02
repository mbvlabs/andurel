# anduin v0.8.0 release preflight

Status: **READY FOR READ-ONLY REVIEW.** After that review, the parent may make
the local release commit and local tag. No commit, tag, push, publication or
deployment was performed by this preflight.

Prepared: 2026-11-02.

- Worktree: `/home/mbv/work/andurel-rc1-work`
- Branch: `fix/rc1-final-readiness`
- Baseline HEAD: `289b1778de20f86210dbec749a1c622ff6018d65`
- Release scope: `pkg/anduin` only, version `v0.8.0`

The anduin code in this file set is the previously independently accepted
final code. This preflight does not reopen or rewrite those acceptance claims.

## Release identity

- Tag to create: `pkg/anduin/v0.8.0`
- Changelog heading: `## 0.8.0 - 2026-11-02`
- The `Unreleased` section is empty; all previously unreleased anduin entries
  were moved under `0.8.0`.
- `pkg/anduin/go.mod` has no version field to change: Go module versions come
  from the tag. `go.mod` and `go.sum` are unchanged by this preflight.
- The tag must point at the commit that contains this changelog cut and the
  exact `pkg/anduin` source below. The parent performs the local commit/tag;
  the user pushes.

## Exact source diff file set

`git status --porcelain -- pkg/anduin` at preflight:

```text
 M pkg/anduin/CHANGELOG.md
 M pkg/anduin/context.go
 M pkg/anduin/doc.go
 M pkg/anduin/handler.go
 M pkg/anduin/host.go
 M pkg/anduin/router.go
?? pkg/anduin/delivery_writer.go
?? pkg/anduin/errorlifecycle.go
?? pkg/anduin/router_error_context_test.go
?? pkg/anduin/router_error_lifecycle_test.go
```

Modified files (`git diff --stat -- pkg/anduin`, 6 files, 319 insertions,
31 deletions):

| File | Diff |
| --- | --- |
| `pkg/anduin/CHANGELOG.md` | +81 |
| `pkg/anduin/context.go` | +21 |
| `pkg/anduin/doc.go` | +22 |
| `pkg/anduin/handler.go` | +32/-2 |
| `pkg/anduin/host.go` | +14/-3 |
| `pkg/anduin/router.go` | +149/-26 |

Added files (working-tree content, 3,636 lines total):

| File | Lines |
| --- | --- |
| `pkg/anduin/delivery_writer.go` | 96 |
| `pkg/anduin/errorlifecycle.go` | 512 |
| `pkg/anduin/router_error_context_test.go` | 1,326 |
| `pkg/anduin/router_error_lifecycle_test.go` | 1,702 |

Working-tree blob IDs (`git hash-object`) for exact identification:

```text
35a64ea281c1ad205f85626ea411455b097af6eb  pkg/anduin/CHANGELOG.md
58987d51e3d6ef8893668605852bda7ba5c3fbe4  pkg/anduin/context.go
7d54ca7c46bc276974c6aabde0602f6b96223756  pkg/anduin/doc.go
b85d419e96110a93eb6a7ad29a08fda9c8fc0e0d  pkg/anduin/handler.go
f411def76c0c1055de41571690b6cc2fa0394a7e  pkg/anduin/host.go
279e5346d7826996442f207336c031040fe747e4  pkg/anduin/router.go
2e622646c4f2c4f64afa4148b8ad555f4802afcb  pkg/anduin/delivery_writer.go
b18858f7a03c97d4bf44c546eee158f656e8895b  pkg/anduin/errorlifecycle.go
cf834f73b62ecb64dcbdea5a4f5508a22c600be9  pkg/anduin/router_error_context_test.go
eb4dcf716b5b05c97985799e00ba562936129356  pkg/anduin/router_error_lifecycle_test.go
```

No other file under `pkg/anduin` changed. In particular
`pkg/anduin/go.mod` and `pkg/anduin/go.sum` have no status entries. No shared
pin/golden file (`layout/versions/versions.go`, `testdata/golden/...`),
framework `go.mod`/`go.work`, release notes file or other package was modified
by this release step. Other worktree changes outside `pkg/anduin` are the
pre-existing accepted rc.1 work and are not part of this release file set.

## Change made in this preflight

`pkg/anduin/CHANGELOG.md` only:

- Moved the existing `Unreleased` contents under
  `## 0.8.0 - 2026-11-02`, matching the existing heading/date format.
- Left `## Unreleased` in place with no entries.

No code, test, module metadata or pin file was edited.

## Module metadata checks

`go mod edit -json` reports:

- Module: `github.com/mbvlabs/andurel/pkg/anduin`
- Go: `1.27.3`
- No `replace` directives.
- Direct requirements:
  - `github.com/mbvlabs/andurel/pkg/routing v0.9.0`
  - `github.com/mbvlabs/andurel/pkg/telemetry v0.2.3`
  - `go.opentelemetry.io/proto/otlp v1.11.0`
  - `google.golang.org/protobuf v1.36.12`

The root `go.work` was overridden with `GOWORK=off` for every release command,
so the module resolved and tested standalone with no workspace or local
replacement. The preflight environment was:

```sh
export GOWORK=off
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org
cd /home/mbv/work/andurel-rc1-work/pkg/anduin
```

`GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and `GOFLAGS` were unset/empty.

Results:

| Check | Result |
| --- | --- |
| `go mod edit -json` (replace scan) | no `replace` directives |
| `go mod verify` | `all modules verified` |
| `go mod tidy -diff` | empty output, exit 0 (metadata already tidy; no edit) |
| `go list -m all` | resolved; routing `v0.9.0` and telemetry `v0.2.3` present |
| `git status --porcelain -- pkg/anduin/go.mod pkg/anduin/go.sum` | empty |

`go.sum` contains verified entries for every direct requirement, including:

```text
github.com/mbvlabs/andurel/pkg/routing v0.9.0 h1:e7C5aMzgz44JE6/4HS1Uvr3NEhEjAMpYXW8HibzvUZk=
github.com/mbvlabs/andurel/pkg/telemetry v0.2.3 h1:T70mFoL8oKDNByxQt+qbew3GiNVMlqNYBYxknrXWcE0=
go.opentelemetry.io/proto/otlp v1.11.0 h1:5rrYs0Ykyj50sdU/JU0x8etU+LubXWb+gED6TbEdMIk=
google.golang.org/protobuf v1.36.12 h1:pJOKDDOyeXErUroCihFAd5LQuwXBSpVnKGrj5o/fwxc=
```

## Public dependency evidence

Every direct pinned dependency was fetched through the public Go proxy
(`https://proxy.golang.org`) with checksum verification enabled. Both the
`.info` and `.mod` endpoints returned HTTP 200:

| Module | Version | Proxy `.info` | Proxy `.mod` |
| --- | --- | --- | --- |
| `github.com/mbvlabs/andurel/pkg/routing` | `v0.9.0` | 200 | 200 |
| `github.com/mbvlabs/andurel/pkg/telemetry` | `v0.2.3` | 200 | 200 |
| `go.opentelemetry.io/proto/otlp` | `v1.11.0` | 200 | 200 |
| `google.golang.org/protobuf` | `v1.36.12` | 200 | 200 |

Check command:

```sh
curl -fsS -o /dev/null -w '%{http_code}\n' \
  "https://proxy.golang.org/<module>/@v/<version>.info"
curl -fsS -o /dev/null -w '%{http_code}\n' \
  "https://proxy.golang.org/<module>/@v/<version>.mod"
```

The public `go list -m -json` origins resolved routing to
`refs/tags/pkg/routing/v0.9.0` (commit `892630e6d62337e0b0c2d0df70c724a76f7412c9`)
and telemetry to `refs/tags/pkg/telemetry/v0.2.3` (commit
`f55963112cb50a84ed9fd7322e94fe0e905d7a33`). Indirect dependencies resolved in
the same public graph and passed `go mod verify`.

## Standalone verification

All commands were run from `pkg/anduin` with the exported environment above.

| Command | Result |
| --- | --- |
| `gofmt -l .` | empty output, exit 0 |
| `go fix ./...` | exit 0; no working-tree changes |
| `gofmt -l .` (after fix) | empty output, exit 0 |
| `go vet ./...` | exit 0 |
| `go test -count=1 ./...` | PASS: `pkg/anduin` 0.244s, `pkg/anduin/anduintest` 0.006s |
| `CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' go test -race -count=1 ./...` | PASS: `pkg/anduin` 1.412s, `pkg/anduin/anduintest` 1.036s |
| `go build ./...` | exit 0 |
| `git diff --check -- pkg/anduin` | exit 0, no whitespace errors |

Zig: `0.14.1` at
`/home/mbv/.local/share/mise/installs/zig/0.14.1/zig`; `CGO_ENABLED=1` with
that CC was used only for the race command.

## Guardrails

- No commit, tag, push, publish or deployment was performed.
- No version edit in `pkg/anduin/go.mod` or `pkg/anduin/go.sum`.
- No shared pin, golden, framework module, release notes or other package file
  was modified by this release step.
- Prior evidence, acceptance records and handoff documents were read only and
  were not rewritten.
- This report is the only new non-source file from this preflight.

## Reviewer handoff

Read-only reviewer should confirm:

1. The `pkg/anduin` file set and hash list above match the reviewed accepted
   source.
2. `CHANGELOG.md` has an empty `Unreleased` section followed by
   `## 0.8.0 - 2026-11-02` with the existing entries intact.
3. `go.mod`/`go.sum` are unchanged and contain no local replace.
4. The standalone public-proxy commands and results above are reproducible.

After that, the parent may commit the release file set locally and create the
local `pkg/anduin/v0.8.0` tag. Push and publication remain the user's actions.
