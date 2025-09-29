# Authentication Recipe Implementation Plan

## Overview
Implement a complete authentication recipe system for Andurel that can be enabled via `andurel new myapp --recipes auth`. The implementation follows The Copenhagen Book best practices and uses cookie-based sessions with a generic token system for email verification and password reset flows.

## CLI Interface
```bash
andurel new myapp --recipes auth
andurel new myapp --recipes auth,dashboard  # Future: multiple recipes
```

## Architecture

### Database Schema

**users table** (PostgreSQL):
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    email VARCHAR(255) UNIQUE NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    password BYTEA NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE
);
```

**users table** (SQLite):
```sql
CREATE TABLE users (
    id TEXT NOT NULL PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    password BLOB NOT NULL,
    email_verified INTEGER NOT NULL DEFAULT 0
);
```

**tokens table** (PostgreSQL):
```sql
CREATE TABLE tokens (
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    meta_information JSONB NOT NULL
);
```

**tokens table** (SQLite):
```sql
CREATE TABLE tokens (
    id TEXT NOT NULL PRIMARY KEY,
    created_at DATETIME NOT NULL,
    hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    meta_information TEXT NOT NULL
);
```

### Token System

**Token Types (Scopes)**:
- `email_verification` - 24 hour expiry
- `password_reset` - 1 hour expiry

**Token Resources**:
- `users` - Links token to user table

**Token Metadata Structure**:
```json
{
  "resource": "users",
  "resource_id": "uuid-here",
  "scope": "email_verification"
}
```

**Token Functions**:
- `NewHashedToken()` - Generate token, hash with SHA256, store hash, return plain token
- `GetHashedToken()` - Hash provided token, lookup in DB
- `DeleteToken()` - Remove token after use or expiry

### Session Management

Use existing cookie-based session infrastructure (gorilla/sessions). No database-backed sessions.

**Session Data**:
- `user_id` (UUID)
- `authenticated` (bool)
- `created_at` (time)

**Session Lifecycle**:
- Created on successful login
- Destroyed on logout or password change
- Protected by existing CSRF middleware
- HttpOnly, Secure, SameSite=Lax cookies

### Password Security

**Algorithm**: Argon2id
- Memory: 19,456 KiB (19 MB)
- Iterations: 2
- Parallelism: 1
- Salt: 16 bytes (128-bit) cryptographically secure random

**Library**: `golang.org/x/crypto/argon2`

### File Structure
```
layout/
  recipes/
    auth/
      auth.go                           # Recipe orchestration
      templates/
        # Migrations
        migration_001_users_pg.tmpl
        migration_001_users_sqlite.tmpl
        migration_002_tokens_pg.tmpl
        migration_002_tokens_sqlite.tmpl

        # SQLC Queries
        queries_users.tmpl
        queries_tokens.tmpl

        # Models
        models_user.tmpl
        models_token.tmpl

        # Controllers
        controllers_auth.tmpl

        # Middleware
        middleware_auth.tmpl

        # Services
        services_email.tmpl

        # Views
        views_auth_login.tmpl
        views_auth_signup.tmpl
        views_auth_forgot_password.tmpl
        views_auth_reset_password.tmpl
        views_auth_verify_email.tmpl
        views_auth_resend_verification.tmpl

        # Routes
        router_routes_auth.tmpl
