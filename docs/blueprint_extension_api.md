# Blueprint Extension API

## Overview

The Blueprint Extension API provides a structured, type-safe way for extensions to contribute to scaffold generation. It replaces the previous slot-based system with a typed builder API that ensures uniqueness, canonical ordering, and conflict-free composition of multiple extensions.

## Key Concepts

### Blueprint

A `Blueprint` is a structured representation of the entire scaffold configuration. It contains sections for:

- **Controllers**: Imports, dependencies, fields, and constructor initializations
- **Routes**: Route definitions and imports
- **Models**: Model definitions and imports
- **Config**: Configuration fields and environment variables
- **Migrations**: Database migrations

### Builder

The `Builder` provides a typed API for adding elements to a blueprint. It:

- Enforces uniqueness (no duplicate dependencies, fields, etc.)
- Maintains deterministic ordering
- Supports method chaining
- Automatically manages insertion order

### Extensions

Extensions use the Builder API through the `Context.Builder()` method to make structured contributions that automatically merge without conflicts.

## Using the Blueprint API

### Basic Extension Structure

```go
package myextension

import (
	"fmt"
	"github.com/mbvlabs/andurel/layout/extensions"
)

type Extension struct{}

func (e Extension) Name() string {
	return "my-extension"
}

func (e Extension) Apply(ctx *extensions.Context) error {
	// Get the builder
	builder := ctx.Builder()
	if builder == nil {
		return fmt.Errorf("my-extension: builder is nil")
	}

	// Use builder methods to add to the scaffold
	builder.AddImport("fmt")
	builder.AddControllerDependency("myService", "service.Service")
	builder.AddControllerField("MyFeature", "MyFeature")
	builder.AddConstructor("myFeature", "newMyFeature(myService)")

	// Create extension-specific files
	err := ctx.ProcessTemplate(
		"my-extension/templates/my_feature.tmpl",
		"controllers/my_feature.go",
		nil,
	)
	if err != nil {
		return fmt.Errorf("my-extension: failed to create file: %w", err)
	}

	return nil
}

func Register() error {
	return extensions.Register(Extension{})
}
```

Call `myextension.Register()` during application startup (for example inside `registerBuiltinExtensions`).

### Builder Methods

#### Controller Section

```go
// Add an import to the controllers package
builder.AddImport("github.com/foo/bar")

// Add a dependency parameter to the controller constructor
builder.AddControllerDependency("db", "database.DB")

// Add a dependency that requires an initializer and its import path
builder.AddControllerDependencyWithInitAndImport(
	"queue",
	"queue.Queue",
	"queue.NewInMemoryQueue()",
	"github.com/example/project/queue",
)

// Add a field to the Controllers struct
builder.AddControllerField("MyController", "MyController")

// Add a constructor initialization statement
builder.AddConstructor("myController", "newMyController(db)")
```

#### Routes Section

```go
// Add an import to the routes package
builder.AddRouteImport("middleware")

// Note: Route addition API is available but not demonstrated in current examples
```

#### Models Section

```go
// Add an import to the models package
builder.AddModelImport("time")

// Note: Model addition API is available but not demonstrated in current examples
```

#### Config Section

```go
// Add a field to the config struct
builder.AddConfigField("Port", "int")

// Add an environment variable mapping
builder.AddEnvVar("PORT", "Port", "8080")
```

### Composition Example

Two extensions can contribute to the same scaffold without conflicts:

**Extension 1: Queue Worker**
```go
builder.AddImport(fmt.Sprintf("%s/queue", moduleName))
builder.AddControllerDependency("queue", "queue.Queue")
builder.AddControllerField("QueueWorker", "QueueWorker")
builder.AddConstructor("queueWorker", "newQueueWorker(queue)")
```

**Extension 2: Email Service**
```go
builder.AddImport(fmt.Sprintf("%s/email", moduleName))
builder.AddControllerDependency("emailService", "email.Service")
builder.AddControllerField("EmailService", "EmailService")
builder.AddConstructor("emailService", "newEmailService(emailService)")
```

**Result**: Both extensions' contributions are merged into a single controller with:
- All imports combined (deduplicated)
- Both dependencies in the constructor
- Both fields in the struct
- Both constructors in the initializer

The order is deterministic based on insertion sequence.

## Template Integration

Templates can access blueprint data directly:

```go
// controllers_controller.tmpl
import (
{{- range .Blueprint.Controllers.Imports.SortedItems}}
	"{{.}}"
{{- end}}
)

type Controllers struct {
{{- range .Blueprint.Controllers.SortedFields}}
	{{.Name}} {{.Type}}
{{- end}}
}

func New(
{{- range $i, $dep := .Blueprint.Controllers.SortedDependencies}}
	{{$dep.Name}} {{$dep.Type}},
{{- end}}
) (Controllers, error) {
{{- range .Blueprint.Controllers.SortedConstructors}}
	{{.VarName}} := {{.Expression}}
{{- end}}

	return Controllers{
{{- range .Blueprint.Controllers.SortedFields}}
		{{.Name}}: {{.Name | lower}},
{{- end}}
	}, nil
}
```

## Creating Extensions

### 1. Create Extension Directory

```bash
mkdir -p layout/extensions/my-extension/templates
```

### 2. Implement Extension Interface

Create `layout/extensions/my-extension/extension.go`:

```go
package myextension

import (
	"fmt"
	"github.com/mbvlabs/andurel/layout/extensions"
)

type Extension struct{}

func (e Extension) Name() string {
	return "my-extension"
}

func (e Extension) Apply(ctx *extensions.Context) error {
	builder := ctx.Builder()

	// Add your contributions
	builder.AddImport("...")
	builder.AddControllerDependency("...", "...")

	// Create files if needed
	err := ctx.ProcessTemplate("...", "...", nil)

	return err
}

func Register() error {
	return extensions.Register(Extension{})
}
```

### 3. Register Extension

Call the extension's `Register` function from `layout/layout.go` (typically inside `registerBuiltinExtensions`):

```go
import (
	myextension "github.com/mbvlabs/andurel/layout/extensions/my-extension"
)

func registerBuiltinExtensions() error {
	registerBuiltinOnce.Do(func() {
		if err := myextension.Register(); err != nil {
			registerBuiltinErr = err
		}
	})

	return registerBuiltinErr
}
```

### 4. Use Extension

```bash
andurel new myproject --extensions my-extension
```

## Testing Extensions

Create a test that scaffolds with multiple extensions:

```go
func TestExtensionComposition(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Scaffold with extensions
	err := layout.Scaffold(
		tmpDir,
		"testproject",
		"",
		"sqlite",
		[]string{"extension-1", "extension-2"},
	)

	// Verify merged output
	controller, err := os.ReadFile(filepath.Join(tmpDir, "controllers/controller.go"))
	// Assert both extensions' contributions are present
}
```

## Benefits

1. **Type Safety**: Structured types instead of raw strings
2. **Uniqueness**: Automatic deduplication by name
3. **Deterministic Ordering**: Predictable output based on insertion order
4. **Conflict-Free Composition**: Multiple extensions merge automatically
5. **Clear Contracts**: Well-defined API with typed builders

## Reference Implementation

See the included sample extensions for complete examples:

- `layout/extensions/queue-worker/` - Adds queue worker functionality
- `layout/extensions/email-service/` - Adds email service functionality

These demonstrate:
- Using the Builder API
- Creating extension-specific files
- Composing without conflicts
