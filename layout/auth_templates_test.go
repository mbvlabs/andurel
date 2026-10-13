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

	assertTemplateContains(t, "models_token.tmpl",
		"func (t Tokens) FindByUserAndScope(",
		"func (t Tokens) IncrementAttempts(ctx context.Context, id uuid.UUID) (int32, error)",
		"func (t Tokens) DestroyByUserAndScope(",
		"GetTokenByUserAndScope",
	)
	assertTemplateContains(t, "models_queries_token.tmpl",
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

	assertTemplateContains(t, "controllers_sessions_inertia.tmpl",
		"services.ErrEmailNotVerified",
		"ResendEmailVerification",
		"setPendingUser",
		"middleware.IPRateLimiter",
	)

	assertTemplateContains(t, "controllers_confirmations_inertia.tmpl",
		"pendingUserID",
		"UserID: pendingUserID(ctx)",
	)

	assertTemplateContains(t, "controllers_registrations_inertia.tmpl",
		"setPendingUser",
		"PendingUserID",
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
