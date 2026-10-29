# Content and route-usage verification

`andurel verify` has two project-declared checks that are not Go analyzers:

- the **content** stage checks a declared Markdown tree and catalog;
- the **frontend route-usage** rule checks that full-URL route helpers are not
  evaluated during module initialization.

Both are deterministic, read-only, and offline by default. This document is the
schema and behavior contract behind `andurel verify --help`.

## Content manifest: `andurel.content.toml`

The declaration is a standalone file at the project root, not a table in
`andurel.toml`. `andurel.toml` is rewritten by `andurel tool sync`, upgrade, and
scaffold regeneration; a hand-maintained content table there would be dropped.

```toml
schemaVersion = 1

# Root-relative Markdown documents governed by the built-in invariants.
# '*' matches inside one path segment; '**' matches any number of segments.
include = ["content/**/*.md"]
exclude = ["content/drafts/**"]

# Paths that must exist (file or directory). Checked independently of exclude.
required = ["content/index.md"]

# Optional catalog file. Its schema is the same schemaVersion plus [[entry]]
# tables:
#   [[entry]]
#   slug = "getting-started"
#   file = "content/docs/getting-started.md"
catalogFile = "content/catalog.toml"

# Inline catalog entries are combined with catalogFile entries.
[[catalog]]
slug = "install"
file = "content/docs/install.md"

[slugs]
# Full-match regular expression for every catalog slug. The default is
# '^[a-z0-9]+(?:-[a-z0-9]+)*$' (one kebab-case segment). Set the pattern to
# allow nested slugs such as 'docs/getting-started'. An empty value disables
# the shape check; duplicate slugs still fail.
pattern = "^[a-z0-9]+(?:-[a-z0-9]+)*$"

[links]
# "ignore" (default) checks only that the target file/directory exists.
# "check" also verifies GitHub-style heading anchors.
anchors = "ignore"
# The only supported value. Network link checking is not part of the
# deterministic default and is never performed.
external = "ignore"

# Optional project-declared deterministic checker(s). Run in addition to the
# built-in invariants, never instead of them.
[[checkers]]
name = "site-docs"
run = ["go", "run", "./internal/docscheck"]
dir = "."
timeoutSeconds = 120
```

Unknown keys and wrong value types are configuration failures: a typo in the
manifest must not silently disable a check.

### Rule IDs

| Rule | Checks |
| --- | --- |
| `content-config` | The manifest parses, `schemaVersion = 1`, globs/paths are root-relative and stay inside the project, the catalog file exists and parses, `[links]`/`[[checkers]]` values are supported, and the manifest, `catalogFile`, and checker `dir` resolve through symlinks inside the resolved project root. |
| `content-catalog` | Every catalog entry has a non-empty `file`, targets a `.md` file, the file exists, is not excluded, and is inside `include`. |
| `content-slug` | Slugs are non-empty, match `[slugs].pattern`, and are unique across inline and catalog-file entries. |
| `content-required` | Every `required` path exists as a file or directory. |
| `content-link` | Local links and images resolve; optional heading anchors exist; a `file:line` and fix are reported. |
| `content-checker` | A declared checker exits 0 within its timeout from its declared directory; nonzero output becomes diagnostics. |

A project without `andurel.content.toml` reports the content stage as
`skipped` with `not_applicable: true`. It never reports content as checked.

### Rule selection and execution

`--rules` controls what the content stage **executes**, not only which findings
are printed. With `--rules content-link`, only the local-link built-in runs; a
declared `[[checkers]]` subprocess does not start, so its side effects and exit
status cannot leak into a differently scoped run. `--rules content-checker`
runs only the declared checkers, and `--rules content-config` runs only the
manifest/catalog validation that `Load` always performs. An empty selection
runs every content rule. Configuration failures are always reported: a broken
manifest is a blocking failure even when the remaining selected rules would
otherwise pass, because no content invariant is trustworthy without a valid
declaration.

The reported `rules` object records `selected`, `executed`, and `skipped`
for the stage. Configuration validation always runs even when its rule is not
selected; if it fails, the selected rules that did not run are reported as
skipped.

### Link semantics

Checked:

- relative links and images: `[text](docs/page.md)`, `![alt](images/logo.png)`;
- reference definitions: `[label]: docs/page.md`;
- `#fragment` links inside the same document;
- percent-encoded targets (`my%20file.md`), `?query` stripping, and
  `file#anchor` targets;
- directory targets (`docs/`) require `index.md` or `README.md`;
- paths are resolved and compared **case-sensitively** against the working
  tree, so results do not depend on a case-insensitive host filesystem;
- a `..` that escapes the project root is a failure.

Not checked (documented limit, not a silent pass):

- `http://`, `https://`, `mailto:`, `tel:`, other URI schemes, and
  protocol-relative `//host` URLs are never fetched;
- site-absolute `/path` URLs are routing concerns outside the project tree;
- fenced code blocks and inline code spans are ignored; indented code blocks
  are not specially ignored;
