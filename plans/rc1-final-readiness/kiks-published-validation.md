# Kiks published anduin v0.8.0 validation

Status: **READY FOR INDEPENDENT REVIEW.** Metadata-only update in `pkg/kiks`.
No kiks runtime code, changelog, version or tag was changed. The framework
`Kiks` pin remains `v0.6.1`.

Prepared: 2026-11-02.

- Worktree: `/home/mbv/work/andurel-rc1-work`
- Branch: `fix/rc1-final-readiness`
- HEAD: `718076edfafe0735032bf6a2d457b5c09bbd8251` (`feat(anduin): release v0.8.0`)
- Module: `github.com/mbvlabs/andurel/pkg/kiks`
- Change: require published anduin `v0.8.0` so the accepted
  `pkg/kiks/router_error_test.go` tests using `anduin.CatchErrors` and the
  coordinated host APIs build with `GOWORK=off`.
- Expected diff: `pkg/kiks/go.mod` and `pkg/kiks/go.sum` only, with no `replace`
  directive. The pre-existing accepted test and changelog edits are preserved.

## Scope and guardrails

- No kiks release, tag, changelog cut or module version was created. The
  `## Unreleased` changelog section and all accepted entries are untouched.
- No tracked `pkg/kiks/*.go` file changed. `git diff --name-only --
  'pkg/kiks/*.go'` is empty.
- `pkg/kiks/router_error_test.go` (untracked, pre-existing accepted work) and
  `pkg/kiks/CHANGELOG.md` (modified, pre-existing accepted work) were not
  edited by this metadata update.
- `layout/versions/versions.go` still has `Kiks = "v0.6.1"` and has no working
  tree diff. Framework pins and scaffold goldens are outside this task.
- No root module, `go.work`, `pkg/inertia` or any other shared module/file was
  changed. Inertia work is a separate parallel module.
- No commit, tag, push, publication or deployment was performed.

## Published dependency verification

Environment for every command:

```sh
cd /home/mbv/work/andurel-rc1-work/pkg/kiks
export GOWORK=off
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org
unset GOPRIVATE GONOSUMDB GONOPROXY GOFLAGS
```

`go env GOWORK GOPROXY GOSUMDB GOFLAGS GOPRIVATE` reported:

```text
off
https://proxy.golang.org
sum.golang.org


```

### anduin v0.8.0 origin

`go list -m -json github.com/mbvlabs/andurel/pkg/anduin@v0.8.0` (and the public
`.info` file in the module cache) reports:

```json
{
  "Path": "github.com/mbvlabs/andurel/pkg/anduin",
  "Version": "v0.8.0",
  "Time": "2026-11-02T20:00:50Z",
  "Sum": "h1:jWpOUeyy2a7DblnBebgg3Hs3S8jC9Af3eaJztzwL8SU=",
  "GoModSum": "h1:2lRaNyJmrygNQ8lsyHTv4JVTCvnXC7UAWYaqS6mZKMI=",
  "Origin": {
    "VCS": "git",
    "URL": "https://github.com/mbvlabs/andurel",
    "Subdir": "pkg/anduin",
    "Hash": "718076edfafe0735032bf6a2d457b5c09bbd8251",
    "Ref": "refs/tags/pkg/anduin/v0.8.0"
  }
}
```

Local tag and commit checks:

```sh
git rev-parse 'pkg/anduin/v0.8.0^{commit}'
# 718076edfafe0735032bf6a2d457b5c09bbd8251
git log -1 --format='%H %s' 'pkg/anduin/v0.8.0^{commit}'
# 718076edfafe0735032bf6a2d457b5c09bbd8251 feat(anduin): release v0.8.0
```

The downloaded module directory was also compared against the `pkg/anduin`
tree at commit `718076ed`:

```sh
git archive 718076ed pkg/anduin | tar -x -C /tmp/opencode/anduin-release-718076ed
MODDIR="$(cd pkg/kiks && GOWORK=off go env GOMODCACHE)/github.com/mbvlabs/andurel/pkg/anduin@v0.8.0"
diff -r --exclude=LICENSE /tmp/opencode/anduin-release-718076ed/pkg/anduin "$MODDIR"
cmp "$MODDIR/LICENSE" LICENSE
```

