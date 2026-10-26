package upgrade

import (
	"context"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// upgradeE2ESourceVersions are the released CLIs the upgrade test scaffolds
// with. Keep the previous pre-release and the oldest supported one.
var upgradeE2ESourceVersions = []string{
	"v2.0.0-alpha.4",
	"v2.0.0-alpha.5",
}

const upgradeE2ETargetVersion = "v2.0.0-beta.1"

// TestUpgradeToHEAD scaffolds with each released CLI in
// upgradeE2ESourceVersions, upgrades the result with this checkout, applies
// every manual action the upgrade reports, and requires the result to build and
// vet cleanly.
//
// The test downloads the released CLI and modules, so it only runs in nightly
// and release-readiness. Set ANDUREL_UPGRADE_E2E=1 to enable it.
//
// Set ANDUREL_UPGRADE_E2E_LOCAL_PKG=1 to replace every pkg/* module with this
// checkout after the upgrade, the way `just local-replace` does. Use it on a
// branch whose package changes are not published yet; the default variant
// stays on the published pins for nightly and release-readiness.
func TestUpgradeToHEAD(t *testing.T) {
	if os.Getenv("ANDUREL_UPGRADE_E2E") != "1" {
		t.Skip("upgrade end-to-end test requires ANDUREL_UPGRADE_E2E=1")
	}
	localPkg := os.Getenv("ANDUREL_UPGRADE_E2E_LOCAL_PKG") == "1"

	for _, source := range upgradeE2ESourceVersions {
		t.Run(source, func(t *testing.T) {
			testUpgradeToHEAD(t, source, localPkg)
		})
	}
}

func testUpgradeToHEAD(t *testing.T, source string, localPkg bool) {
	parent := t.TempDir()
	appName := "upgradee2e"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	scaffold := exec.CommandContext(
		ctx,
		"go",
		"run",
		"github.com/mbvlabs/andurel/v2@"+source,
		"new",
		appName,
	)
	scaffold.Dir = parent
	scaffold.Env = append(os.Environ(), "GOWORK=off")
	if output, err := scaffold.CombinedOutput(); err != nil {
		t.Fatalf("scaffold with %s: %v\n%s", source, err, output)
	}

	root := filepath.Join(parent, appName)
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "upgrade-e2e@example.com")
	gitRun(t, root, "config", "user.name", "Upgrade E2E")
	commitUpgradeTree(t, root, "scaffold from "+source)

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{TargetVersion: upgradeE2ETargetVersion},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("upgrade %s to %s: %v", source, upgradeE2ETargetVersion, err)
	}

	for _, action := range report.ManualActions {
		if err := applyUpgradeE2EManualAction(root, action); err != nil {
			t.Fatalf(
				"manual action %q (%s) was not applied: %v",
				action.ID,
				action.Title,
				err,
			)
		}
	}

	if localPkg {
		replaceWithLocalPackages(t, root)
	}

	runUpgradeE2EGo(t, root, "mod", "tidy")
	runUpgradeE2EGo(t, root, "build", "./...")
	runUpgradeE2EGo(t, root, "vet", "./...")
}

// replaceWithLocalPackages points every pkg/* module in the upgraded project at
// this checkout, mirroring `just local-replace`.
func replaceWithLocalPackages(t *testing.T, root string) {
	t.Helper()

	framework := localCheckoutRoot(t)
	moduleFiles, err := filepath.Glob(filepath.Join(framework, "pkg", "*", "go.mod"))
	if err != nil {
		t.Fatalf("find pkg modules: %v", err)
	}
	if len(moduleFiles) == 0 {
		t.Fatalf("no pkg modules under %s", framework)
	}
	sort.Strings(moduleFiles)

	for _, moduleFile := range moduleFiles {
		moduleDir := filepath.Dir(moduleFile)
		module := filepath.Base(moduleDir)
		replacement := "github.com/mbvlabs/andurel/pkg/" + module + "=" + moduleDir
		runUpgradeE2EGo(t, root, "mod", "edit", "-replace", replacement)
	}
}

