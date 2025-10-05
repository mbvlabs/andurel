<p align="center">
  <img width="400" height="300" alt="andurel-logo" src="https://github.com/user-attachments/assets/8261d514-c070-44c0-a96a-4132045855fc" />
</p>

# Andurel - Rails-like Web Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.24.4%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Andurel is a comprehensive web development framework for Go, inspired by Ruby on Rails. It provides a productive development environment with code generation, database migrations, MVC architecture, and modern web development tools.

## Features

- **Rapid Development** - Rails-like conventions and code generators
- **Full Stack Framework** - MVC architecture with routing, middleware, and templating
- **Code Generation** - Automatic generation of models, controllers, views, and resources
- **Database Integration** - PostgreSQL support with migrations and SQLC
- **Type-Safe Templates** - Built on Go's templ for compile-time HTML safety
- **Modern Web Stack** - Echo framework, sessions, middleware, and more
- **Testing Support** - Built-in testing utilities and golden file testing
- **Project Scaffolding** - Complete project setup with best practices

## Quick Start

### Installation

```bash
go install github.com/mbvlabs/andurel@latest
```

### Create a New Project

```bash
andurel new myapp
cd myapp
just setup linux  # or mac/windows
cp .env.example .env
# Edit .env with your database configuration
just run
```

## Extension Framework

The extension workflow lives in `layout/layout.go` and executes the same sequence for every scaffold run:

1. **Bootstrap template state** – `Scaffold` creates a `TemplateData` value populated with project metadata and randomly generated secrets. The struct (defined in `layout/template_data.go`) also owns the scaffold blueprint, giving extensions a structured builder for composing imports, dependencies, routes, and more.
2. **Register built-ins** – `registerBuiltinExtensions` runs once per process and wires in extensions shipped with Andurel. Behind the scenes each extension calls `extensions.Register`, which keeps the registry synchronized and prevents duplicate names.
3. **Resolve requested extensions** – `resolveExtensions` validates user input, trims whitespace, de-duplicates names, and fetches the concrete `extensions.Extension` implementations. If the name is unknown the function reports an error that includes the list of registered extensions.
4. **Apply extensions** – For each resolved extension, `Scaffold` builds an `extensions.Context` containing:
   - `TargetDir`: the directory being scaffolded.
   - `Data`: the shared `TemplateData` instance. Because it is passed by reference, blueprint contributions from one extension are visible to the next.
   - `ProcessTemplate`: a closure that renders files through `ProcessTemplateFromRecipe`, allowing extensions to emit templates from their own embedded `layout/extensions/*/templates` directories.
   - `AddPostStep`: a mechanism for deferring additional work (such as formatting, migrations, or asset generation) until after the scaffold is rendered.

   Each extension’s `Apply` method can create files, enrich the blueprint, or schedule post steps.
5. **Re-render blueprint templates** – After every extension finishes, `rerenderBlueprintTemplates` replays the base templates that consume structured blueprint data, ensuring controller wiring and imports stay in sync.
6. **Run post steps and formatters** – Deferred callbacks execute next, followed by the templ/go fmt/go mod tidy pipeline. Both base templates and extension recipes are rendered via `renderTemplate`, which attaches a small helper func map (currently `lower`) for common template needs.

The contract that extensions implement lives in `layout/extensions/extension.go`. It defines the `TemplateData` interface, the `Context` type that flows through `Apply`, and an embedded filesystem (`//go:embed */templates/*.tmpl`) that collects template recipes next to their extension code. This shared surface area allows downstream packages to plug in without import cycles while giving them access to the same rendering facilities as the core layout.

In short, extensions collaborate around a single `TemplateData` instance and its shared blueprint, render customized templates through the provided context, and schedule any finishing tasks as post steps. The default scaffold remains unchanged when no extensions are supplied, but the pipeline is ready for richer behaviours as new extensions are introduced.

## Architecture

### Project Structure

```
myapp/
├── assets/           # Static assets (CSS, JS, images)
├── cmd/
│   ├── app/         # Main application entry point
│   └── migrate/     # Database migration tool
├── config/          # Application configuration
├── controllers/     # Request handlers and business logic
├── css/            # Tailwind CSS and custom styles
├── database/
│   ├── migrations/  # SQL migration files
│   └── queries/     # SQLC query definitions
├── models/          # Data models and database interactions
├── router/          # HTTP routing and middleware
├── views/           # HTML templates (templ)
└── justfile         # Build and development commands
```

## CLI Commands

### Project Management

```bash
# Create a new project
andurel new <project-name> [--repo username]

# Example with GitHub repo
andurel new myapp --repo mbvlabs
```

### Code Generation

```bash
# Generate a complete resource (model + controller + views)
andurel generate resource User users

# Generate individual components
andurel generate model User users [--refresh]
andurel generate controller User [--with-views]
andurel generate view User [--with-controller]

# Aliases
andurel g model User users
andurel g model User users --refresh
andurel g controller User --with-views
andurel g controller User
andurel generate view User 
andurel generate view User --with-controller
andurel g resource Product products
```

## Framework Philosophy

Andurel follows Rails-inspired conventions:

1. **Convention over Configuration** - Sensible defaults reduce boilerplate
2. **DRY (Don't Repeat Yourself)** - Code generation eliminates repetition  
3. **MVC Architecture** - Clear separation of concerns
4. **RESTful Design** - Standard HTTP verb mapping to controller actions
5. **Type Safety** - Leverage Go's type system for compile-time guarantees

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Ensure tests pass: `go test ./...`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by Ruby on Rails and its productive development philosophy
- Built on the shoulders of excellent Go libraries and tools
- Community feedback and contributions

---
