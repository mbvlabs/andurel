package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newLlmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "Output framework documentation for LLM consumption",
		Long:  "Generates comprehensive documentation about the Andurel framework that can be used by AI assistants to understand and work with the project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmDocumentation)
			return err
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
			_, err := os.Stdout.WriteString(llmControllersDocumentation)
			return err
		},
	}
}

func newLlmModelsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Model-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmModelsDocumentation)
			return err
		},
	}
}

func newLlmViewsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "views",
		Short: "View-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmViewsDocumentation)
			return err
		},
	}
}

func newLlmRouterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "router",
		Short: "Router-specific LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmRouterDocumentation)
			return err
		},
	}
}

func newLlmHypermediaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hypermedia",
		Short: "Hypermedia architecture and Datastar usage (client + server)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmHypermediaDocumentation)
			return err
		},
	}
}

func newLlmJobsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "jobs",
		Short: "Background jobs LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmJobsDocumentation)
			return err
		},
	}
}

func newLlmConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Configuration and environment LLM documentation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stdout.WriteString(llmConfigDocumentation)
			return err
		},
	}
}

const llmDocumentation = `# Andurel Framework - LLM Reference (Overview)

## CLI Commands
` + "```bash" + `
andurel llm
andurel llm controllers
andurel llm models
andurel llm views
andurel llm router
andurel llm hypermedia
andurel llm jobs
andurel llm config
andurel doctor --verbose # for checking project health, use in-place of go vet and go build
` + "```" + `

Andurel is a Rails-like web framework for Go that prioritizes development speed with just enough convention.

## Purpose
- Build full-stack web apps quickly with generators and conventions
- Type safety across SQL (SQLC), HTML (Templ), and Go
- Batteries included: Echo, Datastar, River, sessions, CSRF, telemetry, email, auth

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

## CLI Commands
` + "```bash" + `
andurel llm controllers
andurel generate controller User
andurel generate resource Product
andurel generate fragment User Search /search
` + "```" + `

Controllers handle HTTP requests, interact with models, and render views. They follow REST conventions and support both traditional page rendering and hypermedia (SSE) responses.

## Where controllers live
- controllers/             # HTTP handlers
- controllers/controller.go # Base render utility
- controllers/cache.go     # Generic caching implementation
- controllers/pages.go     # Static page handlers
- controllers/assets.go    # Asset serving with caching

## Controller structure

Controllers are structs with dependencies injected via constructors:

` + "```go" + `
type Users struct {
	db    storage.Pool
	cache *Cache[templ.Component]
}

func NewUsers(db storage.Pool) Users {
	cache, _ := NewCacheBuilder[templ.Component]().Build()
	return Users{db: db, cache: cache}
}
` + "```" + `

Methods follow Echo's handler signature and use short receiver names:

` + "```go" + `
func (u Users) Index(etx *echo.Context) error {
	users, err := models.AllUsers(etx.Request().Context(), u.db.Conn())
	if err != nil {
		return render(etx, views.InternalError())
	}
	return render(etx, views.UsersIndex(users))
}
` + "```" + `

## Rendering views

The render() helper renders templ components with automatic cookie/flash injection:

` + "```go" + `
func render(etx *echo.Context, t templ.Component) error {
	return renderer.Render(etx, t, []renderer.CookieKey{
		cookies.AppKey,
		cookies.FlashKey,
	})
}
` + "```" + `

Usage:
` + "```go" + `
return render(etx, views.UserShow(user))
return render(etx, views.NotFound())
return render(etx, views.InternalError())
` + "```" + `

### Partial rendering with fragments

For hypermedia responses, use renderer.ExtractFragment to render only a named fragment from a templ component:

` + "```go" + `
// Extract a single fragment
partial := renderer.ExtractFragment(views.UserShow(user), "user-card")
return render(etx, partial)

// Extract multiple fragments
partial := renderer.ExtractFragments(views.UserShow(user), []string{"user-card", "user-stats"})
return render(etx, partial)
` + "```" + `

Define fragments in templ views using the @fragment directive:

` + "```templ" + `
templ UserShow(user User) {
	@base() {
		@userCard(user)
		@userStats(user)
	}
}

templ userCard(user User) {
	@templ.Fragment("user-card") {
		<div id="user-card">...</div>
	}
}
` + "```" + `

This enables updating specific parts of the page via hypermedia without re-rendering the full layout.

## RESTful actions

Generated resource controllers include these actions:

| Action   | Method | Path              | Purpose                    |
|----------|--------|-------------------|----------------------------|
| Index    | GET    | /resources        | List (with pagination)     |
| Show     | GET    | /resources/:id    | Display single resource    |
| New      | GET    | /resources/new    | Display create form        |
| Create   | POST   | /resources        | Handle form submission     |
| Edit     | GET    | /resources/:id/edit | Display edit form        |
| Update   | PUT    | /resources/:id    | Handle update submission   |
| Destroy  | DELETE | /resources/:id    | Delete resource            |

## Request handling

### Path parameters
` + "```go" + `
userID, err := uuid.Parse(etx.Param("id"))
if err != nil {
	return render(etx, views.BadRequest())
}
` + "```" + `

### Query parameters (pagination)
` + "```go" + `
page := int64(1)
if p := etx.QueryParam("page"); p != "" {
	if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
		page = int64(parsed)
	}
}
perPage := int64(20)
if pp := etx.QueryParam("per_page"); pp != "" {
	if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 {
		perPage = int64(parsed)
	}
}
` + "```" + `

### Form payloads
Define typed structs with json tags for form binding:

` + "```go" + `
type CreateUserPayload struct {
	Name     string ` + "`json:\"name\"`" + `
	Email    string ` + "`json:\"email\"`" + `
	Age      int32  ` + "`json:\"age\"`" + `
	Birthday string ` + "`json:\"birthday\"`" + `  // dates as strings
}

func (u Users) Create(etx *echo.Context) error {
	var payload CreateUserPayload
	if err := etx.Bind(&payload); err != nil {
		slog.ErrorContext(etx.Request().Context(), "binding error", "error", err)
		return render(etx, views.BadRequest())
	}
	// ...
}
` + "```" + `

### Type conversions for model data
` + "```go" + `
// UUID fields
userID := func() uuid.UUID {
	if payload.UserID == "" {
		return uuid.Nil
	}
	parsed, _ := uuid.Parse(payload.UserID)
	return parsed
}()

// Time fields
birthday := func() time.Time {
	if payload.Birthday == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", payload.Birthday)
	return t
}()
` + "```" + `

## Flash messages

Add user feedback via flash messages:

` + "```go" + `
// Success
cookies.AddFlash(etx, cookies.FlashSuccess, "User created successfully")

// Error
cookies.AddFlash(etx, cookies.FlashError, fmt.Sprintf("Failed: %v", err))

// Warning / Info
cookies.AddFlash(etx, cookies.FlashWarning, "Session will expire soon")
cookies.AddFlash(etx, cookies.FlashInfo, "New features available")
` + "```" + `

Flash messages are automatically displayed via the base layout and cleared after display.

## Sessions

Create and manage user sessions:

` + "```go" + `
// Create session after login
cookies.CreateAppSession(etx, user)

// Destroy session on logout
cookies.DestroyAppSession(etx)

// Access session data in views via context (handled by render)
` + "```" + `

## Error handling patterns

` + "```go" + `
func (u Users) Update(etx *echo.Context) error {
	// Parse ID
	userID, err := uuid.Parse(etx.Param("id"))
	if err != nil {
		return render(etx, views.BadRequest())
	}

	// Find resource
	user, err := models.FindUser(etx.Request().Context(), u.db.Conn(), userID)
	if err != nil {
		return render(etx, views.NotFound())
	}

	// Bind payload
	var payload UpdateUserPayload
	if err := etx.Bind(&payload); err != nil {
		return render(etx, views.BadRequest())
	}

	// Update with flash feedback
	if err := user.Update(etx.Request().Context(), u.db.Conn(), data); err != nil {
		cookies.AddFlash(etx, cookies.FlashError, fmt.Sprintf("Update failed: %v", err))
		return etx.Redirect(http.StatusSeeOther, routes.UserEdit.URL(userID))
	}

	cookies.AddFlash(etx, cookies.FlashSuccess, "User updated")
	return etx.Redirect(http.StatusSeeOther, routes.UserShow.URL(userID))
}
` + "```" + `

## Caching

Use the generic cache for expensive operations:

` + "```go" + `
type Pages struct {
	cache *Cache[templ.Component]
}

func (p Pages) Home(etx *echo.Context) error {
	component, err := p.cache.Get("home", func() (templ.Component, error) {
		return views.Home(), nil
	})
	if err != nil {
		return render(etx, views.InternalError())
	}
	return render(etx, component)
}
` + "```" + `

Cache builder with options:
` + "```go" + `
cache, _ := NewCacheBuilder[templ.Component]().
	WithSize(100).
	WithDefaultTTL(15 * time.Minute).
	Build()
` + "```" + `

## Hypermedia responses

For Datastar/SSE responses instead of full page renders:

` + "```go" + `
// Redirect via SSE (client-side navigation)
hypermedia.Redirect(etx, routes.UserShow.URL(userID))

// Patch DOM elements
hypermedia.PatchElementTempl(etx, "#user-list", views.UserListPartial(users))

// Update signals
hypermedia.MarshalAndPatchSignals(etx, map[string]any{"loading": false})
` + "```" + `

For full hypermedia patterns, see: andurel llm hypermedia

## Related documentation
- Views and templates: andurel llm views
- Hypermedia/Datastar: andurel llm hypermedia
- Routes and middleware: andurel llm router
- Models and queries: andurel llm models
`

