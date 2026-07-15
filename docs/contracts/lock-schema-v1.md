# Andurel lock schema 1

The normative structural fixture is [`contracts/lock-schema-v1.schema.json`](../../contracts/lock-schema-v1.schema.json). [`contracts/lock-schema-v1.example.json`](../../contracts/lock-schema-v1.example.json) shows the complete download and version-check shape.

`schemaVersion` controls the structure and interpretation of `andurel.lock`. `version` records the framework version that owns the generated framework files.

## Decoding order

Readers must follow this order:

1. Decode only `schemaVersion` without decoding the complete lock.
2. Reject a missing value.
3. Accept schema 1.
4. Reject a newer schema with an error that tells the user to upgrade the Andurel CLI.
5. Decode the complete schema 1 value.
6. Validate all required fields before using or writing any lock data.

A reader must not partially decode a future schema as schema 1. Reading and validation do not mutate the lock file.

## Schema 1 validation

- `schemaVersion` is exactly `1`.
- `version` is non-empty and `tools` is present.
- Every tool has a non-empty version, a version check, and either a project-built path or verified download metadata.
- Download URLs use HTTPS.
- Download metadata has a supported archive kind, an exact binary name, and a syntactically valid SHA-256 digest for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.
- Version-check arguments are non-empty.
- A configured version-check regular expression must compile and is authoritative for extraction.
- When `versionCheck.regexp` is omitted, readers use the documented generic expression `v?([0-9]+\.[0-9]+\.[0-9]+)`.
- Unknown optional fields may be ignored. A schema 1 writer must not require an older schema 1 reader to understand an unknown field.
- Scaffolding and version upgrades set `scaffoldConfig.inertiaRoot` to `views/root.go.html` for Inertia projects unless a custom value already exists. Generated Inertia initialization reads this field at runtime.

## Compatible schema 1 changes

The following may remain schema 1:

- adding an optional field whose absence preserves current behavior;
- adding metadata that old readers may safely ignore;
- adding a new optional tool entry that uses an already-supported download and version-check interpretation;
- tightening validation only to reject values that were already invalid under this contract;
- clarifying documentation without changing structure or meaning.

## Changes requiring schema 2

The following require a schema increment and a separately designed upgrade path:

- removing or renaming a field;
- changing a field's JSON type, required status, default, or meaning;
- changing how `version`, download metadata, SHA-256 values, platform keys, or version checks are interpreted;
- permitting an executable installation without a digest for the current platform;
- changing an existing archive value or platform key incompatibly;
- making previously optional metadata necessary for correct operation;
- any change that causes a valid schema 1 lock to be interpreted differently.

Writers always emit the current schema explicitly. A missing `schemaVersion` is invalid and cannot be passed to the automated upgrader. Projects created with v1.0.0-rc.2 or v1.0.0-rc.3 require the [manual RC-to-v1 procedure](../upgrade-rc-base-scaffold-prompt.md). There is no downgrade path for a future schema. Preserve the original lock and use a newer Andurel CLI when a future schema is encountered.
