package layout

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/extensions"
	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestGeneratedConfigModuleDoesNotDuplicateProviders(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app"}
	data.SetBlueprint(initializeBlueprint("example.com/app"))
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("process templates: %v", err)
	}

	configModule, err := os.ReadFile(filepath.Join(root, "config/config.go"))
	if err != nil {
		t.Fatalf("read config module: %v", err)
	}
	content := string(configModule)
	for _, provider := range []string{"NewEmailCfg", "NewAuthCfg"} {
		if got := strings.Count(content, provider); got != 1 {
			t.Errorf("config/config.go %s occurrences = %d, want 1", provider, got)
		}
	}
	if strings.Contains(content, "NewInertiaCfg") {
		t.Error("non-inertia config module should not provide NewInertiaCfg")
	}
}

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

func TestGeneratedConfigEnvDefaults(t *testing.T) {
	appConfig := readGeneratedApplicationTemplate(t, "config_app.tmpl")
	for _, want := range []string{
		"Host:          DefaultAppHost",
		"env.Parse(&appCfg)",
		"validation.Validate(&appCfg)",
		"func (c AppCfg) Validate() error",
		`env:"SESSION_KEY"`,
	} {
		if !strings.Contains(appConfig, want) {
			t.Errorf("config_app.tmpl missing %q", want)
		}
	}
	if strings.Contains(appConfig, "func (c AppCfg) GetDomain() string      { return c.Domain }\nfunc (c AppCfg) GetBaseURL()") {
		t.Error("config_app.tmpl should separate getters with blank lines instead of packing them")
	}
	if !strings.Contains(appConfig, "func (c AppCfg) GetEnvironment() string { return c.Environment }\n\nfunc (c AppCfg) GetProjectName()") {
		t.Error("config_app.tmpl should put a blank line between consecutive getters")
	}
	if !strings.Contains(appConfig, "panic(err)\n\t}\n\n\tif err := validation.Validate(&appCfg)") {
		t.Error("config_app.tmpl should separate env parse and validation with a blank line")
	}
	if strings.Contains(appConfig, "envDefault:") ||
		strings.Contains(appConfig, "RequiredIfNoDef") ||
		strings.Contains(appConfig, `env:"SESSION_KEY,required"`) ||
		strings.Contains(appConfig, "load(&appCfg)") {
		t.Error("config_app.tmpl should parse env and validate in NewAppCfg without envDefault, required tags, or load helpers")
	}

	if _, exists := baseTemplateMappings["config_env.tmpl"]; exists {
		t.Error("config_env.tmpl should not be mapped")
	}

	envExample := readGeneratedApplicationTemplate(t, "env.tmpl")
	for _, want := range []string{
		"DB_KIND=postgres",
		"DB_HOST=127.0.0.1",
		"DB_PASSWORD=postgres",
		"CSRF_STRATEGY=header_only",
		"CSRF_TRUSTED_ORIGINS=",
		"SESSION_KEY={{.SessionKey}}",
		"SESSION_ENCRYPTION_KEY={{.SessionEncryptionKey}}",
		"TOKEN_SIGNING_KEY={{.TokenSigningKey}}",
		"PEPPER={{.Pepper}}",
	} {
		if !strings.Contains(envExample, want) {
			t.Errorf("env.tmpl missing %q", want)
		}
	}
	for _, unexpected := range []string{
		"ENVIRONMENT=",
		"PROJECT_NAME=",
		"DOMAIN=",
		"HOST=",
		"PORT=",
		"QUEUE_",
		"INERTIA_",
		"SESSION_MAX_AGE=",
		"CORS_",
		"DEFAULT_SENDER_SIGNATURE=",
		"PREVIOUS_PEPPERS=",
	} {
		if strings.Contains(envExample, unexpected) {
			t.Errorf("env.tmpl should not include defaultable %q", unexpected)
		}
	}

	databaseConfig := readGeneratedApplicationTemplate(t, "config_database.tmpl")
	if !strings.Contains(databaseConfig, "Host:                     DefaultDatabaseHost") {
		t.Error("config_database.tmpl does not seed DB defaults before env parsing")
	}
	if !strings.Contains(databaseConfig, "func (c Database) Validate() error") {
		t.Error("config_database.tmpl missing Validate")
	}
	if !strings.Contains(appConfig, "validation.NewBuilder()") {
		t.Error("config_app.tmpl Validate should use pkg/validation")
	}

	authConfig := readGeneratedApplicationTemplate(t, "config_auth.tmpl")
	if !strings.Contains(authConfig, `env:"PEPPER"`) {
		t.Error("config_auth.tmpl does not configure PEPPER")
	}
	if strings.Contains(authConfig, `env:"PEPPER,required"`) {
		t.Error("config_auth.tmpl should not mark PEPPER required via env tags")
	}
	if !strings.Contains(authConfig, "func (c AuthCfg) Validate() error") {
		t.Error("config_auth.tmpl missing Validate")
	}
}

