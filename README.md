<p align="center">
  <img width="400" height="300" alt="andurel-logo" src="https://github.com/user-attachments/assets/8261d514-c070-44c0-a96a-4132045855fc" />
</p>

# Andurel - Rails-like Web Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.24.4%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbvlabs/andurel.svg)](https://pkg.go.dev/github.com/mbvlabs/andurel)
[![Go Report Card](https://goreportcard.com/badge/github.com/mbvlabs/andurel)](https://goreportcard.com/report/github.com/mbvlabs/andurel)
![Coverage](https://img.shields.io/badge/Coverage-0%25-red)

---

**Andurel is a comprehensive web development framework for Go.
It prioritizes development speed.** Inspired by Ruby on Rails, it uses just enough conventions to let you build full-stack web applications incredibly fast.

Join the discord [here](https://discord.gg/TnTBZHvat3)

## Platform Support

Andurel currently supports **Linux** and **macOS** only. Windows is not supported at this time.

If you'd like to help bring Windows support to Andurel, please see [issue #382](https://github.com/mbvlabs/andurel/issues/382) - contributions are welcome!

---

## Why Andurel?

Development speed is everything. Andurel eliminates boilerplate and lets you focus on building features:

- **Instant Scaffolding** - Generate complete CRUD resources with one command
- **Live Reload** - Hot reloading for Go, templates, and CSS with `andurel run` powered by [Shadowfax](https://github.com/mbvlabs/shadowfax)
- **Type Safety Everywhere** - SQLC for SQL, Templ for HTML, Go for logic
- **Batteries Included** - Echo, Datastar, background jobs, sessions, CSRF protection, telemetry, email support, authentication, optional extensions (workflows, docker, aws-ses)
- **Just enough Convention** - Convention over configuration is great to a certain point. Andurel provides just enough sensible defaults that just work and get out of your way.
- **PostgreSQL-Backed** - Built on PostgreSQL with River job queues, pgx driver, and UUID support

The core philosophy around resource generation in andurel, is that it should be a one-time operation that creates everything you need for a fully functional CRUD interface. After that, you can modify and extend the generated code as needed but it's yours to manage going forward.

## Core Technologies

- **[Echo](https://echo.labstack.com/)** - High-performance HTTP framework
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS framework
- **[SQLC](https://sqlc.dev/)** - Type-safe SQL code generation
- **[Templ](https://templ.guide/)** - Type-safe HTML templates
- **[Datastar](https://data-star.dev/)** - Hypermedia-driven frontend interactivity
- **[River](https://riverqueue.com/)** - PostgreSQL-backed background jobs and workflows
- **[OpenTelemetry](https://opentelemetry.io/)** - Built-in observability
- **[PostgreSQL](https://www.postgresql.org/)** - Powerful open-source database with pgx driver and native UUID support
- **[Shadowfax](https://github.com/mbvlabs/shadowfax)** - Andurel specific app runner

## Quick Start

This is subject to change as Andurel is in beta.

I have not documented every feature or command yet, only enough to get you started and trying out the framework.

Once the framework reaches a release candidate, I will provide more comprehensive documentation and guides.

### Installation

```bash
go install github.com/mbvlabs/andurel@v1.0.0-beta.2
```

### Create Your First Project

Andurel gives you choices when creating a new project:

> Note: `--css vanilla` is currently WIP and not properly supported before `v1.0.0`. Use Tailwind for now.

```bash
# Create a new project with defaults (PostgreSQL + Tailwind CSS)
andurel new myapp

# Add extensions for additional features:
andurel new myapp -e docker              # Add Dockerfile for containerization
andurel new myapp -e aws-ses             # Add AWS SES email integration

cd myapp

# Sync tools
andurel tool sync

# Configure environment
cp .env.example .env

# Note: you need to edit .env with your database details

# Apply database migrations
andurel migrate up

# Run the development server (with live reload)
andurel run
```

Your app is now running on `http://localhost:8080`

### Database Lifecycle Commands

Andurel provides commands to manage your database lifecycle:

```bash
# Create the configured database
andurel database create                    # Requires .env to be filled out with DB credentials

# Drop the configured database (prompts for confirmation)
andurel database drop
andurel database drop --force              # Allow dropping system databases

# Drop and recreate the database
andurel database nuke
andurel database nuke --force              # Allow nuking system databases

# Full rebuild: drop, recreate, migrate, and seed
andurel database rebuild
andurel database rebuild --force           # Allow rebuilding system databases
andurel database rebuild --skip-seed       # Skip seeding after migrations
```

### Generate Your First Resource

```bash
# Create a migration and add the columns you need. Resource generation requires
# an `id` primary key (uuid/serial/bigserial/string-supported types). `created_at`
# and `updated_at` are optional but recommended.
andurel migrate new create_products_table

# Create a complete resource with model, controller, views, and routes
andurel generate resource Product
```

This single command creates everything you need for a full CRUD interface.

## CLI Commands

### Run
Starts the development server (hot reload).

```bash
andurel run (alias: r)
```

### New
Scaffolds a new Andurel project.

```bash
andurel new [project-name] --css/-c --extensions/-e
```

### Generate
Code and scaffolding generators.

```bash
andurel generate (aliases: g, gen)
andurel generate model [name] --table-name --skip-factory      (alias: m)
andurel generate controller [model_name] --with-views          (alias: c)
andurel generate view [model_name] --with-controller           (alias: v)
andurel generate resource [name] --table-name                  (alias: r)
```

### Database
Database lifecycle and seed helpers.

```bash
andurel database (aliases: d, db)
andurel database seed
andurel database create
andurel database drop
andurel database nuke
andurel database rebuild
```

### Migrate
Goose migration helpers.

```bash
andurel migrate (aliases: m, mig)
andurel migrate new [name]
andurel migrate up
andurel migrate down
andurel migrate status
andurel migrate fix
andurel migrate reset
andurel migrate up-to [version]
andurel migrate down-to [version]
```

### Query
SQLC query generation helpers.

```bash
andurel query (alias: q)
andurel query generate [table_name]
andurel query refresh [table_name]
andurel query compile
andurel query validate
```

### View
Templ code generation.

```bash
andurel view (alias: v)
andurel view compile
andurel view format
```

### App
App utilities and helpers.

```bash
andurel app (alias: a)
andurel app console    # alias: c
andurel app dblab      # alias: d
andurel app mailpit    # alias: m
```

### Tool
Manage project tools and binaries.

```bash
andurel tool (alias: t)
andurel tool sync
andurel tool set-version <tool> <version>
```

### Extension
Manage project extensions.

```bash
andurel extension (aliases: ext, e)
andurel extension add [extension-name]
andurel extension list    # alias: ls
```

### LLM
Emit framework docs for AI assistants.

```bash
andurel llm
```

### Upgrade
Upgrade framework-managed files.

```bash
andurel upgrade --dry-run
```

### Doctor
Run project diagnostics.

```bash
andurel doctor --verbose
```

## Project Structure

```
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
│   └── sqlc.yaml       # SQLC user overlay config
├── email/               # Email functionality
│   ├── email.go        # Email client and sending logic
│   ├── base_layout.templ    # Base email template layout
│   └── components.templ     # Reusable email components
├── internal/            # Internal framework packages
│   ├── hypermedia/     # Datastar/SSE helpers
│   ├── renderer/       # Template rendering
│   ├── routing/        # Routing utilities
│   ├── server/         # Server configuration
│   └── storage/        # Storage utilities (+ SQLC base/effective config)
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
```

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
- **[Tailwind CSS](https://tailwindcss.com/)** - Utility-first CSS framework
- **[Cobra](https://cobra.dev/)** - CLI framework

## Acknowledgments

Inspired by Ruby on Rails and its philosophy that developer happiness and productivity matter. Built for developers who want to move fast without sacrificing type safety or code quality.

---

### Sites build with Andurel

Here is a collection of sites and projects, I've built with this framework:
- [MBV Blog](https://mortenvistisen.com) | personal blog
- [Master Golang](https://mastergolang.com) | course platform
- [Palantir](https://github.com/mbvlabs/palantir) | open sourced analytics platform (WIP)

If you build something cool with Andurel, let me know and I will add it to the list (or open a PR)!

---

## Author

Created by [Morten Vistisen](https://mortenvistisen.com)

Feel free to reach out to me on:
- [Twitter/X](https://x.com/mbvlabs)
- [Mail](mailto:andurel@mbvlabs.com)

If you have any questions!
