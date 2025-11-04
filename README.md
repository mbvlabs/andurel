<p align="center">
  <img width="400" height="300" alt="andurel-logo" src="https://github.com/user-attachments/assets/8261d514-c070-44c0-a96a-4132045855fc" />
</p>

# Andurel - Rails-like Web Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.24.4%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**Andurel is a comprehensive web development framework for Go that prioritizes development speed.** Inspired by Ruby on Rails, it combines convention over configuration with Go's performance and type safety to help you build full-stack web applications incredibly fast.

## Why Andurel?

Development speed is everything. Andurel eliminates boilerplate and lets you focus on building features:

- **Lightning Fast Setup** - New projects ready in seconds with `andurel new`
- **Instant Scaffolding** - Generate complete CRUD resources with one command
- **Live Reload** - Hot reloading for Go, templates, and CSS with `andurel run`
- **Type Safety Everywhere** - SQLC for SQL, Templ for HTML, Go for logic
- **Batteries Included** - Echo, Datastar, background jobs, sessions, CSRF protection, optional auth and email
- **Convention over Configuration** - Sensible defaults that just work
- **Your Choice** - Pick PostgreSQL or SQLite, Tailwind or vanilla CSS

## Core Technologies

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP framework
- **[SQLC](https://sqlc.dev/)** - Type-safe SQL code generation
- **[Templ](https://templ.guide/)** - Type-safe HTML templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity
- **[River](https://riverqueue.com/)** - PostgreSQL-backed background jobs
- **PostgreSQL or SQLite** - Choose your database
- **Tailwind CSS or vanilla CSS** - Choose your styling approach

## Quick Start

### Installation

```bash
go install github.com/mbvlabs/andurel@latest
```

### Create Your First Project

Andurel gives you choices when creating a new project:

```bash
# Create a new project with defaults (PostgreSQL + Tailwind CSS)
andurel new myapp

# Or customize your stack:
andurel new myapp -d sqlite              # Use SQLite instead of PostgreSQL
andurel new myapp -c vanilla             # Use vanilla CSS instead of Tailwind
andurel new myapp -d sqlite -c vanilla   # SQLite + vanilla CSS

# Add extensions for common features:
andurel new myapp -e auth                # Add authentication
andurel new myapp -e email               # Add email support
andurel new myapp -e auth,email          # Add multiple extensions

cd myapp

# Configure environment
cp .env.example .env
# Edit .env with your database settings

# Run the development server (with live reload)
andurel run
```

Your app is now running at `http://localhost:8080` with automatic reloading for Go, Templ, and CSS changes.

### Generate Your First Resource

```bash
# Create a complete resource with model, controller, views, and routes
andurel generate resource Product

# Or use the shorthand
andurel g resource Product
```

This single command creates everything you need for a full CRUD interface.

## Project Structure

Generated projects follow a clear, Rails-inspired structure:

```
myapp/
├── assets/              # Static assets (compiled CSS, images)
├── bin/                 # Compiled binaries (console, migration, run, tailwindcli)
├── cmd/
│   ├── app/            # Main web application
│   ├── console/        # Interactive database console
│   ├── migration/      # Migration runner
│   └── run/            # Development server orchestrator
├── config/              # Application configuration
│   ├── app.go          # Sessions, tokens, security
│   ├── database.go     # Database connection
│   └── config.go       # Main config aggregator
├── controllers/         # HTTP request handlers
│   ├── controller.go   # Base controller utilities
│   ├── pages.go        # Page controllers
│   └── assets.go       # Asset serving
├── css/                 # Source CSS files (Tailwind input)
├── database/
│   ├── migrations/     # SQL migration files
│   ├── queries/        # SQLC query definitions
│   └── sqlc.yaml       # SQLC configuration
├── models/              # Data models and business logic
│   ├── model.go        # Base model setup
│   └── internal/db/    # Generated SQLC code (do not edit)
├── queue/               # Background job processing (River)
│   ├── jobs/           # Job definitions
│   └── workers/        # Worker implementations
├── router/              # Routes and middleware
│   ├── router.go       # Main router setup
│   ├── routes/         # Route definitions
│   ├── cookies/        # Cookie and session helpers
│   └── middleware/     # Custom middleware
├── views/               # Templ templates
│   ├── *.templ         # Template source files
│   └── *_templ.go      # Generated Go code (do not edit)
├── .env.example         # Example environment variables
└── go.mod               # Go module definition
```

## CLI Commands

Andurel provides a comprehensive CLI for all development tasks. Most commands have short aliases to speed up your workflow.

### Development Server

```bash
# Run development server with hot reload for Go, Templ, and CSS
andurel run
```

This orchestrates Air (Go live reload), Templ watch, and Tailwind CSS compilation.

### Code Generation

Generate complete resources or individual components with type-safe code:

```bash
# Generate everything: model + controller + views + routes
andurel generate resource Product
andurel g resource Product  # short alias

# Generate individual components
andurel generate model User              # Create model
andurel generate model User --refresh    # Refresh after schema changes
andurel generate controller User         # Controller only
andurel generate controller User --with-views  # Controller + views
andurel generate view User               # Views only
andurel generate view User --with-controller   # Views + controller

# Short aliases work too
andurel g model User
andurel gen controller Product --with-views
```

### Database Migrations

```bash
# Create a new migration
andurel migration new create_users_table
andurel m new add_email_to_users  # short alias

# Run migrations
andurel migration up           # Apply all pending migrations
andurel m up                   # short alias

# Rollback migrations
andurel migration down         # Rollback last migration
andurel m down                 # short alias

# Advanced migration commands
andurel migration up-to [version]    # Apply up to specific version
andurel migration down-to [version]  # Rollback to specific version
andurel migration reset              # Reset and reapply all migrations
andurel migration fix                # Fix migration version gaps
```

### SQLC Code Generation

```bash
# Generate type-safe Go code from SQL queries
andurel sqlc generate
andurel s generate     # short alias

# Validate SQL without generating code
andurel sqlc compile
andurel s compile      # short alias
```

### App Management

```bash
# Open interactive database console
andurel app console
andurel a c          # short alias
```

### Project Creation

```bash
# Create a new project with defaults (PostgreSQL + Tailwind CSS)
andurel new myapp

# Customize your stack:
andurel new myapp -d sqlite              # Use SQLite
andurel new myapp -c vanilla             # Use vanilla CSS
andurel new myapp -d sqlite -c vanilla   # Both options

# Add extensions:
andurel new myapp -e auth                # Authentication
andurel new myapp -e email               # Email support
andurel new myapp -e auth,email          # Multiple extensions

# With custom GitHub repo:
andurel new myapp --repo username
```

### LLM Documentation

```bash
# Output comprehensive framework docs for AI assistants
andurel llm
```

## Key Features

### Type Safety Everywhere

Andurel enforces type safety at every layer:

- **SQLC** - Generate type-safe Go code from SQL queries, catch errors at compile time
- **Templ** - Type-safe HTML templates with Go syntax, no runtime template errors
- **Echo** - Strongly-typed HTTP handlers and middleware
- **Validation** - Built-in struct validation with go-playground/validator

### Background Jobs with River

Built-in PostgreSQL-backed job queue for async processing:

```go
// Define a job
type EmailJobArgs struct {
    UserID uuid.UUID
}

func (EmailJobArgs) Kind() string { return "email_job" }

// Enqueue anywhere in your app
insertOnly.Client.Insert(ctx, EmailJobArgs{UserID: userID}, nil)
```

### Database Support

Choose your database when creating a project:

- **PostgreSQL** (default) - Full feature support including River background jobs, ideal for production and concurrent workloads
- **SQLite** - Lightweight, serverless database perfect for simpler applications, prototypes, or when you don't need background jobs

Both databases are fully supported with type-safe SQLC code generation. The choice depends on your application requirements, not just development vs production.

### Live Development Experience

`andurel run` orchestrates three watch processes:

1. **Air** - Rebuilds Go code on changes
2. **Templ** - Recompiles templates on save
3. **Tailwind CSS** - Regenerates styles as you write

### Security Built-in

- **CSRF Protection** - Automatic token validation for non-API routes
- **Session Management** - Encrypted cookie sessions with gorilla/sessions
- **Flash Messages** - Built-in support for temporary user notifications
- **Secure Defaults** - Password hashing, SQL injection prevention, XSS protection

### RESTful Routing

Fluent route builder with type-safe URL generation:

```go
var UserShowPage = newRouteBuilder("show").
    SetNamePrefix("users").
    SetPath("/users/:id").
    SetMethod(http.MethodGet).
    SetCtrl("Users", "Show").
    WithMiddleware(auth.Required).
    RegisterWithID()

// Generate URLs type-safely
url := routes.UserShowPage.URL(userID)
```

### Extensions

Andurel includes optional extensions that add common functionality to your projects:

- **auth** - Complete authentication system with login, registration, password reset, and session management
- **email** - Email sending capabilities with template support, perfect for transactional emails and notifications

Add extensions when creating a project with the `-e` flag:

```bash
andurel new myapp -e auth,email
```

Extensions integrate seamlessly with your chosen database and CSS framework, generating all necessary models, controllers, views, and routes.

### Frontend Interactivity with Datastar

Andurel uses Datastar for hypermedia-driven interactivity, allowing you to build dynamic user interfaces without writing JavaScript:

- Server-side rendering with progressive enhancement
- Reactive updates using HTML attributes
- Form validation and submission without page reloads
- Real-time updates and polling
- Clean separation between backend logic and frontend behavior

Datastar keeps your application logic on the server while providing a smooth, modern user experience.

## Framework Philosophy

Andurel combines Rails conventions with Go's strengths:

1. **Convention over Configuration** - Sensible defaults, minimal setup
2. **Development Speed First** - Code generation eliminates boilerplate
3. **Type Safety** - Compile-time guarantees prevent runtime errors
4. **MVC Architecture** - Clear separation of concerns
5. **RESTful Design** - Standard HTTP patterns and routing

## Development Workflow

### Creating a New Feature

Here's a typical workflow for adding a new resource:

```bash
# 1. Create a migration for your database table
andurel m new create_products_table

# 2. Edit the migration file in database/migrations/
# Add your CREATE TABLE statement

# 3. Run the migration
andurel m up

# 4. Generate the complete resource (model + controller + views + routes)
andurel g resource Product

# 5. Start the development server
andurel run

# Your CRUD interface is ready at http://localhost:8080/products
```

### Making Schema Changes

```bash
# 1. Create a migration
andurel m new add_description_to_products

# 2. Edit the migration file
# Add your ALTER TABLE statement

# 3. Apply the migration
andurel m up

# 4. Refresh the model to regenerate code
andurel g model Product --refresh
```

### Working with Background Jobs

```bash
# 1. Define your job in queue/jobs/
# 2. Implement the worker in queue/workers/
# 3. Enqueue jobs from controllers or models using insertOnly.Client.Insert()
```

## AI-Friendly

Andurel includes comprehensive documentation for AI assistants:

```bash
andurel llm
```

This outputs detailed framework information that helps AI coding assistants understand your project structure, available commands, and conventions. Paste this into your AI assistant's context for better code generation.

## Contributing

Contributions are welcome! Here's how to get started:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run quality checks: `go vet ./...` and `golangci-lint run`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Setup

```bash
git clone https://github.com/mbvlabs/andurel
cd andurel
go mod download
go test ./...
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Tech Stack

Andurel is built on top of excellent open-source projects:

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP router and framework
- **[SQLC](https://sqlc.dev/)** - Type-safe SQL code generation
- **[Templ](https://templ.guide/)** - Type-safe Go templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity
- **[River](https://riverqueue.com/)** - Fast PostgreSQL-backed job queue
- **[pgx](https://github.com/jackc/pgx)** - PostgreSQL driver and toolkit
- **[Air](https://github.com/cosmtrek/air)** - Live reload for Go apps
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS framework (optional)
- **[Cobra](https://cobra.dev/)** - CLI framework

## Acknowledgments

Inspired by Ruby on Rails and its philosophy that developer happiness and productivity matter. Built for developers who want to move fast without sacrificing type safety or code quality.

---
