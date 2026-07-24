package layout

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
)

func TestGeneratedAPIControllerLivesInAPIPackage(t *testing.T) {
	root := t.TempDir()
	if err := processTemplatedFiles(root, &TemplateData{ModuleName: "example.com/app"}); err != nil {
		t.Fatalf("process templates: %v", err)
	}

	apiController, err := os.ReadFile(filepath.Join(root, "controllers/api/api.go"))
	if err != nil {
		t.Fatalf("read API controller: %v", err)
	}
	if !strings.HasPrefix(string(apiController), "package api\n") {
		t.Error("API controller is not in package api")
	}
	if _, err := os.Stat(filepath.Join(root, "controllers/api.go")); !os.IsNotExist(err) {
		t.Error("legacy controllers/api.go was generated")
	}

	controller, err := os.ReadFile(filepath.Join(root, "controllers/controller.go"))
	if err != nil {
		t.Fatalf("read root controller: %v", err)
	}
	for _, want := range []string{
		`"example.com/app/controllers/api"`,
		"api.NewAPI",
		"c api.API",
	} {
		if !strings.Contains(string(controller), want) {
			t.Errorf("controllers/controller.go missing %q", want)
		}
	}
}

func TestGeneratedUserAndTokenModelTemplates(t *testing.T) {
	user := readGeneratedApplicationTemplate(t, "models_user.tmpl")
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

	token := readGeneratedApplicationTemplate(t, "models_token.tmpl")
	if !strings.Contains(token, "Model(&TokenEntity{}).Count(ctx)") {
		t.Error("models_token.tmpl does not use Bun Count")
	}
	if strings.Contains(token, "Model(&TokenEntity{}).Scan(ctx, &totalCount)") {
		t.Error("models_token.tmpl still scans a model into the pagination count")
	}
}

func TestGeneratedAuthenticationTemplates(t *testing.T) {
	authConfig := readGeneratedApplicationTemplate(t, "config_auth.tmpl")
	if !strings.Contains(authConfig, `env:"PREVIOUS_PEPPERS"`) {
		t.Error("config_auth.tmpl does not configure PREVIOUS_PEPPERS")
	}

	identity := readGeneratedApplicationTemplate(t, "services_identity.tmpl")
	for _, want := range []string{"previousPeppers []string", "tokenSigningKey string", "cfg.App.TokenSigningKey"} {
		if !strings.Contains(identity, want) {
			t.Errorf("services_identity.tmpl missing %q", want)
		}
	}

	authentication := readGeneratedApplicationTemplate(t, "services_authentication.tmpl")
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

	registration := readGeneratedApplicationTemplate(t, "services_registration.tmpl")
	if got := strings.Count(registration, "i.tokenSigningKey"); got != 3 {
		t.Errorf("services_registration.tmpl token signing uses = %d, want 3", got)
	}
	reset := readGeneratedApplicationTemplate(t, "services_reset_password.tmpl")
	if got := strings.Count(reset, "i.tokenSigningKey"); got != 3 {
		t.Errorf("services_reset_password.tmpl token signing uses = %d, want 3", got)
	}
}

func TestGeneratedRateLimiterAndLifecycleTemplates(t *testing.T) {
	rateLimiter := readGeneratedApplicationTemplate(t, "router_middleware_auth.tmpl")
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

	mainTemplate := readGeneratedApplicationTemplate(t, "cmd_app_main.tmpl")
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
	if got := strings.Count(mainTemplate, "stopAndWait(ctx, p.Stop, done)"); got != 1 {
		t.Errorf("cmd_app_main.tmpl queue stop wiring occurrences = %d, want 1", got)
	}

	serverTemplate := readGeneratedApplicationTemplate(t, "framework_elements_server_server.tmpl")
	if strings.Contains(serverTemplate, "shutdowner.Shutdown") {
		t.Error("server Start still owns component shutdown")
	}
}

func TestGeneratedSessionRecoveryTemplates(t *testing.T) {
	root := t.TempDir()
	if err := processTemplatedFiles(root, &TemplateData{ModuleName: "example.com/app"}); err != nil {
		t.Fatalf("process templates: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "router/cookies/session.go")); err != nil {
		t.Fatalf("generated shared session loader: %v", err)
	}

	sessionRecovery := readGeneratedApplicationTemplate(t, "router_cookies_session.tmpl")
	for _, want := range []string{
		"func RecoverInvalidSessions",
		"securecookie.Error",
		"decodeError.IsDecode()",
		"decodeError.IsUsage()",
		"decodeError.IsInternal()",
		"clear(sess.Values)",
		"sess.Save(c.Request(), c.Response())",
		"func getSession",
	} {
		if !strings.Contains(sessionRecovery, want) {
			t.Errorf("router_cookies_session.tmpl missing %q", want)
		}
	}

	for _, templateName := range []string{"router_cookies_cookies.tmpl", "router_cookies_flash.tmpl"} {
		content := readGeneratedApplicationTemplate(t, templateName)
		if strings.Contains(content, "session.Get(") {
			t.Errorf("%s bypasses the recoverable session loader", templateName)
		}
		if !strings.Contains(content, "getSession(") {
			t.Errorf("%s does not use the recoverable session loader", templateName)
		}
	}

	middleware := readGeneratedApplicationTemplate(t, "router_middleware_middleware.tmpl")
	if !strings.Contains(middleware, "cookies.RecoverInvalidSessions(c)") {
		t.Error("router_middleware_middleware.tmpl does not recover invalid session cookies")
	}

	if got := baseTemplateMappings["router_cookies_session.tmpl"]; got != "router/cookies/session.go" {
		t.Fatalf("session recovery template target = %q, want router/cookies/session.go", got)
	}

	goMod := readGeneratedApplicationTemplate(t, "go_mod.tmpl")
	if !strings.Contains(goMod, "github.com/gorilla/securecookie v1.1.2") {
		t.Error("go_mod.tmpl does not declare securecookie as a direct dependency")
	}
}

func readGeneratedApplicationTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := fs.ReadFile(layouttemplates.Files, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