// localCheckoutRoot returns the repository root for this checkout, which owns
// the pkg/* modules.
func localCheckoutRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve the upgrade e2e test file")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func applyUpgradeE2EManualAction(root string, action ManualAction) error {
	switch action.ID {
	case "package-api-v2.0.0-alpha.5":
		return applyUpgradeE2EPackageAPI(root)
	case "storage-storagetest-v2.0.0-alpha.5":
		return applyUpgradeE2EStorageTestCluster(root)
	case "inertia-vite-fullurl-v2.0.0":
		return applyUpgradeE2EInertiaFullURL(root)
	case "routing-hosts-v2.0.0-alpha.5":
		return applyUpgradeE2ERoutingHosts(root)
	case "session-revocation-v2.0.0-alpha.5":
		return applyUpgradeE2ESessionRevocation(root)
	case "email-smtp-v2.0.0-alpha.5":
		return applyUpgradeE2EEmailSMTP(root)
	case "validation-validate-v2.0.0-alpha.5":
		return applyUpgradeE2EValidation(root)
	default:
		return fmt.Errorf(
			"no automated e2e implementation for manual action %q (%s)",
			action.ID,
			action.Title,
		)
	}
}

// applyUpgradeE2EPackageAPI follows the package-api-v2.0.0-alpha.5
// instructions against internal/runtime, which is application-owned.
func applyUpgradeE2EPackageAPI(root string) error {
	path := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	replacements := []struct {
		description string
		old         string
		new         string
	}{
		{
			description: "anduin.NewServer drops the environment and shutdowner arguments",
			old: "\tsrv := anduin.NewServer(\n\t\tctx,\n\t\tcfg.HTTP.Host,\n" +
				"\t\tcfg.HTTP.Port,\n\t\tcfg.App.Environment,\n\t\thandler,\n\t\tnil,\n",
			new: "\tsrv := anduin.NewServer(\n\t\tctx,\n\t\tcfg.HTTP.Host,\n" +
				"\t\tcfg.HTTP.Port,\n\t\thandler,\n",
		},
		{
			description: "Server.Start takes no arguments",
			old:         "return a.Server.Start(ctx, a.Config.App.Environment)",
			new:         "return a.Server.Start()",
		},
		{
			description: "inertia.NewRenderer takes inertia.RendererConfig",
			old: "\treturn inertia.NewRenderer(\n\t\tcfg.Inertia.ContainerID,\n" +
				"\t\troutes.ViteBuild.FullURL(),\n\t\tcfg.Inertia.EntryPoint,\n" +
				"\t\tcfg.Inertia.ViteDevURL,\n\t\tcfg.Inertia.SSRURL,\n" +
				"\t\tcfg.Inertia.SSRRequestTimeout,\n\t\tcfg.Inertia.SSRMaxResponseBytes,\n",
			new: "\treturn inertia.NewRenderer(\n\t\tinertia.RendererConfig{\n" +
				"\t\t\tContainerID:         cfg.Inertia.ContainerID,\n" +
				"\t\t\tBuildPathURL:        routes.ViteBuild.FullURL(),\n" +
				"\t\t\tEntryPoint:          cfg.Inertia.EntryPoint,\n" +
				"\t\t\tViteDevURL:          cfg.Inertia.ViteDevURL,\n" +
				"\t\t\tSSRURL:              cfg.Inertia.SSRURL,\n" +
				"\t\t\tSSRTimeout:          cfg.Inertia.SSRRequestTimeout,\n" +
				"\t\t\tSSRMaxResponseBytes: cfg.Inertia.SSRMaxResponseBytes,\n" +
				"\t\t},\n",
		},
		{
			description: "cache.Store.Close runs on shutdown",
			old: "\t\tclosers: []func(context.Context) error{\n" +
				"\t\t\tfunc(context.Context) error { return db.Close() },\n" +
				"\t\t\ttel.Shutdown,\n\t\t},",
			new: "\t\tclosers: []func(context.Context) error{\n" +
				"\t\t\tfunc(context.Context) error { return db.Close() },\n" +
				"\t\t\tfunc(context.Context) error { return cacheStore.Close() },\n" +
				"\t\t\ttel.Shutdown,\n\t\t},",
		},
	}

	for _, replacement := range replacements {
		if !strings.Contains(string(content), replacement.old) {
			return fmt.Errorf(
				"%s: expected snippet not found in %s",
				replacement.description,
				path,
			)
		}
		content = []byte(strings.Replace(string(content), replacement.old, replacement.new, 1))
	}
	if err := writeGofmtFile(path, content); err != nil {
		return err
	}

	return applyUpgradeE2ESSRRuntime(root)
}

