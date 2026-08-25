# Andurel Inertia v3 adapter

Andurel's Echo-native Inertia v3 protocol implementation is independently
versioned at `github.com/mbvlabs/andurel/pkg/inertia`. Generated controllers and
the router import that package directly. The package provides Vite integration,
protocol behavior, and the SSR runtime. The application owns its compiled templ
document at `views/root.templ`. Generated `config/inertia.go` contains the ENV-backed settings, translates them
into options, and wires the renderer lifecycle. The browser adapter remains the official
`@inertiajs/*` client.

This document is the protocol and migration baseline for the adapter. The
official Inertia v3 protocol and the 3.x Laravel adapter are normative. Gonertia
is only a design reference.

## Request and response contract

The adapter recognizes an Inertia visit only when `X-Inertia` is `true`. It
parses the version, partial component, partial `only` and `except` paths, reset
paths, error bag, infinite-scroll merge intent, and retained once-prop keys.
Comma-separated header values are trimmed, empty values are discarded, and the
request context used by prop resolvers is the Echo request context.

Initial visits receive a buffered templ document. Inertia visits receive a JSON
page object. Valid page responses set `X-Inertia: true`; all responses append
`X-Inertia` to `Vary` without replacing existing values. Control responses such
as version reloads and location visits use status 409 and never carry the
Inertia page-response header.

The required page fields are `component`, `props`, `url`, and `version`. Empty
conditional metadata is omitted. `props.errors` is always a JSON object. Flash
is a page field rather than an ordinary prop. Page props replace shared props,
except that the protected `errors` field cannot be supplied as an ordinary
shared or page prop.

Nested partial reload paths match in both directions: selecting a parent keeps
its descendants, while selecting a descendant retains the parent structure.
`only` is evaluated before `except`. Filters are ignored when the partial
component does not equal the rendered component. The same paths govern prop
resolution and merge/once metadata.

Optional and deferred props are not evaluated on a full visit. Deferred props
announce their group. A selected deferred prop is evaluated during a matching
partial reload; a rescued failure omits its value and adds its path to
`rescuedProps`. Already retained once props are omitted from `props` but remain
in `onceProps`. Explicitly selecting a once prop or marking it fresh forces it
to resolve. Reset paths remove merge metadata for that path.

For Inertia requests, 302 responses after POST, PUT, PATCH, and DELETE become 303.
Empty successful responses redirect back. External location visits use
`X-Inertia-Location`; fragment redirects use `X-Inertia-Redirect`. Version
mismatches trigger a location reload only for GET requests and include the
current version header. Generated session integration reflashes consumed flash
messages whenever a handler redirects, preserving them across redirect chains.

## Public API decisions

`Props` is an Andurel-owned map type. `FromStruct` reflects exported struct
fields and honors their JSON tags, keeping those tags as the source of
browser-facing field names. Resolver
callbacks accept `*echo.Context` and return `(any, error)`. Prop policies are
composable through the value returned by `Prop` constructors rather than being
encoded in application domain types.

History encryption is client metadata only. The adapter emits
`encryptHistory`, `clearHistory`, and `preserveFragment`; encryption of browser
history is performed by the official client.

SSR remains an option on a single `Page` call. The `inertia` package contains
the bounded HTTP renderer and optional managed process runtime. `Renderer.Start`
and `Renderer.Shutdown` expose lifecycle without coupling the package to Fx.
Production falls back to client-side rendering; verification can opt into
fail-fast behavior.

Generated configuration also exposes `INERTIA_CONTAINER_ID`,
`INERTIA_VITE_DEV_URL`, and `INERTIA_PROTOCOL_DEBUG`. SSR mode is selected with
`INERTIA_SSR_MODE`: `disabled`, `managed`, or `external`. Managed mode requires Node.js 22 or newer and the bundle configured
by `INERTIA_SSR_BUNDLE` (default `assets/dist/ssr/ssr.js`). The application composition root starts, health-checks, monitors, and shuts
down that process through the renderer lifecycle. External mode only connects to `INERTIA_SSR_URL`; its operator owns
the process lifecycle. Request timeout, startup timeout, response-size limit,
and fail-fast behavior have separate configuration values.

Generated resource payload structs carry JSON tags. Their corresponding
TypeScript declarations are generated next to application TypeScript types and
page components import those declarations instead of redefining response
shapes. Database model structs are never passed directly to an Inertia page.

## Gonertia audit

| Gonertia API or pattern | Decision | Andurel replacement |
| --- | --- | --- |
| Reusable `Inertia` engine | Adapt | Echo-native `Renderer` initialized once |
| `Props` map | Adapt | Andurel `Props` plus `FromStruct` |
| `Render` / application `Page` | Adapt | generated `Renderer.Page` |
| `Redirect`, `Location` | Adapt | Echo-native helpers with v3 fragment rules; empty responses redirect to the request referrer |
| Generic `net/http` middleware and buffered writer | Replace | Narrow Echo middleware using Echo response state |
| Static shared props | Keep | `WithShared` |
| Request shared props | Adapt | providers receiving the active Echo context |
| `Always`, `Optional`, `Defer` | Adapt | composable v3 prop policy values |
| Top-level partial filtering | Replace | JSON-tag-aware nested dot-path resolver |
| Merge, prepend, deep merge, match | Adapt | emit metadata only for selected/resolved paths |
| Old once-prop behavior | Replace | retention header, keys, expiry, and fresh semantics |
| Scroll prop | Adapt | v3 merge intent and `scrollProps` metadata |
| Validation context helpers | Adapt | response option and named error-bag handling |
| Flash provider | Adapt | generated request/session provider and page `flash` |
| Global `WithSSR` engine option | Replace | per-response `WithSSR()` page option |
| HTTP SSR gateway | Adapt | `SSRRenderer` plus bounded `HTTPRenderer` |
| Go `html/template` root | Replace | application-owned `views/root.templ` supplied with `WithRoot` |
| Custom JSON marshaller | Omit | fixed `encoding/json` for protocol safety |
| Vite helpers | Adapt | package defaults configured by options and the official v3 plugin |
| Precognition | Intentionally omit | no Andurel validation protocol exists yet |

## Version and migration policy

New scaffolds pin the official browser adapters to Inertia major version 3 and
use `@inertiajs/vite`. The doctor command treats v2 packages, Gonertia imports,
or missing generated templ output as migration diagnostics. Existing custom
Gonertia options are not translated silently: upgrades call them out for manual
migration to an Andurel renderer or page option.

Controllers use the standalone `Renderer` directly for `Page`, `Redirect`,
`Location`, and `Middleware`. Applications customize the initial document in
`views/root.templ`; `config/inertia.go` supplies it with `WithRoot`.
For local diagnostics, `WithProtocolDebug(true)` logs request classification
and emitted metadata keys without logging prop or session values.
