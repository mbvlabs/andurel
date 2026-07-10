# DDL supported by model generation

Andurel builds generated model metadata from the up statements in SQL migrations. The parser intentionally supports a conservative schema-changing subset. A migration that can change generated model fields but falls outside this subset fails with the migration filename, a statement preview, the unsupported reason, and a remediation to split the statement.

## Supported schema-changing statements

- `CREATE TABLE [IF NOT EXISTS] [schema.]table (...)` with unquoted identifiers and an explicit column list.
- Column types, including parameterized `VARCHAR`, `CHAR`, `NUMERIC`, and `DECIMAL`, plus PostgreSQL timestamp variants and existing custom type names.
- Column `NOT NULL`, `PRIMARY KEY`, `UNIQUE`, `DEFAULT`, and `REFERENCES` clauses.
- Table-level `PRIMARY KEY`, `FOREIGN KEY`, `UNIQUE`, and `CHECK` constraints. Named `FOREIGN KEY`, `UNIQUE`, and `CHECK` constraints are accepted.
- `ALTER TABLE [IF EXISTS] [schema.]table` with comma-separated `ADD COLUMN`, `DROP COLUMN`, `ALTER COLUMN TYPE`, `SET NOT NULL`, `DROP NOT NULL`, `SET DEFAULT`, `DROP DEFAULT`, `RENAME COLUMN`, and `RENAME TO` operations.
- `DROP TABLE [IF EXISTS] [schema.]table` for one table without `CASCADE`.

The parser preserves migration order and applies supported statements to the same catalog used by model generation. Duplicate columns, duplicate primary-key structures, empty definitions, malformed foreign keys, unbalanced delimiters, multiple top-level statements, and unterminated strings, identifiers, or comments fail deterministically.

## Unsupported schema-changing statements

The parser rejects syntax that may alter model fields but cannot be represented safely. Examples include `CREATE TABLE AS`, quoted table or column identifiers, unknown `ALTER TABLE` or `ALTER COLUMN` operations, primary-key constraint changes that cannot be mapped safely, multi-table drops, `CASCADE` drops, views or materialized views, procedural blocks, and other unknown `CREATE`, `ALTER`, `DROP`, or dynamic SQL statements.

Split these migrations into supported statements or update the generated model explicitly before retrying generation. Andurel does not guess at a partial catalog because doing so could silently remove, rename, or mistype generated fields.

## Model-neutral statements

Statements that can be proven not to change generated table structure may produce a warning and are otherwise ignored. This includes data-only `INSERT`, `UPDATE`, `DELETE`, and `TRUNCATE` statements, transaction control, comments, grants, revocations, session settings, `VACUUM`, `ANALYZE`, and index creation or removal. `SELECT INTO`, procedural calls, dynamic SQL, and cascading type or schema drops are not considered model-neutral.