- HTML links (`<a href>`, `<img src>`) and multiline inline link destinations
  are outside the supported Markdown subset;
- anchor checking recognizes ATX headings (`# Heading`), not explicit HTML
  anchors or setext headings.

### Declared checker contract

`[[checkers]]` exists for catalogs that are dynamic rather than a static file
map (for example a Go `internal/docs` catalog plus embedded Markdown). The
checker is a real declared invariant, not a shell-hook substitute:

- `run` is an **argv array**, executed with `exec.Command` — no shell, no
  interpolation, no environment expansion. Use `["sh", "-c", "..."]` yourself
  if a shell is genuinely wanted.
- `dir` is relative to the project root (default: project root). It is
  resolved through symlinks against the resolved project root; a `dir` whose
  nearest existing ancestor resolves outside the project is a configuration
  failure, and the checker is never started there. The same resolved
  containment applies to `catalogFile`, including a not-yet-created child below
  a symlinked parent.
- `timeoutSeconds` defaults to 120 and is capped at 600.
- Exit code 0 passes; a passing checker's output is ignored.
- A nonzero exit fails the stage. Output lines shaped
  `path:line: message` become structured `content-checker` findings with that
  file and line; other lines are reported one finding per line with the checker
  name. Timeout, missing executable, or an invalid working directory also
  fail.
- The checker inherits the caller's environment and gets no stdin.

A dynamic-catalog project should still declare its static Markdown roots under
`include` so the built-in link/required checks apply, and use one checker for
catalog-to-document mapping that static TOML cannot express. The checker and
the built-ins run together; a checker cannot replace or disable them.

## Route-usage rule

Rule ID: `frontend-route-usage` (layer `frontend`, executed in the frontend
stage).

Generated `resources/js/routes.ts` resolves a helper's host from
`configureRouteHosts` at call time. A call evaluated while a module initializes
therefore freezes an empty or stale origin, and process-global origins can leak
across SSR requests. The rule reads the helper names from the generated
`resources/js/routes.ts` (it does not hard-code route names) and scans
`resources/js` for module-initialization calls.

Flagged:

- `export const href = routes.homePage()` and other module-scope constants;
- exported arrays/objects with eager calls (`export const links = [{ href:
  routes.homePage() }]`);
- namespace imports and aliases (`import * as R`, `import { routes as appRoutes
  }`), module-scope object aliases (`const r = routes`), and destructured
  helpers (`const { homePage } = routes`);
- one level of locally declared wrapper functions/arrow functions that return
  a full-URL helper and are called at module scope;
- immediately invoked function/arrow expressions and bare top-level calls;
- calls inside a template interpolation at module scope.

Allowed:

- `routes.homePage.path()` (explicit relative URL) and `.host` property access;
- calls in component render bodies, event handlers, and function bodies;
- exported or module-scope function/arrow definitions that are not invoked at
  module initialization.

Out of scope (documented limit; the Inertia runtime's request-scoped host
context is authoritative):

- re-exports and aliases that do not resolve to the generated routes module;
- dynamic property access (`routes[name]()`);
- cross-module wrappers;
- default exports/imports from `routes.ts`.

The analysis is token-based, not a TypeScript program: it has no type checker.
It resolves relative ESM specifiers and the `compilerOptions.paths` aliases of
the project's `tsconfig.json` (the generated `@/*` alias maps to
`resources/js/*`) to determine whether an import names
`resources/js/routes.ts`.

## Changed scope

Under `andurel verify --changed [--base <ref>]`:

- tracked staged/unstaged changes, untracked files, deletions, and renames are
  included; the default base is `HEAD`;
- a non-Git project, an invalid base, or `--changed` combined with Go package
  patterns is a usage error, never an empty green run;
- an empty change set is an explicit no-op: every selected stage is skipped and
  `scope.noop` is true; the run does not claim whole-project health;
- Go stages cover the changed packages and their in-module reverse
  dependencies, including external test packages (`XTestImports`); global
  files (`go.mod`, `go.sum`, `go.work`, `go.work.sum`, `andurel.toml`,
  `andurel.lock`, `bin/`) widen Go stages to `./...`;
- the frontend, generated, and content stages check the whole project when they
  run, and the reason is recorded in `scope.widened`;
- the content stage runs when the declaration or Markdown changes, when a
  declared `catalogFile`, catalog target, or `required` path changes, and when
  a file is deleted or renamed. A declared `[[checkers]]` entry is opaque: its
  argv and Go imports can read any project file, so any otherwise-unclassified
  change conservatively runs the whole content stage. A project with no
  `andurel.content.toml` keeps content not-applicable even in changed mode;
- the JSON report records `scope.mode`, `scope.base`, `scope.changed_files`,
  `scope.go_packages`, `scope.stages`, `scope.rules`, `scope.widened`, and
  `scope.noop`; each stage records selected/executed/skipped rules when it has
  rule granularity.
