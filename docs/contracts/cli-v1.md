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

Removing or renaming a frozen command, alias, flag, response field, error code, or exit code, changing a field's JSON type or meaning, or changing a projection wire shape requires Andurel v2. Compatible v1 additions may introduce a new command, optional flag, optional response field, or new typed error for a previously unspecified failure. Scripts must ignore unknown optional object fields.

Human output, progress text, help prose, and diagnostic wording may improve within v1 unless a documented example explicitly identifies the text as a machine contract. Automation should use `--json`, `--agent`, or a projection flag instead of parsing human output.