// applyUpgradeE2ESSRRuntime follows the same action's NewSSRRuntime change in
// cmd/ssr, another application-owned entrypoint.
func applyUpgradeE2ESSRRuntime(root string) error {
	path := filepath.Join(root, "cmd", "ssr", "main.go")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(content)
	old := "\truntime, err := inertia.NewSSRRuntime(\n\t\tcfg.SSRRuntime,\n" +
		"\t\tcfg.SSRBundle,\n\t\tcfg.SSRListen,\n\t\tcfg.SSRStartupTimeout,\n" +
		"\t\tcfg.SSRMinimumMajor,\n\t\tcfg.SSRRequestTimeout,\n\t\tassets.Files,\n\t)\n"
	if !strings.Contains(text, old) {
		return nil
	}
	new := "\truntime, err := inertia.NewSSRRuntime(inertia.SSRRuntimeConfig{\n" +
		"\t\tExecutable:     cfg.SSRRuntime,\n" +
		"\t\tBundlePath:     cfg.SSRBundle,\n" +
		"\t\tListenURL:      cfg.SSRListen,\n" +
		"\t\tStartupTimeout: cfg.SSRStartupTimeout,\n" +
		"\t\tMinimumMajor:   cfg.SSRMinimumMajor,\n" +
		"\t\tHealthTimeout:  cfg.SSRRequestTimeout,\n" +
		"\t\tBundleFS:       assets.Files,\n" +
		"\t})\n"
	return writeGofmtFile(path, []byte(strings.Replace(text, old, new, 1)))
}

// applyUpgradeE2EStorageTestCluster follows the
// storage-storagetest-v2.0.0-alpha.5 instructions. The alpha.4 scaffold has no
// test-cluster helper references, so this is normally a no-op.
func applyUpgradeE2EStorageTestCluster(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		updated := strings.ReplaceAll(text, "storage.TestCluster", "storagetest.Cluster")
		updated = strings.ReplaceAll(updated, "storage.NewTestCluster", "storagetest.NewCluster")
		if updated == text {
			return nil
		}
		return fmt.Errorf(
			"%s uses storage test-cluster helpers; update its imports by hand",
			path,
		)
	})
}

type upgradeE2EReplacement struct {
	description string
	old         string
	new         string
	all         bool
}

func upgradeE2EModulePath(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", fmt.Errorf("no module line in %s/go.mod", root)
}

func applyUpgradeE2EReplacements(path string, replacements []upgradeE2EReplacement) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)
	for _, replacement := range replacements {
		if !strings.Contains(text, replacement.old) {
			return fmt.Errorf(
				"%s: expected snippet not found in %s",
				replacement.description,
				path,
			)
		}
		if replacement.all {
			text = strings.ReplaceAll(text, replacement.old, replacement.new)
		} else {
			text = strings.Replace(text, replacement.old, replacement.new, 1)
		}
	}

	if !strings.HasSuffix(path, ".go") {
		return os.WriteFile(path, []byte(text), 0o644)
	}

	return writeGofmtFile(path, []byte(text))
}