func TestGeneratedAuthenticationTemplates(t *testing.T) {
	authConfig := readGeneratedApplicationTemplate(t, "config_auth.tmpl")
	if !strings.Contains(authConfig, `env:"PREVIOUS_PEPPERS"`) {
		t.Error("config_auth.tmpl does not configure PREVIOUS_PEPPERS")
	}

	identity := readGeneratedApplicationTemplate(t, "services_identity.tmpl")
	for _, want := range []string{"AppIdentityProvider", "TokenProvider", "AuthProvider", "GetTokenSigningKey()", "GetPepper()", "GetPreviousPeppers()", "baseURL", "defaultSenderSignature"} {
		if !strings.Contains(identity, want) {
			t.Errorf("services_identity.tmpl missing %q", want)
		}
	}
	if strings.Contains(identity, `"{{.ModuleName}}/config"`) || strings.Contains(identity, "config.AppCfg") {
		t.Error("services_identity.tmpl should not import config; it should take local interfaces")
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

func TestGeneratedDatabaseTemplatesUseStandaloneStorage(t *testing.T) {
	databaseConfig := readGeneratedApplicationTemplate(t, "config_database.tmpl")
	for _, want := range []string{
		"DefaultDatabaseHost",
		"DB_PASSWORD",
		"func NewDatabaseCfg() Database",
		`env:"DB_HOST"`,
	} {
		if !strings.Contains(databaseConfig, want) {
			t.Errorf("config_database.tmpl missing %q", want)
		}
	}

	mainTemplate := readGeneratedApplicationTemplate(t, "cmd_app_main.tmpl")
	if !strings.Contains(mainTemplate, "databaseModule") {
		t.Error("cmd_app_main.tmpl does not use databaseModule")
	}
	if !strings.Contains(mainTemplate, "func newDatabase(lifecycle fx.Lifecycle, ctx context.Context, cfg config.Database)") {
		t.Error("cmd_app_main.tmpl does not define newDatabase with config.Database")
	}
	if !strings.Contains(mainTemplate, "fx.Annotate(newDatabase, fx.As(new(storage.Connection)), fx.As(fx.Self()))") {
		t.Error("cmd_app_main.tmpl does not provide *storage.Postgres as storage.Connection")
	}
	if strings.Contains(mainTemplate, "fx.As(fx.Self(), new(storage.Connection))") {
		t.Error("cmd_app_main.tmpl maps fx.As targets positionally; Self and Connection must be separate As annotations")
	}

	queueTemplate := readGeneratedApplicationTemplate(t, "cmd_queue_main.tmpl")
	if !strings.Contains(queueTemplate, "fx.Annotate(newDatabase, fx.As(new(storage.Connection)), fx.As(fx.Self()))") {
		t.Error("cmd_queue_main.tmpl does not provide *storage.Postgres as storage.Connection")
	}

	if _, exists := baseTemplateMappings["psql_database.tmpl"]; exists {
		t.Error("legacy database/database.go template is still mapped")
	}
	if got := baseTemplateMappings["database_migrations.tmpl"]; got != "migrations/migrations.go" {
		t.Errorf("database migrations template target = %q, want migrations/migrations.go", got)
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
	if !strings.Contains(mainTemplate, "srv.Start(ctx, appCfg.GetEnvironment())") {
		t.Error("cmd_app_main.tmpl does not start the server with the application environment")
	}
	for _, unwanted := range []string{"startQueueProcessor", "queue.WorkersModule", `"{{.ModuleName}}/queue"`} {
		if strings.Contains(mainTemplate, unwanted) {
			t.Errorf("cmd_app_main.tmpl still contains queue processor wiring %q", unwanted)
		}
	}
	if !strings.Contains(mainTemplate, "databaseModule") {
		t.Error("cmd_app_main.tmpl does not install databaseModule")
	}
	if !strings.Contains(mainTemplate, "queueInsertModule") {
		t.Error("cmd_app_main.tmpl does not install queueInsertModule")
	}

	queueMain := readGeneratedApplicationTemplate(t, "cmd_queue_main.tmpl")
	for _, want := range []string{"queue.Module,", "queueProcessorModule,", "mailclients.NewMailpit("} {
		if !strings.Contains(queueMain, want) {
			t.Errorf("cmd_queue_main.tmpl missing %q", want)
		}
	}

	queueConfig := readGeneratedApplicationTemplate(t, "config_queue.tmpl")
	if got := strings.Count(queueConfig, "type QueueCfg struct"); got != 1 {
		t.Errorf("config_queue.tmpl QueueCfg declarations = %d, want 1", got)
	}
	if !strings.Contains(queueConfig, "func (cfg QueueCfg) RiverConfig()") {
		t.Error("config_queue.tmpl missing RiverConfig method")
	}

	serverTemplate := readStandalonePackageFile(t, "server", "server.go")
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
		"func (s *Session) RecoverInvalidSessions",
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
	if !strings.Contains(middleware, "middleware.ValidateSession(session)") {
		t.Error("router_middleware_middleware.tmpl does not recover invalid session cookies")
	}
	if !strings.Contains(middleware, "middleware.RegisterRequestMeta(session)") {
		t.Error("router_middleware_middleware.tmpl does not register request metadata with injected session")
	}

	if _, exists := baseTemplateMappings["application_metadata.tmpl"]; exists {
		t.Error("legacy application_metadata template is still mapped")
	}

	if got := baseTemplateMappings["router_cookies_session.tmpl"]; got != "router/cookies/session.go" {
		t.Fatalf("session recovery template target = %q, want router/cookies/session.go", got)
	}

	goMod := readGeneratedApplicationTemplate(t, "go_mod.tmpl")
	if !strings.Contains(goMod, "github.com/gorilla/securecookie v1.1.2") {
		t.Error("go_mod.tmpl does not declare securecookie as a direct dependency")
	}
}

func TestGeneratedGoTemplatesSeparateFunctions(t *testing.T) {
	checkGoTemplateSpacing := func(t *testing.T, fsys fs.FS, root string) {
		t.Helper()
		err := fs.WalkDir(fsys, root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
				return nil
			}

			content, err := fs.ReadFile(fsys, path)
			if err != nil {
				return err
			}
			if !isGoSourceTemplate(content) {
				return nil
			}

			assertGoFunctionSpacing(t, string(content), path)
			assertReturnAfterBranchSpacing(t, string(content), path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk templates: %v", err)
		}
	}

	checkGoTemplateSpacing(t, layouttemplates.Files, ".")
	checkGoTemplateSpacing(t, extensions.Files, "templates")
}

func TestGeneratedScaffoldGoFilesSeparateFunctions(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app"}
	data.SetBlueprint(initializeBlueprint("example.com/app"))
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("process templates: %v", err)
	}

	assertScaffoldGoSpacing(t, root)
}

func TestGeneratedInertiaScaffoldGoFilesSeparateFunctions(t *testing.T) {
	root := t.TempDir()
	if err := Scaffold(root, "testapp", "postgresql", "test", nil, "react", ""); err != nil {
		t.Fatalf("scaffold inertia project: %v", err)
	}

	assertScaffoldGoSpacing(t, root)
}

func assertScaffoldGoSpacing(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_templ.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		assertGoFunctionSpacing(t, string(content), rel)
		assertReturnAfterBranchSpacing(t, string(content), rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk generated scaffold: %v", err)
	}
}

func isGoSourceTemplate(content []byte) bool {
	for line := range strings.SplitSeq(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		return strings.HasPrefix(trimmed, "package ")
	}
	return false
}

func assertGoFunctionSpacing(t *testing.T, content, name string) {
	t.Helper()

	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		prev := strings.TrimSpace(lines[i-1])
		cur := strings.TrimSpace(lines[i])
		if prev == "" || cur == "" {
			continue
		}
		if strings.Contains(prev, "{{") || strings.Contains(prev, "}}") ||
			strings.Contains(cur, "{{") || strings.Contains(cur, "}}") {
			continue
		}

		if strings.HasPrefix(cur, "func ") && strings.HasPrefix(prev, "func ") {
			t.Errorf("%s:%d: consecutive func declarations must be separated by a blank line", name, i+1)
		}
		if strings.HasPrefix(cur, "func ") && prev == "}" {
			t.Errorf("%s:%d: func must be preceded by a blank line after closing brace", name, i+1)
		}
	}
}

func assertReturnAfterBranchSpacing(t *testing.T, content, name string) {
	t.Helper()

	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if !shouldHaveBlankBeforeReturn(lines, i) {
			continue
		}
		t.Errorf("%s:%d: return must be preceded by a blank line after branching", name, i+1)
	}
}

