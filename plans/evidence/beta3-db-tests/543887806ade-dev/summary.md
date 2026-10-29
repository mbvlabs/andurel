# Beta.3 F20 database-test gate

- result: **pass**
- mode: dev
- proof shape: development only; temporary `pkg/*` replaces are not release-shaped proof
- candidate: 543887806adece0389c0ca9d1a028eff8dd8ea24
- evidence: /home/mbv/work/worktrees/andurel-beta3-fixes/finish-db-tests/plans/evidence/beta3-db-tests/543887806ade-dev
- reason: generated named database tests ran uncached and verify reported per-package totals with no hidden DB skips

## Steps

| Step | Exit |
| --- | --- |
| 01-build-cli | 0 |
| 01b-cli-version | 0 |
| 02-docker-server | 0 |
| 03-new | 0 |
| 04-local-replace | 0 |
| 04b-go-mod-tidy | 0 |
| 05-pnpm-install | 0 |
| 06-pnpm-build | 0 |
| 07-named-db-tests | 0 |
| 08-verify-with-tests | 0 |

Evidence in this directory:

- `environment.txt`
- `candidate-commit.txt`
- `candidate-status.txt`
- `app-name.txt`
- `steps.tsv`
- `named-db-passes.tsv`
- `named-db-pass-events.txt`
- `verify-go-test.json`
- `verify-package-totals.txt`
- `verify-totals.txt`
- `*.command`, `*.stdout`, `*.stderr`, `*.status` for every recorded step