// applyUpgradeE2ERoutingHosts follows the routing-hosts-v2.0.0-alpha.5
// instructions. It runs after the package API action, which rewrites
// newRenderer to the RendererConfig form.
func applyUpgradeE2ERoutingHosts(root string) error {
	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/kiks\"\n",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/kiks\"\n\t\"github.com/mbvlabs/andurel/pkg/routing\"\n",
		},
		{
			description: "build the host registry in New",
			old:         "func New(ctx context.Context, cfg config.Config) (*App, error) {\n\ttel, err := newTelemetry",
			new: "func New(ctx context.Context, cfg config.Config) (*App, error) {\n" +
				"\thosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
				"\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n\t}\n\n" +
				"\ttel, err := newTelemetry",
		},
		{
			description: "pass hosts to services.NewIdentity",
			old:         "\tidentity := services.NewIdentity(transactor, users, tokens, queueInsert, cfg.Auth, cfg.Mail)",
			new: "\tidentity := services.NewIdentity(\n\t\ttransactor,\n\t\tusers,\n\t\ttokens,\n" +
				"\t\tqueueInsert,\n\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t\thosts,\n\t)",
		},
		{
			description: "pass hosts to newRenderer",
			old:         "\trenderer, err := newRenderer(cfg)",
			new:         "\trenderer, err := newRenderer(cfg, hosts)",
		},
		{
			description: "install the hosts registry on the router",
			old:         "\trouter := anduin.NewRouter(\n\t\tanduin.WithErrorHandler(handlers.MapError(renderer)),",
			new:         "\trouter := anduin.NewRouter(\n\t\tanduin.WithHosts(hosts),\n\t\tanduin.WithErrorHandler(handlers.MapError(renderer)),",
		},
		{
			description: "newRenderer takes the hosts registry",
			old:         "func newRenderer(cfg config.Config) (*inertia.Renderer, error) {",
			new:         "func newRenderer(cfg config.Config, hosts *routing.Hosts) (*inertia.Renderer, error) {",
		},
		{
			description: "the Inertia build URL takes the hosts registry",
			old:         "routes.ViteBuild.FullURL()",
			new:         "routes.ViteBuild.FullURL(hosts)",
		},
	}); err != nil {
		return err
	}

	queuePath := filepath.Join(root, "internal", "runtime", "queue.go")
	if err := applyUpgradeE2EReplacements(queuePath, []upgradeE2EReplacement{
		{
			description: "import fmt",
			old:         "\t\"context\"\n\t\"errors\"\n\t\"slices\"",
			new:         "\t\"context\"\n\t\"errors\"\n\t\"fmt\"\n\t\"slices\"",
		},
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/routing\"\n\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
		},
		{
			description: "build the host registry in NewQueue",
			old:         "func NewQueue(ctx context.Context, cfg config.Config) (*Queue, error) {\n\ttel, err := newTelemetry",
			new: "func NewQueue(ctx context.Context, cfg config.Config) (*Queue, error) {\n" +
				"\thosts, err := routing.NewHosts(cfg.App.Hosts)\n" +
				"\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"runtime: configure hosts: %w\", err)\n\t}\n\n" +
				"\ttel, err := newTelemetry",
		},
		{
			description: "pass hosts to services.NewIdentity",
			old:         "\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t)",
			new:         "\t\tcfg.Auth,\n\t\tcfg.Mail,\n\t\thosts,\n\t)",
		},
	}); err != nil {
		return err
	}

	identityPath := filepath.Join(root, "services", "identity.go")
	if err := applyUpgradeE2EReplacements(identityPath, []upgradeE2EReplacement{
		{
			description: "import the routing package",
			old:         "\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
			new:         "\t\"github.com/mbvlabs/andurel/pkg/routing\"\n\t\"github.com/mbvlabs/andurel/pkg/storage\"\n)",
		},
		{
			description: "store the hosts registry on Identity",
			old:         "\tdefaultSenderSignature string\n}",
			new:         "\tdefaultSenderSignature string\n\thosts                  *routing.Hosts\n}",
		},
		{
			description: "NewIdentity takes the hosts registry",
			old:         "\tmailCfg config.Mail,\n) Identity {",
			new:         "\tmailCfg config.Mail,\n\thosts *routing.Hosts,\n) Identity {",
		},
		{
			description: "assign the hosts registry",
			old:         "\t\tdefaultSenderSignature: mailCfg.DefaultSenderSignature,\n\t}",
			new:         "\t\tdefaultSenderSignature: mailCfg.DefaultSenderSignature,\n\t\thosts:                  hosts,\n\t}",
		},
	}); err != nil {
		return err
	}

	resetPath := filepath.Join(root, "services", "reset_password.go")
	if err := applyUpgradeE2EReplacements(resetPath, []upgradeE2EReplacement{
		{
			description: "the reset URL takes the hosts registry",
			old:         "routes.PasswordEdit.FullURL(token)",
			new:         "routes.PasswordEdit.FullURL(i.hosts, token)",
		},
	}); err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "config", "app.go"), []upgradeE2EReplacement{
		{
			description: "remove the package-level ConfigureHosts call",
			old: "\tif err := routing.ConfigureHosts(cfg.Hosts); err != nil {\n" +
				"\t\treturn App{}, fmt.Errorf(\"config: app: %w\", err)\n\t}\n\n\treturn cfg, nil",
			new: "\treturn cfg, nil",
		},
	})
}

