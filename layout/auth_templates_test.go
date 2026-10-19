package layout

import (
	"io/fs"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/v2/layout/templates"
)

// TestGeneratedAuthTemplatesBindVerificationToSession guards the S1 fix: email
// verification codes must be bound to the pending user carried in the session
// and must lock out after a bounded number of failed attempts.
func TestGeneratedAuthTemplatesBindVerificationToSession(t *testing.T) {
	assertTemplateContains(t, "services_registration.tmpl",
		"FindByUserAndScope",
		"maxVerificationAttempts",
		"attempts, err := tokens.IncrementAttempts(ctx, token.ID)",
		"verificationAttemptAllowed",
		"verificationLocked",
		"Invalid or expired code",
		"func (i Identity) ResendEmailVerification(",
	)
	assertTemplateMissing(t, "services_registration.tmpl", "FindByScopeAndHash")

	assertTemplateContains(t, "domains_token.tmpl",
		"type TokenScope string",
		"Scope:  string(scope),",
		"func (t Tokens) FindByUserAndScope(",
		"func (t Tokens) IncrementAttempts(ctx context.Context, id uuid.UUID) (int32, error)",
		"func (t Tokens) DestroyByUserAndScope(",
		"GetTokenByUserAndScope",
	)
	tokenTemplate := strings.Join(strings.Fields(readLayoutTemplate(t, "domains_token.tmpl")), " ")
	for _, declaration := range []string{
		`UserEmailVerification TokenScope = "user_email_verification"`,
		`UserResetPassword TokenScope = "user_password_reset"`,
	} {
		if !strings.Contains(tokenTemplate, declaration) {
			t.Errorf("domains_token.tmpl missing %q", declaration)
		}
	}
	assertTemplateMissing(t, "domains_token.tmpl", "validationFailure")
	assertTemplateMissing(t, "domains_user.tmpl", "validationFailure")
	assertTemplateMissing(t, "domains_errors.tmpl", "ValidationError")
	assertTemplateMissing(t, "domains_errors.tmpl", "ErrViolation")
	assertTemplateMissing(t, "handlers_errors.tmpl", "ErrViolation")
	assertTemplateMissing(t, "domains_token.tmpl", "func (ts TokenScope) ToString(")
	assertTemplateMissing(t, "domains_token.tmpl", "func (ts TokenScope) String(")
	assertTemplateContains(t, "handlers_auth.tmpl", "validation.As(err)")
	assertTemplateContains(t, "handlers_errors.tmpl", "validation.As(err)")
	assertTemplateContains(t, "domains_queries_token.tmpl",
		"-- name: GetTokenByUserAndScope :one",
		"-- name: IncrementTokenAttempts :one",
		"RETURNING attempts;",
		"-- name: DeleteTokensByUserAndScope :exec",
	)
	assertTemplateContains(t, "database_migrations_tokens.tmpl",
		"user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE",
		"attempts",
		"ON tokens (user_id, scope, created_at DESC)",
	)

	assertTemplateContains(t, "handlers_auth.tmpl",
		"services.ErrEmailNotVerified",
		"ResendEmailVerification",
		"setPendingUser",
		"middleware.RateLimit",
	)

	assertTemplateContains(t, "handlers_auth.tmpl",
		"pendingUserID",
		"UserID: pendingUserID(ctx)",
	)

	assertTemplateContains(t, "handlers_auth.tmpl",
		"setPendingUser",
		"PendingUserID",
	)
}

func TestGeneratedUserEmailConflictsAreClassified(t *testing.T) {
	assertTemplateContains(t, "domains_user.tmpl",
		`pgErr.Code == "23505"`,
		`pgErr.TableName == "users"`,
		`pgErr.ConstraintName == "users_email_key"`,
		"return errors.Join(ErrConflict, err)",
		"return User{}, userWriteError(err)",
	)
	assertTemplateContains(t, "database_migrations_users.tmpl",
		"CONSTRAINT users_email_key UNIQUE (email)",
	)
	assertTemplateContains(t, "handlers_auth.tmpl",
		"errors.Is(err, domains.ErrConflict)",
		`outcome = "conflict"`,
	)
	assertTemplateContains(t, "handlers_errors.tmpl",
		"errors.Is(err, domains.ErrConflict)",
		`renderer.Page(ctx, "Errors/Conflict", inertia.Props{})`,
		"Status(http.StatusConflict)",
	)
	assertTemplateMissing(t, "handlers_errors.tmpl", "anduin.Conflict(")
	assertTemplateContains(t, "handlers_errors.tmpl",
		"func wantsJSON(ctx context.Context) bool",
		"anduin.Header(ctx, inertia.HeaderInertia)",
		"return mapJSONError(ctx, err)",
	)
	// Wrapping every unmapped error in a 500 would turn anduin.BadRequest into a 500.
	assertTemplateMissing(t, "handlers_errors.tmpl", "anduin.Error(ctx, http.StatusInternalServerError, err)")
}

