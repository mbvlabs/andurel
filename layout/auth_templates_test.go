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
		"func (t Tokens) FindByUserAndScope(",
		"func (t Tokens) IncrementAttempts(ctx context.Context, id uuid.UUID) (int32, error)",
		"func (t Tokens) DestroyByUserAndScope(",
		"GetTokenByUserAndScope",
	)
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
		"middleware.IPRateLimiter",
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

// TestGeneratedServicesCommitWritesAndJobsTogether guards the transaction
// design: multi-step use cases run in storage.AtomicOps, bind receivers and
// the queue with InTx, and never hold the raw connection or transaction.
func TestGeneratedServicesCommitWritesAndJobsTogether(t *testing.T) {
	for _, name := range []string{"services_registration.tmpl", "services_reset_password.tmpl"} {
		assertTemplateContains(t, name,
			"storage.AtomicOps(ctx, i.transactor, func(ctx context.Context, tx storage.Tx) error {",
			"i.queue.InTx(tx)",
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