// applyUpgradeE2ESessionRevocation follows the
// session-revocation-v2.0.0-alpha.5 instructions: migration, queries, domain,
// cookie and AuthOnly.
func applyUpgradeE2ESessionRevocation(root string) error {
	migration := `-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN session_version;
-- +goose StatementEnd
`
	migrationPath := filepath.Join(
		root,
		"migrations",
		"20261026000000_add_session_version_to_users.sql",
	)
	if err := os.WriteFile(migrationPath, []byte(migration), 0o644); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "domains", "user.sql"), []upgradeE2EReplacement{
		{
			description: "add session_version to CreateUser",
			old:         "\tpassword,\n\tis_admin\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7\n)",
			new:         "\tpassword,\n\tis_admin,\n\tsession_version\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7, $8\n)",
		},
		{
			description: "add session_version to UpdateUser",
			old:         "\tis_admin = $5,\n\tupdated_at = $6\nWHERE id = $1\nRETURNING *;",
			new:         "\tis_admin = $5,\n\tsession_version = $6,\n\tupdated_at = $7\nWHERE id = $1\nRETURNING *;",
		},
		{
			description: "add the BumpSessionVersion query",
			old:         "-- name: DeleteUser :exec\nDELETE FROM users\nWHERE id = $1;\n",
			new: "-- name: DeleteUser :exec\nDELETE FROM users\nWHERE id = $1;\n\n" +
				"-- name: BumpSessionVersion :exec\nUPDATE users\nSET session_version = session_version + 1\nWHERE id = $1;\n",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(
		filepath.Join(root, "domains", "internal", "queries", "models.go"),
		[]upgradeE2EReplacement{
			{
				description: "add SessionVersion to UserRow",
				old:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n}",
				new:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n\tSessionVersion   int32              `andurel:\"session_version\"`\n}",
			},
		},
	); err != nil {
		return err
	}

	columnsOld := `[]string{"id", "created_at", "updated_at", "email", "email_validated_at", "password", "is_admin"}`
	columnsNew := `[]string{"id", "created_at", "updated_at", "email", "email_validated_at", "password", "is_admin", "session_version"}`
	if err := applyUpgradeE2EReplacements(
		filepath.Join(root, "domains", "internal", "queries", "user.sql.go"),
		[]upgradeE2EReplacement{
			{
				description: "add the generated BumpSessionVersion query",
				old:         "const countUsers = `-- name: CountUsers :one",
				new: "const bumpSessionVersion = `-- name: BumpSessionVersion :exec\n" +
					"UPDATE users\nSET session_version = session_version + 1\nWHERE id = $1\n`\n\n" +
					"func (q *Queries) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
					"\t_, err := q.db.Exec(ctx, bumpSessionVersion, id)\n\treturn err\n}\n\n" +
					"const countUsers = `-- name: CountUsers :one",
			},
			{
				description: "add session_version to the generated CreateUser SQL",
				old: "\tpassword,\n\tis_admin\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7\n)\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin",
				new: "\tpassword,\n\tis_admin,\n\tsession_version\n) VALUES (\n\t$1, $2, $3, $4, $5, $6, $7, $8\n)\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin, session_version",
			},
			{
				description: "add SessionVersion to CreateUserParams",
				old:         "\tPassword         []byte\n\tIsAdmin          bool\n}\n\nfunc (q *Queries) CreateUser",
				new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n}\n\nfunc (q *Queries) CreateUser",
			},
			{
				description: "pass SessionVersion to CreateUser",
				old:         "\t\targ.IsAdmin,\n\t)\n\treturn narsilc.Read[T, UserRow]",
				new:         "\t\targ.IsAdmin,\n\t\targ.SessionVersion,\n\t)\n\treturn narsilc.Read[T, UserRow]",
			},
			{
				description: "select session_version in generated reads",
				old:         ", is_admin\nFROM users",
				new:         ", is_admin, session_version\nFROM users",
				all:         true,
			},
			{
				description: "read session_version in generated read helpers",
				old:         columnsOld,
				new:         columnsNew,
				all:         true,
			},
			{
				description: "add session_version to the generated UpdateUser SQL",
				old: "\tis_admin = $5,\n\tupdated_at = $6\nWHERE id = $1\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin",
				new: "\tis_admin = $5,\n\tsession_version = $6,\n\tupdated_at = $7\nWHERE id = $1\n" +
					"RETURNING id, created_at, updated_at, email, email_validated_at, password, is_admin, session_version",
			},
			{
				description: "add SessionVersion to UpdateUserParams",
				old:         "\tPassword         []byte\n\tIsAdmin          bool\n\tUpdatedAt        pgtype.Timestamptz",
				new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n\tUpdatedAt        pgtype.Timestamptz",
			},
			{
				description: "pass SessionVersion to UpdateUser",
				old:         "\t\targ.IsAdmin,\n\t\targ.UpdatedAt,",
				new:         "\t\targ.IsAdmin,\n\t\targ.SessionVersion,\n\t\targ.UpdatedAt,",
			},
		},
	); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "domains", "user.go"), []upgradeE2EReplacement{
		{
			description: "add SessionVersion to User",
			old:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n}",
			new:         "\tIsAdmin          bool               `andurel:\"is_admin\"`\n\tSessionVersion   int32              `andurel:\"session_version\"`\n}",
		},
		{
			description: "default new users to session version 1",
			old:         "\t\tIsAdmin:          false,\n\t}",
			new:         "\t\tIsAdmin:          false,\n\t\tSessionVersion:   1,\n\t}",
		},
		{
			description: "pass SessionVersion on create",
			old:         "\t\tIsAdmin:          entity.IsAdmin,\n\t})",
			new:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tSessionVersion:   entity.SessionVersion,\n\t})",
		},
		{
			description: "add SessionVersion to UpdateUserData",
			old:         "\tPassword         []byte\n\tIsAdmin          bool\n}",
			new:         "\tPassword         []byte\n\tIsAdmin          bool\n\tSessionVersion   int32\n}",
		},
		{
			description: "carry SessionVersion through Update",
			old:         "\t\tIsAdmin:          data.IsAdmin,\n\t}",
			new:         "\t\tIsAdmin:          data.IsAdmin,\n\t\tSessionVersion:   data.SessionVersion,\n\t}",
		},
		{
			description: "pass SessionVersion to UpdateUser",
			old:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tUpdatedAt:        entity.UpdatedAt,",
			new:         "\t\tIsAdmin:          entity.IsAdmin,\n\t\tSessionVersion:   entity.SessionVersion,\n\t\tUpdatedAt:        entity.UpdatedAt,",
		},
		{
			description: "bump the session version when the password changes",
			old: "	return u.Update(ctx, UpdateUserData{\n" +
				"\t\tID:               current.ID,\n\t\tEmail:            current.Email,\n" +
				"\t\tEmailValidatedAt: current.EmailValidatedAt,\n\t\tPassword:         []byte(hashedPassword),\n" +
				"\t\tIsAdmin:          current.IsAdmin,\n\t})\n}",
			new: "	updated, err := u.Update(ctx, UpdateUserData{\n" +
				"\t\tID:               current.ID,\n\t\tEmail:            current.Email,\n" +
				"\t\tEmailValidatedAt: current.EmailValidatedAt,\n\t\tPassword:         []byte(hashedPassword),\n" +
				"\t\tIsAdmin:          current.IsAdmin,\n\t\tSessionVersion:   current.SessionVersion,\n\t})\n" +
				"\tif err != nil {\n\t\treturn User{}, err\n\t}\n\n" +
				"\tif err := u.BumpSessionVersion(ctx, id); err != nil {\n\t\treturn User{}, err\n\t}\n" +
				"\tupdated.SessionVersion++\n\n\treturn updated, nil\n}\n\n" +
				"func (u Users) BumpSessionVersion(ctx context.Context, id uuid.UUID) error {\n" +
				"\treturn u.queries.BumpSessionVersion(ctx, id)\n}",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "middleware", "cookies.go"), []upgradeE2EReplacement{
		{
			description: "replace IsAdmin with SessionVersion in the cookie",
			old:         "\tIsAdmin         bool   `json:\"is_admin,omitempty\"`",
			new:         "\tSessionVersion  int32  `json:\"session_version,omitempty\"`",
		},
	}); err != nil {
		return err
	}

	if err := applyUpgradeE2EReplacements(filepath.Join(root, "handlers", "auth.go"), []upgradeE2EReplacement{
		{
			description: "store the session version instead of IsAdmin",
			old:         "\t\tIsAdmin:         entity.IsAdmin,",
			new:         "\t\tSessionVersion:  entity.SessionVersion,",
			all:         true,
		},
	}); err != nil {
		return err
	}

	modulePath, err := upgradeE2EModulePath(root)
	if err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "middleware", "auth.go"), []upgradeE2EReplacement{
		{
			description: "import errors and uuid for AuthOnly",
			old:         "\t\"context\"\n\t\"net/http\"\n\t\"strconv\"\n\t\"time\"\n",
			new:         "\t\"context\"\n\t\"errors\"\n\t\"net/http\"\n\t\"strconv\"\n\t\"time\"\n\t\"uuid\"\n",
		},
		{
			description: "import domains for AuthOnly",
			old:         "\t\"" + modulePath + "/routes\"",
			new:         "\t\"" + modulePath + "/domains\"\n\t\"" + modulePath + "/routes\"",
		},
		{
			description: "replace AuthOnly with the database-backed version",
			old: "func AuthOnly() anduin.Middleware {\n" +
				"\treturn func(next anduin.Handler) anduin.Handler {\n" +
				"\t\treturn func(ctx context.Context) error {\n" +
				"\t\t\tapp, err := kiks.Get[*App](ctx)\n" +
				"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
				"\t\t\tif app != nil && app.IsAuthenticated {\n\t\t\t\treturn next(ctx)\n\t\t\t}\n\n" +
				"\t\t\treturn anduin.Redirect(ctx, routes.SessionNew.URL(), http.StatusSeeOther)\n" +
				"\t\t}\n\t}\n}",
			new: authOnlyE2EImplementation(),
		},
	})
}

