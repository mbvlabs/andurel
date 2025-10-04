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

## Extension Framework (Baseline)

- Slot-aware `TemplateData` is now part of the layout package, giving extensions deterministic hooks for injecting code and metadata during scaffolding.
- Extensions interact with the scaffold flow through a shared context and registry; templates contributed by extensions are rendered with the same helpers as the core layout.

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
