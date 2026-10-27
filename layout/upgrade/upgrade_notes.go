package upgrade

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/versions"
	"golang.org/x/mod/semver"
)

// inertiaViteFullURLVersion is the first released V2 pre-release that shipped
// the absolute Vite build URL for Inertia (#805). Projects already on or past
// it get no note.
const inertiaViteFullURLVersion = "v2.0.0-alpha.3"

// packageAPIVersion is the first release whose scaffold pins the package APIs
// that take config structs and drop unused arguments.
const packageAPIVersion = "v2.0.0-alpha.5"

// storageTestClusterVersion is the first release whose scaffold imports the
// test-container helpers from pkg/storage/storagetest.
const storageTestClusterVersion = "v2.0.0-alpha.5"

// betaTwoFixesVersion is the first release whose scaffold derives a valid
// default email sender, bundles SSR dependencies into the production build
// only, and renders a single <title> under SSR.
const betaTwoFixesVersion = "v2.0.0-beta.2"

// goNativeRebuildVersion is the release that replaced Echo and Fx with the
// Go-native domains/services/handlers layout. Projects older than it cannot be
// upgraded automatically; they must be ported by hand.
const goNativeRebuildVersion = "v2.0.0-alpha.4"

// ManualAction describes an application-owned change that an upgrade cannot
// apply without risking user code.
type ManualAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