func authOnlyE2EImplementation() string {
	return "func AuthOnly(users domains.Users) anduin.Middleware {\n" +
		"\treturn func(next anduin.Handler) anduin.Handler {\n" +
		"\t\treturn func(ctx context.Context) error {\n" +
		"\t\t\tapp, err := kiks.Get[*App](ctx)\n" +
		"\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n" +
		"\t\t\tif app == nil || !app.IsAuthenticated {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\tuserID, err := uuid.Parse(app.UserID)\n" +
		"\t\t\tif err != nil {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\tuser, err := users.Find(ctx, userID)\n" +
		"\t\t\tif err != nil {\n" +
		"\t\t\t\tif errors.Is(err, domains.ErrNotFound) {\n\t\t\t\t\treturn rejectSession(ctx)\n\t\t\t\t}\n\n" +
		"\t\t\t\treturn err\n\t\t\t}\n\n" +
		"\t\t\tif user.SessionVersion != app.SessionVersion {\n\t\t\t\treturn rejectSession(ctx)\n\t\t\t}\n\n" +
		"\t\t\treturn next(WithViewer(ctx, user))\n\t\t}\n\t}\n}\n\n" +
		"func rejectSession(ctx context.Context) error {\n" +
		"\tif err := kiks.Destroy[*App](ctx); err != nil {\n\t\treturn err\n\t}\n\n" +
		"\treturn anduin.Redirect(ctx, routes.SessionNew.URL(), http.StatusSeeOther)\n}\n\n" +
		"type Viewer struct {\n\tID      uuid.UUID\n\tIsAdmin bool\n}\n\ntype viewerKey struct{}\n\n" +
		"func WithViewer(ctx context.Context, user domains.User) context.Context {\n" +
		"\treturn context.WithValue(ctx, viewerKey{}, Viewer{ID: user.ID, IsAdmin: user.IsAdmin})\n}\n\n" +
		"func ViewerFrom(ctx context.Context) (Viewer, bool) {\n" +
		"\tviewer, ok := ctx.Value(viewerKey{}).(Viewer)\n\n\treturn viewer, ok\n}"
}

