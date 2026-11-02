# rc.1 package-release pin integration (T08 framework pin/golden slice)

Status: **PIN AND GOLDEN INTEGRATION COMPLETE IN THE SHARED WORKTREE;
RELEASE-SHAPED FRESH-SCAFFOLD GOLDENS PASS. FULL RELEASE GATES REMAIN FOR THE
ORCHESTRATOR.** No commit, tag, push, publication or deployment was performed.

Prepared: 2026-11-02. Worker scope: framework `layout/versions` pins and
expected scaffold goldens only. This record does not claim final rc.1 readiness
and does not repeat the package release review.

## Scope and ownership boundaries

Changed in this slice:

| Path | Change |
| --- | --- |
| `layout/versions/versions.go` | `Anduin` `v0.7.1` -> `v0.8.0`; `Inertia` `v0.14.0` -> `v0.14.1` |
| `testdata/golden/new/postgresql-inertia-react/go.mod.golden` | Regenerated anduin/inertia pins |
| `testdata/golden/new/postgresql-inertia-svelte/go.mod.golden` | Regenerated anduin/inertia pins |
| `testdata/golden/new/postgresql-inertia-vue/go.mod.golden` | Regenerated anduin/inertia pins |
| `plans/rc1-final-readiness/pin-integration.md` | This evidence record |
| `plans/rc1-final-readiness/evidence/pin-integration-*` | Command logs and exact outputs below |

Deliberately not changed:

- `Kiks` remains `v0.6.1`; no kiks package was published and the kiks scaffold
  pin is unchanged.
- `pkg/kiks/go.mod`, `pkg/kiks/go.sum` and `pkg/kiks/CHANGELOG.md` were already
  modified by the accepted kiks metadata owner before this slice. This worker
  did not edit them.
- `pkg/inertia/go.mod`/`go.sum` already require published anduin `v0.8.0` from
  the inertia release commit; this worker did not edit them.
- `pkg/anduin` module metadata and changelog are part of published commit
  `718076edfafe0735032bf6a2d457b5c09bbd8251`; not reopened here.
- Root `go.mod`, `go.work`, `go.sum`, `go.work.sum` have no `pkg/anduin`,
  `pkg/inertia` or `pkg/kiks` version pins to update. `go.work` contains only
  local `use` directives. No root module edit was made.
- `testdata/golden/new/*/andurel.lock.golden` did not change; tool pins
  (including narsilc `v0.6.3`) are unchanged.
- `layout/template_data.go`, `layout/upgrade/plan.go` and
  `internal/goldentest/candidate_proxy_test.go` read `layout/versions`; no
  literal version edit was needed.
- `docs/release-notes/v2.0.0-rc.1.md` and `plans/rc1-final-readiness/overview.md`
  remain owned by the orchestrator and were not touched.

## Published artifact evidence

Confirmed through the public proxy and checksum database with no workspace and
no local replacement from `/tmp/opencode`:

```sh
GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  go mod download -json \
  github.com/mbvlabs/andurel/pkg/anduin@v0.8.0 \
  github.com/mbvlabs/andurel/pkg/inertia@v0.14.1
```

Exit 0, empty stderr. Full JSON: `evidence/pin-integration-module-download.json`.

| Module | Version | Module sum | `go.mod` sum | Origin commit | Origin ref |
| --- | --- | --- | --- | --- | --- |
| `github.com/mbvlabs/andurel/pkg/anduin` | `v0.8.0` | `h1:jWpOUeyy2a7DblnBebgg3Hs3S8jC9Af3eaJztzwL8SU=` | `h1:2lRaNyJmrygNQ8lsyHTv4JVTCvnXC7UAWYaqS6mZKMI=` | `718076edfafe0735032bf6a2d457b5c09bbd8251` | `refs/tags/pkg/anduin/v0.8.0` |
| `github.com/mbvlabs/andurel/pkg/inertia` | `v0.14.1` | `h1:TT54ERsQ+IVGEz4/7ZmXVmWcYIM61+YZPjQRdDTRrN8=` | `h1:gWtvWsMb5fM/D0xNZEZ59oLc/o1eViPZw9ZRhUN4RZA=` | `c6c724fa8beb0b563fd4966d3d7748af70ad6f46` | `refs/tags/pkg/inertia/v0.14.1` |

These are the same publication identities recorded in
`publication-handoff.md`; the values above were re-read from the public module
graph for this integration and were not hand-written.

All twelve verified framework pins pass the public gate:

```text
$ just check-published-pins
ok   github.com/mbvlabs/andurel/pkg/anduin@v0.8.0
ok   github.com/mbvlabs/andurel/pkg/auth@v0.3.0
ok   github.com/mbvlabs/andurel/pkg/cache@v0.2.1
ok   github.com/mbvlabs/andurel/pkg/email@v0.6.1
ok   github.com/mbvlabs/andurel/pkg/env@v0.1.1
ok   github.com/mbvlabs/andurel/pkg/inertia@v0.14.1
ok   github.com/mbvlabs/andurel/pkg/kiks@v0.6.1
ok   github.com/mbvlabs/andurel/pkg/lifecycle@v0.1.1
ok   github.com/mbvlabs/andurel/pkg/routing@v0.9.1
ok   github.com/mbvlabs/andurel/pkg/storage@v0.15.1
ok   github.com/mbvlabs/andurel/pkg/telemetry@v0.2.3
ok   github.com/mbvlabs/andurel/pkg/validation@v0.4.0
12 published pkg/* pins verified
```

Exit 0. Output: `evidence/pin-integration-check-published-pins.txt`.

## Exact pin diff

`git diff --numstat` for the four changed files: 8 insertions, 8 deletions.
No other tracked file in this slice changed.

```diff
diff --git a/layout/versions/versions.go b/layout/versions/versions.go
index 8e9e232f..d0029d61 100644
--- a/layout/versions/versions.go
+++ b/layout/versions/versions.go
@@ -25,7 +25,7 @@ const (
 	// NarsilcModulePath is the module path of the narsilc query code generator.
 	NarsilcModulePath = "github.com/mbvlabs/narsilc"
 	// Anduin is the standalone Andurel HTTP module version verified with this framework.
-	Anduin = "v0.7.1"
+	Anduin = "v0.8.0"
 	// Auth is the standalone Andurel auth module version verified with this framework.
 	Auth = "v0.3.0"
 	// Cache is the standalone Andurel cache module version verified with this framework.
@@ -35,7 +35,7 @@ const (
 	// Lifecycle is the standalone Andurel process lifecycle module version verified with this framework.
 	Lifecycle = "v0.1.1"
 	// Inertia is the standalone Andurel Inertia module version verified with this framework.
-	Inertia = "v0.14.0"
+	Inertia = "v0.14.1"
 	// Email is the standalone Andurel email module version verified with this framework.
 	Email = "v0.6.1"
 	// Env is the standalone Andurel environment module version verified with this framework.
diff --git a/testdata/golden/new/postgresql-inertia-react/go.mod.golden b/testdata/golden/new/postgresql-inertia-react/go.mod.golden
index c477e43c..39d4a26d 100644
--- a/testdata/golden/new/postgresql-inertia-react/go.mod.golden
+++ b/testdata/golden/new/postgresql-inertia-react/go.mod.golden
@@ -6,12 +6,12 @@ require (
 	github.com/a-h/templ v0.3.1020
 	github.com/gosimple/slug v1.15.0
 	github.com/jackc/pgx/v5 v5.11.0
-	github.com/mbvlabs/andurel/pkg/anduin v0.7.1
+	github.com/mbvlabs/andurel/pkg/anduin v0.8.0
 	github.com/mbvlabs/andurel/pkg/auth v0.3.0
 	github.com/mbvlabs/andurel/pkg/cache v0.2.1
 	github.com/mbvlabs/andurel/pkg/email v0.6.1
 	github.com/mbvlabs/andurel/pkg/env v0.1.1
-	github.com/mbvlabs/andurel/pkg/inertia v0.14.0
+	github.com/mbvlabs/andurel/pkg/inertia v0.14.1
 	github.com/mbvlabs/andurel/pkg/kiks v0.6.1
 	github.com/mbvlabs/andurel/pkg/lifecycle v0.1.1
 	github.com/mbvlabs/andurel/pkg/routing v0.9.1
```

`postgresql-inertia-svelte` and `postgresql-inertia-vue` have the identical
diff (same content hash), and all three golden files are identical after the
update. The complete diff is also saved as
`evidence/pin-integration.diff`.

Exact file identity after this slice:

| File | SHA-256 | Git blob |
| --- | --- | --- |
| `layout/versions/versions.go` | `09b9db42bb5a89b7306c7594fea7fd222578a451b913b15ac071aead75d68de4` | `d0029d614bf21f1dbdf8c4e7813205efdbdd2456` |
| `testdata/golden/new/postgresql-inertia-react/go.mod.golden` | `5bb9daa16d7e370287db172dc1b1c98867406c80466340e960e525056c891ecb` | `39d4a26daba57afe8a803cfbc93a9fb03235d427` |
| `testdata/golden/new/postgresql-inertia-svelte/go.mod.golden` | `5bb9daa16d7e370287db172dc1b1c98867406c80466340e960e525056c891ecb` | `39d4a26daba57afe8a803cfbc93a9fb03235d427` |
| `testdata/golden/new/postgresql-inertia-vue/go.mod.golden` | `5bb9daa16d7e370287db172dc1b1c98867406c80466340e960e525056c891ecb` | `39d4a26daba57afe8a803cfbc93a9fb03235d427` |

Before the update all three golden files had SHA-256
`e6ed7accb99f89bc913846c33d641848c5fe9858393666725f80cb75da4a8588`
(439 golden files hashed before and after). The 436 unchanged golden hashes
were compared byte-for-byte against the pre-update manifest; only the three
`go.mod.golden` files differ. No manual golden edit was made.