```

---

## Step-by-Step Implementation

### Step 1: Create Recipe Package Structure

**Location**: `layout/recipes/auth/`

**Files to Create**:
1. `layout/recipes/auth/auth.go` - Main recipe orchestration file
2. `layout/recipes/auth/templates/` - Directory for all auth templates

**auth.go responsibilities**:
- Export `ProcessAuthRecipe(targetDir, templateData)` function
- Define auth-specific template mappings
- Handle conditional processing based on database type
- Add auth dependencies to existing files (routes, controllers)

**Template mapping structure**:
```go
map[string]string{
    "migration_001_users_pg.tmpl": "database/migrations/001_users.sql",
    "migration_002_tokens_pg.tmpl": "database/migrations/002_tokens.sql",
    "queries_users.tmpl": "database/queries/users.sql",
    // ... etc
}
```

### Step 2: Create Database Migrations

**Files to Create**:
1. `layout/recipes/auth/templates/migration_001_users_pg.tmpl`
2. `layout/recipes/auth/templates/migration_001_users_sqlite.tmpl`
3. `layout/recipes/auth/templates/migration_002_tokens_pg.tmpl`
4. `layout/recipes/auth/templates/migration_002_tokens_sqlite.tmpl`

**Migration naming**: Use goose format with `-- +goose Up` and `-- +goose Down` directives

**PostgreSQL users migration**:
- UUID primary key with `gen_random_uuid()`
- `email` with UNIQUE constraint and index
- `password` as BYTEA
- `email_verified` as BOOLEAN defaulting to FALSE
- `is_admin` as BOOLEAN defaulting to FALSE
- Timestamps with `TIMESTAMPTZ` and `DEFAULT NOW()`

**SQLite users migration**:
- TEXT primary key (store UUID as string)
- Manual UUID generation in application code
- `password` as BLOB
- `email_verified` as INTEGER (0/1)
- `is_admin` as INTEGER (0/1)
- Timestamps as DATETIME (manual handling)

**PostgreSQL tokens migration**:
- UUID primary key
- `hash` as TEXT (stores SHA256 hash)
- `meta_information` as JSONB
- Indexes on `hash` and `expires_at`

**SQLite tokens migration**:
- TEXT primary key (UUID as string)
- `hash` as TEXT
- `meta_information` as TEXT (JSON string)
- Indexes on `hash` and `expires_at`

### Step 3: Create SQLC Queries

**Files to Create**:
1. `layout/recipes/auth/templates/queries_users.tmpl`
2. `layout/recipes/auth/templates/queries_tokens.tmpl`

**User Queries**:
```sql
-- name: CreateUser :one
-- name: GetUserByEmail :one
-- name: GetUserByID :one
-- name: UpdateUserPassword :exec
-- name: MarkEmailVerified :exec
-- name: UpdateUser :exec
```

**Token Queries**:
```sql
-- name: InsertToken :exec
-- name: QueryTokenByHash :one
-- name: DeleteToken :exec
-- name: DeleteExpiredTokens :exec
-- name: DeleteUserTokensByScope :exec
```

**Database-specific handling**:
- Use template conditionals `{{if eq .Database "postgresql"}}` for type differences
- PostgreSQL: Use `pgtype.UUID`, `pgtype.Timestamptz`, `[]byte` for BYTEA
- SQLite: Use `string` for UUIDs, `time.Time` for dates, `[]byte` for BLOB

### Step 4: Create User Model

**File to Create**: `layout/recipes/auth/templates/models_user.tmpl`

**User struct**:
```go
type User struct {
    ID            uuid.UUID
    CreatedAt     time.Time
    UpdatedAt     time.Time
    Email         string
    IsAdmin       bool
    Password      []byte
    EmailVerified bool
}
```

**Functions to implement**:

1. **`HashPassword(password string) ([]byte, error)`**
   - Generate 16-byte cryptographically secure salt
   - Use `argon2.IDKey()` with Copenhagen Book params
   - Return: `salt + hash` as single byte slice (first 16 bytes = salt)

2. **`VerifyPassword(hashedPassword []byte, plainPassword string) bool`**
   - Extract salt from first 16 bytes
   - Hash plain password with extracted salt
   - Constant-time comparison using `subtle.ConstantTimeCompare()`

3. **`CreateUser(ctx, dbtx, email, password string) (User, error)`**
   - Validate email format (use go-playground/validator)
   - Validate password length (min 8, max 72)
   - Hash password
   - Generate UUID (PostgreSQL can use DB default, SQLite needs manual)
   - Insert via SQLC
   - Return User struct

4. **`GetUserByEmail(ctx, dbtx, email string) (User, error)`**
   - Query via SQLC
   - Convert DB types to User struct

5. **`UpdatePassword(ctx, dbtx, userID, newPassword string) error`**
   - Hash new password
   - Update via SQLC
   - Invalidate session (clear cookie)

6. **`MarkEmailVerified(ctx, dbtx, userID uuid.UUID) error`**
   - Set email_verified = true via SQLC

**Validation**:
```go
type CreateUserPayload struct {
    Email    string `validate:"required,email,max=255"`
    Password string `validate:"required,min=8,max=72"`
}
```

### Step 5: Create Token Model

**File to Create**: `layout/recipes/auth/templates/models_token.tmpl`

**Follow the provided pattern exactly**:

**Types**:
```go
type (
    Scope    string
    Resource string
)

