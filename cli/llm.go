package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLlmCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "llm",
		Short: "Output framework documentation for LLM consumption",
		Long:  "Generates comprehensive documentation about the Andurel framework that can be used by AI assistants to understand and work with the project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(llmDocumentation)
			return nil
		},
	}
}

const llmDocumentation = `# Andurel Framework - LLM Reference

Andurel is a comprehensive web development framework for Go that prioritizes development speed. Inspired by Ruby on Rails, it uses just enough conventions to let you build full-stack web applications incredibly fast.

## Stack
- Echo (routing), SQLC (type-safe SQL), Templ (templates), River (background jobs), PostgreSQL, Tailwind CSS, Datastar (SSE hypermedia), Goose (migrations), pgx/v5 (database driver), OpenTelemetry (observability)

## Project Structure
` + "```" + `
├── assets/              # Static assets
│   ├── css/            # Compiled CSS files
│   ├── js/            # JavaScript files
│   └── assets.go              
├── clients/             # External service clients
│   └── email/          # Email client (Mailpit/AWS SES)
├── cmd/
│   ├── app/            # Main web application
│   └── run/            # Development server orchestrator
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

## Key Commands
` + "```bash" + `
andurel templ generate 				  			# Compile Templ templates	
andurel generate resource Product    			# Full CRUD (model+controller+views+routes)
andurel database migration new create_table		# New migration
andurel database migration up                 	# Apply migrations
andurel database queries generate          		# Generate Go from SQL queries
` + "```" + `

**4. Use in controllers:**
` + "```go" + `
product, err := models.FindProduct(ctx, p.db, productID)
newProduct, err := models.CreateProduct(ctx, p.db, models.CreateProductData{
    Name: "Widget",
    Price: 999,
})
` + "```" + `

## Views (Templ)
` + "```templ" + `
package views

import "t/internal/hypermedia"

templ ProductShow(product models.Product) {
    @base() {
        <h1>{ product.Name }</h1>
        <p>${ product.Price }</p>
    }
}

// Datastar forms with hypermedia
templ ProductForm() {
    <form data-on:submit={ hypermedia.DataAction(http.MethodPost, routes.ProductCreate.URL()) }>
        <input type="text" data-bind="name" required/>
        <input type="number" data-bind="price" required/>
        <button type="submit">Create</button>
    </form>
}
` + "```" + `

## Hypermedia (Datastar SSE)
Andurel includes built-in Datastar support via ` + "`internal/hypermedia`" + ` package.

**In views** - Generate Datastar actions:
` + "```go" + `
hypermedia.DataAction(http.MethodPost, routes.ProductCreate.URL())
hypermedia.DataAction(http.MethodPut, routes.ProductUpdate.URL(id), hypermedia.ActionTypeForm)
` + "```" + `

**In controllers** - Server-sent events:
` + "```go" + `
// Redirect client (common after form submission)
hypermedia.Redirect(c, routes.HomePage.URL())

// Read client state from request
var payload struct{ Name string }
hypermedia.ReadSignals(c.Request(), &payload)

// Update DOM elements
hypermedia.PatchElementTempl(c, views.ProductCard(product),
    hypermedia.WithSelector("#product-" + id),
    hypermedia.WithModeOuter(),
)

// Update client state (signals)
hypermedia.MarshalAndPatchSignals(c, map[string]any{"count": 5})

// Remove elements
hypermedia.RemoveElementByID(c, "product-" + id)

// Execute JavaScript
hypermedia.ExecuteScript(c, "console.log('done')")
` + "```" + `

Patch modes: ` + "`outer`" + `, ` + "`inner`" + `, ` + "`append`" + `, ` + "`prepend`" + `, ` + "`before`" + `, ` + "`after`" + `, ` + "`replace`" + `, ` + "`remove`" + `

## Background Jobs (River)
` + "```go" + `
// queue/jobs/send_email.go
type SendEmailArgs struct{ To string }
func (SendEmailArgs) Kind() string { return "send_email" }

// queue/workers/send_email_worker.go
type SendEmailWorker struct {
    river.WorkerDefaults[SendEmailArgs]
}
func (w *SendEmailWorker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
    // Send email
    return nil
}

// Enqueue in controller:
insertOnly.Client.Insert(ctx, SendEmailArgs{To: "user@example.com"}, nil)
` + "```" + `

## Sessions & Flash Messages
` + "```go" + `
sess, _ := session.Get("session-name", c)
sess.Values["user_id"] = userID
sess.Save(c.Request(), c.Response())

cookies.SetFlash(c, cookies.FlashMessage{Type: "success", Message: "Saved!"})
` + "```" + `

## Common Workflows

## Important Notes
- Never edit ` + "`models/internal/db/*`" + ` (SQLC generated) or ` + "`*_templ.go`" + ` (Templ generated)
- Run ` + "`andurel database queries generate`" + ` after changing ` + "`database/queries/*.sql`" + `
- ` + "`andurel run`" + ` auto-compiles Templ files on change
- CSRF enabled by default (skip with ` + "`/api/*`" + ` prefix)
- Database uses pgx/v5 with connection pooling and OpenTelemetry tracing
- Built-in observability: structured logging (slog), OTLP exporters

## Development Tools (andurel.lock)
Air, Goose, SQLC, Templ, Tailwind CLI, Mailpit (email testing)`