const llmModelsDocumentation = `# Andurel Framework - Models

## CLI Commands
` + "```bash" + `
andurel llm models
andurel generate model Product
andurel generate model Product --table-name=inventory
andurel generate model Product --skip-factory
andurel query generate user_roles
andurel query refresh user_roles
andurel query compile
andurel generate resource Product
` + "```" + `

Models are the single source of truth for data access in Andurel. They wrap database queries with Go structs, validation, and business logic. **All data access must go through the models package** - controllers and services never access the database directly.

## Architecture overview

` + "```" + `
┌─────────────────┐
│   Controllers   │  ← HTTP handlers, form binding
└────────┬────────┘
         │ calls
         ▼
┌─────────────────┐
│     Models      │  ← Business logic, validation, type conversion
└────────┬────────┘
         │ uses
         ▼
┌─────────────────┐
│ models/internal/db │  ← SQLC-generated code (DO NOT EDIT)
└────────┬────────┘
         │ executes
         ▼
┌─────────────────┐
│ database/queries │  ← SQL files (SQLC source)
└─────────────────┘
` + "```" + `

**Key principle: Controllers call models, models call queries.** Never bypass this chain.

## Where models live

- models/                  # Model files (one per table)
- models/model.go          # Validator setup and queries instance
- models/errors.go         # Domain error types
- models/internal/db/      # SQLC-generated code (auto-generated, DO NOT EDIT)
- models/factories/        # Test factories for creating test data
- database/queries/        # SQL query definitions (SQLC source files)

## Model structure

Each model wraps a database table with:
1. **Go struct** - Clean types for application use
2. **CRUD functions** - Find, Create, Update, Destroy
3. **Data structs** - Typed parameters for Create/Update
4. **Conversion functions** - Transform between SQLC types and model types

` + "```go" + `
package models

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"myapp/models/internal/db"
	"myapp/internal/storage"
)

// Product is the domain model - clean Go types
type Product struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string
	Description string
	Price       int64
}

// CreateProductData defines what's needed to create a product
type CreateProductData struct {
	Name        string ` + "`validate:\"required,max=255\"`" + `
	Description string
	Price       int64  ` + "`validate:\"required,min=0\"`" + `
}

// CreateProduct validates and persists a new product
func CreateProduct(
	ctx context.Context,
	exec storage.Executor,
	data CreateProductData,
) (Product, error) {
	if err := Validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.InsertProductParams{
		ID:          uuid.New(),
		Name:        data.Name,
		Description: data.Description,
		Price:       data.Price,
	}
	row, err := queries.InsertProduct(ctx, exec, params)
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}
` + "```" + `

## CRUD operations

Generated models include these standard functions:

| Function | Purpose | Returns |
|----------|---------|---------|
| Find{Model}(ctx, exec, id) | Find by primary key | Model, error |
| Create{Model}(ctx, exec, data) | Create with validation | Model, error |
| Update{Model}(ctx, exec, data) | Update with validation | Model, error |
| Destroy{Model}(ctx, exec, id) | Delete by ID | error |
| All{Models}(ctx, exec) | List all records | []Model, error |
| Paginate{Models}(ctx, exec, page, pageSize) | Paginated list | Paginated{Models}, error |
| Upsert{Model}(ctx, exec, data) | Insert or update | Model, error |

## Database executor pattern

All model functions accept a storage.Executor interface, not a direct connection. This enables:
- Regular queries via pool connection
- Transactional operations via tx

` + "```go" + `
// Regular query - use pool connection
user, err := models.FindUser(ctx, db.Conn(), userID)

// Transaction - use tx
tx, _ := db.Begin(ctx)
defer tx.Rollback()

user, err := models.FindUser(ctx, tx, userID)
if err != nil { return err }

err = models.DestroyUser(ctx, tx, userID)
if err != nil { return err }

tx.Commit()
` + "```" + `

## Data validation

Models use go-playground/validator for struct validation:

` + "```go" + `
type CreateUserData struct {
	Email    string ` + "`validate:\"required,email,max=255\"`" + `
	Password string ` + "`validate:\"required,min=8,max=72\"`" + `
}

// Validation happens automatically in Create/Update functions
user, err := models.CreateUser(ctx, exec, data)
if errors.Is(err, models.ErrDomainValidation) {
	// Handle validation error
}
` + "```" + `

## Type conversions (SQLC → Model)

SQLC generates types with pgtype wrappers. Models convert these to clean Go types:

` + "```go" + `
// rowToProduct converts SQLC row to domain model
func rowToProduct(row db.Product) Product {
	return Product{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt.Time,    // pgtype.Timestamptz → time.Time
		UpdatedAt:   row.UpdatedAt.Time,
		Name:        row.Name,
		Description: row.Description,
		Price:       row.Price,
	}
}
` + "```" + `

Common conversions:
- pgtype.Timestamptz → time.Time (use .Time field)
- pgtype.Text → string (use .String field)
- Nullable fields → zero values or explicit checks

## Database queries (SQLC)

SQL queries live in database/queries/ and are compiled by SQLC into Go code.

### Query file structure (database/queries/products.sql)
` + "```sql" + `
-- name: QueryProductByID :one
select * from products where id=$1;

-- name: QueryProducts :many
select * from products;

-- name: InsertProduct :one
insert into
    products (id, created_at, updated_at, name, description, price)
values
    ($1, now(), now(), $2, $3, $4)
returning *;

-- name: UpdateProduct :one
update products
    set updated_at=now(), name=$2, description=$3, price=$4
where id = $1
returning *;

-- name: DeleteProduct :exec
delete from products where id=$1;

-- name: QueryPaginatedProducts :many
select * from products
order by created_at desc
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountProducts :one
select count(*) from products;
` + "```" + `

### Adding custom queries

Add new queries to the SQL file, then run sqlc generate:

` + "```sql" + `
-- name: QueryProductsByCategory :many
select * from products where category_id = $1 order by name;

-- name: QueryProductsInPriceRange :many
select * from products where price between $1 and $2;
` + "```" + `

Then wrap in a model function:

` + "```go" + `
func FindProductsByCategory(
	ctx context.Context,
	exec storage.Executor,
	categoryID uuid.UUID,
) ([]Product, error) {
	rows, err := queries.QueryProductsByCategory(ctx, exec, categoryID)
	if err != nil {
		return nil, err
	}

	products := make([]Product, len(rows))
	for i, row := range rows {
		products[i] = rowToProduct(row)
	}
	return products, nil
}
` + "```" + `

## Two approaches to data access

### 1. Full models (recommended for most tables)
- Model file with struct, validation, business logic
- SQL queries generated automatically
- Use for entities with business rules

` + "```bash" + `
andurel generate model Product
` + "```" + `

### 2. Queries-only (for simple/junction tables)
- SQL queries without model wrapper
- Lighter-weight functions that use SQLC types directly
- Use for junction tables, lookup tables, or simple CRUD

` + "```bash" + `
andurel query generate user_roles
` + "```" + `

**Important:** Even with queries-only, all data access must still go through the ` + "`models/`" + ` package.
The ` + "`models/internal/db`" + ` package is internal to Go and cannot be imported from outside ` + "`models/`" + `.
Queries-only generates simpler wrapper functions in ` + "`models/`" + ` that don't include full model
structs, validation, or business logic - but controllers still call these wrapper functions,
never the internal queries directly.

When to use queries-only:
- Junction tables (user_roles, product_categories)
- Lookup/reference tables with no business logic
- Tables accessed rarely or only in specific contexts

## Factories (test data)

Factories create test data with sensible defaults:

` + "```go" + `
// Build in-memory (no database)
product := factories.BuildProduct()

// Create and persist to database
product, err := factories.CreateProduct(ctx, exec)

// With custom values
product := factories.BuildProduct(
	factories.WithProductsName("Custom Name"),
	factories.WithProductsPrice(9999),
)

// Create multiple
products, err := factories.CreateProducts(ctx, exec, 10)
` + "```" + `

Factories are generated automatically with models unless --skip-factory is used.

## Best practices

1. **All data access through models** - Never import models/internal/db outside the models package (Go's internal package rules prevent this anyway)
2. **Validate in models** - Use validate tags on data structs
3. **Keep models focused** - Business logic for one entity, not orchestration
4. **Use transactions for multi-step operations** - Pass tx as executor
5. **Custom queries in SQL files** - Don't use raw SQL strings in Go code
6. **Run sqlc after schema changes** - Keep generated code in sync

## Controller usage pattern

` + "```go" + `
func (p Products) Create(etx echo.Context) error {
	var payload CreateProductPayload
	if err := etx.Bind(&payload); err != nil {
		return render(etx, views.BadRequest())
	}

	// Models handle validation and persistence
	product, err := models.CreateProduct(
		etx.Request().Context(),
		p.db.Conn(),
		models.CreateProductData{
			Name:        payload.Name,
			Description: payload.Description,
			Price:       payload.Price,
		},
	)
	if err != nil {
		if errors.Is(err, models.ErrDomainValidation) {
			cookies.AddFlash(etx, cookies.FlashError, "Invalid product data")
			return etx.Redirect(http.StatusSeeOther, routes.ProductNew.URL())
		}
		return render(etx, views.InternalError())
	}

	cookies.AddFlash(etx, cookies.FlashSuccess, "Product created")
	return etx.Redirect(http.StatusSeeOther, routes.ProductShow.URL(product.ID))
}
` + "```" + `

## Related documentation
- Controllers and request handling: andurel llm controllers
- Views and templates: andurel llm views
- Database migrations: andurel migrate --help
`

