package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLlmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Output framework documentation for LLM consumption",
		Long:  "Generates comprehensive documentation about the Andurel framework that can be used by AI assistants to understand and work with the project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmDocumentation)
			return nil
		},
	}

	cmd.AddCommand(newLlmControllersCommand())
	cmd.AddCommand(newLlmModelsCommand())
	cmd.AddCommand(newLlmViewsCommand())
	cmd.AddCommand(newLlmRouterCommand())
	cmd.AddCommand(newLlmHypermediaCommand())
	cmd.AddCommand(newLlmJobsCommand())
	cmd.AddCommand(newLlmConfigCommand())

	return cmd
}

func newLlmControllersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "controllers",
		Short: "Controller-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmControllersDocumentation)
			return nil
		},
	}
}

func newLlmModelsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Model-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmModelsDocumentation)
			return nil
		},
	}
}

func newLlmViewsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "views",
		Short: "View-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmViewsDocumentation)
			return nil
		},
	}
}

func newLlmRouterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "router",
		Short: "Router-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmRouterDocumentation)
			return nil
		},
	}
}

func newLlmHypermediaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hypermedia",
		Short: "Hypermedia architecture and Datastar usage (client + server)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmHypermediaDocumentation)
			return nil
		},
	}
}

func newLlmJobsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "jobs",
		Short: "Background jobs LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmJobsDocumentation)
			return nil
		},
	}
}

func newLlmConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Configuration and environment LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmConfigDocumentation)
			return nil
		},
	}
}

const llmDocumentation = `# Andurel Framework - LLM Reference (Overview)

Andurel is a Rails-like web framework for Go that prioritizes development speed with just enough convention.

## Purpose
- Build full-stack web apps quickly with generators and conventions
- Type safety across SQL (SQLC), HTML (Templ), and Go
- Batteries included: Echo, Datastar, River, sessions, CSRF, telemetry, email, auth

## Key Commands
` + "```bash" + `
andurel run                        # Dev server with live reload
andurel generate resource Product  # CRUD resource
andurel migrate up      # Apply migrations
andurel migrate new create_products_table
` + "```" + `

## Project Structure
` + "```" + `
myapp/
├── assets/              # Static assets
│   ├── css/            # Compiled CSS files
│   ├── js/            # JavaScript files
│   └── assets.go
├── clients/             # External service clients
│   └── email/          # Email client (Mailpit/AWS SES)
├── cmd/
│   ├── app/            # Main web application
├── bin/
│   └── shadowfax       # Development server orchestrator
├── config/              # Application configuration
│   ├── app.go          # Sessions, tokens, security
│   ├── database.go     # Database connection
│   ├── email.go        # Email configuration
│   ├── telemetry.go    # Logging, tracing, metrics config
│   └── config.go       # Main config aggregator
├── controllers/         # HTTP request handlers
│   ├── controller.go   # Base controller utilities
│   ├── cache.go        # Cache control utilities
│   ├── pages.go        # Page controllers
│   └── assets.go       # Asset serving
├── css/                 # Source CSS files (Tailwind input)
├── database/
│   ├── migrations/     # SQL migration files
│   ├── queries/        # SQLC query definitions
│   └── sqlc.yaml       # SQLC configuration
├── email/               # Email functionality
│   ├── email.go        # Email client and sending logic
│   ├── base_layout.templ    # Base email template layout
│   └── components.templ     # Reusable email components
├── internal/            # Internal framework packages
│   ├── hypermedia/     # Datastar/SSE helpers
│   ├── renderer/       # Template rendering
│   ├── routing/        # Routing utilities
│   ├── server/         # Server configuration
│   └── storage/        # Storage utilities
├── models/              # Data models and business logic
│   ├── model.go        # Base model setup
│   ├── factories/      # Model factories for testing
│   └── internal/db/    # Generated SQLC code (do not edit)
├── queue/               # Background job processing
│   ├── jobs/           # Job definitions
│   ├── workers/        # Worker implementations
├── router/              # Routes and middleware
│   ├── router.go       # Main router setup
│   ├── routes/         # Route definitions
│   ├── cookies/        # Cookie and session helpers
│   └── middleware/     # Custom middleware
├── services/            # Business logic services
│   ├── authentication.go    # Authentication service
│   ├── registration.go      # User registration service
│   └── reset_password.go    # Password reset service
├── telemetry/           # Observability setup
│   ├── logger.go       # Structured logging
│   ├── tracer.go       # Distributed tracing
│   ├── metrics.go      # Application metrics
│   └── helpers.go      # Telemetry utilities
├── views/               # Templ templates
│   ├── components/     # Reusable template components
│   ├── *.templ         # Template source files
│   └── *_templ.go      # Generated Go code (do not edit)
├── .env.example         # Example environment variables
├── .gitignore           # Git ignore patterns
├── andurel.lock         # Framework version lock file
├── Dockerfile           # Container build (docker ext)
├── go.mod               # Go module definition
└── go.sum               # Go module checksums
` + "```" + `

## More Detail (subcommands)
- andurel llm controllers
- andurel llm models
- andurel llm views
- andurel llm router
- andurel llm hypermedia
- andurel llm jobs
- andurel llm config
`