const (
    ScopeEmailVerification Scope = "email_verification"
    ScopeResetPassword     Scope = "password_reset"

    ResourceUser Resource = "users"
)
```

**Structs**:
```go
type MetaInformation struct {
    Resource   Resource  `validate:"required" json:"resource"`
    ResourceID uuid.UUID `validate:"required,uuid" json:"resource_id"`
    Scope      Scope     `validate:"required" json:"scope"`
}

type Token struct {
    ID         uuid.UUID
    CreatedAt  time.Time
    Expiration time.Time
    Value      string
    Meta       MetaInformation
}

type NewTokenPayload struct {
    Expiration time.Time       `validate:"required"`
    Meta       MetaInformation `validate:"required" json:"meta"`
}
```

**Functions to implement** (copy from provided example):
1. `generateToken()` - 15-byte base32 encoded
2. `generateHash(token)` - SHA256 hash, hex encoded
3. `NewHashedToken(ctx, dbtx, data)` - Create and return plain token
4. `newToken(ctx, dbtx, expiration, meta, token)` - Internal helper
5. `GetHashedToken(ctx, dbtx, token)` - Hash and lookup
6. `DeleteToken(ctx, dbtx, tokenID)` - Remove token
7. `(t Token) IsValid() bool` - Check expiry

**Database-specific handling**:
- PostgreSQL: Use `pgtype.UUID`, `pgtype.Timestamptz`
- SQLite: Use `uuid.UUID.String()`, `time.Time` directly
- JSON marshaling for meta_information works for both

### Step 6: Create Email Service

**File to Create**: `layout/recipes/auth/templates/services_email.tmpl`

**Interface**:
```go
type EmailService interface {
    SendVerificationEmail(ctx context.Context, email, token string) error
    SendPasswordResetEmail(ctx context.Context, email, token string) error
}
```

**Console Implementation** (for development):
```go
type ConsoleEmailService struct {
    baseURL string
}

func NewConsoleEmailService(baseURL string) *ConsoleEmailService {
    return &ConsoleEmailService{baseURL: baseURL}
}

func (c *ConsoleEmailService) SendVerificationEmail(ctx context.Context, email, token string) error {
    link := c.baseURL + "/verify-email?token=" + token
    slog.InfoContext(ctx, "Verification email",
        "to", email,
        "link", link,
    )
    return nil
}

