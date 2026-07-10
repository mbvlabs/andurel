# Public Go API contract

Andurel v1 supports every exported identifier in every importable package listed in [`contracts/public-packages.txt`](../../contracts/public-packages.txt). The exact baseline, including exported types, functions, methods, fields, variables, constants, and Go documentation, is recorded in [`contracts/public-api.txt`](../../contracts/public-api.txt).

Removing or renaming a listed identifier, narrowing a signature, changing an exported field incompatibly, or otherwise breaking source compatibility requires Andurel v2. Compatible additions remain possible in a v1 minor release. Security and correctness fixes may change behavior when retaining the old behavior would violate a documented guarantee.

The compatibility promise applies to source code compiled against supported v1 packages. It does not freeze undocumented internal packages, generated text formatting, command-line prose, or implementation details. Deprecation may precede a v2 removal, but a deprecated v1 identifier remains supported throughout v1.

## Common ownership rules

- Callers retain ownership of arguments unless an identifier explicitly documents transfer or retention.
- Returned slices, maps, pointers, and structs are caller-owned unless they refer to an explicitly shared registry or cache.
- A function or method that accepts a destination path, project root, writer, registry, builder, manager, or mutable receiver may update that target as part of its named operation.
- Query, naming, validation, conversion, and detection helpers do not mutate caller-owned values unless their signature requires a mutable output.
- Errors may wrap an underlying cause. Callers may use `errors.Is` and `errors.As` where the returned error type supports them.

## Concurrency rules

No exported value is safe for concurrent mutation unless this document or its Go documentation says it is. Read-only use is safe only when no goroutine can mutate the same value or its referenced data.

The `pkg/cache.FileSystemCache` methods and package-level filesystem cache helpers are safe for concurrent calls. Embedded `embed.FS` values are read-only after initialization. Other managers, builders, generators, registries, test suites, lock values, and upgrade values require caller synchronization.

## Package mutation summary

| Packages | Mutation and side effects |
| --- | --- |
| `cli`, `cli/output` | `NewRootCommand` returns a caller-owned Cobra tree. Output functions write only to the command's configured streams and may read flag state. |
| `generator` and its public subpackages | Generators and managers may create, replace, format, or inspect project files. Builder-style and setter methods mutate their receivers. Metadata-only `Build`, parse, detect, validate, and naming operations do not write unless their Go documentation says otherwise. |
| `layout` | Scaffold, extension, sync, and lock write operations mutate the target project or lock receiver. Predicate and catalog lookup helpers are read-only. Returned tool specifications are copies and may be changed by the caller. |
| `layout/blueprint` | Builder methods mutate the supplied blueprint. A blueprint and its nested collections must not be mutated concurrently. |
| `layout/cmds` | Run and download functions execute processes or write the named destination. Callers must serialize operations that share a destination. |
| `layout/extensions` | `Register` mutates the process-wide registry. Registration must finish before concurrent calls to `Get` or `Names`. Extension application mutates the target project. |
| `layout/templates`, `generator/templates`, `skills` | Embedded files are immutable. Rendering writes only to returned values or explicitly supplied destinations. Skill walking invokes the callback synchronously and does not retain callback data. |
| `layout/upgrade` | A non-dry-run upgrade mutates the project. Dry-run behavior is read-only. An upgrader is single-use and requires caller synchronization. |
| `layout/versions`, `pkg/constants`, `pkg/naming` | Constants and conversion helpers have no shared mutation. Returned strings are caller-owned values. |
| `pkg/cache` | Cache values and package helpers use internal locking. Stored pointer, slice, or map values are not deep-copied, so callers remain responsible for the concurrency of the stored value itself. |
| `pkg/errors` | Constructors return caller-owned errors and contexts. Fluent context and builder methods mutate their receivers and are not safe for concurrent calls. |
| `pkg/testing` | Suite registration mutates suite maps, and run methods execute registered callbacks. A suite requires caller synchronization. |

## Updating the baseline

Run `scripts/update-contracts.sh`, review every change, and commit the regenerated files only for an intentional compatible addition or an approved pre-v1 contract correction. `scripts/check-contracts.sh` fails when source and fixtures drift.

Pull requests and releases run pinned `apidiff` checks against the applicable stable baseline. RC.3 is the pre-v1 audit baseline. After `v1.0.0` is published, the stable v1 tag becomes the release baseline for later v1 changes.