const llmControllersDocumentation = `# Andurel Framework - Controllers

TODO: Controllers documentation split-out.`

const llmModelsDocumentation = `# Andurel Framework - Models

TODO: Models documentation split-out.`

const llmViewsDocumentation = `# Andurel Framework - Views

Andurel views are written in templ (https://templ.guide). Views compile to Go code and are rendered by controllers.

## Where views live
- views/                # templ templates (.templ) + generated *_templ.go
- views/components/     # shared components (head, form elements, toasts)
- views/layout.templ    # base layout (named base)
- views/home.templ, views/not_found.templ, views/bad_request.templ, views/internal_error.templ
- auth views: views/login.templ, views/registration.templ, views/reset_password.templ, views/confirm_email.templ
- generated CRUD views: views/<table>_resource.templ

## Rendering pipeline (how views get used)
- Controllers call render(etx, views.SomeView(...)). This uses internal/renderer to render a templ.Component.
- For controller patterns and request payload handling, see: andurel llm controllers.

## Templ essentials (useful for Andurel views)
- templ files are normal Go packages with imports.
- Components are defined as functions:
` + "```templ" + `
package views

templ Home() {
  @base() {
    <main>...</main>
  }
}
` + "```" + `
- Control flow uses Go syntax: if / switch / for.
- Use {{ ... }} for raw Go blocks (e.g., local variables).
- Expressions in text or attributes are written as { expr } and are HTML-escaped by default.
- All tags must be closed (templ enforces this).

### Attributes, classes, styles
- String attributes: <a href={ templ.URL(path) }>.
- Boolean attributes: add a ? after the name to bind (e.g., disabled?={ isDisabled }).
- Conditional attributes: use if blocks inside the element.
- Class composition:
` + "```templ" + `
<button class={ "btn", templ.KV("is-loading", loading) }>Save</button>
` + "```" + `
- Styles are sanitized by default. Use templ.SafeCSS / templ.SafeCSSProperty only for trusted values.
- For trusted HTML, use templ.Raw (sparingly).

### Components and layout composition
- Layouts are regular components. The default layout is views.base with a children slot.
- Use children with { children... } inside layouts.
- You can pass components as parameters or use templ.Join to aggregate multiple components.

### Context in views
- templ provides an implicit ctx (context.Context) inside components.
- Andurel injects cookie-related context via renderer.Render; use router/cookies helpers (e.g., cookies.GetFlashesCtx(ctx)) as in the base layout.

## Andurel view conventions
- Wrap pages with @base(...) for consistent layout, assets, and flash toasts.
- Configure page metadata with head options:
` + "```templ" + `
@base(components.SetTitle("Dashboard"), components.SetDescription("Your account")) {
  <main>...</main>
}
` + "```" + `
- Reuse shared components:
  - components.SetupHead (head/meta + asset links)
  - components.InputField / components.Textarea / components.SubmitButton
  - components.ToastMessage (flash UI)
- CRUD views generated by andurel generate view/resource include forms and actions wired for hypermedia.
  - Any data-* attributes used for hypermedia interactivity are explained in: andurel llm hypermedia.
  - Server-side form handling + bindings are covered in: andurel llm controllers.

## Tooling
` + "```bash" + `
andurel views generate   # templ generate
andurel views format     # templ fmt (views + email)
andurel generate view User
andurel generate resource Product
andurel run              # dev server, watches templ changes
` + "```" + `
`