// TestGeneratedServicesCommitWritesAndJobsTogether guards the transaction
// design: multi-step use cases run in storage.AtomicOps, bind receivers and
// the queue with InTx, and never hold the raw connection or transaction.
func TestGeneratedServicesCommitWritesAndJobsTogether(t *testing.T) {
	assertTemplateContains(t, "services_registration.tmpl", "i.queue.InTx(tx)")
	for _, name := range []string{"services_registration.tmpl", "services_reset_password.tmpl"} {
		assertTemplateContains(t, name,
			"storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {",
		)
		for _, unwanted := range []string{"storage.Connection", "storage.Transaction", "storage.Unwrap", "domains.Store"} {
			assertTemplateMissing(t, name, unwanted)
		}
	}

	assertTemplateContains(t, "domains_user.tmpl",
		"func NewUsers(db storage.Connection) Users {",
		"func (u Users) InTx(tx storage.Tx) Users {",
	)
	assertTemplateContains(t, "domains_token.tmpl",
		"func NewTokens(db storage.Connection) Tokens {",
		"func (t Tokens) InTx(tx storage.Tx) Tokens {",
		"transactor: t.transactor.InTx(tx),",
		"func (t Tokens) ReplaceCode(",
	)
	assertTemplateContains(t, "runtime_runtime.tmpl",
		"transactor := storage.NewTransactor(db)",
		"services.NewIdentity(transactor, users, tokens, queueInsert, cfg.Auth, cfg.Mail)",
	)
}

// TestGeneratedAuthEmailsKeepSecretsOutOfJobArgs guards M5: verification codes
// and reset URLs must not be rendered at enqueue time, because job args are
// persisted in river_job for the retention period. Services enqueue a user id;
// the worker mints the secret and renders the email.
func TestGeneratedAuthEmailsKeepSecretsOutOfJobArgs(t *testing.T) {
	for _, name := range []string{"services_registration.tmpl", "services_reset_password.tmpl"} {
		assertTemplateMissing(t, name, "jobs.SendTransactionalEmailArgs")
	}
	assertTemplateContains(t, "services_registration.tmpl",
		"jobs.SendVerificationEmailArgs{UserID: created.ID}",
		"jobs.SendVerificationEmailArgs{UserID: user.ID}",
		"func (i Identity) VerificationEmail(",
	)
	assertTemplateContains(t, "services_reset_password.tmpl",
		"jobs.SendPasswordResetEmailArgs{UserID: user.ID}",
		"func (i Identity) PasswordResetEmail(",
		"tokens.DestroyByUserAndScope(ctx, user.ID, domains.UserResetPassword)",
	)
	for _, name := range []string{
		"psql_queue_jobs_send_verification_email.tmpl",
		"psql_queue_jobs_send_password_reset_email.tmpl",
	} {
		assertTemplateContains(t, name, "UserID uuid.UUID")
		for _, unwanted := range []string{"Code", "Token", "URL", "email.TransactionalData"} {
			assertTemplateMissing(t, name, unwanted)
		}
	}
	assertTemplateContains(t, "psql_queue_workers_workers.tmpl",
		"NewSendVerificationEmailWorker(identity, transactional)",
		"NewSendPasswordResetEmailWorker(identity, transactional)",
	)
	assertTemplateContains(t, "runtime_queue.tmpl",
		"queue.NewWorkers(identity, transactional, marketing)",
	)
}

func TestBlueprintCarriesPendingUserCookieField(t *testing.T) {
	bp := initializeBlueprint("example.com/app")

	for _, field := range bp.Cookies.SortedAppFields() {
		if field.Name == "PendingUserID" && field.Type == "string" {
			return
		}
	}

	t.Fatal("blueprint cookies are missing the PendingUserID session field")
}

func assertTemplateContains(t *testing.T, name string, wants ...string) {
	t.Helper()

	content := readLayoutTemplate(t, name)
	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Errorf("%s missing %q", name, want)
		}
	}
}

func assertTemplateMissing(t *testing.T, name string, unwanted string) {
	t.Helper()

	if content := readLayoutTemplate(t, name); strings.Contains(content, unwanted) {
		t.Errorf("%s must not contain %q", name, unwanted)
	}
}

func readLayoutTemplate(t *testing.T, name string) string {
	t.Helper()

	content, err := fs.ReadFile(layouttemplates.Files, name)
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}

	return string(content)
}
