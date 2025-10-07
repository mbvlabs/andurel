# Blueprint Main Section

The Main section of the blueprint allows extensions to contribute to the `cmd/app/main.go` file.

## Capabilities

### Imports
Add import paths beyond controller dependencies:
```go
builder.AddMainImport("myapp/email")
```

### Initializations
Add service initialization code with dependency tracking:
```go
builder.AddMainInitialization(
    "emailSender",           // variable name
    "email.NewMailHog()",    // initialization expression
    "cfg",                   // dependencies (optional, variadic)
)
```

### Background Workers
Start goroutines during application startup:
```go
builder.AddBackgroundWorker(
    "email-worker",                    // name
    "worker.Start(ctx, emailSender)",  // function call
    "emailSender",                     // dependencies (optional, variadic)
)
```

### Pre-Run Hooks
Execute setup code before the server starts:
```go
builder.AddPreRunHook(
    "migrate",
    "if err := migrate(db); err != nil { return err }",
)
```

## Order of Execution

In `cmd/app/main.go`, the order is:
1. Context setup
2. Config loading
3. Database initialization
4. Controller dependency initializations (from Blueprint.Controllers)
5. Main initializations (from Blueprint.Main)
6. Pre-run hooks
7. Controller setup
8. Router setup
9. Background workers start
10. Server start

## Example: Email Extension

```go
func (e Extension) Apply(ctx *extensions.Context) error {
    builder := ctx.Builder()
    moduleName := ctx.Data.GetModuleName()

    // Add import
    builder.AddMainImport(fmt.Sprintf("%s/email", moduleName))

    // Initialize service
    builder.AddMainInitialization("emailSender", "email.NewMailHog()", "cfg")

    // Make available to controllers
    builder.AddControllerDependency("emailSender", "email.Sender")

    return nil
}
```
