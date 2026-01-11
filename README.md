<p align="center">
  <img width="400" height="300" alt="andurel-logo" src="https://github.com/user-attachments/assets/8261d514-c070-44c0-a96a-4132045855fc" />
</p>

# Andurel - Rails-like Web Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.24.4%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**Andurel is a comprehensive web development framework for Go that prioritizes development speed.** Inspired by Ruby on Rails, it uses just enough conventions to let you build full-stack web applications incredibly fast.

Join the discord [here](https://discord.gg/SsZpxSJX)

## Why Andurel?

Development speed is everything. Andurel eliminates boilerplate and lets you focus on building features:

- **Instant Scaffolding** - Generate complete CRUD resources with one command
- **Live Reload** - Hot reloading for Go, templates, and CSS with `andurel run`
- **Type Safety Everywhere** - SQLC for SQL, Templ for HTML, Go for logic
- **Batteries Included** - Echo, Datastar, background jobs, sessions, CSRF protection, telemetry, email support, authentication, optional extensions (workflows, docker, aws-ses)
- **Just enough Convention** - Convention over configuration is great to a certain point. Andurel provides just enough sensible defaults that just work and get out of your way.
- **PostgreSQL-Backed** - Built on PostgreSQL with River job queues, pgx driver, and UUID support

## Core Technologies

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP framework
- **[SQLC](https://sqlc.dev/)** - Type-safe SQL code generation
- **[Templ](https://templ.guide/)** - Type-safe HTML templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity
- **[River](https://riverqueue.com/)** - PostgreSQL-backed background jobs and workflows
- **[OpenTelemetry](https://opentelemetry.io/)** - Built-in observability
- **[PostgreSQL](https://www.postgresql.org/)** - Powerful open-source database with pgx driver and native UUID support
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

# Or customize your styling:
andurel new myapp -c vanilla             # Use vanilla CSS instead of Tailwind

# Add extensions for additional features:
andurel new myapp -e workflows           # Add workflow orchestration
andurel new myapp -e docker              # Add Dockerfile for containerization
andurel new myapp -e aws-ses             # Add AWS SES email integration
andurel new myapp -e workflows,docker,aws-ses   # Add multiple extensions

cd myapp

# Sync tools
andurel tool sync

# Configure environment
cp .env.example .env
# Edit .env with your database settings

# Run the development server (with live reload)
andurel run
```

Your app is now running at `http://localhost:8080`

### Generate Your First Resource

```bash
# Create a migration
andurel db migration new create_products_table

# Create a complete resource with model, controller, views, and routes
andurel generate resource Product
```

This single command creates everything you need for a full CRUD interface.

## Project Structure

```
myapp/
├── assets/              # Static assets
│   ├── css/            # Compiled CSS files
│   └── js/             # JavaScript files
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
│   └── workflow/       # Workflow orchestration (workflows ext)
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
andurel generate model Product --table products_catalog  # Use custom table name
andurel generate controller User         # Controller only
andurel generate controller User --with-views  # Controller + views
andurel generate view User               # Views only
andurel generate view User --with-controller   # Views + controller

# Short aliases work too
andurel g model User
andurel gen controller Product --with-views
```

### Database Commands

All database-related commands are grouped under `andurel database` (alias `db`):

```bash
# Migrations
andurel db migration new create_users_table
andurel db m new add_email_to_users  # short alias

# Run migrations
andurel db migration up           # Apply all pending migrations
andurel db m up                   # short alias

# Rollback migrations
andurel db migration down         # Rollback last migration
andurel db m down                 # short alias

# Advanced migration commands
andurel db migration up-to [version]    # Apply up to specific version
andurel db migration down-to [version]  # Rollback to specific version
andurel db migration reset              # Reset and reapply all migrations
andurel db migration fix                # Fix migration version gaps
andurel db migration status             # Show current migration version and pending migrations
```

### SQL Query Generation (SQLC)

```bash
# Generate type-safe Go code from SQL queries
andurel db queries generate
andurel db q generate     # short alias

# Validate SQL without generating code
andurel db queries compile
andurel db q compile      # short alias
```

### Database Seeding

```bash
# Run database seeds (edit database/seeds/main.go to customize)
andurel db seed           # Executes go run ./database/seeds
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

# Customize your styling:
andurel new myapp -c vanilla             # Use vanilla CSS

# Add extensions:
andurel new myapp -e workflows           # Workflow orchestration
andurel new myapp -e docker              # Docker containerization
andurel new myapp -e aws-ses             # AWS SES email integration
andurel new myapp -e workflows,docker,aws-ses   # Multiple extensions

# With custom GitHub repo:
andurel new myapp --repo username
```

### LLM Documentation

```bash
# Output comprehensive framework docs for AI assistants
andurel llm
```

## CSS Framework Choice

Choose your styling approach when creating a new project:

```bash
# Tailwind CSS (default) - utility-first CSS with JIT compilation
andurel new myapp

# Vanilla CSS - Open Props CSS variables with custom utilities
andurel new myapp -c vanilla
```

**Tailwind CSS** includes:
- `/css/base.css` - Tailwind imports and plugins (@tailwindcss/typography, @tailwindcss/forms)
- `/css/theme.css` - Custom theme configuration
- Live compilation via tailwindcli during development

**Vanilla CSS** includes:
- `/assets/css/normalize.css` - CSS reset
- `/assets/css/open_props.css` - CSS custom properties
- `/assets/css/buttons.css` - Pre-built button styles
- `/assets/css/style.css` - Main stylesheet

## Flexible Model Generation

Models support flexible table name mapping for working with existing databases or custom naming conventions:

- **Default Convention** - Automatically pluralizes model names to table names (User → users)
- **Custom Table Names** - Override with `--table` flag to map to any table name
- **Legacy Database Support** - Work with existing schemas that don't follow Rails conventions

## Background Jobs

Built-in background job processing with River, PostgreSQL-backed job queue:

```go
// Define a job
type EmailJobArgs struct {
    UserID uuid.UUID
}

func (EmailJobArgs) Kind() string { return "email_job" }

// Enqueue anywhere in your app
insertOnly.Client.Insert(ctx, EmailJobArgs{UserID: userID}, nil)
```

**Queue Management** - RiverUI provides a web interface for monitoring and managing background jobs. Access it during development to view job status, retry failed jobs, and monitor queue performance.

### PostgreSQL-Powered

Andurel is built on PostgreSQL for robust, production-ready applications:

- **River Job Queue** - Fast, reliable background job processing with built-in web UI
- **UUID Support** - Native UUID primary keys for distributed systems
- **pgx Driver** - High-performance PostgreSQL driver with connection pooling
- **Type-Safe Queries** - SQLC generates Go code from your SQL queries
- **Workflow Orchestration** - Complex multi-step processes with the workflows extension

## Live Development Experience

`andurel run` orchestrates three watch processes:

1. **Air** - Rebuilds Go code on changes
2. **Templ** - Recompiles templates on save
3. **Tailwind CSS** - Regenerates styles as you write

## Email Support

Built-in email functionality for sending transactional emails and notifications:

- **Template Support** - Type-safe email templates using Templ
- **Mailpit Integration** - Pre-configured Mailpit client for development testing
- **AWS SES Support** - Optional AWS Simple Email Service integration via the `aws-ses` extension
- **Flexible Configuration** - Easy-to-configure SMTP/SES settings via environment variables
- **Ready to Use** - Email infrastructure included in every new project by default

```go
// Mailpit (default for development)
mailClient := mailpit.NewClient(&cfg.Mailpit)

// AWS SES (production - requires aws-ses extension)
sesClient := awsses.NewClient(ctx, &cfg.AwsSes)
```

## Telemetry and Observability

Built-in OpenTelemetry integration for comprehensive application monitoring:

- **Structured Logging** - JSON-formatted logs with configurable levels and exporters (stdout, file, OTLP)
- **Distributed Tracing** - Request tracing with support for Jaeger, Zipkin, and OTLP exporters
- **Metrics Collection** - Application metrics with Prometheus and OTLP exporters
- **Resource Detection** - Automatic environment and runtime metadata collection
- **Production Ready** - Pre-configured exporters and sensible defaults for immediate use

### RESTful Routing

Fluent route builder with type-safe URL generation:

```go
const UserPrefix = "/users"

var PasswordEdit = routing.NewRouteWithToken(
	"/password/:token/edit",
	"edit_user_password",
	UserPrefix,
)

// Generate URLs type-safely
url := routes.PasswordEdit.URL(token)
```

## Extensions

Andurel includes optional extensions that add common functionality to your projects:

- **workflows** - River-based workflow orchestration for managing complex multi-step background processes with task dependencies
- **docker** - Production-ready Dockerfile and .dockerignore for containerized deployments
- **aws-ses** - AWS Simple Email Service integration for production email delivery with transactional and marketing support

Add extensions when creating a project with the `-e` flag:

```bash
andurel new myapp -e workflows
andurel new myapp -e aws-ses
andurel new myapp -e workflows,docker,aws-ses
```

Extensions integrate seamlessly with your chosen CSS framework, generating all necessary configurations and code.

## Framework Philosophy

Andurel combines Rails conventions with Go's strengths:

1. **Just enough conventions** - Sensible defaults, minimal setup
2. **Development Speed First** - Code generation eliminates boilerplate
3. **Type Safety** - Compile-time guarantees prevent runtime errors
4. **MVC Architecture** - Clear separation of concerns
5. **RESTful Design** - Standard HTTP patterns and routing

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
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity (RC6)
- **[River](https://riverqueue.com/)** - Fast PostgreSQL-backed job queue and workflows
- **[OpenTelemetry](https://opentelemetry.io/)** - Observability framework for logs, traces, and metrics
- **[pgx](https://github.com/jackc/pgx)** - PostgreSQL driver and toolkit
- **[Air](https://github.com/cosmtrek/air)** - Live reload for Go apps
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS framework (optional)
- **[Cobra](https://cobra.dev/)** - CLI framework

## Acknowledgments

Inspired by Ruby on Rails and its philosophy that developer happiness and productivity matter. Built for developers who want to move fast without sacrificing type safety or code quality.

---
