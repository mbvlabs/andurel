# CLI v1 contract

[`contracts/cli-v1.json`](../../contracts/cli-v1.json) is the deterministic baseline for public command paths, aliases, locally declared and persistent flags, JSON-tagged response fields, structured error codes, and exit codes.

The success envelope has `ok`, optional `data`, optional `summary`, and optional `breadcrumbs`. The error envelope has `ok`, `code`, `error`, optional `hint`, and optional `exit_code`. Normal `--json` and `--agent` output retains these envelopes.

Projection output is intentionally outside the envelope:

- `--jq` selects from the command data payload and writes the selected value directly as JSON.
- `--ids-only` writes one identifier per line.
- `--count` writes one base-10 integer followed by a newline.
- Projection flags are mutually exclusive.
- A command that does not support a requested projection rejects it as a usage error.

Command descriptions and human-oriented prose are not frozen by the fixture. Command paths, aliases, flags, wire fields, typed codes, exit codes, and the projection shapes are frozen scripting contracts for v1.