func (c *ConsoleEmailService) SendPasswordResetEmail(ctx context.Context, email, token string) error {
    link := c.baseURL + "/reset-password?token=" + token
    slog.InfoContext(ctx, "Password reset email",
        "to", email,
        "link", link,
    )
    return nil
}
```

**Production note**: User can swap with SMTP/SendGrid/Postmark implementation

### Step 7: Create Authentication Middleware

**File to Create**: `layout/recipes/auth/templates/middleware_auth.tmpl`

**Middleware 1: RequireAuth**
```go
func RequireAuth() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            sess, err := session.Get("session", c)
            if err != nil {
                return c.Redirect(http.StatusSeeOther, "/login")
            }

            userID, ok := sess.Values["user_id"]
            if !ok {
                return c.Redirect(http.StatusSeeOther, "/login")
            }

            authenticated, ok := sess.Values["authenticated"].(bool)
            if !ok || !authenticated {
                return c.Redirect(http.StatusSeeOther, "/login")
            }

            c.Set("user_id", userID)

            return next(c)
        }
    }
}
```

**Middleware 2: RequireVerifiedEmail**
```go
func RequireVerifiedEmail(db database.XXX) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            userID := c.Get("user_id").(uuid.UUID)

            user, err := models.GetUserByID(c.Request().Context(), db, userID)
            if err != nil {
                return c.Redirect(http.StatusSeeOther, "/resend-verification")
            }

            if !user.EmailVerified {
                return c.Redirect(http.StatusSeeOther, "/resend-verification")
            }

            return next(c)
        }
    }
}
```

**Middleware 3: LoginRateLimiter**
```go
func LoginRateLimiter() echo.MiddlewareFunc {
    rateLimitCacheBuilder, _ := otter.NewBuilder[string, int32](10_000)
    rateLimit, _ := rateLimitCacheBuilder.WithTTL(10 * time.Minute).Build()

    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            ip := c.RealIP()

            hits, found := rateLimit.Get(ip)
            if !found {
                if ok := rateLimit.Set(ip, 1); !ok {
                    return next(c)
                }
                return next(c)
            }

            if hits <= 5 {
                if ok := rateLimit.Set(ip, hits+1); !ok {
                    return next(c)
                }
                return next(c)
            }

            if hits > 5 {
                sse := datastar.NewSSE(c.Response(), c.Request())
                sse.PatchElements(
                    "<p class='text-error-content'>Too many login attempts from your IP address. Please try again later.</p>",
                    datastar.WithSelectorID("loginRes"),
                    datastar.WithModeInner(),
                )
                return nil
            }

            return next(c)
        }
    }
}
```

**Database-specific handling**:
- Use template conditionals for `database.Postgres` vs `database.SQLite` types

### Step 8: Create Authentication Controllers

**File to Create**: `layout/recipes/auth/templates/controllers_auth.tmpl`

**Auth Controller Struct**:
```go
type Auth struct {
    db           database.XXX
    emailService services.EmailService
}

