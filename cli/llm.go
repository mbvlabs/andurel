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

Rails-like web framework for Go with convention over configuration.

## Stack
- Echo (routing), SQLC (type-safe SQL), Templ (templates), River (background jobs), PostgreSQL

## Project Structure
` + "```" + `
├── cmd/
│   ├── app/          # Main application entry point
│   └── run/          # Development orchestrator (Air, Templ, Tailwind)
├── config/           # Configuration (sessions, database, app settings)
├── controllers/      # HTTP request handlers (Echo handlers)
├── database/
│   ├── migrations/   # Goose SQL migrations
│   └── queries/      # SQLC query definitions (*.sql)
├── email/            # Email templates and sending logic
├── clients/          # External service clients (e.g., Mailpit)
├── models/           # Business logic and validation
│   ├── factories/    # Test data factories
│   └── internal/db/  # Generated SQLC code (DO NOT EDIT)
├── queue/
│   ├── jobs/         # Job argument definitions
│   └── workers/      # River worker implementations
├── router/
│   ├── routes/       # Route definitions (fluent builder pattern)
│   ├── middleware/   # Custom middleware
│   └── cookies/      # Session/flash helpers
├── services/         # Business logic layer
├── views/            # Templ templates (*.templ → *_templ.go)
│   └── components/   # Reusable components
├── css/              # Tailwind source (css/base.css → assets/css/style.css)
├── assets/           # Compiled static files
└── internal/         # Framework internals (rendering, routing, storage, server, hypermedia)
` + "```" + `

## Key Commands
` + "```bash" + `
andurel run                          # Start dev server (hot reload)
andurel generate resource Product    # Full CRUD (model+controller+views+routes)
andurel db migration new create_table   # New migration
andurel db migration up                 # Apply migrations
andurel sqlc generate                # Generate Go from SQL queries
andurel app console                  # Database REPL
` + "```" + `

## Routes (router/routes/)
` + "```go" + `
// Define routes
var ProductShow = routing.NewRouteWithID(
    "/products/:id",  // path
    "show",           // name
    "products",       // prefix (becomes "products.show")
)

var ProductsList = routing.NewSimpleRoute(
    "/products",
    "index",
    "products",
)
` + "```" + `

Route constructors: ` + "`NewSimpleRoute`" + `, ` + "`NewRouteWithID`" + `, ` + "`NewRouteWithSlug`" + `, ` + "`NewRouteWithToken`" + `, ` + "`NewRouteWithFile`" + `, ` + "`NewRouteWithMultipleIDs`" + `

Connect routes to controllers (in router/connect_*_routes.go):
` + "```go" + `
func registerProductsRoutes(handler *echo.Echo, ctrl controllers.Products) error {
    errs := []error{}

    _, err := handler.AddRoute(echo.Route{
        Method:  http.MethodGet,
        Path:    routes.ProductShow.Path(),
        Name:    routes.ProductShow.Name(),
        Handler: ctrl.Show,
    })
    if err != nil {
        errs = append(errs, err)
    }

    return errors.Join(errs...)
}
` + "```" + `

## Controllers
` + "```go" + `
type Products struct {
    db         storage.Pool
    insertOnly queue.InsertOnly
}

func (p Products) Show(c *echo.Context) error {
    id := c.Param("id")
    product, _ := db.GetProduct(ctx, uuid.MustParse(id))
    return render(c, views.ProductShow(product))
}

func (p Products) Create(c *echo.Context) error {
    // ... create product
    // Generate URLs:
    return c.Redirect(http.StatusSeeOther, routes.ProductShow.URL(product.ID))
}
` + "```" + `

URL generation: ` + "`routes.HomePage.URL()`" + `, ` + "`routes.ProductShow.URL(productID)`" + `, ` + "`routes.PasswordEdit.URL(token)`" + `

## Models
Models wrap SQLC-generated queries with validation and business logic.

**1. Define SQL queries** (database/queries/products.sql):
` + "```sql" + `
-- name: QueryProductByID :one
SELECT * FROM products WHERE id = $1;

-- name: InsertProduct :one
INSERT INTO products (id, name, price) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateProduct :one
UPDATE products SET name = $2, price = $3 WHERE id = $1 RETURNING *;
` + "```" + `

**2. Run** ` + "`andurel sqlc generate`" + ` to generate code in ` + "`models/internal/db/`" + `

**3. Create model wrapper** (models/product.go):
` + "```go" + `
package models

type Product struct {
    ID    uuid.UUID
    Name  string
    Price int64
}

type CreateProductData struct {
    Name  string ` + "`validate:\"required,max=255\"`" + `
    Price int64  ` + "`validate:\"required,min=0\"`" + `
}

func FindProduct(ctx context.Context, exec storage.Executor, id uuid.UUID) (Product, error) {
    row, err := queries.QueryProductByID(ctx, exec, id)
    if err != nil {
        return Product{}, err
    }
    return rowToProduct(row)
}

func CreateProduct(ctx context.Context, exec storage.Executor, data CreateProductData) (Product, error) {
    if err := Validate.Struct(data); err != nil {
        return Product{}, errors.Join(ErrDomainValidation, err)
    }
    params := db.InsertProductParams{
        ID:    uuid.New(),
        Name:  data.Name,
        Price: data.Price,
    }
    row, err := queries.InsertProduct(ctx, exec, params)
    if err != nil {
        return Product{}, err
    }
    return rowToProduct(row)
}

func rowToProduct(row db.Product) (Product, error) {
    return Product{ID: row.ID, Name: row.Name, Price: row.Price}, nil
}
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

## Configuration (.env)
` + "```env" + `
ENVIRONMENT=development
DB_KIND=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=myapp
DB_USER=postgres
DB_PASSWORD=postgres
SESSION_KEY=<hex-string>           # openssl rand -hex 32
SESSION_ENCRYPTION_KEY=<hex-string>
TOKEN_SIGNING_KEY=<hex-string>
PEPPER=<hex-string>
` + "```" + `

## Authentication
Pre-built system includes:
- User registration with email confirmation
- Login/logout with session management
- Password reset flow with tokens
- Argon2 password hashing
- CSRF protection (automatic for non-` + "`/api/*`" + ` routes)

## Sessions & Flash Messages
` + "```go" + `
sess, _ := session.Get("session-name", c)
sess.Values["user_id"] = userID
sess.Save(c.Request(), c.Response())

cookies.SetFlash(c, cookies.FlashMessage{Type: "success", Message: "Saved!"})
` + "```" + `

## Common Workflows

**New resource:**
` + "```bash" + `
andurel db migration new create_product_table
andurel db migration up
andurel generate resource Product
` + "```" + `

## Important Notes
- Never edit ` + "`models/internal/db/*`" + ` (SQLC generated) or ` + "`*_templ.go`" + ` (Templ generated)
- Run ` + "`andurel sqlc generate`" + ` after changing ` + "`database/queries/*.sql`" + `
- ` + "`andurel run`" + ` auto-compiles Templ files on change
- CSRF enabled by default (skip with ` + "`/api/*`" + ` prefix)
- Database uses pgx/v5 with connection pooling and OpenTelemetry tracing
- Built-in observability: structured logging (slog), OTLP exporters

## Development Tools (andurel.lock)
Air, Goose, SQLC, Templ, Tailwind CLI, Mailpit (email testing)`