const llmRouterDocumentation = `# Andurel Framework - Router

TODO: Router documentation split-out.`

const llmHypermediaDocumentation = `# Andurel Framework - Hypermedia

Andurel follows a hypermedia-driven architecture where the server sends HTML and state updates over the wire, rather than JSON APIs consumed by client-side JavaScript frameworks. This approach keeps application logic on the server while enabling rich, interactive UIs.

## Implementation: Datastar

Andurel uses Datastar (https://data-star.dev) as its hypermedia library. Datastar provides:
- Declarative HTML attributes for client-side interactivity
- Server-Sent Events (SSE) for server-to-client updates
- A lightweight client runtime (~15KB) with no build step required

The server-side protocol is wrapped in the internal/hypermedia package for convenience.

## Client-side (views)

Datastar interaction happens via HTML attributes on elements in templ views. These attributes are generated in templates and interpreted by the Datastar client script.

### Form submit (scaffolded auth)
` + "```templ" + `
<form data-indicator:submitting data-on:submit={ "!$submitting && " + fmt.Sprintf("@post('%s')", routes.SessionCreate.URL()) }>
  <input type="email" id="email" data-bind="email" data-attr:disabled="$submitting" required/>
  <input type="password" id="password" data-bind="password" data-attr:disabled="$submitting" required/>
  @components.SubmitButton("Login")
</form>
` + "```" + `
Notes:
- data-on:submit triggers a Datastar action.
- data-bind keeps client signals in sync with inputs.
- data-indicator / data-attr:disabled / data-show are UI state helpers.

### CRUD forms (scaffolded resource views)
` + "```templ" + `
<form data-on:submit={ hypermedia.DataAction(http.MethodPost, routes.ProjectCreate.URL()) }>
  @components.InputField("Name", "name", "text", "", false, components.FieldProp{})
  <button type="submit">Create Project</button>
</form>
` + "```" + `
Notes:
- hypermedia.DataAction builds the Datastar action string (e.g., @post('/path')).
- For PUT/DELETE, resource views use the same helper with different methods.

### Toasts + UI state (scaffolded components)
` + "```templ" + `
<div data-signals:visible="true" data-show="$visible" data-init__delay.5000ms="$visible = false">
  <button data-on:click="$visible = false">...</button>
</div>
` + "```" + `
Notes:
- data-signals seeds signal state on render.
- data-show toggles element visibility.
- data-init__delay shows how to run client-side initialization with delay.

## Server-side (internal/hypermedia)

Server handlers respond with SSE events that the client interprets as patch/merge actions.

### Single-response helpers
- PatchElements / PatchElementTempl: send HTML patches.
- PatchSignals / MarshalAndPatchSignals: send signal updates.
- MergeSignals: merge signals.
- ExecuteScript / Redirect / ReplaceURL / Prefetch: run client JS via SSE.
- ReadSignals: parse Datastar signal payloads from requests.

### Long-lived streaming
- Broadcaster: maintains a streaming SSE connection and can send multiple events over time.

## How to choose server vs client responsibilities
- Use view attributes for UI state and user interactions.
- Use server helpers when you need to patch DOM or signal state from handlers.

## Where to look
- Client usage: ` + "`views/*.templ`" + `, ` + "`views/components/*.templ`" + `
- Server helpers: ` + "`internal/hypermedia/*`" + ` (generated from layout/templates/*hypermedia*)
- Controller patterns using views: see ` + "`andurel llm controllers`" + `
`

const llmJobsDocumentation = `# Andurel Framework - Background Jobs

TODO: Jobs documentation split-out.`

const llmConfigDocumentation = `# Andurel Framework - Configuration

TODO: Configuration documentation split-out.`
