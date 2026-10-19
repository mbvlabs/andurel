# Scaffold Pattern Updates (from `../exploration`)

This document summarizes the patterns extracted from `../exploration` and applied across the Andurel scaffold templates (`layout/templates/`), pattern docs (`cli/patterns/`), CLI domain introspection (`cli/domain_inspect.go`), and skills (`skills/andurel/`).

---

## 1. Use `pkg/validation` Directly Instead of `domains.ValidationError`

Previously, `domains/errors.go` defined a duplicate `ValidationError` struct, and both `domains/` (`validationFailure`) and `services/` (`validationError` in `services/service.go`) translated `pkg/validation` errors into `domains.ValidationError`.

Since `pkg/validation` already provides `ValidationErrors`, `.ToMap()`, and `validation.As(err)` (backed by `errors.AsType[ValidationErrors]`), the duplicate layer has been removed.

### `domains/errors.go`
```go
package domains

import "errors"

var (
	ErrNotFound = errors.New("domain: not found")
	ErrConflict = errors.New("domain: conflict")
)
```

### `domains/user.go` & `domains/token.go`
Validation errors from `b.Err()` and `validation.Validate(&entity)` are returned directly without a `validationFailure` wrapper:
```go
b := validation.NewBuilder()
entity.validate(b)
data.PasswordPair.validate(b)
if err := b.Err(); err != nil {
	return User{}, err
}
```
```go
if err := validation.Validate(&entity); err != nil {
	return User{}, err
}
```

### `services/authentication.go`, `services/registration.go`, `services/reset_password.go`
Services return `b.Err()` directly without `validationError(err)`:
```go
b := validation.NewBuilder()
b.Required("email", data.Email)
b.Required("password", data.Password)
if err := b.Err(); err != nil {
	return domains.User{}, err
}
```
And custom field errors in services use `validation.NewBuilder()`:
```go
func invalidVerificationCode() error {
	b := validation.NewBuilder()
	b.Add("code", "invalid", invalidVerificationCodeMessage)

	return errors.Join(ErrInvalidVerificationCode, b.Err())
}
```

### `handlers/auth.go` & `handlers/errors.go`
Handlers unwrap field errors via `validation.As(err)`:
```go
func validationFields(err error) (map[string]string, bool) {
	if validationErr, ok := validation.As(err); ok {
		return validationErr.ToMap(), true
	}

	return nil, false
}
```
```go
func MapError(renderer *inertia.Renderer) anduin.ErrorHandler {
	return func(ctx context.Context, err error) error {
		if validationErr, ok := validation.As(err); ok {
			return renderer.Page(ctx, "Errors/Validation", inertia.Props{}).
				ValidationErrors(validationErr.ToMap()).
				Status(http.StatusUnprocessableEntity).
				Render()
		}

		switch {
		case errors.Is(err, domains.ErrNotFound):
			return renderer.Page(ctx, "Errors/NotFound", inertia.Props{}).
				Status(http.StatusNotFound).
				Render()

		case errors.Is(err, domains.ErrConflict):
			return anduin.Conflict(err)
		}

		return anduin.Error(ctx, http.StatusInternalServerError, err)
	}
}
```

---

## 2. Typed `TokenScope` in `domains/token.go`

Token scopes are owned by the `domains` package as a typed `TokenScope` with `String()` (implementing `fmt.Stringer`), rather than raw untyped string constants scattered across `services/registration.go` and `services/reset_password.go`.

### `domains/token.go`
```go
type TokenScope string

func (ts TokenScope) String() string {
	return string(ts)
}

const (
	UserEmailVerification TokenScope = "user_email_verification"
	UserResetPassword     TokenScope = "user_password_reset"
)
```

All scope-taking methods on `Tokens` accept `TokenScope`:
```go
func (t Tokens) FindByScopeAndHash(
	ctx context.Context,
	secret string,
	scope TokenScope,
	plainToken string,
) (Token, error) {
	hash := HashForStorage(plainToken, secret)

	entity, err := t.queries.GetTokenByScopeAndHash[Token](
		ctx,
		queries.GetTokenByScopeAndHashParams{
			Scope: scope.String(),
			Hash:  hash,
		},
	)
	// ...
}
```

### `services/registration.go` & `services/reset_password.go`
```go
code, err := tokens.ReplaceCode(
	ctx,
	i.tokenSigningKey,
	domains.UserEmailVerification,
	time.Now().Add(24*time.Hour),
	meta,
	user.ID,
)
```
```go
token, err := tokens.Create(
	ctx,
	i.tokenSigningKey,
	domains.UserResetPassword,
	time.Now().Add(1*time.Hour),
	meta,
	user.ID,
)
```

---

## 3. Domain File Layout & Table Inspection

In `domains/user.go` and `domains/token.go`:
- Default scaffolds omit `// andurel:table`; inspection marks conventional names with `table_source: "inferred"`, not as verified schema facts. Custom names and irregular plurals may use the optional annotation, reported as `table_source: "annotation"`. This is inspection metadata, not SQL configuration.
- The persistence receiver struct (`Users`, `Tokens`), constructor (`NewUsers`, `NewTokens`), and `InTx` method appear before their query methods so transaction binding is discoverable immediately.

### `domains/token.go` (before query methods)
```go
// Tokens holds a Transactor because ReplaceCode must be atomic standalone or
// join its caller's transaction when bound with InTx.
type Tokens struct {
	transactor storage.Transactor
	queries    *queries.Queries
}

func NewTokens(db storage.Connection) Tokens {
	return Tokens{transactor: storage.NewTransactor(db), queries: queries.New(db)}
}

func (t Tokens) InTx(tx storage.Tx) Tokens {
	return Tokens{
		transactor: t.transactor.InTx(tx),
		queries:    queries.New(storage.Unwrap(tx)),
	}
}
```

### `domains/user.go` (before query methods)
```go
type Users struct {
	queries *queries.Queries
}

func NewUsers(db storage.Connection) Users {
	return Users{queries: queries.New(db)}
}

func (u Users) InTx(tx storage.Tx) Users {
	return Users{queries: queries.New(storage.Unwrap(tx))}
}
```

---

## 4. Modern Go Idioms & `golines` Formatting

- Replaced `sort.Slice` with `slices.Sort` in `config/app.go`:
```go
slices.Sort(names)
for _, name := range names {
	add(c.Hosts[name])
}
```
- Pre-wrapped long lines across scaffold templates (`handlers/assets.go`, `handlers/auth.go`, `queue/send_transactional_email.go`, `domains/token.go`, `config/cache.go`) to match `golines -m 100` so newly scaffolded projects are immediately formatted cleanly.

---

## 5. Stripped Redundant Explanatory Comments Across Scaffold Templates

Removed comments that merely restate code, while retaining package guidance,
functional directives, and explanations of invariants: verification attempt
counts must commit on invalid codes, row locks serialize attempts, reset failures
roll back token consumption, and queued email commits with its associated write.
`services/service.go` remains the package documentation entry point, not a
validation translation layer.

Known duplicate-email constraint failures map to `domains.ErrConflict` on user
creation and updates, and the shared error mapper returns HTTP 409. Unexpected
database failures retain their original diagnostics.
