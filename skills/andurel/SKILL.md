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

Check project health:

```bash
andurel doctor --json
```