func newAuth(db database.XXX, emailService services.EmailService) Auth {
    return Auth{db, emailService}
}
```

**Controllers to implement**:

1. **`ShowLogin(c echo.Context) error`**
   - Render login form
   - Check if already authenticated, redirect to "/"

2. **`Login(c echo.Context) error`**
   - Parse email/password from form
   - Validate input
   - Lookup user by email
   - Verify password (constant-time)
   - Generic error: "Invalid email or password"
   - Create session on success
   - Redirect to "/"

3. **`ShowSignup(c echo.Context) error`**
   - Render signup form
   - Check if already authenticated, redirect to "/"

4. **`Signup(c echo.Context) error`**
   - Parse email/password/password_confirm
   - Validate passwords match
   - Create user
   - Generate email verification token (24h expiry)
   - Send verification email
   - Create session (authenticated but not verified)
   - Redirect to "/resend-verification" with message

5. **`Logout(c echo.Context) error`**
   - Destroy session
   - Redirect to "/login"

6. **`VerifyEmail(c echo.Context) error`**
   - Parse token from query string
   - Lookup hashed token
   - Validate token scope and expiry
   - Mark user email as verified
   - Delete token
   - Redirect to "/" with success flash

7. **`ShowResendVerification(c echo.Context) error`**
   - Render resend verification form
   - Show current user email

8. **`ResendVerification(c echo.Context) error`**
   - Get user from session
   - Delete old verification tokens for user
   - Generate new verification token
   - Send verification email
   - Show success message

9. **`ShowForgotPassword(c echo.Context) error`**
   - Render forgot password form

10. **`ForgotPassword(c echo.Context) error`**
    - Parse email from form
    - Lookup user (don't reveal if exists)
    - Generate password reset token (1h expiry)
    - Send password reset email
    - Always show success message (prevent enumeration)

11. **`ShowResetPassword(c echo.Context) error`**
    - Parse token from query string
    - Validate token exists and not expired
    - Render reset password form

12. **`ResetPassword(c echo.Context) error`**
    - Parse token, new password, confirm password
    - Validate token
    - Validate passwords match
    - Update user password
    - Delete token
    - Destroy all sessions for user
    - Redirect to "/login" with success flash

**Error Handling**:
- Use Datastar SSE for form errors
- Generic messages to prevent user enumeration
- Flash messages for success/error flows

**Session Management**:
```go
sess, _ := session.Get("session", c)
sess.Values["user_id"] = user.ID
sess.Values["authenticated"] = true
sess.Values["created_at"] = time.Now()
sess.Save(c.Request(), c.Response())
```

### Step 9: Create Authentication Views

**Files to Create**:
1. `layout/recipes/auth/templates/views_auth_login.tmpl`
2. `layout/recipes/auth/templates/views_auth_signup.tmpl`
3. `layout/recipes/auth/templates/views_auth_forgot_password.tmpl`
4. `layout/recipes/auth/templates/views_auth_reset_password.tmpl`
5. `layout/recipes/auth/templates/views_auth_verify_email.tmpl`
6. `layout/recipes/auth/templates/views_auth_resend_verification.tmpl`

**Common Elements**:
- All forms use POST method
- CSRF token via `{{.Ctx.Get("csrf")}}`
- Datastar attributes for SSE responses
- Error display div with id for targeting
- Flash message display
- Consistent styling with existing views

**Login View**:
```templ
templ Login() {
    @layout.Base() {
        <div class="container mx-auto max-w-md mt-8">
            <h1 class="text-3xl font-bold mb-6">Login</h1>

            <form method="POST" action="/login">
                <input type="hidden" name="_csrf" value={ ctx.Value("csrf").(string) } />

                <div class="form-control mb-4">
                    <label class="label" for="email">Email</label>
                    <input type="email" name="email" id="email" class="input input-bordered" required />
                </div>

                <div class="form-control mb-4">
                    <label class="label" for="password">Password</label>
                    <input type="password" name="password" id="password" class="input input-bordered" required />
                </div>

                <div id="loginRes" class="mb-4"></div>

                <button type="submit" class="btn btn-primary w-full">Login</button>
            </form>

            <div class="mt-4 text-center">
                <a href="/forgot-password" class="link">Forgot password?</a>
            </div>

            <div class="mt-4 text-center">
                <span>Don't have an account?</span>
                <a href="/signup" class="link">Sign up</a>
            </div>
        </div>
    }
}
```

**Signup View**:
- Email field
- Password field (min 8 chars)
- Confirm password field
- Submit button
- Link to login
- Error display div

**Forgot Password View**:
- Email field only
- Submit button
- Link back to login

**Reset Password View**:
- Hidden token field
- New password field
- Confirm password field
- Submit button

**Verify Email Success View**:
- Success message
- Link to home/dashboard

**Resend Verification View**:
- Display current user email
- Resend button
- Message about checking spam

### Step 10: Create Authentication Routes

**File to Create**: `layout/recipes/auth/templates/router_routes_auth.tmpl`

**Route Definitions**:
```go
package routes

import "net/http"

const authNamePrefix = "auth"