Result: the module content is identical to commit `718076ed` except for the
synthetic module-root `LICENSE` added by the Go module zip, which is byte-for-byte
the repository root `LICENSE`.

### Public proxy endpoints

All returned HTTP 200 via `https://proxy.golang.org`:

| Module | Version | `.info` | `.mod` | `.zip` |
| --- | --- | --- | --- | --- |
| `github.com/mbvlabs/andurel/pkg/anduin` | `v0.8.0` | 200 | 200 | 200 |
| `github.com/mbvlabs/andurel/pkg/routing` | `v0.9.0` | 200 | 200 | 200 |

Routing is the transitive minimum required by published anduin `v0.8.0`. Its
public origin is `refs/tags/pkg/routing/v0.9.0` at
`892630e6d62337e0b0c2d0df70c724a76f7412c9`, with
`h1:e7C5aMzgz44JE6/4HS1Uvr3NEhEjAMpYXW8HibzvUZk=`. Tidy moves request routing
from an indirect to a direct requirement because the accepted
`router_error_test.go` imports `github.com/mbvlabs/andurel/pkg/routing`
directly. This is test metadata, not a kiks runtime change.

## Commands and results

`go mod edit` was run first, followed by tidy regenerating the real public
hashes. Both commands ran from `pkg/kiks` with the environment above.

```sh
go mod edit -require=github.com/mbvlabs/andurel/pkg/anduin@v0.8.0
go mod tidy
```

| Command | Result |
| --- | --- |
| `go mod edit -require=github.com/mbvlabs/andurel/pkg/anduin@v0.8.0` | exit 0, no output |
| `go mod tidy` | exit 0, no output |
| `gofmt -l .` | empty output, exit 0 |
| `go fix ./...` | exit 0, no suggested or applied changes |
| `gofmt -l .` (after fix) | empty output, exit 0 |
| `go vet ./...` | exit 0 |
| `go test -count=1 ./...` | `ok github.com/mbvlabs/andurel/pkg/kiks 2.113s` |
| `CGO_ENABLED=1 CC='/home/mbv/.local/share/mise/installs/zig/0.14.1/zig cc' go test -race -count=1 ./...` | `ok github.com/mbvlabs/andurel/pkg/kiks 3.137s` |
| `go test -mod=readonly -count=1 ./...` | `ok github.com/mbvlabs/andurel/pkg/kiks 2.119s` |
| `go mod verify` | `all modules verified` |
| `go mod tidy -diff` | empty output, exit 0 |
| `go mod edit -json` replace scan | `"Replace": null`; no local replacement |
| `git diff --check -- pkg/kiks` | exit 0, no whitespace errors |

Zig: `0.14.1` at
`/home/mbv/.local/share/mise/installs/zig/0.14.1/zig`; `CGO_ENABLED=1` with
that `CC` was used only for the race command.

## Exact diff

`git diff -- pkg/kiks/go.mod pkg/kiks/go.sum`:

