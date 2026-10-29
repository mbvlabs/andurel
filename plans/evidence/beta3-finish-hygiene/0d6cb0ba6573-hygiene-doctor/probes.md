# F40/F06 hygiene and doctor probes

acceptance: full

| Probe | Status | Exit |
| --- | --- | --- |
| 01-new-react-pnpm-harness | pass | 0 |
| 02-local-replacements | pass | 0 |
| 03-lockfile-frozen-install | pass | 0 |
| 04-setup-positive | pass | 0 |
| 05-doctor-missing-env | pass | 0 |
| 06-doctor-malformed-env | pass | 0 |
| 07-doctor-export-only-unreachable-db | pass | 0 |
| 08-hygiene-ignored-outputs-secrets | pass | 0 |
| 09-verify-scoped-changed-content | pass | 0 |

passed=9 blocked=0 failed=0

## Coverage

- F40: selected harness skill install and git tracking, no lockfile shipped by `andurel new`, generated pnpm lockfile, `pnpm install --frozen-lockfile` success without rewrite, frozen drift rejection, ignored `.env`/`node_modules`/`bin`/`dist`/`assets/dist`/`tmp`/database, and setup negative -> positive after `andurel tool sync` + `pnpm install`.
- F06: doctor missing configuration, malformed configuration without echoing the rejected secret, exported-only configuration, and an unreachable database with a secret-safe diagnostic; each negative exits nonzero and names the blocking check.
- Scoped/changed/content: `verify --only content` not_applicable/ok/fail, and `verify --only gofmt --changed` records the changed file and package scope.