var authRoutes = []Route{
    {
        Name:         authNamePrefix + ".show_login",
        Path:         "/login",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "ShowLogin",
    },
    {
        Name:         authNamePrefix + ".login",
        Path:         "/login",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "Login",
        Middleware:   []func(next echo.HandlerFunc) echo.HandlerFunc{
            middleware.LoginRateLimiter(),
        },
    },
    {
        Name:         authNamePrefix + ".show_signup",
        Path:         "/signup",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "ShowSignup",
    },
    {
        Name:         authNamePrefix + ".signup",
        Path:         "/signup",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "Signup",
    },
    {
        Name:         authNamePrefix + ".logout",
        Path:         "/logout",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "Logout",
    },
    {
        Name:         authNamePrefix + ".verify_email",
        Path:         "/verify-email",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "VerifyEmail",
    },
    {
        Name:         authNamePrefix + ".show_resend_verification",
        Path:         "/resend-verification",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "ShowResendVerification",
    },
    {
        Name:         authNamePrefix + ".resend_verification",
        Path:         "/resend-verification",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "ResendVerification",
    },
    {
        Name:         authNamePrefix + ".show_forgot_password",
        Path:         "/forgot-password",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "ShowForgotPassword",
    },
    {
        Name:         authNamePrefix + ".forgot_password",
        Path:         "/forgot-password",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "ForgotPassword",
    },
    {
        Name:         authNamePrefix + ".show_reset_password",
        Path:         "/reset-password",
        Method:       http.MethodGet,
        Handler:      "Auth",
        HandleMethod: "ShowResetPassword",
    },
    {
        Name:         authNamePrefix + ".reset_password",
        Path:         "/reset-password",
        Method:       http.MethodPost,
        Handler:      "Auth",
        HandleMethod: "ResetPassword",
    },
}
```

### Step 11: Update Existing Templates to Support Auth Recipe

**Templates to Modify**:

1. **`layout/templates/router_routes_routes.tmpl`**
   - Add conditional import and append of authRoutes
   ```go
   {{if .WithAuth}}
   r = append(r, authRoutes...)
   {{end}}
   ```

2. **`layout/templates/controllers_controller.tmpl`**
   - Add Auth field to Controllers struct
   - Initialize Auth controller in newControllers
   ```go
   type Controllers struct {
       Pages  Pages
       Assets Assets
       Api    Api
       {{if .WithAuth}}
       Auth   Auth
       {{end}}
   }
   ```

3. **`layout/templates/cmd_app_main.tmpl`**
   - Add EmailService initialization
   ```go
   {{if .WithAuth}}
   emailService := services.NewConsoleEmailService(cfg.App.BaseURL)
   {{end}}
   ```

4. **`layout/templates/config_config.tmpl`**
   - Add BaseURL to app config
   ```go
   type app struct {
       Port    string
       BaseURL string  // Add this
   }
   ```

5. **`layout/templates/env.tmpl`**
   - Add BASE_URL environment variable
   ```
   BASE_URL=http://localhost:3000
   ```

### Step 12: Update Recipe Orchestration in auth.go

**File**: `layout/recipes/auth/auth.go`

**Responsibilities**:
1. Define template mappings for all auth templates
2. Process each template with database-specific paths
3. Handle migration numbering (001, 002)
4. Export `ProcessAuthRecipe()` function

**Implementation**:
```go
package auth

import (
    "fmt"
    "path/filepath"

    "github.com/mbvlabs/andurel/layout"
)

func ProcessAuthRecipe(targetDir string, data layout.TemplateData) error {
    mappings := getTemplateMappings(data.Database)

    for tmplFile, targetPath := range mappings {
        fullTmplPath := filepath.Join("auth", "templates", tmplFile)
        if err := layout.ProcessTemplateFromRecipe(targetDir, fullTmplPath, targetPath, data); err != nil {
            return fmt.Errorf("failed to process auth template %s: %w", tmplFile, err)
        }
    }

    return nil
}