func manualActionsForUpgrade(
	fromVersion string,
	toVersion string,
	includeInertiaMigration bool,
) ([]ManualAction, error) {
	actions := []ManualAction{}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, inertiaViteFullURLVersion) {
		var instructions strings.Builder
		instructions.WriteString(
			"Inertia production tags now need an absolute Vite build URL so secondary-host pages load the entry from the primary origin.\n\n",
		)
		instructions.WriteString(
			"The composition root is application-owned, so Andurel does not rewrite NewInertia automatically.\n\n",
		)
		instructions.WriteString(
			"1. In NewInertia (or the inertia renderer provider), pass routes.ViteBuild.FullURL() instead of Path():\n\n```go\n",
		)
		instructions.WriteString(
			"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n",
		)
		instructions.WriteString("```\n\n")
		instructions.WriteString("2. Format and verify the migration:\n\n```text\n")
		instructions.WriteString("gofmt -w internal/runtime\n")
		instructions.WriteString("go fix ./...\ngo vet ./...\n```\n")

		actions = append(actions, ManualAction{
			// The ID predates the alpha.3 gate. It stays unchanged because
			// structured upgrade output exposes it as a stable identifier.
			ID:           "inertia-vite-fullurl-v2.0.0",
			Title:        "Pass ViteBuild.FullURL() to the Inertia renderer",
			Instructions: instructions.String(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "package-api-v2.0.0-alpha.5",
			Title:        "Update internal/runtime for the anduin, cache, inertia and storage APIs",
			Instructions: packageAPIInstructions(includeInertiaMigration),
		})
	}

	if crossesVersion(fromVersion, toVersion, storageTestClusterVersion) {
		actions = append(actions, ManualAction{
			ID:           "storage-storagetest-v2.0.0-alpha.5",
			Title:        "Move test-cluster helpers to pkg/storage/storagetest",
			Instructions: storageTestClusterInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "routing-hosts-v2.0.0-alpha.5",
			Title:        "Replace the routing host registry with routing.NewHosts",
			Instructions: routingHostsInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "session-revocation-v2.0.0-alpha.5",
			Title:        "Add session revocation to users, queries and AuthOnly",
			Instructions: sessionRevocationInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "email-smtp-v2.0.0-alpha.5",
			Title:        "Add the SMTP sender and pass config.App to newEmailSenders",
			Instructions: emailSMTPInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, packageAPIVersion) {
		actions = append(actions, ManualAction{
			ID:           "validation-validate-v2.0.0-alpha.5",
			Title:        "Call the domain Validate method instead of validation.Validate",
			Instructions: validationInstructions(),
		})
	}

	if crossesVersion(fromVersion, toVersion, betaTwoFixesVersion) {
		actions = append(actions, ManualAction{
			ID:           "email-sender-v2.0.0-beta.2",
			Title:        "Strip the port from the default email sender and validate it",
			Instructions: emailSenderInstructions(),
		})
	}

	if includeInertiaMigration &&
		crossesVersion(fromVersion, toVersion, betaTwoFixesVersion) {
		actions = append(actions, ManualAction{
			ID:           "vite-ssr-bundle-v2.0.0-beta.2",
			Title:        "Bundle SSR dependencies into the production build only",
			Instructions: viteSSRBundleInstructions(),
		})
		actions = append(actions, ManualAction{
			ID:           "root-title-ssr-v2.0.0-beta.2",
			Title:        "Render the fallback <title> only for client-rendered pages",
			Instructions: rootTitleInstructions(),
		})
	}

	return actions, nil
}

func emailSenderInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"The default DEFAULT_SENDER_SIGNATURE was \"noreply@\"+app.Domain, and app.Domain is HOST_PRIMARY " +
			"with its port (noreply@localhost:8080). pkg/email rejects that address, so every queued auth email " +
			"was cancelled. config/email.go is application-owned, so fix it by hand:\n\n",
	)
	instructions.WriteString(
		"1. Derive the default from the host without its port:\n\n```go\n" +
			"DefaultSenderSignature: e.String(\"DEFAULT_SENDER_SIGNATURE\", \"noreply@\"+senderHost(app.Domain)),\n\n" +
			"// senderHost strips the port from the app domain. HOST_PRIMARY carries a port\n" +
			"// and an email address cannot contain one.\n" +
			"func senderHost(domain string) string {\n" +
			"\thost, _, err := net.SplitHostPort(domain)\n" +
			"\tif err == nil && host != \"\" {\n\t\treturn host\n\t}\n\n" +
			"\treturn domain\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Fail at startup on an invalid sender instead of cancelling every email job. " +
			"Replace the empty check in NewMail with:\n\n```go\n" +
			"sender := strings.TrimSpace(cfg.DefaultSenderSignature)\n" +
			"address, err := mail.ParseAddress(sender)\n" +
			"if err != nil || address.Name != \"\" || address.Address != sender {\n" +
			"\treturn Mail{}, fmt.Errorf(\"config: mail: DEFAULT_SENDER_SIGNATURE %q must be a bare email address\", sender)\n" +
			"}\n" +
			"cfg.DefaultSenderSignature = sender\n```\n\n" +
			"Import \"net\" and \"net/mail\". The sender is also the SMTP envelope sender, so a display name is rejected.\n\n",
	)
	instructions.WriteString("Then format and verify:\n\n```text\ngofmt -w config\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func viteSSRBundleInstructions() string {
	return "cmd/ssr extracts the embedded assets/dist/ssr/ssr.js to a temporary directory, where Node cannot " +
		"resolve react, @inertiajs/* or the other bare imports Vite leaves external. Bundle them into the " +
		"production SSR build only; setting noExternal for the dev server breaks SSR under `andurel run`.\n\n" +
		"In vite.config.ts, inside `defineConfig(({ command }) => ({ ... }))`, add:\n\n```ts\n" +
		"ssr: command === 'build' ? { noExternal: true } : undefined,\n```\n\n" +
		"Then rebuild and check that the bundle runs on its own:\n\n```text\nandurel build\n```\n"
}

func rootTitleInstructions() string {
	return "With SSR, the renderer writes the page's Inertia <Head> after the root template's static head, so a " +
		"page that sets its own title ended up with two <title> elements and browsers showed the first one. " +
		"In resources/root.templ, render the fallback only for client-rendered pages:\n\n```templ\n" +
		"if data.SSR == nil {\n\t<title>{ data.ProjectName }</title>\n}\n```\n\n" +
		"Then regenerate the root template and verify:\n\n```text\nandurel build\ngo vet ./...\n```\n"
}

func storageTestClusterInstructions() string {
	var instructions strings.Builder
	instructions.WriteString(
		"pkg/storage no longer imports testing or testcontainers. The test-cluster helpers moved to pkg/storage/storagetest with their methods unchanged:\n\n",
	)
	instructions.WriteString(
		"1. In your test files, import the new package and rename the helpers:\n\n```go\n" +
			"import \"github.com/mbvlabs/andurel/pkg/storage/storagetest\"\n\n" +
			"var testCluster *storagetest.Cluster\n\n" +
			"cluster, err := storagetest.NewCluster(context.Background())\n```\n\n",
	)
	instructions.WriteString(
		"2. There is no compatibility alias. Replace `storage.TestCluster` with `storagetest.Cluster` and " +
			"`storage.NewTestCluster` with `storagetest.NewCluster`; `Close` and `NewTestDB` keep their signatures.\n\n",
	)
	fmt.Fprintf(&instructions,
		"3. The upgrade pins pkg/storage %s. Format and verify:\n\n```text\n"+
			"gofmt -w .\ngo fix ./...\ngo vet ./...\ngolangci-lint run ./...\n```\n",
		versions.Storage,
	)

	return instructions.String()
}

func packageAPIInstructions(inertia bool) string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"The upgrade pins pkg/anduin %s, pkg/cache %s, pkg/inertia %s, pkg/kiks %s and pkg/storage %s. "+
			"internal/runtime is application-owned, so update it by hand:\n\n",
		versions.Anduin,
		versions.Cache,
		versions.Inertia,
		versions.Kiks,
		versions.Storage,
	)
	instructions.WriteString(
		"1. anduin.NewServer drops the environment and shutdowner arguments, and Server.Start takes none:\n\n```go\n" +
			"srv := anduin.NewServer(ctx, cfg.HTTP.Host, cfg.HTTP.Port, handler, anduin.WithTimeouts(/* ... */))\n" +
			"err := srv.Start()\n```\n\n",
	)
	instructions.WriteString(
		"2. cache.Store has Close. Close the store on shutdown, after the server stops:\n\n```go\n" +
			"closers = append(closers, func(context.Context) error { return cacheStore.Close() })\n```\n\n",
	)
	instructions.WriteString(
		"3. storage.RunInTransaction is gone. Use storage.AtomicOps with a storage.Transactor.\n\n",
	)
	instructions.WriteString(
		"River v0.40 adds main-line migration 7 (river_notification, drops river_client). " +
			"New projects ship it; existing projects add it with `andurel generate migration add_river_notification_drop_river_client` " +
			"and copy the up and down SQL from River's riverdriver/riverpgxv5/migration/main/007_*.sql.\n\n",
	)
	if inertia {
		instructions.WriteString(
			"4. In internal/runtime, inertia.NewRenderer takes inertia.RendererConfig; in cmd/ssr/main.go, inertia.NewSSRRuntime takes inertia.SSRRuntimeConfig. Both replace positional arguments:\n\n```go\n" +
				"inertia.NewRenderer(inertia.RendererConfig{\n\tContainerID:  cfg.Inertia.ContainerID,\n\tBuildPathURL: routes.ViteBuild.FullURL(),\n\t// ...\n})\n\n" +
				"inertia.NewSSRRuntime(inertia.SSRRuntimeConfig{\n\tExecutable:     cfg.SSRRuntime,\n\tBundlePath:     cfg.SSRBundle,\n\tListenURL:      cfg.SSRListen,\n\tStartupTimeout: cfg.SSRStartupTimeout,\n\tMinimumMajor:   cfg.SSRMinimumMajor,\n\tHealthTimeout:  cfg.SSRRequestTimeout,\n\tBundleFS:       assets.Files,\n})\n```\n\n",
		)
		instructions.WriteString(
			"5. pnpm 10+ blocks esbuild's install script, so `pnpm install` fails. Add pnpm-workspace.yaml:\n\n```yaml\n" +
				"allowBuilds:\n  esbuild: true\nonlyBuiltDependencies:\n  - esbuild\n```\n\n",
		)
		instructions.WriteString(
			"6. Svelte projects: @inertiajs/svelte v3 `useForm` returns a runes object, not a store. " +
				"In resources/js/Pages/Auth/*.svelte replace `$form.` with `form.`.\n\n",
		)
	}
	instructions.WriteString("Then format and verify:\n\n```text\ngofmt -w internal/runtime\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func routingHostsInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/routing %s replaces the package-level host registry with an explicit *routing.Hosts. "+
			"routing.ConfigureHosts, routing.LookupHost, routing.Hosts and routing.HostBaseURL are gone, "+
			"and every FullURL now takes the registry as its first argument:\n\n",
		versions.Routing,
	)
	instructions.WriteString(
		"1. Delete the ConfigureHosts call from NewApp in config/app.go. The runtime builds and validates the registry instead:\n\n```go\n" +
			"// remove\nif err := routing.ConfigureHosts(cfg.Hosts); err != nil {\n\treturn App{}, fmt.Errorf(\"config: app: %w\", err)\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Build the registry in internal/runtime/runtime.go and pass it to the router:\n\n```go\n" +
			"hosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
			"if err != nil {\n\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n}\n\n" +
			"router := anduin.NewRouter(\n\tanduin.WithHosts(hosts),\n\tanduin.WithErrorHandler(handlers.MapError(renderer)),\n\t// ...\n)\n```\n\n",
	)
	instructions.WriteString(
		"3. Do the same in NewQueue in internal/runtime/queue.go before it builds services.NewIdentity.\n\n",
	)
	instructions.WriteString(
		"4. Thread the registry through the identity service and every FullURL call:\n\n```go\n" +
			"// services/identity.go\n" +
			"type Identity struct {\n\t// ...\n\thosts *routing.Hosts\n}\n\n" +
			"func NewIdentity(\n\ttransactor storage.Transactor,\n\tusers domains.Users,\n\ttokens domains.Tokens,\n\tqueue storage.InsertQueue,\n\tauthCfg config.Auth,\n\tmailCfg config.Mail,\n\thosts *routing.Hosts,\n) Identity {\n\treturn Identity{\n\t\t// ...\n\t\thosts: hosts,\n\t}\n}\n\n" +
			"// services/reset_password.go\nrpEmail := appemail.ResetPassword{ResetURL: routes.PasswordEdit.FullURL(i.hosts, token)}\n\n" +
			"// internal/runtime/runtime.go\nrenderer, err := newRenderer(cfg, hosts)\n\n" +
			"func newRenderer(cfg config.Config, hosts *routing.Hosts) (*inertia.Renderer, error) {\n" +
			"\treturn inertia.NewRenderer(\n\t\tinertia.RendererConfig{\n" +
			"\t\t\tContainerID:  cfg.Inertia.ContainerID,\n" +
			"\t\t\tBuildPathURL: routes.ViteBuild.FullURL(hosts),\n" +
			"\t\t\t// ...\n\t\t},\n\t\t// ...\n\t)\n}\n```\n\n",
	)
	instructions.WriteString(
		"5. Update every other FullURL call to put the registry first, for example " +
			"`routes.PasswordEdit.FullURL(token)` becomes `routes.PasswordEdit.FullURL(hosts, token)` " +
			"and `Route.FullURL(id)` becomes `Route.FullURL(hosts, id)`. Replace " +
			"`routing.LookupHost(name)` with `hosts.Lookup(name)`, `routing.Hosts()` with `hosts.All()` " +
			"and `routing.HostBaseURL(name)` with `hosts.BaseURL(name)`.\n\n",
	)
	instructions.WriteString(
		"Run `andurel patterns show composition` for the full runtime wiring, then format and verify:\n\n```text\n" +
			"gofmt -w .\ngo fix ./...\ngo vet ./...\n```\n",
	)

	return instructions.String()
}

func sessionRevocationInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/auth %s and the alpha.5 scaffold make sessions revocable: a per-user session_version is stored "+
			"in the cookie and checked against the database on every authenticated request. Password resets bump "+
			"the version, so every other session stops working immediately.\n\n",
		versions.Auth,
	)
	instructions.WriteString(
		"This is a recommended security fix, not a compile requirement: an alpha.4 app keeps building without it.\n\n",
	)
	instructions.WriteString(
		"1. Add a migration:\n\n```text\nandurel generate migration add_session_version_to_users\n```\n\n" +
			"Use this SQL:\n\n```sql\n-- +goose Up\n-- +goose StatementBegin\n" +
			"ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1;\n" +
			"-- +goose StatementEnd\n\n-- +goose Down\n-- +goose StatementBegin\n" +
			"ALTER TABLE users DROP COLUMN session_version;\n-- +goose StatementEnd\n```\n\n",
	)
	instructions.WriteString(
		"2. Update domains/user.sql: include session_version in the CreateUser INSERT and UpdateUser SET, then add:\n\n```sql\n" +
			"-- name: BumpSessionVersion :exec\nUPDATE users\nSET session_version = session_version + 1\nWHERE id = $1;\n```\n\n" +
			"Run `andurel sync queries` to regenerate domains/internal/queries. Run `andurel patterns show domain` " +
			"to see the full users domain with SessionVersion in the User struct and its Create, Save and " +
			"ChangePassword parameter structs.\n\n",
	)
	instructions.WriteString(
		"3. Add the domain method and revoke on password change:\n\n```go\n" +
			"func (u Users) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
			"\treturn u.queries.BumpSessionVersion(ctx, id)\n}\n\n" +
			"// In ChangePassword, after the password write succeeds (Save in the alpha.5 domain, Update\n" +
			"// in an alpha.4 domain):\n" +
			"if err := users.BumpSessionVersion(ctx, id); err != nil {\n\treturn err\n}\n```\n\n",
	)
	instructions.WriteString(
		"4. Update the session cookie fields: keep UserID and PendingUserID, replace IsAdmin with SessionVersion, " +
			"and keep IsAuthenticated:\n\n```go\ntype Session struct {\n" +
			"\tUserID          string `json:\"user_id,omitempty\"`\n" +
			"\tPendingUserID   string `json:\"pending_user_id,omitempty\"`\n" +
			"\tSessionVersion  int32  `json:\"session_version,omitempty\"`\n" +
			"\tIsAuthenticated bool   `json:\"is_authenticated,omitempty\"`\n}\n```\n\n" +
			"If your alpha.4 cookie is still named App, keep the name or rename it to Session; the fields are what matter. " +
			"In handlers/auth.go set `SessionVersion: entity.SessionVersion` instead of `IsAdmin: entity.IsAdmin` " +
			"wherever the session cookie is created.\n\n",
	)
	instructions.WriteString(
		"5. Replace AuthOnly with the database-backed version and pass the users repository at the call site:\n\n```go\n" +
			"func AuthOnly(users domains.Users) anduin.Middleware {\n" +
			"\treturn func(next anduin.Handler) anduin.Handler {\n" +
			"\t\treturn func(ctx context.Context) error {\n" +
			"\t\t\tsession, err := kiks.Get[*Session](ctx)\n" +
			"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
			"\t\t\tif session == nil || !session.IsAuthenticated {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\tuserID, err := uuid.Parse(session.UserID)\n" +
			"\t\t\tif err != nil {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\tuser, err := users.Find(ctx, userID)\n" +
			"\t\t\tif err != nil {\n\t\t\t\tif errors.Is(err, domains.ErrNotFound) {\n\t\t\t\t\treturn rejectSession(ctx)\n\t\t\t\t}\n\n" +
			"\t\t\t\treturn err\n\t\t\t}\n\n" +
			"\t\t\tif user.SessionVersion != session.SessionVersion {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
			"\t\t\treturn next(WithViewer(ctx, user))\n\t\t}\n\t}\n}\n```\n\n" +
			"Then register protected pages with `middleware.AuthOnly(users)`; `andurel patterns show composition` " +
			"has the matching WithViewer, Viewer and rejectSession helpers.\n\n",
	)
	instructions.WriteString(
		"Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n",
	)

	return instructions.String()
}