const llmViewsDocumentation = `# Andurel Framework - Views

## CLI Commands
` + "```bash" + `
andurel llm views
andurel view compile
andurel view format
andurel generate view User
andurel generate resource Product
andurel run
` + "```" + `

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
- String attributes: <a href={ path }>.
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

`

const llmRouterDocumentation = `# Andurel Framework - Router

## CLI Commands
` + "```bash" + `
andurel llm router
andurel generate resource Product
andurel generate controller Product
` + "```" + `

The router package handles HTTP routing, middleware, sessions, and cookies. Andurel uses Echo v5 as its underlying web framework with a typed route system for compile-time safety.

## Where router code lives

- router/                      # Main router package
- router/router.go             # Router setup and global middleware
- router/routes/               # Named route definitions
- router/middleware/           # Custom middleware
- router/cookies/              # Session and flash message handling
- internal/routing/            # Route type implementations (generated, DO NOT EDIT)

## Architecture overview

` + "```" + `
┌─────────────────────────────────────────────────────────────┐
│                     Global Middleware                        │
│  (tracing → logging → session → context → CORS → CSRF)      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Route Registration                        │
│  router.RegisterXxxRoutes(controller) → echo.Route          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Route-level Middleware                      │
│           (AuthOnly, IPRateLimiter, custom)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Controller Handler                        │
└─────────────────────────────────────────────────────────────┘
` + "```" + `

## Named routes

Routes are defined as typed variables in router/routes/. This provides compile-time safety and centralized URL management.

### Route types

| Type | Constructor | Parameter | Usage |
|------|-------------|-----------|-------|
| Route | NewSimpleRoute | none | Static paths |
| RouteWithUUIDID | NewRouteWithUUIDID | uuid.UUID | Single :id parameter (UUID PK) |
| RouteWithSerialID | NewRouteWithSerialID | int32 | Single :id parameter (serial PK) |
| RouteWithBigSerialID | NewRouteWithBigSerialID | int64 | Single :id parameter (bigserial PK) |
| RouteWithStringID | NewRouteWithStringID | string | Single :id parameter (string PK) |
| RouteWithIDs | NewRouteWithMultipleIDs | map[string]uuid.UUID | Multiple ID params |
| RouteWithSlug | NewRouteWithSlug | string | :slug parameter |
| RouteWithToken | NewRouteWithToken | string | :token parameter |
| RouteWithFile | NewRouteWithFile | string | :file parameter |

### Defining routes

Routes are defined with path, name, and optional prefix:

` + "```go" + `
package routes

import "myapp/internal/routing"

// Prefix groups related routes
const ProductsPrefix = "/products"

// Simple route (no parameters)
var ProductIndex = routing.NewSimpleRoute(
	"",                    // path (empty = prefix only)
	"products.index",      // name
	ProductsPrefix,        // prefix
)

// Route with UUID parameter
var ProductShow = routing.NewRouteWithUUIDID(
	"/:id",
	"products.show",
	ProductsPrefix,
)

// Route with multiple IDs
var ProductCategoryShow = routing.NewRouteWithMultipleIDs(
	"/:product_id/categories/:category_id",
	"products.categories.show",
	ProductsPrefix,
)

// Route with token (e.g., password reset)
var PasswordEdit = routing.NewRouteWithToken(
	"/password/:token/edit",
	"users.edit_password",
	"/users",
)
` + "```" + `

### Using routes

Routes provide Path() for registration and URL() for link generation:

` + "```go" + `
// In route registration
r.e.AddRoute(echo.Route{
	Method:  http.MethodGet,
	Path:    routes.ProductShow.Path(),   // "/products/:id"
	Name:    routes.ProductShow.Name(),   // "products.show"
	Handler: products.Show,
})

// In controllers (redirects)
return etx.Redirect(http.StatusSeeOther, routes.ProductShow.URL(product.ID))

// In views (links)
<a href={ routes.ProductShow.URL(product.ID) }>View</a>

// Multiple IDs
url := routes.ProductCategoryShow.URL(map[string]uuid.UUID{
	"product_id":  productID,
	"category_id": categoryID,
})
` + "```" + `

## Registering routes

Routes are registered via methods on the Router struct:

` + "```go" + `
package router

import (
	"errors"
	"net/http"

	"myapp/controllers"
	"myapp/router/middleware"
	"myapp/router/routes"

	"github.com/labstack/echo/v5"
)

func (r Router) RegisterProductsRoutes(products controllers.Products) error {
	errs := []error{}

	// GET /products
	_, err := r.e.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    routes.ProductIndex.Path(),
		Name:    routes.ProductIndex.Name(),
		Handler: products.Index,
	})
	if err != nil {
		errs = append(errs, err)
	}

	// GET /products/:id
	_, err = r.e.AddRoute(echo.Route{
		Method:  http.MethodGet,
		Path:    routes.ProductShow.Path(),
		Name:    routes.ProductShow.Name(),
		Handler: products.Show,
	})
	if err != nil {
		errs = append(errs, err)
	}

	// POST /products (with route-level middleware)
	_, err = r.e.AddRoute(echo.Route{
		Method:  http.MethodPost,
		Path:    routes.ProductCreate.Path(),
		Name:    routes.ProductCreate.Name(),
		Handler: products.Create,
		Middlewares: []echo.MiddlewareFunc{
			middleware.AuthOnly,
		},
	})
	if err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
` + "```" + `

## Middleware

### Global middleware

Global middleware is configured in SetupGlobalMiddleware and applies to all routes:

` + "```go" + `
middlewares := []echo.MiddlewareFunc{
	mw.TraceRouteAttributes(tel),    // Add route info to traces
	mw.Logger(tel),                  // Request logging and metrics
	session.Middleware(store),       // Session management
	mw.ValidateSession,              // Session validation hook
	mw.RegisterAppContext,           // Inject app session into context
	mw.RegisterFlashMessagesContext, // Inject flash messages into context
	echomw.CORSWithConfig(...),      // CORS handling
	csrfMiddleware,                  // CSRF protection
	echomw.Recover(),                // Panic recovery (must be last)
}
` + "```" + `

Order matters: middlewares execute in order listed, with Recover() last to catch panics.

### Route-level middleware

Apply middleware to specific routes:

` + "```go" + `
_, err = r.e.AddRoute(echo.Route{
	Method:  http.MethodPost,
	Path:    routes.SessionCreate.Path(),
	Name:    routes.SessionCreate.Name(),
	Handler: sessions.Create,
	Middlewares: []echo.MiddlewareFunc{
		middleware.IPRateLimiter(5, routes.SessionNew),  // 5 attempts per 10 min
	},
})
` + "```" + `

### Authentication middleware

Protect routes that require authentication:

` + "```go" + `
// router/middleware/auth.go
func AuthOnly(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if cookies.GetApp(c).IsAuthenticated {
			return next(c)
		}
		return c.Redirect(http.StatusSeeOther, routes.SessionNew.URL())
	}
}

// Usage in route registration
Middlewares: []echo.MiddlewareFunc{
	middleware.AuthOnly,
}
` + "```" + `

### Rate limiting middleware

IP-based rate limiting with configurable limits:

` + "```go" + `
func IPRateLimiter(
	limit int32,
	redirectURL routing.Route,
) func(next echo.HandlerFunc) echo.HandlerFunc

// Example: 5 requests per 10 minutes, redirect to login on limit
middleware.IPRateLimiter(5, routes.SessionNew)
` + "```" + `

### Custom middleware pattern

` + "```go" + `
func MyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		// Skip for assets/API if needed
		if strings.Contains(c.Request().URL.Path, routes.AssetsPrefix) {
			return next(c)
		}

		// Pre-processing
		ctx := c.Request().Context()

		// Call next handler
		err := next(c)

		// Post-processing
		return err
	}
}
` + "```" + `

## Sessions and cookies

### App session

Store user session data (authentication state, user info):

` + "```go" + `
// router/cookies/cookies.go

// App struct holds session data (fields generated based on auth config)
type App struct {
	IsAuthenticated bool
	UserID          uuid.UUID
	UserEmail       string
}

// Create session after login
func CreateAppSession(c *echo.Context, user models.User) error

// Destroy session on logout
func DestroyAppSession(c *echo.Context) error

// Get session from echo context (in handlers)
func GetApp(c *echo.Context) App

// Get session from context.Context (in views via renderer)
func GetAppCtx(ctx context.Context) App
` + "```" + `

Usage in controllers:

` + "```go" + `
func (s Sessions) Create(etx *echo.Context) error {
	// ... authenticate user ...

	// Create session
	if err := cookies.CreateAppSession(etx, user); err != nil {
		return render(etx, views.InternalError())
	}

	return etx.Redirect(http.StatusSeeOther, routes.HomePage.URL())
}

func (s Sessions) Destroy(etx *echo.Context) error {
	if err := cookies.DestroyAppSession(etx); err != nil {
		return render(etx, views.InternalError())
	}

	return etx.Redirect(http.StatusSeeOther, routes.SessionNew.URL())
}
` + "```" + `

Usage in views:

` + "```templ" + `
templ navbar() {
	if cookies.GetAppCtx(ctx).IsAuthenticated {
		<a href={ routes.SessionDestroy.URL() }>Logout</a>
	} else {
		<a href={ routes.SessionNew.URL() }>Login</a>
	}
}
` + "```" + `

### Flash messages

One-time messages displayed after redirects:

` + "```go" + `
// router/cookies/flash.go

type FlashType string

const (
	FlashSuccess FlashType = "success"
	FlashError   FlashType = "error"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

// Add flash in controller
func AddFlash(c *echo.Context, flashType FlashType, msg string) error

// Get flashes (consumed on read)
func GetFlashes(c *echo.Context) ([]FlashMessage, error)

// Get from context (in views)
func GetFlashesCtx(ctx context.Context) []FlashMessage
` + "```" + `

Usage:

` + "```go" + `
// In controller
cookies.AddFlash(etx, cookies.FlashSuccess, "Product created successfully")
cookies.AddFlash(etx, cookies.FlashError, fmt.Sprintf("Failed: %v", err))
return etx.Redirect(http.StatusSeeOther, routes.ProductIndex.URL())
` + "```" + `

Flash messages are automatically displayed via the base layout component.

## CSRF protection

Andurel supports two CSRF strategies configured via APP_CSRF_STRATEGY:

| Strategy | Description |
|----------|-------------|
| header_only | Modern browsers only; relies on Sec-Fetch-Site header |
| header_or_legacy_token | Falls back to X-CSRF-Token header or _csrf form field |

CSRF middleware automatically:
- Skips API and asset routes
- Sets secure cookie options in production
- Validates trusted origins

For forms using hypermedia/Datastar, CSRF is handled automatically via cookies.

## Special routes

### Assets prefix

Routes under /assets skip session/CSRF middleware for performance:

` + "```go" + `
const AssetsPrefix = "/assets"

// Middleware skipping pattern
if strings.Contains(c.Request().URL.Path, routes.AssetsPrefix) {
	return next(c)
}
` + "```" + `

### API prefix

Routes under /api skip CSRF and session context:

` + "```go" + `
const APIPrefix = "/api"
` + "```" + `

### Custom routes

Register catch-all and special handlers:

` + "```go" + `
func (r *Router) RegisterCustomRoutes(
	riverHandler interface{ ServeHTTP(http.ResponseWriter, *http.Request) },
	notFoundHandler echo.HandlerFunc,
) {
	r.e.Any("/riverui*", echo.WrapHandler(riverHandler))  // River UI dashboard
	r.e.RouteNotFound("/*", notFoundHandler)              // 404 handler
}
` + "```" + `

Generators automatically:
- Create route definitions in router/routes/
- Create route registration methods in router/
- Wire up controller handlers

## Related documentation
- Controllers and request handling: andurel llm controllers
- Views and templates: andurel llm views
- Hypermedia/Datastar: andurel llm hypermedia
`

