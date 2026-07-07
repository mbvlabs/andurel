---
name: andurel
description: Use this skill when working in an Andurel project or generating Andurel code.
---

# Andurel

Use this skill when working in an Andurel project or generating Andurel code.

## Agent Invariants

- Prefer `andurel --agent --help` and `andurel commands --json` for discovery.
- Run `andurel project info --json` before generation.
- Use `--json` or `--jq` when extracting data.
- Use `--dry-run --json` before mutating commands when intent is uncertain.
- Inspect returned artifact arrays before assuming which files changed.
- Follow the repository rules for verification.

## Common Workflows

Inspect a project:

```bash
andurel project info --json
andurel routes --json
andurel models --json
andurel migrations --json
```

Preview scaffold generation:

```bash
andurel generate scaffold Product --dry-run --json
```

Generate and review artifacts:

```bash
andurel generate scaffold Product --json
```

Generate a named database seed:

1. Inspect the relevant models and existing factories in `models/factories`.
2. Add a seed function to `database/seeds`, using only exported model/factory/storage primitives.
3. Register it in `seeds.Registry` with a stable lowercase name.
4. Keep the seed idempotence expectations explicit in code comments when it may be re-run.
5. Verify the seed is discoverable:

```bash
andurel database seed --list
andurel database seed development
andurel database seed test
```

Check project health:

```bash
andurel doctor --json
```
