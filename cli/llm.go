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

const llmDocumentation = `# Andurel Framework Documentation

Andurel is a comprehensive Rails-like web development framework for Go that focuses on speed of development for fullstack applications. This documentation is designed to help AI agents understand and work with Andurel projects.

## Framework Philosophy

Andurel follows Rails-like conventions with Go's performance and type safety. It combines:
- Echo web framework for HTTP routing
- SQLC for type-safe database queries
- Templ for type-safe HTML templates
- River for background job processing
- Convention over configuration

## Project Structure

` + "```" + `
.
├── assets/          # Static assets (compiled CSS, images, etc.)
├── bin/             # Compiled binaries (console, migration, run, tailwindcli)
├── cmd/             # Application entry points
│   ├── app/         # Main web application
│   ├── console/     # Interactive database console
│   ├── migration/   # Migration runner
│   └── run/         # Development server (air + templ + tailwind watch)
├── config/          # Application configuration
│   ├── app.go       # App config (sessions, tokens)
│   ├── config.go    # Main config aggregator
│   └── database.go  # Database configuration
├── controllers/     # HTTP request handlers
│   ├── controller.go    # Base controller utilities
│   ├── pages.go         # Page controllers
│   ├── api.go           # API controllers
│   └── assets.go        # Asset serving
├── css/             # Source CSS files (input for Tailwind)
├── database/        # Database schema and queries
│   ├── migrations/  # SQL migration files
│   ├── queries/     # SQL query files for SQLC
│   └── sqlc.yaml    # SQLC configuration
├── models/          # Data models and business logic
│   ├── model.go         # Base model setup
│   ├── errors.go        # Model error definitions
│   └── internal/db/     # Generated SQLC code (DO NOT EDIT)
├── queue/           # Background job processing
│   ├── jobs/        # Job definitions
│   └── workers/     # Worker implementations
├── router/          # Route definitions and middleware
│   ├── router.go        # Main router setup
│   ├── routes/          # Route definitions by domain
│   ├── cookies/         # Cookie and session helpers
│   └── middleware/      # Custom middleware
├── views/           # Templ templates
│   ├── *.templ          # Template source files
│   ├── *_templ.go       # Generated Go code (DO NOT EDIT)
│   └── components/      # Reusable view components
├── .env.example     # Example environment variables
└── go.mod           # Go module definition
` + "```" + `

## Available Commands

### Running the Application

#### ` + "`andurel run`" + `
Runs the development server with hot reloading for Go, Templ, and CSS changes.
This orchestrates three processes:
- Air for live Go compilation
- Templ watch for template compilation
- Tailwind CLI for CSS compilation

The server runs on port 8080 by default.

### App Management

#### ` + "`andurel app console`" + ` (alias: ` + "`andurel a c`" + `)
Opens an interactive console to interact with the database.
Allows direct database queries and model manipulation in a REPL environment.

### Code Generation

#### ` + "`andurel generate model [name]`" + ` (aliases: ` + "`andurel g model`" + `, ` + "`andurel gen model`" + `)
Generates a new model with the specified name.
- Table name is automatically pluralized (User → users)
- Creates model file in ` + "`models/`" + `
- Generates CRUD operations and database functions
- Use ` + "`--refresh`" + ` flag to regenerate SQL queries after schema changes

Example:
` + "```bash" + `
andurel generate model User
andurel generate model User --refresh  # After schema changes
` + "```" + `

#### ` + "`andurel generate controller [model_name]`" + ` (aliases: ` + "`andurel g controller`" + `, ` + "`andurel gen controller`" + `)
Generates a new resource controller with full CRUD actions.
- Creates controller with index, show, new, create, edit, update, destroy actions
- Automatically generates corresponding routes
- Model must already exist before generating controller
- Use ` + "`--with-views`" + ` to also generate view templates

Examples:
` + "```bash" + `
andurel generate controller User              # Controller only
andurel generate controller User --with-views # Controller + views
` + "```" + `

#### ` + "`andurel generate view [model_name]`" + ` (aliases: ` + "`andurel g view`" + `, ` + "`andurel gen view`" + `)
Generates view templates for the specified resource.
- Model must already exist
- Creates index, show, new, edit views
- Use ` + "`--with-controller`" + ` to also generate a resource controller

Examples:
` + "```bash" + `
andurel generate view User                    # Views only
andurel generate view User --with-controller  # Views + controller
` + "```" + `

#### ` + "`andurel generate resource [name]`" + ` (aliases: ` + "`andurel g resource`" + `, ` + "`andurel gen resource`" + `)
Generates a complete resource: model + controller + views + routes.
- Equivalent to running model, controller, and view generators together
- Table name is automatically pluralized
- Most comprehensive generator for full CRUD functionality

Example:
` + "```bash" + `
andurel generate resource Product  # Creates everything for products
` + "```" + `

### Database Migrations

#### ` + "`andurel migration new [name]`" + ` (aliases: ` + "`andurel m new`" + `, ` + "`andurel mig new`" + `)
Creates a new SQL migration file in ` + "`database/migrations/`" + `.
- Migration files are timestamped (e.g., 20240101120000_create_users_table.sql)
- Write raw SQL for schema changes

Example:
` + "```bash" + `
andurel migration new create_users_table
` + "```" + `

#### ` + "`andurel migration up`" + ` (aliases: ` + "`andurel m up`" + `, ` + "`andurel mig up`" + `)
Applies all pending migrations.
Runs migrations in chronological order.

#### ` + "`andurel migration down`" + ` (aliases: ` + "`andurel m down`" + `, ` + "`andurel mig down`" + `)
Rolls back the most recent migration.

#### ` + "`andurel migration up-to [version]`" + ` (aliases: ` + "`andurel m up-to`" + `, ` + "`andurel mig up-to`" + `)
Applies migrations up to a specific version.

#### ` + "`andurel migration down-to [version]`" + ` (aliases: ` + "`andurel m down-to`" + `, ` + "`andurel mig down-to`" + `)
Rolls back migrations down to a specific version.

#### ` + "`andurel migration reset`" + ` (aliases: ` + "`andurel m reset`" + `, ` + "`andurel mig reset`" + `)
Resets database by rolling back all migrations and reapplying them.
Useful for development but dangerous in production.

#### ` + "`andurel migration fix`" + ` (aliases: ` + "`andurel m fix`" + `, ` + "`andurel mig fix`" + `)
Re-numbers migrations to fix gaps in version numbers.

### SQLC Code Generation

#### ` + "`andurel sqlc generate`" + ` (aliases: ` + "`andurel s generate`" + `)
Generates Go code from SQL queries in ` + "`database/queries/`" + `.
- Output goes to ` + "`models/internal/db/`" + `
- Creates type-safe database access functions
- Run after adding/modifying SQL queries

#### ` + "`andurel sqlc compile`" + ` (aliases: ` + "`andurel s compile`" + `)
Compiles SQLC queries to check for errors without generating code.
Useful for validating SQL before generating.

## Framework Patterns and Conventions

### Routing

Routes are defined in ` + "`router/routes/`" + ` using a fluent builder pattern:

` + "```go" + `
var UserShowPage = newRouteBuilder("show").
    SetNamePrefix("users").           // Route name: users.show
    SetPath("/users/:id").             // URL path
    SetMethod(http.MethodGet).         // HTTP method
    SetCtrl("Users", "Show").          // Controller and method
    WithMiddleware(auth.Required).     // Optional middleware
    WithSitemap().                     // Include in sitemap
    RegisterWithID()                   // Returns RouteWithID type
` + "```" + `

Route types:
- ` + "`Register()`" + ` - Returns ` + "`Route`" + ` (no parameters)
- ` + "`RegisterWithID()`" + ` - Returns ` + "`RouteWithID`" + ` (accepts UUID)
- ` + "`RegisterWithSlug()`" + ` - Returns ` + "`RouteWithSlug`" + ` (accepts string slug)
- ` + "`RegisterWithToken()`" + ` - Returns ` + "`RouteWithToken`" + ` (accepts string token)

All routes must be added to the ` + "`Registry`" + ` slice.

### Controllers

Controllers are structs with methods that match the Echo handler signature:

` + "```go" + `
type Users struct {
    db         database.Postgres
    insertOnly queue.InsertOnly
    cache      otter.CacheWithVariableTTL[string, templ.Component]
}

func (u Users) Show(c echo.Context) error {
    return render(c, views.UserShow())
}
` + "```" + `

Controller methods receive an ` + "`echo.Context`" + ` and return an ` + "`error`" + `.
Use the ` + "`render()`" + ` helper to render templ components.

### Models

Models use SQLC for database access:

` + "```go" + `
// In database/queries/users.sql:
-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING *;

// After running ` + "`andurel sqlc generate`" + `, use in models:
user, err := queries.GetUserByID(ctx, db, userID)
newUser, err := queries.CreateUser(ctx, db, CreateUserParams{
    Email: "user@example.com",
    Name: "John Doe",
})
` + "```" + `

Models can include business logic and validation using ` + "`github.com/go-playground/validator/v10`" + `.

### Views (Templ Templates)

Views use Templ for type-safe HTML templates:

` + "```templ" + `
// views/users/show.templ
package views

import "llmtext/models"

templ UserShow(user models.User) {
    @base() {
        <main class="container mx-auto p-4">
            <h1>{ user.Name }</h1>
            <p>{ user.Email }</p>
        </main>
    }
}
` + "```" + `

After editing ` + "`.templ`" + ` files:
- Run ` + "`go tool templ generate`" + ` to compile to Go code
- Or use ` + "`andurel run`" + ` which watches for changes

### Background Jobs

Background jobs use River (backed by PostgreSQL):

` + "```go" + `
// In queue/jobs/example_job.go
type ExampleJobArgs struct {
    UserID uuid.UUID
}

func (ExampleJobArgs) Kind() string { return "example_job" }

// In queue/workers/example_worker.go
type ExampleWorker struct {
    river.WorkerDefaults[ExampleJobArgs]
    db database.Postgres
}

func (w *ExampleWorker) Work(ctx context.Context, job *river.Job[ExampleJobArgs]) error {
    // Do work here
    return nil
}

// Enqueue a job (in controller or model):
_, err := insertOnly.Client.Insert(ctx, ExampleJobArgs{
    UserID: userID,
}, nil)
` + "```" + `

### Middleware

Custom middleware follows the Echo middleware pattern:

` + "```go" + `
func AuthRequired(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Authentication logic
        if !authenticated {
            return c.Redirect(http.StatusFound, "/login")
        }
        return next(c)
    }
}
` + "```" + `

Middleware can be applied:
- Globally in ` + "`router/router.go`" + `
- Per-route using ` + "`WithMiddleware()`" + `
- Per-group using route groups

### Sessions and Cookies

Sessions are handled via gorilla/sessions:

` + "```go" + `
// Get session
sess, _ := session.Get("session-name", c)

// Set value
sess.Values["user_id"] = userID
sess.Save(c.Request(), c.Response())

// Flash messages
cookies.SetFlash(c, cookies.FlashMessage{
    Type: "success",
    Message: "User created successfully",
})
` + "```" + `

### CSRF Protection

CSRF protection is automatic for non-API routes:
- Enabled for all routes except ` + "`/api/*`" + ` and ` + "`/assets/*`" + `
- Token stored in ` + "`_csrf`" + ` cookie
- Automatically validated on POST/PUT/DELETE requests

## Configuration

### Environment Variables

Configuration is loaded from ` + "`.env`" + ` file using ` + "`github.com/joho/godotenv`" + `.

Required variables (see ` + "`.env.example`" + `):

` + "```env" + `
# Environment
ENVIRONMENT=development  # development, staging, production

# Database (PostgreSQL)
DB_KIND=postgres
DB_PORT=5432
DB_HOST=127.0.0.1
DB_NAME=andurel
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable

# Application
PROJECT_NAME=llmtext
DOMAIN=localhost:8080
PROTOCOL=http

# Security (generate with ` + "`openssl rand -hex 32`" + `)
SESSION_KEY=<hex-string>
SESSION_ENCRYPTION_KEY=<hex-string>
TOKEN_SIGNING_KEY=<hex-string>
PEPPER=<hex-string>

# Email
DEFAULT_SENDER_SIGNATURE=info@example.com
` + "```" + `

### Database Options

Andurel supports two database backends:

**PostgreSQL** (recommended for production):
- Set ` + "`DB_KIND=postgres`" + `
- Requires PostgreSQL server
- Full SQLC feature support
- Background job support via River

### CSS Options

Two CSS configuration options:

**Tailwind CSS** (default):
- Source CSS in ` + "`css/base.css`" + `
- Compiled to ` + "`assets/css/style.css`" + `
- Includes watch mode with ` + "`andurel run`" + `
- Tailwind CLI binary located in ` + "`bin/tailwindcli`" + `

**Vanilla CSS**:
- Skip Tailwind setup
- Write CSS directly in ` + "`assets/css/`" + `
- No build step required

## Development Workflow

### Starting Development

1. Copy ` + "`.env.example`" + ` to ` + "`.env`" + ` and configure
2. Generate security keys if needed:
   ` + "```bash" + `
   openssl rand -hex 32  # Run 3 times for 3 keys
   ` + "```" + `
3. Create initial migration:
   ` + "```bash" + `
   andurel migration new create_initial_schema
   # Edit database/migrations/[timestamp]_create_initial_schema.sql
   ` + "```" + `
4. Run migrations:
   ` + "```bash" + `
   andurel migration up
   ` + "```" + `
5. Start development server:
   ` + "```bash" + `
   andurel run
   ` + "```" + `

### Adding a New Feature

**For a complete resource (model + controller + views + routes):**
` + "```bash" + `
andurel generate resource Product
` + "```" + `

**For individual components:**

1. Create migration:
   ` + "```bash" + `
   andurel migration new create_products_table
   # Edit migration file
   andurel migration up
   ` + "```" + `

2. Add SQLC queries in ` + "`database/queries/products.sql`" + `:
   ` + "```sql" + `
   -- name: GetProduct :one
   SELECT * FROM products WHERE id = $1;
   ` + "```" + `

3. Generate model:
   ` + "```bash" + `
   andurel generate model Product
   andurel sqlc generate
   ` + "```" + `

4. Generate controller with views:
   ` + "```bash" + `
   andurel generate controller Product --with-views
   ` + "```" + `

5. Routes are automatically added to ` + "`router/routes/`" + `

### Making Schema Changes

1. Create migration:
   ` + "```bash" + `
   andurel migration new add_description_to_products
   ` + "```" + `

2. Edit migration file:
   ` + "```sql" + `
   -- Add this to the migration file
   ALTER TABLE products ADD COLUMN description TEXT;
   ` + "```" + `

3. Apply migration:
   ` + "```bash" + `
   andurel migration up
   ` + "```" + `

4. Update SQLC queries if needed

5. Refresh model:
   ` + "```bash" + `
   andurel generate model Product --refresh
   ` + "```" + `

### Testing Changes

1. For Go code quality:
   ` + "```bash" + `
   go vet ./...
   golangci-lint run
   ` + "```" + `

2. Manual testing via browser at ` + "`http://localhost:8080`" + `

3. For templ template errors, check compilation:
   ` + "```bash" + `
   go tool templ generate
   ` + "```" + `

## Common Patterns

### Pagination

` + "```go" + `
const pageSize = 20
offset := (page - 1) * pageSize
users, err := queries.ListUsers(ctx, db, ListUsersParams{
    Limit: pageSize,
    Offset: offset,
})
` + "```" + `

### Form Handling

` + "```go" + `
func (u Users) Create(c echo.Context) error {
    email := c.FormValue("email")
    name := c.FormValue("name")

    user, err := queries.CreateUser(ctx, db, CreateUserParams{
        Email: email,
        Name: name,
    })
    if err != nil {
        return err
    }

    cookies.SetFlash(c, cookies.FlashMessage{
        Type: "success",
        Message: "User created successfully",
    })

    return c.Redirect(http.StatusFound, routes.UserShowPage.URL(user.ID))
}
` + "```" + `

### JSON API Responses

` + "```go" + `
func (a API) GetUser(c echo.Context) error {
    id := c.Param("id")
    user, err := queries.GetUserByID(ctx, db, uuid.MustParse(id))
    if err != nil {
        return c.JSON(http.StatusNotFound, map[string]string{
            "error": "User not found",
        })
    }

    return c.JSON(http.StatusOK, user)
}
` + "```" + `
## Important Notes

- Always run ` + "`andurel sqlc generate`" + ` after modifying SQL queries
- Always run ` + "`go tool templ generate`" + ` after modifying ` + "`.templ`" + ` files (or use ` + "`andurel run`" + `)
- Never edit files in ` + "`models/internal/db/`" + ` directly - they're generated by SQLC
- Never edit ` + "`*_templ.go`" + ` files - they're generated by Templ
- Use ` + "`--refresh`" + ` flag when regenerating models after schema changes
- CSRF tokens are automatically handled for HTML forms
- API routes should be prefixed with ` + "`/api`" + ` to skip CSRF checks
- Asset routes should be prefixed with ` + "`/assets`" + ` for proper caching

## Debugging Tips

- Check logs output by ` + "`andurel run`" + ` for compilation errors
- Use ` + "`andurel app console`" + ` for interactive database debugging
- Verify routes are registered correctly by checking ` + "`router/routes/routes.go`" + `
- Check ` + "`.env`" + ` file has all required variables
- For SQLC errors, run ` + "`andurel sqlc compile`" + ` to validate queries
- For templ errors, check syntax in ` + "`.templ`" + ` files matches templ specification

External tools:
- SQLC - Type-safe SQL code generation
- Templ - Template compilation
- Air - Live reload for Go
- Tailwind CLI - CSS framework (optional)`