const llmHypermediaDocumentation = `# Andurel Framework - Hypermedia

## CLI Commands
` + "```bash" + `
andurel llm hypermedia
andurel generate resource Product
` + "```" + `

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

## CLI Commands
` + "```bash" + `
andurel llm jobs
` + "```" + `

Andurel uses River (https://riverqueue.com) for background job processing. River is a PostgreSQL-backed job queue that provides reliable, transactional job processing with automatic retries.

## Where job code lives

- queue/                    # Main queue package
- queue/queue.go            # River client setup (Processor, InsertOnly)
- queue/jobs/               # Job argument definitions
- queue/workers/            # Worker implementations
- queue/workers/workers.go  # Worker registration
- internal/storage/         # Queue interfaces (generated, DO NOT EDIT)

## Architecture overview

` + "```" + `
┌─────────────────────────────────────────────────────────────┐
│                      Controllers/Services                    │
│              queue.Insert(ctx, jobs.MyJobArgs{...})         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     River Client                             │
│              Inserts job into PostgreSQL                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   PostgreSQL (river_job)                     │
│              Persistent, transactional storage               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     River Workers                            │
│             Process jobs from queue asynchronously           │
└─────────────────────────────────────────────────────────────┘
` + "```" + `

## Defining jobs

Jobs are defined as argument structs in queue/jobs/:

` + "```go" + `
// queue/jobs/send_welcome_email.go
package jobs

type SendWelcomeEmailArgs struct {
	UserID    uuid.UUID
	UserEmail string
	UserName  string
}

// Kind returns a unique identifier for this job type
func (SendWelcomeEmailArgs) Kind() string { return "send_welcome_email" }
` + "```" + `

The Kind() method must return a unique string identifier for the job type.

## Implementing workers

Workers process jobs and live in queue/workers/:

` + "```go" + `
// queue/workers/send_welcome_email.go
package workers

import (
	"context"

	"github.com/riverqueue/river"

	"myapp/email"
	"myapp/queue/jobs"
)

type SendWelcomeEmailWorker struct {
	river.WorkerDefaults[jobs.SendWelcomeEmailArgs]
	sender email.TransactionalSender
}

func NewSendWelcomeEmailWorker(sender email.TransactionalSender) *SendWelcomeEmailWorker {
	return &SendWelcomeEmailWorker{sender: sender}
}

func (w *SendWelcomeEmailWorker) Work(
	ctx context.Context,
	job *river.Job[jobs.SendWelcomeEmailArgs],
) error {
	// Access job arguments
	args := job.Args

	// Do the work
	err := w.sender.Send(ctx, args.UserEmail, "Welcome!", "...")
	if err != nil {
		// Return error to retry (River handles retry logic)
		return err
	}

	return nil
}
` + "```" + `

### Error handling in workers

` + "```go" + `
func (w *MyWorker) Work(ctx context.Context, job *river.Job[jobs.MyArgs]) error {
	err := doWork(job.Args)
	if err != nil {
		// Permanent failure - don't retry
		if isPermanentError(err) {
			return river.JobCancel(err)
		}

		// Transient failure - retry with backoff
		return err
	}

	return nil
}
` + "```" + `

## Registering workers

Workers are registered in queue/workers/workers.go:

` + "```go" + `
package workers

import (
	"github.com/riverqueue/river"

	"myapp/email"
)

func Register(
	transactionalSender email.TransactionalSender,
	marketingSender email.MarketingSender,
) (*river.Workers, error) {
	wrks := river.NewWorkers()

	if err := river.AddWorkerSafely(wrks, NewSendTransactionalEmailWorker(transactionalSender)); err != nil {
		return nil, err
	}

	if err := river.AddWorkerSafely(wrks, NewSendMarketingEmailWorker(marketingSender)); err != nil {
		return nil, err
	}

	// Add more workers here...

	return wrks, nil
}
` + "```" + `

## Queue clients

Andurel provides two queue client types:

### Processor (full client)

Processes jobs from the queue. Used by the main application server:

` + "```go" + `
// Creates client that can insert AND process jobs
processor, err := queue.NewProcessor(ctx, db, workers)

// Start processing
processor.Start(ctx)

// Graceful shutdown
processor.Stop(ctx)
` + "```" + `

### InsertOnly (insert-only client)

Only inserts jobs, doesn't process them. Useful for CLI tools or services that only enqueue work:

` + "```go" + `
// Creates client that can only insert jobs
insertOnly, err := queue.NewInsertOnly(db, workers)
` + "```" + `

## Enqueueing jobs

### Basic insertion

` + "```go" + `
// In a controller or service
result, err := queue.Insert(ctx, jobs.SendWelcomeEmailArgs{
	UserID:    user.ID,
	UserEmail: user.Email,
	UserName:  user.Name,
}, nil)
` + "```" + `

### With options

` + "```go" + `
result, err := queue.Insert(ctx, jobs.SendWelcomeEmailArgs{...}, &river.InsertOpts{
	Queue:       "high_priority",           // Custom queue
	MaxAttempts: 5,                          // Max retry attempts
	ScheduledAt: time.Now().Add(time.Hour), // Delay execution
	Priority:    1,                          // Lower = higher priority
})
` + "```" + `

### Transactional insertion

Insert jobs atomically with database changes:

` + "```go" + `
tx, _ := db.Begin(ctx)
defer tx.Rollback()

// Create user
user, err := models.CreateUser(ctx, tx, userData)
if err != nil {
	return err
}

// Enqueue welcome email (only inserted if tx commits)
_, err = queue.InsertTx(ctx, tx, jobs.SendWelcomeEmailArgs{
	UserID:    user.ID,
	UserEmail: user.Email,
}, nil)
if err != nil {
	return err
}

tx.Commit()
` + "```" + `

### Bulk insertion

` + "```go" + `
// Insert many jobs efficiently
params := []river.InsertManyParams{
	{Args: jobs.SendEmailArgs{Email: "a@example.com"}},
	{Args: jobs.SendEmailArgs{Email: "b@example.com"}},
	{Args: jobs.SendEmailArgs{Email: "c@example.com"}},
}

// Returns results for each job
results, err := queue.InsertMany(ctx, params)

// Or insert fast without individual results
count, err := queue.InsertManyFast(ctx, params)
` + "```" + `

## River UI

River provides a web UI for monitoring jobs, accessible at /riverui when the app is running.

## Queue configuration

Configure queues in queue/queue.go:

` + "```go" + `
riverClient, err := river.NewClient(riverpgxv5.New(db.Conn()), &river.Config{
	Queues: map[string]river.QueueConfig{
		river.QueueDefault: {MaxWorkers: 100},
		"high_priority":    {MaxWorkers: 50},
		"low_priority":     {MaxWorkers: 10},
	},
	Logger:  slog.Default(),
	Workers: workers,
})
` + "```" + `

## Best practices

1. **Keep jobs small** - Store IDs and fetch data in worker, not large payloads
2. **Make jobs idempotent** - Jobs may run more than once on failure
3. **Use transactional inserts** - Ensure jobs only enqueue if related DB changes commit
4. **Handle errors appropriately** - Use river.JobCancel for permanent failures
5. **Monitor via River UI** - Check /riverui for failed/stuck jobs

## Related documentation
- Controllers (enqueueing jobs): andurel llm controllers
- Configuration: andurel llm config
`

