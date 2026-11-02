# beta.7 WP6 evidence provenance

On 2026-11-02, Terra medium session `ses_e67fd45b6ffe7udlDDZH2i0u35`
reported these completed commands from `/home/mbv/work/narsilc-wp6`:

```sh
export TMPDIR=/home/mbv/work/wp6-validation-scratch/tmp
export GOTMPDIR=/home/mbv/work/wp6-validation-scratch/gotmp
export GOFLAGS=-p=2
# POSTGRESQL_SERVER_URI and MYSQL_SERVER_URI selected disposable audit-owned
# servers on 127.0.0.1:34454 and 127.0.0.1:34455 respectively.
go test -count=1 --tags=examples -timeout 20m ./...
go test -count=1 -v -timeout 20m ./internal/endtoend -run 'TestReplay/managed-db'
go fix -diff ./...
go vet ./...
gofmt -l internal/compiler/analyze.go internal/compiler/column_refs.go internal/endtoend/testdata/join_left_same_table/mysql/go/query.sql.go
```

All commands passed; formatting and fix checks produced no changes. The explicit
managed replay passed 911 cases and skipped 111. The new WP6 fixtures opt out of
that context and were exercised by the full suite's applicable replay contexts,
not counted as passing managed cases. Both audit-owned database containers were
removed. This preserves the agent's exact command/result report, not an
independently retained raw test log or a new run by the coordinating session.

Independently retained [CLI/storage evidence](beta7-final-cli-storage-verification.md)
records building that checkout with `go build -o /tmp/opencode/beta7-final-narsilc ./cmd/narsilc`
from `/home/mbv/work/narsilc-wp6`, and using the local executable and a scratch
tool pin to verify rejection of invalid function arguments in a generated app.
The [settled scaffold report](beta7-final-settled-scaffold-verification.md)
records its separate 49-test generated-app verify using local WP6. Neither is
proof that a compatible WP6 tool is published or pinned for release.