func getTemplateMappings(database string) map[string]string {
    migrationSuffix := "_pg.tmpl"
    if database == "sqlite" {
        migrationSuffix = "_sqlite.tmpl"
    }

    return map[string]string{
        // Migrations
        "migration_001_users" + migrationSuffix:  "database/migrations/001_users.sql",
        "migration_002_tokens" + migrationSuffix: "database/migrations/002_tokens.sql",

        // Queries
        "queries_users.tmpl":  "database/queries/users.sql",
        "queries_tokens.tmpl": "database/queries/tokens.sql",

        // Models
        "models_user.tmpl":  "models/user.go",
        "models_token.tmpl": "models/token.go",

        // Services
        "services_email.tmpl": "services/email.go",

        // Middleware
        "middleware_auth.tmpl": "router/middleware/auth.go",

        // Controllers
        "controllers_auth.tmpl": "controllers/auth.go",

        // Views
        "views_auth_login.tmpl":              "views/auth/login.templ",
        "views_auth_signup.tmpl":             "views/auth/signup.templ",
        "views_auth_forgot_password.tmpl":    "views/auth/forgot_password.templ",
        "views_auth_reset_password.tmpl":     "views/auth/reset_password.templ",
        "views_auth_verify_email.tmpl":       "views/auth/verify_email.templ",
        "views_auth_resend_verification.tmpl": "views/auth/resend_verification.templ",

        // Routes
        "router_routes_auth.tmpl": "router/routes/auth.go",
    }
}
```

### Step 13: Update Main Scaffold Function

**File**: `layout/layout.go`

**Changes**:

1. Add `recipes` parameter to `Scaffold()`:
```go
func Scaffold(targetDir, projectName, repo, database string, recipes []string) error {
    // ...
    templateData := TemplateData{
        ProjectName:          projectName,
        ModuleName:           moduleName,
        Database:             database,
        SessionKey:           generateRandomHex(64),
        SessionEncryptionKey: generateRandomHex(32),
        TokenSigningKey:      generateRandomHex(32),
        PasswordSalt:         generateRandomHex(16),
        WithAuth:             slices.Contains(recipes, "auth"),
    }
    // ...
}
```

2. Add recipe processing after core templates:
```go
if templateData.WithAuth {
    fmt.Print("Processing auth recipe...\n")
    if err := auth.ProcessAuthRecipe(targetDir, templateData); err != nil {
        return fmt.Errorf("failed to process auth recipe: %w", err)
    }
}
```

3. Update `TemplateData` struct:
```go
type TemplateData struct {
    ProjectName          string
    ModuleName           string
    Database             string
    SessionKey           string
    SessionEncryptionKey string
    TokenSigningKey      string
    PasswordSalt         string
    WithAuth             bool  // Add this
}
```

4. Export `ProcessTemplateFromRecipe()` helper:
```go
func ProcessTemplateFromRecipe(targetDir, templateFile, targetPath string, data TemplateData) error {
    // Same as processTemplate but reads from recipes embed.FS
}
```

### Step 14: Update CLI Command

**File**: `cli/new_project.go`

**Changes**:

1. Add `--recipes` flag:
```go
projectCmd.Flags().StringSliceP("recipes", "R", []string{}, "Recipes to include (comma-separated: auth, etc.)")
```

2. Parse recipes flag:
```go
recipes, err := cmd.Flags().GetStringSlice("recipes")
if err != nil {
    return err
}
```

3. Validate recipes:
```go
validRecipes := []string{"auth"}
for _, recipe := range recipes {
    if !slices.Contains(validRecipes, recipe) {
        return fmt.Errorf("invalid recipe: %s - valid options are: %s", recipe, strings.Join(validRecipes, ", "))
    }
}
```

4. Pass recipes to Scaffold:
```go
if err := layout.Scaffold(basePath, projectName, repo, database, recipes); err != nil {
    return err
}
```

5. Update success message:
```go
if slices.Contains(recipes, "auth") {
    fmt.Printf("  Auth enabled - visit /login or /signup\n")
}
```

### Step 15: Add Dependencies

**Required Go packages**:
- `golang.org/x/crypto/argon2` - Password hashing
- `github.com/google/uuid` - UUID generation
- `github.com/maypok86/otter` - Rate limit caching

**Add to template**: These should be imported in the generated code, go mod tidy will handle installation.

### Step 16: Testing Checklist

After implementation, test the following flows:

**Signup Flow**:
1. Visit /signup
2. Enter email and password
3. User created with email_verified=false
4. Verification token generated
5. Console logs verification link
6. Session created

**Email Verification Flow**:
1. Click verification link from console
2. Token validated
3. User email_verified set to true
4. Token deleted
5. Redirect to home with flash message

**Login Flow**:
1. Visit /login
2. Enter valid credentials
3. Password verified with constant-time comparison
4. Session created
5. Redirect to home

**Login Rate Limiting**:
1. Attempt login 6+ times from same IP
2. After 5 attempts, see rate limit message
3. Wait 10 minutes, rate limit clears

**Forgot Password Flow**:
1. Visit /forgot-password
2. Enter email
3. Reset token generated (1h expiry)
4. Console logs reset link
5. Generic success message (no enumeration)

**Reset Password Flow**:
1. Click reset link from console
2. Token validated
3. Enter new password
4. Password updated
5. Token deleted
6. All sessions destroyed
7. Redirect to login

**Middleware Protection**:
1. Add RequireAuth middleware to a route
2. Visit route without authentication
3. Redirect to /login
4. Login and revisit route
5. Access granted

**Database Differences**:
1. Test entire flow on PostgreSQL
2. Test entire flow on SQLite
3. Verify migrations run correctly
4. Verify SQLC types handled correctly

---

## Security Considerations

✓ **Argon2id** with Copenhagen Book parameters
✓ **Constant-time** password comparison
✓ **SHA256 hashing** for tokens
✓ **32-byte tokens** (256-bit entropy)
✓ **Rate limiting** on login (5 attempts/10min)
✓ **CSRF protection** (existing middleware)
✓ **HttpOnly, Secure, SameSite** cookies
✓ **Token expiry** (24h verification, 1h reset)
✓ **Generic error messages** (prevent enumeration)
✓ **Session regeneration** on login
✓ **Session invalidation** on password change
✓ **Database indexes** on lookups
✓ **Cascade deletes** for referential integrity
✓ **No session fixation** (new session on auth)

---

## Future Enhancements

1. **OAuth Integration** - GitHub, Google, etc.
2. **Two-Factor Authentication** - TOTP/SMS
3. **Remember Me** - Extended session duration
4. **Account Lockout** - After N failed attempts
5. **Password History** - Prevent reuse
6. **Admin Recipe** - User management interface
7. **Audit Logging** - Track auth events
8. **Email Templates** - HTML email designs
9. **Magic Links** - Passwordless authentication
10. **Session Management UI** - View/revoke active sessions

---

## Implementation Order Summary

1. Create recipe package structure (`auth.go`)
2. Create database migrations (users, tokens)
3. Create SQLC queries (users, tokens)
4. Create user model (hashing, verification)
5. Create token model (generation, validation)
6. Create email service (console implementation)
7. Create auth middleware (RequireAuth, rate limiting)
8. Create auth controllers (login, signup, reset, verify)
9. Create auth views (templ templates)
10. Create auth routes (route definitions)
11. Update existing templates (conditional auth support)
12. Update recipe orchestration (`auth.go`)
13. Update main scaffold function (`layout.go`)
14. Update CLI command (`new_project.go`)
15. Add dependencies
16. Test all flows

---

## Questions Before Starting?

- Should password requirements be configurable?
No, keep it as now with the validation rule
- Default session duration (currently 30 days)?
yes, keep it as is
- Token cleanup job - manual or automatic?
no, will be later
- Should we add a "remember me" checkbox on login?
yes, but only extend session duration
- Navigation menu updates - automatic or manual?
no, user will handle