func emailSMTPInstructions() string {
	var instructions strings.Builder
	fmt.Fprintf(&instructions,
		"pkg/email %s adds a production SMTP transport and the alpha.5 scaffold rejects Mailpit when "+
			"ENVIRONMENT=production. `newEmailSenders` also takes config.App so it can make that check.\n\n",
		versions.Email,
	)
	instructions.WriteString(
		"This action is optional for local development, but production apps still on Mailpit must switch to SMTP.\n\n",
	)
	instructions.WriteString(
		"1. Add the SMTP fields to config.Mail and read them in NewMail in config/email.go:\n\n```go\n" +
			"const (\n\tMailpitDriver MailDriver = \"mailpit\"\n\tSMTPDriver    MailDriver = \"smtp\"\n)\n\n" +
			"type Mail struct {\n\t// ...\n\tSMTPHost     string\n\tSMTPPort     string\n\tSMTPUsername string\n\tSMTPPassword string\n\tSMTPTLS      string\n}\n```\n\n",
	)
	instructions.WriteString(
		"2. Change newEmailSenders to take config.App and add the SMTP case:\n\n```go\n" +
			"func newEmailSenders(\n\tcfg config.Mail,\n\tapp config.App,\n) (email.TransactionalSender, email.MarketingSender, error) {\n" +
			"\tif err := validateEmailConfig(cfg, app); err != nil {\n\t\treturn nil, nil, err\n\t}\n\n" +
			"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n\t\t// ...\n" +
			"\tcase config.SMTPDriver:\n" +
			"\t\tclient, err := email.NewSMTP(email.SMTPConfig{\n" +
			"\t\t\tHost:     cfg.SMTPHost,\n\t\t\tPort:     cfg.SMTPPort,\n" +
			"\t\t\tUsername: cfg.SMTPUsername,\n\t\t\tPassword: cfg.SMTPPassword,\n" +
			"\t\t\tTLS:      email.SMTPTLSMode(cfg.SMTPTLS),\n\t\t})\n" +
			"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create SMTP sender: %w\", err)\n\t\t}\n\n" +
			"\t\treturn client, client, nil\n\tdefault:\n" +
			"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}\n```\n\n",
	)
	instructions.WriteString(
		"3. Pass cfg.App at the call site in internal/runtime/queue.go (the function stays in internal/runtime/runtime.go):\n\n```go\n" +
			"transactional, marketing, err := newEmailSenders(cfg.Mail, cfg.App)\n```\n\n" +
			"Then set EMAIL_PROVIDER=smtp and SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD and SMTP_TLS " +
			"(starttls, tls, or none). `andurel patterns show composition` has the complete sender setup.\n\n",
	)
	instructions.WriteString("Format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n")

	return instructions.String()
}

func validationInstructions() string {
	return "pkg/validation " + versions.Validation + " removes the deprecated validation.Validate function and " +
		"the Validatable interface, which the alpha.4 scaffold still calls in domains/user.go and domains/token.go.\n\n" +
		"Replace each call with the value's own Validate method:\n\n```go\n" +
		"// before\nif err := validation.Validate(&entity); err != nil {\n\treturn err\n}\n\n" +
		"// after\nif err := entity.Validate(); err != nil {\n\treturn err\n}\n```\n\n" +
		"Generated domains already expose Validate; `andurel patterns show domain` shows the current shape. " +
		"Then format and verify:\n\n```text\ngofmt -w .\ngo fix ./...\ngo vet ./...\n```\n"
}

func crossesVersion(fromVersion, toVersion, boundary string) bool {
	from, fromOK := canonicalUpgradeVersion(fromVersion)
	to, toOK := canonicalUpgradeVersion(toVersion)
	boundary, boundaryOK := canonicalUpgradeVersion(boundary)
	return fromOK && toOK && boundaryOK &&
		semver.Compare(from, boundary) < 0 &&
		semver.Compare(to, boundary) >= 0
}

func canonicalUpgradeVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	version = semver.Canonical(version)
	return version, version != ""
}