func shouldHaveBlankBeforeReturn(lines []string, i int) bool {
	trimmed := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(trimmed, "return") {
		return false
	}
	if strings.Contains(lines[i], "{{") || strings.Contains(lines[i], "}}") {
		return false
	}

	j := i - 1
	for j >= 0 && strings.TrimSpace(lines[j]) == "" {
		j--
	}
	if j < 0 || strings.TrimSpace(lines[j]) != "}" {
		return false
	}
	if strings.Contains(lines[j], "{{") || strings.Contains(lines[j], "}}") {
		return false
	}

	return j == i-1
}

func TestGeneratedSQLCArtifacts(t *testing.T) {
	root := t.TempDir()
	data := &TemplateData{ModuleName: "example.com/app"}
	if err := processTemplatedFiles(root, data); err != nil {
		t.Fatalf("process templates: %v", err)
	}
	if err := storage.WriteSQLCConfig(root); err != nil {
		t.Fatalf("write sqlc config: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "models/queries/.gitkeep")); err != nil {
		t.Fatalf("models/queries/.gitkeep: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(root, "sqlc.yaml"))
	if err != nil {
		t.Fatalf("read sqlc.yaml: %v", err)
	}
	if string(written) != string(storage.SQLCConfig()) {
		t.Fatal("sqlc.yaml does not match storage module config")
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

func readStandalonePackageFile(t *testing.T, packageName, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "pkg", packageName, name))
	if err != nil {
		t.Fatalf("read standalone package file %s/%s: %v", packageName, name, err)
	}
	return string(content)
}