```diff
diff --git a/pkg/kiks/go.mod b/pkg/kiks/go.mod
index 52ed8d04..2a9a6d6e 100644
--- a/pkg/kiks/go.mod
+++ b/pkg/kiks/go.mod
@@ -5,7 +5,8 @@ go 1.27.3
 require (
 	github.com/gorilla/securecookie v1.1.2
 	github.com/jackc/pgx/v5 v5.10.0
-	github.com/mbvlabs/andurel/pkg/anduin v0.6.2
+	github.com/mbvlabs/andurel/pkg/anduin v0.8.0
+	github.com/mbvlabs/andurel/pkg/routing v0.9.0
 )
 
 require (
@@ -19,7 +20,6 @@ require (
 	github.com/jackc/pgpassfile v1.0.0 // indirect
 	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
 	github.com/lmittmann/tint v1.2.0 // indirect
-	github.com/mbvlabs/andurel/pkg/routing v0.8.0 // indirect
 	github.com/mbvlabs/andurel/pkg/telemetry v0.2.3 // indirect
 	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
 	go.opentelemetry.io/contrib/bridges/otelslog v0.20.1 // indirect
diff --git a/pkg/kiks/go.sum b/pkg/kiks/go.sum
index fdabf1b1..81b697a3 100644
--- a/pkg/kiks/go.sum
+++ b/pkg/kiks/go.sum
@@ -32,10 +32,10 @@ github.com/jackc/puddle/v2 v2.2.2 h1:PR8nw+E/1w0GLuRFSmiioY6UooMp6KJv0/61nB7icHo
 github.com/jackc/puddle/v2 v2.2.2/go.mod h1:vriiEXHvEE654aYKXXjOvZM39qJ0q+azkZFrfEOc3H4=
 github.com/lmittmann/tint v1.2.0 h1:AogHRHy8HUJUnNJBHJlYa+fR4YY8mko2cnCp67xn9JY=
 github.com/lmittmann/tint v1.2.0/go.mod h1:HIS3gSy7qNwGCj+5oRjAutErFBl4BzdQP6cJZ0NfMwE=
-github.com/mbvlabs/andurel/pkg/anduin v0.6.2 h1:e0g3U2nJ8egERB6dDKAvqTm5RLwmzCmXq83RgB82yj4=
-github.com/mbvlabs/andurel/pkg/anduin v0.6.2/go.mod h1:gpQY/fbQ4fmNjb3D/cHsLzgcBsuOiqLVIaqdOJnZ+7U=
-github.com/mbvlabs/andurel/pkg/routing v0.8.0 h1:nOX0FbZKRkULSZWW39fikGh9P5rpY2yyMjciiPgsEUo=
-github.com/mbvlabs/andurel/pkg/routing v0.8.0/go.mod h1:tDaz5LVwJXJn733LZhYrsbEWZL7bYNrl4YqQzaBAWpI=
+github.com/mbvlabs/andurel/pkg/anduin v0.8.0 h1:jWpOUeyy2a7DblnBebgg3Hs3S8jC9Af3eaJztzwL8SU=
+github.com/mbvlabs/andurel/pkg/anduin v0.8.0/go.mod h1:2lRaNyJmrygNQ8lsyHTv4JVTCvnXC7UAWYaqS6mZKMI=
+github.com/mbvlabs/andurel/pkg/routing v0.9.0 h1:e7C5aMzgz44JE6/4HS1Uvr3NEhEjAMpYXW8HibzvUZk=
+github.com/mbvlabs/andurel/pkg/routing v0.9.0/go.mod h1:tDaz5LVwJXJn733LZhYrsbEWZL7bYNrl4YqQzaBAWpI=
 github.com/mbvlabs/andurel/pkg/telemetry v0.2.3 h1:T70mFoL8oKDNByxQt+qbew3GiNVMlqNYBYxknrXWcE0=
 github.com/mbvlabs/andurel/pkg/telemetry v0.2.3/go.mod h1:gcZRxndv7wvMptVsdHCWiRs9jbVWButoNPJsH2axx10=
 github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
```

`git diff --stat -- pkg/kiks/go.mod pkg/kiks/go.sum` is:

```text
 pkg/kiks/go.mod | 4 ++--
 pkg/kiks/go.sum | 8 ++++----
 2 files changed, 6 insertions(+), 6 deletions(-)
```

No `replace` directive is present before or after the update.

## Preserved accepted work

`git status --short -- pkg/kiks` after the metadata update:

```text
 M pkg/kiks/CHANGELOG.md
 M pkg/kiks/go.mod
 M pkg/kiks/go.sum
?? pkg/kiks/router_error_test.go
```

`pkg/kiks/CHANGELOG.md` and `pkg/kiks/router_error_test.go` are the accepted
test/documentation edits from `plans/rc1-final-readiness/t02-kiks.md`; this
task neither rewrote nor staged them. `git diff --name-only -- 'pkg/kiks/*.go'`
is empty, confirming no kiks runtime source changed.

## Reviewer checklist

1. Confirm the anduin `v0.8.0` public origin hash is
   `718076edfafe0735032bf6a2d457b5c09bbd8251` and the tag commit matches.
2. Confirm the diff is only `pkg/kiks/go.mod` and `pkg/kiks/go.sum` with no
   `replace` directive or workspace dependency.
3. Confirm routing moved to direct `v0.9.0` because the accepted test imports
   it, not because kiks runtime code changed.
4. Re-run the standalone `GOWORK=off` tests, race/Zig, vet, fix, format,
   `go mod verify` and `go mod tidy -diff`.
5. Confirm `layout/versions/versions.go` still has `Kiks = "v0.6.1"` and that
   no kiks release/tag/version cut, commit, push or publication was performed.