## Golden regeneration

Recipe inspected before running: `just update-golden-full` runs
`go clean -testcache` then
`ANDUREL_GOLDEN_FULL=1 go test ./golden/... -v -timeout 45m -update`.
The golden harness itself forces `GOWORK=off` for every generated scaffold and,
without `ANDUREL_GOLDEN_LOCAL_PKG`, selects published mode with
`GOPROXY=https://proxy.golang.org,direct` and `GOSUMDB=sum.golang.org`. No
local package proxy or replacement was used.

Command and exit:

```text
$ just update-golden-full
PASS
ok  github.com/mbvlabs/andurel/v2/golden  76.443s
EXIT: 0
```

Full log: `evidence/pin-integration-update-golden-full.txt`.

Post-update golden status is exactly:

```text
 M testdata/golden/new/postgresql-inertia-react/go.mod.golden
 M testdata/golden/new/postgresql-inertia-svelte/go.mod.golden
 M testdata/golden/new/postgresql-inertia-vue/go.mod.golden
```

No `andurel.lock.golden`, template, handler, generated-frontend or other golden
churn.

Release-shaped confirmation (fresh isolated caches, public proxy/checksums,
`GOWORK=off`, `-update` absent):

```text
$ just test-golden-full-release
PASS
ok  github.com/mbvlabs/andurel/v2/golden  125.520s
EXIT: 0
```

Full log: `evidence/pin-integration-test-golden-full-release.txt`.

## Focused verification run

| Command | Result | Evidence |
| --- | --- | --- |
| `go test ./layout/versions/... ./layout/upgrade -count=1` | PASS (`layout/upgrade` 1.604s; versions has no test files) | `evidence/pin-integration-test-layout-upgrade.txt` |
| `go test ./layout/... -count=1` | PASS (layout 16.082s, blueprint, cmds, upgrade 1.608s) | `evidence/pin-integration-test-layout-all.txt` |
| `go test ./internal/goldentest ./internal/publishedpins ./layout -count=1` | PASS (layout 11.591s) | `evidence/pin-integration-test-internal-layout.txt` |
| `go test ./cli -run 'Test(ListAndurelPackages\|FilterAndurelPackages\|Packages)' -count=1` | PASS | `evidence/pin-integration-test-cli-packages.txt` |
| `just check-published-pins` | PASS, 12/12 public pins | `evidence/pin-integration-check-published-pins.txt` |
| `just check-mod` | PASS, `all modules verified`; tidy diff empty | `evidence/pin-integration-check-mod.txt` |
| `just check-fmt` | PASS, empty output | `evidence/pin-integration-check-fmt.txt` |
| `go fix -diff ./...` | PASS, empty diff, empty stderr | `evidence/pin-integration-go-fix.diff` |
| `just vet` | PASS, root plus all `pkg/*` modules | `evidence/pin-integration-vet.txt` |
| `git diff --check -- layout/versions/versions.go testdata/golden/new` | PASS, exit 0, no whitespace errors | command output |
| `just update-golden-full` | PASS, only expected three golden files changed | `evidence/pin-integration-update-golden-full.txt` |
| `just test-golden-full-release` | PASS, release-shaped fresh scaffold | `evidence/pin-integration-test-golden-full-release.txt` |

## Root module and lock review

- `go.mod` has no `github.com/mbvlabs/andurel/pkg/*` requirement; it is the
  framework CLI module.
- `go.work` lists local `use` directories only. Workspace mode is not release
  proof; golden release runs force `GOWORK=off`.
- `go.work.sum` contains `github.com/envoyproxy/go-control-plane v0.14.0` and
  `github.com/go-faster/errors v0.7.1`; these are unrelated transitive entries,
  not `pkg/inertia`/`pkg/anduin` pins. No root sum was edited.
- `andurel.lock.golden` files are unchanged, so the narsilc tool/runtime pin
  remains the framework default `v0.6.3`, not the site's manual `v0.6.4`.
- No local `replace` directive was added, and no local-package golden mode was
  used as release proof.

## Required full release gates not run by this worker

Not run here; they remain required on the exact candidate before the owner
go/no-go:

- `just check-fix` as a whole-tree gate. `go fix -diff ./...` was empty and the
  changed files pass `check-fmt`; the recipe's final `git diff --exit-code`
  cannot pass on this shared worktree because accepted unrelated rc.1 changes
  are still uncommitted.
- `just lint`
- `just vulncheck`
- `just test-race` (root and affected modules)
- `just test` (full root suite)
- `just test-upgrade-e2e`
- `just check-release-notes v2.0.0-rc.1` (release notes remain an orchestrator
  draft)
- release-readiness workflow / CI artifact preflight for the candidate SHA
- post-publish site/runtime T09, B7-005 published-dependency positive repro
- T07 independent live/image acceptance

No framework version bump (`v2.0.0-rc.1`) and no final rc.1 tag was prepared or
created. No commit, tag, push, publish or deployment was performed.