func applyUpgradeE2EEmailSMTP(root string) error {
	if err := applyUpgradeE2EReplacements(filepath.Join(root, "config", "email.go"), []upgradeE2EReplacement{
		{
			description: "add the SMTP driver constants",
			old:         "\tMailpitDriver MailDriver = \"mailpit\"\n\n\tDefaultMailpitHost = \"0.0.0.0\"\n\tDefaultMailpitPort = \"1025\"",
			new: "\tMailpitDriver MailDriver = \"mailpit\"\n\tSMTPDriver    MailDriver = \"smtp\"\n\n" +
				"\tDefaultMailpitHost = \"0.0.0.0\"\n\tDefaultMailpitPort = \"1025\"\n" +
				"\tDefaultSMTPPort    = \"587\"\n\tDefaultSMTPTLS     = \"starttls\"",
		},
		{
			description: "add the SMTP fields to Mail",
			old:         "\tHost                   string\n\tPort                   string\n}",
			new: "\tHost                   string\n\tPort                   string\n" +
				"\tSMTPHost               string\n\tSMTPPort               string\n" +
				"\tSMTPUsername           string\n\tSMTPPassword           string\n\tSMTPTLS                string\n}",
		},
		{
			description: "read the SMTP environment",
			old:         "\t\tPort:                   env.String(\"MAILPIT_PORT\", DefaultMailpitPort),\n\t}",
			new: "\t\tPort:                   env.String(\"MAILPIT_PORT\", DefaultMailpitPort),\n" +
				"\t\tSMTPHost:               env.String(\"SMTP_HOST\", \"\"),\n" +
				"\t\tSMTPPort:               env.String(\"SMTP_PORT\", DefaultSMTPPort),\n" +
				"\t\tSMTPUsername:           env.String(\"SMTP_USERNAME\", \"\"),\n" +
				"\t\tSMTPPassword:           env.String(\"SMTP_PASSWORD\", \"\"),\n" +
				"\t\tSMTPTLS:                env.String(\"SMTP_TLS\", DefaultSMTPTLS),\n\t}",
		},
	}); err != nil {
		return err
	}

	oldSenders := "func newEmailSenders(\n\tcfg config.Mail,\n" +
		") (email.TransactionalSender, email.MarketingSender, error) {\n" +
		"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n" +
		"\t\tclient, err := email.NewMailpit(email.MailpitConfig{\n" +
		"\t\t\tHost: cfg.Host,\n\t\t\tPort: cfg.Port,\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create Mailpit client: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tdefault:\n" +
		"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}"
	newSenders := "func newEmailSenders(\n\tcfg config.Mail,\n\tapp config.App,\n" +
		") (email.TransactionalSender, email.MarketingSender, error) {\n" +
		"\tif err := validateEmailConfig(cfg, app); err != nil {\n\t\treturn nil, nil, err\n\t}\n\n" +
		"\tswitch cfg.Driver {\n\tcase config.MailpitDriver:\n" +
		"\t\tclient, err := email.NewMailpit(email.MailpitConfig{\n" +
		"\t\t\tHost: cfg.Host,\n\t\t\tPort: cfg.Port,\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create Mailpit client: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tcase config.SMTPDriver:\n" +
		"\t\tclient, err := email.NewSMTP(email.SMTPConfig{\n" +
		"\t\t\tHost:     cfg.SMTPHost,\n\t\t\tPort:     cfg.SMTPPort,\n" +
		"\t\t\tUsername: cfg.SMTPUsername,\n\t\t\tPassword: cfg.SMTPPassword,\n" +
		"\t\t\tTLS:      email.SMTPTLSMode(cfg.SMTPTLS),\n\t\t})\n" +
		"\t\tif err != nil {\n\t\t\treturn nil, nil, fmt.Errorf(\"create SMTP sender: %w\", err)\n\t\t}\n\n" +
		"\t\treturn client, client, nil\n\tdefault:\n" +
		"\t\treturn nil, nil, fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n}\n\n" +
		"func validateEmailConfig(cfg config.Mail, app config.App) error {\n\tswitch cfg.Driver {\n" +
		"\tcase config.MailpitDriver:\n\t\tif app.IsProduction() {\n" +
		"\t\t\treturn fmt.Errorf(\n\t\t\t\t\"email provider %q is for local development; set EMAIL_PROVIDER=%s in production\",\n" +
		"\t\t\t\tconfig.MailpitDriver,\n\t\t\t\tconfig.SMTPDriver,\n\t\t\t)\n\t\t}\n" +
		"\tcase config.SMTPDriver:\n\tdefault:\n" +
		"\t\treturn fmt.Errorf(\"unsupported email provider %q\", cfg.Driver)\n\t}\n\n\treturn nil\n}"

	runtimePath := filepath.Join(root, "internal", "runtime", "runtime.go")
	if err := applyUpgradeE2EReplacements(runtimePath, []upgradeE2EReplacement{
		{
			description: "newEmailSenders takes config.App and supports SMTP",
			old:         oldSenders,
			new:         newSenders,
		},
	}); err != nil {
		return err
	}

	return applyUpgradeE2EReplacements(filepath.Join(root, "internal", "runtime", "queue.go"), []upgradeE2EReplacement{
		{
			description: "pass config.App to newEmailSenders",
			old:         "newEmailSenders(cfg.Mail)",
			new:         "newEmailSenders(cfg.Mail, cfg.App)",
		},
	})
}

func applyUpgradeE2EValidation(root string) error {
	for _, name := range []string{"user.go", "token.go"} {
		path := filepath.Join(root, "domains", name)
		if err := applyUpgradeE2EReplacements(path, []upgradeE2EReplacement{
			{
				description: "call the domain Validate method",
				old:         "if err := validation.Validate(&entity); err != nil {",
				new:         "if err := entity.Validate(); err != nil {",
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

func applyUpgradeE2EInertiaFullURL(root string) error {
	path := filepath.Join(root, "internal", "runtime", "runtime.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(
		string(content),
		"routes.ViteBuild.Path()",
		"routes.ViteBuild.FullURL()",
	)
	if updated == string(content) {
		return nil
	}
	return writeGofmtFile(path, []byte(updated))
}

func writeGofmtFile(path string, content []byte) error {
	formatted, err := format.Source(content)
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func runUpgradeE2EGo(t *testing.T, dir string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
