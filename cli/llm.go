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
		Short: "Hypermedia-specific LLM documentation",
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

## Core Technologies
- Echo, SQLC, Templ, Datastar, River, OpenTelemetry, PostgreSQL (pgx)

## Key Commands
` + "```bash" + `
andurel run                        # Dev server with live reload
andurel generate resource Product  # CRUD resource
andurel database migration up      # Apply migrations
andurel db migration new create_products_table
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

TODO: Views documentation split-out.`

const llmRouterDocumentation = `# Andurel Framework - Router

TODO: Router documentation split-out.`

const llmHypermediaDocumentation = `# Andurel Framework - Hypermedia

TODO: Hypermedia documentation split-out.`

const llmJobsDocumentation = `# Andurel Framework - Background Jobs

TODO: Jobs documentation split-out.`

const llmConfigDocumentation = `# Andurel Framework - Configuration

TODO: Configuration documentation split-out.`