const llmConfigDocumentation = `# Andurel Framework - Configuration

## CLI Commands
` + "```bash" + `
andurel llm config
` + "```" + `

Andurel uses environment variables for configuration, parsed at startup using the env library. Configuration is type-safe and validated.

## Where config code lives

- config/                   # Configuration package
- config/config.go          # Main Config struct and global vars
- config/app.go             # Application settings (host, port, sessions)
- config/database.go        # Database connection settings
- config/telemetry.go       # Observability settings
- config/email.go           # Email provider settings
- config/auth.go            # Authentication settings (if auth enabled)
- .env.example              # Example environment file

## Configuration structure

` + "```go" + `
// config/config.go

type Config struct {
	App       app
	DB        database
	Telemetry telemetry
	Email     email       // if email extension enabled
	Auth      auth        // if auth extension enabled
}

func NewConfig() Config {
	return Config{
		App:       newAppConfig(),
		DB:        newDatabaseConfig(),
		Telemetry: newTelemetryConfig(),
		Email:     newEmailConfig(),
		Auth:      newAuthConfig(),
	}
}
` + "```" + `

## Global variables

Commonly accessed values are exposed as package-level variables:

` + "```go" + `
import "myapp/config"

// Access anywhere in the app
config.Env         // "development" or "production"
config.ProjectName // Project name from PROJECT_NAME
config.ServiceName // Slugified service name for telemetry
config.Domain      // Domain (e.g., "localhost:8080" or "myapp.com")
config.BaseURL     // Full URL (e.g., "http://localhost:8080")
` + "```" + `

## Environment variables

### Application (config/app.go)

| Variable | Default | Description |
|----------|---------|-------------|
| ENVIRONMENT | development | Environment mode (development/production) |
| PROJECT_NAME | andurel | Project name |
| DOMAIN | localhost:8080 | Domain for cookies and URLs |
| PROTOCOL | http | Protocol for BaseURL (http/https) |
| HOST | localhost | Server bind address |
| PORT | 8080 | Server port |
| SESSION_KEY | (required) | 32-byte key for session authentication |
| SESSION_ENCRYPTION_KEY | (required) | 32-byte key for session encryption |
| TOKEN_SIGNING_KEY | (required) | Key for signing tokens (password reset, etc.) |
| CSRF_STRATEGY | header_only | CSRF protection mode |
| CSRF_TRUSTED_ORIGINS | | Comma-separated trusted origins |

### Database (config/database.go)

| Variable | Default | Description |
|----------|---------|-------------|
| DB_HOST | (required) | Database host |
| DB_PORT | (required) | Database port |
| DB_NAME | (required) | Database name |
| DB_USER | (required) | Database user |
| DB_PASSWORD | (required) | Database password |
| DB_KIND | (required) | Database type (postgres) |
| DB_SSL_MODE | (required) | SSL mode (disable/require/verify-full) |

### Telemetry (config/telemetry.go)

| Variable | Default | Description |
|----------|---------|-------------|
| TELEMETRY_SERVICE_NAME | {ProjectName} | Service name for traces/metrics |
| TELEMETRY_SERVICE_VERSION | 1.0.0 | Service version |
| OTLP_LOGS_ENDPOINT | | OTLP endpoint for logs |
| OTLP_METRICS_ENDPOINT | | OTLP endpoint for metrics |
| OTLP_TRACES_ENDPOINT | | OTLP endpoint for traces |
| OTLP_HEADERS | | Headers for OTLP requests |
| TRACE_SAMPLE_RATE | 1.0 | Trace sampling rate (0.0-1.0) |
| TELEMETRY_BATCH_SIZE | 512 | Batch size for telemetry export |
| TELEMETRY_BATCH_TIMEOUT_MS | 5000 | Batch timeout in milliseconds |

### Email (config/email.go)

| Variable | Default | Description |
|----------|---------|-------------|
| MAILPIT_HOST | 0.0.0.0 | Mailpit SMTP host (dev) |
| MAILPIT_PORT | 1025 | Mailpit SMTP port (dev) |

### Authentication (config/auth.go, if enabled)

| Variable | Default | Description |
|----------|---------|-------------|
| PEPPER | (required) | Secret pepper for password hashing |

## Adding custom configuration

### 1. Create a config struct

` + "```go" + `
// config/stripe.go
package config

import "github.com/caarlos0/env/v11"

type stripe struct {
	SecretKey      string ` + "`env:\"STRIPE_SECRET_KEY\"`" + `
	WebhookSecret  string ` + "`env:\"STRIPE_WEBHOOK_SECRET\"`" + `
	PublishableKey string ` + "`env:\"STRIPE_PUBLISHABLE_KEY\"`" + `
}

func newStripeConfig() stripe {
	cfg := stripe{}

	if err := env.ParseWithOptions(&cfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return cfg
}
` + "```" + `

### 2. Add to Config struct

` + "```go" + `
// config/config.go

type Config struct {
	App       app
	DB        database
	Telemetry telemetry
	Stripe    stripe  // Add new field
}

func NewConfig() Config {
	return Config{
		App:       newAppConfig(),
		DB:        newDatabaseConfig(),
		Telemetry: newTelemetryConfig(),
		Stripe:    newStripeConfig(),  // Initialize
	}
}
` + "```" + `

### 3. Use in application

` + "```go" + `
func main() {
	cfg := config.NewConfig()

	stripeClient := stripe.New(cfg.Stripe.SecretKey)
}
` + "```" + `

## Environment tags

The env library supports these struct tags:

` + "```go" + `
type example struct {
	// Required field (no default)
	Required string ` + "`env:\"REQUIRED_VAR\"`" + `

	// Optional with default
	Optional string ` + "`env:\"OPTIONAL_VAR\" envDefault:\"default_value\"`" + `

	// Slice (comma-separated by default)
	List []string ` + "`env:\"LIST_VAR\" envSeparator:\",\"`" + `

	// Nested prefix
	Nested nested ` + "`envPrefix:\"NESTED_\"`" + `
}
` + "```" + `

## Development setup

### Generate secure keys

` + "```bash" + `
# Generate 32-byte keys for sessions
openssl rand -base64 32  # SESSION_KEY
openssl rand -base64 32  # SESSION_ENCRYPTION_KEY
openssl rand -base64 32  # TOKEN_SIGNING_KEY
openssl rand -base64 32  # PEPPER (if using auth)
` + "```" + `

### Example .env file

` + "```bash" + `
# Application
ENVIRONMENT=development
PROJECT_NAME=myapp
DOMAIN=localhost:8080
PROTOCOL=http
HOST=localhost
PORT=8080
SESSION_KEY=your-32-byte-base64-key
SESSION_ENCRYPTION_KEY=your-32-byte-base64-key
TOKEN_SIGNING_KEY=your-32-byte-base64-key
CSRF_STRATEGY=header_only

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=myapp_dev
DB_USER=postgres
DB_PASSWORD=postgres
DB_KIND=postgres
DB_SSL_MODE=disable

# Telemetry (optional)
TELEMETRY_SERVICE_NAME=myapp
OTLP_TRACES_ENDPOINT=http://localhost:4318/v1/traces

# Email (development)
MAILPIT_HOST=localhost
MAILPIT_PORT=1025

# Auth (if enabled)
PEPPER=your-32-byte-base64-key
` + "```" + `

## Production considerations

1. **Use secrets management** - Don't commit .env files; use Vault, AWS Secrets Manager, etc.
2. **Set ENVIRONMENT=production** - Enables secure cookie settings, stricter CSRF
3. **Use HTTPS** - Set PROTOCOL=https in production
4. **Configure telemetry endpoints** - Send traces/metrics to your observability platform
5. **Rotate keys periodically** - Especially SESSION_KEY and TOKEN_SIGNING_KEY

## Accessing config in code

` + "```go" + `
// In main.go or application setup
cfg := config.NewConfig()

// Pass to components that need it
server := server.New(cfg)
router := router.New(cfg)

// Or access globals for simple values
if config.Env == "production" {
	// Production-specific logic
}

url := config.BaseURL + "/path"
` + "```" + `

## Related documentation
- Router (CSRF configuration): andurel llm router
- Background jobs: andurel llm jobs
- Telemetry setup: See telemetry/ package
`
