package layout

import (
	"io/fs"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
)

func TestPhase2UserAndTokenModelTemplates(t *testing.T) {
	user := readPhase2Template(t, "models_user.tmpl")
	for _, want := range []string{
		"CreatedAt:        current.CreatedAt",
		"db.NewDelete()",
		"Column(\"email\")",
		"Column(\"email_validated_at\")",
		"Column(\"password\")",
		"Column(\"is_admin\")",
		"Column(\"updated_at\")",
		"Returning(\"*\")",
		"Model(&UserEntity{}).Count(ctx)",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("models_user.tmpl missing %q", want)
		}
	}
	if strings.Contains(user, "Model(&UserEntity{}).Scan(ctx, &totalCount)") {
		t.Error("models_user.tmpl still scans a model into the pagination count")
	}

	token := readPhase2Template(t, "models_token.tmpl")
	if !strings.Contains(token, "Model(&TokenEntity{}).Count(ctx)") {
		t.Error("models_token.tmpl does not use Bun Count")
	}
	if strings.Contains(token, "Model(&TokenEntity{}).Scan(ctx, &totalCount)") {
		t.Error("models_token.tmpl still scans a model into the pagination count")
	}
}

func TestPhase2AuthenticationTemplates(t *testing.T) {
	authConfig := readPhase2Template(t, "config_auth.tmpl")
	if !strings.Contains(authConfig, `env:"PREVIOUS_PEPPERS"`) {
		t.Error("config_auth.tmpl does not configure PREVIOUS_PEPPERS")
	}

	identity := readPhase2Template(t, "services_identity.tmpl")
	for _, want := range []string{"previousPeppers []string", "tokenSigningKey string", "cfg.App.TokenSigningKey"} {
		if !strings.Contains(identity, want) {
			t.Errorf("services_identity.tmpl missing %q", want)
		}
	}

	authentication := readPhase2Template(t, "services_authentication.tmpl")
	for _, want := range []string{
		"verifyPasswordWithPeppers",
		"models.HashPassword(data.Password, i.pepper)",
		"models.User.Update(ctx, i.db.Executor()",
		"persist password rehash",
	} {
		if !strings.Contains(authentication, want) {
			t.Errorf("services_authentication.tmpl missing %q", want)
		}
	}

	registration := readPhase2Template(t, "services_registration.tmpl")
	if got := strings.Count(registration, "i.tokenSigningKey"); got != 3 {
		t.Errorf("services_registration.tmpl token signing uses = %d, want 3", got)
	}
	reset := readPhase2Template(t, "services_reset_password.tmpl")
	if got := strings.Count(reset, "i.tokenSigningKey"); got != 3 {
		t.Errorf("services_reset_password.tmpl token signing uses = %d, want 3", got)
	}
}

func TestPhase2RateLimiterAndLifecycleTemplates(t *testing.T) {
	rateLimiter := readPhase2Template(t, "router_middleware_auth.tmpl")
	for _, want := range []string{
		"MaximumSize:      1000",
		"ExpiryCreating[string, int32](10 * time.Minute)",
		"cache.Compute(ip",
		"if hits >= limit",
		"return hits + 1, otter.WriteOp",
	} {
		if !strings.Contains(rateLimiter, want) {
			t.Errorf("router_middleware_auth.tmpl missing %q", want)
		}
	}

	mainTemplate := readPhase2Template(t, "cmd_app_main.tmpl")
	for _, want := range []string{
		`startInBackground(appCtx, "queue processor", p.Start)`,
		"stopAndWait(ctx, p.Stop, done)",
		"srv.Start(ctx, config.Env)",
		"[]server.Shutdowner{processor}",
	} {
		contains := strings.Contains(mainTemplate, want)
		if want == "[]server.Shutdowner{processor}" {
			if contains {
				t.Error("cmd_app_main.tmpl gives the server ownership of the queue processor")
			}
			continue
		}
		if !contains {
			t.Errorf("cmd_app_main.tmpl missing %q", want)
		}
	}
	if got := strings.Count(mainTemplate, "p.Stop"); got != 1 {
		t.Errorf("cmd_app_main.tmpl queue stop references = %d, want 1", got)
	}

	serverTemplate := readPhase2Template(t, "framework_elements_server_server.tmpl")
	if strings.Contains(serverTemplate, "shutdowner.Shutdown") {
		t.Error("server Start still owns component shutdown")
	}
}

func readPhase2Template(t *testing.T, name string) string {
	t.Helper()
	content, err := fs.ReadFile(layouttemplates.Files, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
