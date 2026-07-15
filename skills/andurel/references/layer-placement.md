# Layer Placement

Use these rules when changing an Andurel app shaped like this repository.

## Placement Rules

| Code | Put it in |
| --- | --- |
| Database-backed entity structs, Bun table mappings, create/update input structs, validation, persistence methods, finder/query methods, domain errors | `models/` |
| Rules that must hold regardless of HTTP, admin UI, API, jobs, or seeds | `models/` |
| Test factory definitions, generated factory fields, factory options, reusable test data helpers | `models/factories/` |
| Multi-step workflows, transactions, cross-model coordination, policy checks that require current database state, external side effects coordinated with domain writes | `services/` |
| Request parsing, path/query/form payload binding, HTTP status/render choices, flash messages, redirects, CSRF/session handling | `controllers/`, `controllers/admin/`, `controllers/api/` |
| Route names, route paths, URL builders | `router/routes/` |
| Middleware and request/session plumbing | `router/middleware/`, `router/cookies/`, or `internal/request/` |
| Public templ pages, view models, schema helpers, templ-specific presentation labels | `views/` |
| Admin Inertia pages and reusable frontend components | `resources/js/Pages/`, `resources/js/Layouts/`, `resources/js/Components/`, `resources/js/components/ui/` |
| Source CSS and theme primitives | `css/` |
| Compiled/static assets | `assets/` |
| SQL schema changes | `database/migrations/` |
| Seed data | `database/seeds/` or `cmd/seed/` following the existing pattern |
| External provider adapters | `clients/` |
| Email templates and send helpers | `email/` |
| River job argument types | `queue/jobs/` |
| River workers and queue registration | `queue/` |
| App configuration structs and environment loading | `config/` |
| Reusable framework-like packages not specific to one resource | `internal/` |
| Application boot and lifecycle wiring | `cmd/app/` |

## Models

Put business logic in models when it belongs to the domain object itself.

Use models for:

- Entity structs and Bun tags.
- `CreateXData` and `UpdateXData` structs that represent domain input.
- Validation methods such as `Validate() error`.
- Null, JSON, and time normalization needed to build a valid entity.
- CRUD methods, finder methods, pagination, count queries, and domain query methods.
- Domain errors such as not found or validation failures.

Keep model methods independent of HTTP, Echo, Inertia, templ, sessions, cookies, flash messages, and route names.

Good model responsibilities:

- "A featured project requires a description."
- "A slug must be valid."
- "Create a project entity from domain input and validate it before insert."
- "Find a published item by slug."

Avoid putting orchestration in models when it coordinates multiple operations under a policy or transaction. That belongs in a service.

## Factories

Put model factories in `models/factories/`. Factories are test and seed support code; they should create valid model entities using exported model primitives without owning durable business rules themselves.

Use factories for:

- Test data builders for model entities.
- Default field values for generated `Entity` fields.
- Functional options such as `WithProjectName` or `WithProjectOwnerID`.
- Custom helpers that make tests and seeds clearer.

Keep the model `Entity` struct as the source of truth for generated factory fields. When the model changes, check or sync the factory with:

```bash
andurel generate factory ModelName --check --json
andurel generate factory ModelName --sync --json
```

For repo-wide drift checks, use:

```bash
andurel generate factories --check --json
andurel generate factories --sync --json
```

The factory sync command rewrites Andurel generated regions and preserves custom helpers outside those regions. Prefer syncing generated regions over hand-editing boilerplate. Hand-write only the custom helpers, options, or test-specific defaults that the generator cannot infer safely.

Do not put application workflows, authorization, queueing, email, or HTTP behavior in factories. If a setup path needs real application behavior, call the appropriate model or service from the test instead of hiding it in the factory.

## Services

Create or extend a service when the use case is bigger than one model operation.

Use services for:

- Transactions.
- Cross-model coordination.
- Workflow policies that require current database state.
- Coordinating persistence with email, queue insertion, cache invalidation, or external clients.
- Shared application use cases used by more than one delivery surface.

Do not create a service for a pass-through wrapper around one model method. Call the model directly from the controller until a real workflow appears.

Good service responsibilities:

- "Create a featured project only if fewer than three projects are featured, using a serializable transaction."
- "Register a user, create identity records, issue a token, and enqueue an email."
- "Reset a password after validating a token and updating dependent records."

Register new services in `services/service.go` through the existing `fx.Provide` module.

## Controllers

Keep controllers thin and delivery-specific.

Use controllers for:

- Reading params and query strings.
- Binding JSON or form payloads.
- Converting HTTP/admin form input into model data structs.
- Calling a model or service.
- Mapping domain/service errors into pages, validation errors, redirects, flash messages, or API responses.
- Rendering templ pages through `hypermedia` or Inertia pages through `internal/inertia`.
- Registering routes in `RegisterRoutes`.

Do not put durable business rules in controllers. If the same rule must apply outside that one HTTP action, move it to a model or service.

Use `controllers/admin/` for admin Inertia workflows and `controllers/api/` for API endpoints.

Register new controller constructors and route invocations in `controllers/controller.go`.

## Routes

Define route constants in `router/routes/`.

Use:

- `routing.NewSimpleRoute` for fixed paths.
- `routing.NewRouteWithSlug` for slug params.
- `routing.NewRouteWithSerialID` for integer ID params.

Controllers should refer to these route constants for route registration and URL generation. Do not scatter raw path strings through views and controllers when a route constant should exist.

## Views And Frontend

Use `views/` for templ-rendered public pages and emails only when the existing package pattern calls for it. It is acceptable for view files to include presentation adapters that convert model entities into strings, labels, lists, or schema data.

Keep presentation adapters out of models when they exist only for display:

- Label maps.
- Initials and fallback display text.
- Date labels.
- JSON-to-list formatting for rendering.
- Schema.org view helpers.

Use `resources/js/Pages/Admin/...` for admin Inertia screens. Keep page components focused on UI state and form behavior; preserve domain validation in Go models/services.

Use `resources/js/components/ui/` for reusable UI primitives and `resources/js/Components/` for app-level reusable components.

## Queue And Email

Put River argument structs in `queue/jobs/`.

Put worker implementations, queue registration, and queue helpers in `queue/`. Workers should call domain/application code rather than duplicating model rules.

Put email templates and send helpers in `email/`. Put provider-specific implementations in `clients/email/`.

## Internal Packages

Use `internal/` for reusable framework-like support code that is not owned by one resource:

- request context helpers
- routing definitions
- hypermedia rendering
- Inertia integration
- storage abstractions
- validation helpers
- server support

Do not put app-specific domain workflows in `internal/` just to make them feel shared. Use `models/` or `services/`.

## Dependency Wiring

Follow existing `go.uber.org/fx` modules.

- New config providers: `config/`.
- New services: `services/service.go`.
- New controllers: `controllers/controller.go`.
- New queue workers: `queue/` module files.
- Application lifecycle hooks: `cmd/app/main.go` only when startup/shutdown behavior changes.

Prefer constructor injection over package globals, except for the established model receiver singletons such as `models.Project`.

## Decision Checklist

Before adding code, answer:

1. Is this a rule about a domain object being valid or allowed? Put it in `models/`.
2. Does it require a transaction, multiple models, external side effects, or stateful policy checks? Put it in `services/`.
3. Is it about HTTP parsing, session/flash, redirects, or rendering? Put it in `controllers/`.
4. Is it only for how a value is displayed? Put it in `views/` or the relevant Vue component.
5. Is it a route path/name? Put it in `router/routes/`.
6. Is it reusable framework plumbing independent of this app's domain? Put it in `internal/`.
