package upgrade

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func TestInertiaViteFullURLManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "first affected upgrade", from: "v2.0.0-alpha.2", to: "v2.0.0-alpha.3", want: true},
		{name: "skips directly over release", from: "v2.0.0-alpha", to: "v2.1.0", want: true},
		{name: "alpha before the change", from: "v2.0.0-alpha", to: "v2.0.0-alpha.4", want: true},
		{name: "second alpha before the change", from: "v2.0.0-alpha.2", to: "v2.0.0", want: true},
		{name: "alpha that already has the change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "latest alpha to stable", from: "v2.0.0-alpha.3", to: "v2.0.0", want: false},
		{name: "already received note", from: "v2.0.0", to: "v2.1.0", want: false},
		{name: "target predates release", from: "v2.0.0-alpha", to: "v2.0.0-alpha.2", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, true)
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}

			var action *ManualAction
			for i := range actions {
				if actions[i].ID == "inertia-vite-fullurl-v2.0.0" {
					action = &actions[i]
					break
				}
			}
			if got := action != nil; got != test.want {
				t.Fatalf(
					"Inertia Vite FullURL action present = %t, want %t: %#v",
					got,
					test.want,
					actions,
				)
			}
			if !test.want {
				return
			}

			for _, want := range []string{
				"NewInertia",
				"routes.ViteBuild.FullURL()",
				"Path()",
				"gofmt -w internal/runtime",
				"go fix ./...",
				"go vet ./...",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
			if strings.Contains(action.Instructions, "{{.ModuleName}}") {
				t.Errorf(
					"manual action contains an unresolved module placeholder:\n%s",
					action.Instructions,
				)
			}
		})
	}
}

func TestLatestBetaUpgradeToStableEmitsNoManualActions(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-beta.3", "v2.0.0", true)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("latest beta to stable emitted manual actions: %#v", actions)
	}
}

func TestBetaThreeManualActionsVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "beta.2 to beta.3", from: "v2.0.0-beta.2", to: "v2.0.0-beta.3", want: true},
		{name: "beta.2 to stable", from: "v2.0.0-beta.2", to: "v2.0.0", want: true},
		{name: "already on beta.3", from: "v2.0.0-beta.3", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-beta.1", to: "v2.0.0-beta.2", want: false},
	}

	ids := []string{
		"cors-host-origins-v2.0.0-beta.3",
		"csrf-asset-skip-v2.0.0-beta.3",
		"method-not-allowed-page-v2.0.0-beta.3",
		"request-host-middleware-v2.0.0-beta.3",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, true)
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}

			got := map[string]bool{}
			for _, action := range actions {
				got[action.ID] = true
			}
			for _, id := range ids {
				if got[id] != test.want {
					t.Errorf("action %s present = %t, want %t", id, got[id], test.want)
				}
			}
		})
	}

	for id, want := range map[string][]string{
		"cors-host-origins-v2.0.0-beta.3": {
			"anduin.MergeOrigins(cfg.App.Origins(), cfg.HTTP.CORSAllowedOrigins)...",
			"anduin.DefaultCORS(cfg.HTTP.CORSAllowedOrigins...)",
			"cors.Skipper = isViteAssetRequest",
			"AllowCredentials: false",
			"cfg.App.Origins() in the CSRF trusted origins",
			"security-relevant",
		},
		"csrf-asset-skip-v2.0.0-beta.3": {
			"bearerSkipper(ctx) || isSafeViteAssetRequest(ctx)",
			"Unsafe requests to static paths stay protected",
		},
		"method-not-allowed-page-v2.0.0-beta.3": {
			"anduin.ErrMethodNotAllowed",
			`"Errors/NotFound"`,
			"Allow header",
		},
		"request-host-middleware-v2.0.0-beta.3": {
			"anduin.RequestHost(ctx)",
			"middleware.DenyFraming(config.HostAdmin)",
			"routing.HostPrimary",
		},
	} {
		action := findManualAction(t, "v2.0.0-beta.2", "v2.0.0-beta.3", id)
		if action == nil {
			t.Fatalf("missing manual action %s", id)
		}
		for _, text := range want {
			if !strings.Contains(action.Instructions, text) {
				t.Errorf("%s instructions missing %q:\n%s", id, text, action.Instructions)
			}
		}
	}
}

func TestBetaTwoManualActionsVersionGate(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		inertia bool
		want    []string
	}{
		{
			name:    "beta.1 to beta.2",
			from:    "v2.0.0-beta.1",
			to:      "v2.0.0-beta.2",
			inertia: true,
			want: []string{
				"email-sender-v2.0.0-beta.2",
				"vite-ssr-bundle-v2.0.0-beta.2",
				"root-title-ssr-v2.0.0-beta.2",
			},
		},
		{
			name: "non-Inertia project",
			from: "v2.0.0-beta.1",
			to:   "v2.0.0-beta.2",
			want: []string{"email-sender-v2.0.0-beta.2"},
		},
		{name: "already on beta.2", from: "v2.0.0-beta.2", to: "v2.0.0", inertia: true},
		{name: "target predates change", from: "v2.0.0-alpha.5", to: "v2.0.0-beta.1", inertia: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, test.inertia)
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}

			var got []string
			for _, action := range actions {
				if strings.HasSuffix(action.ID, "-v2.0.0-beta.2") {
					got = append(got, action.ID)
				}
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("beta.2 actions = %v, want %v", got, test.want)
			}
		})
	}

	for id, want := range map[string][]string{
		"email-sender-v2.0.0-beta.2":    {"senderHost(app.Domain)", "net.SplitHostPort", "mail.ParseAddress", "go vet ./..."},
		"vite-ssr-bundle-v2.0.0-beta.2": {"vite.config.ts", "command === 'build' ? { noExternal: true } : undefined", "andurel run"},
		"root-title-ssr-v2.0.0-beta.2":  {"resources/root.templ", "if data.SSR == nil", "andurel build"},
	} {
		action := findManualAction(t, "v2.0.0-beta.1", "v2.0.0-beta.2", id)
		if action == nil {
			t.Fatalf("missing manual action %s", id)
		}
		for _, text := range want {
			if !strings.Contains(action.Instructions, text) {
				t.Errorf("%s instructions missing %q:\n%s", id, text, action.Instructions)
			}
		}
	}
}

func TestPackageAPIManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		inertia bool
		want    bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", inertia: true, want: true},
		{name: "non-Inertia project", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", inertia: true, want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", inertia: true, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, test.inertia)
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}

			var action *ManualAction
			for i := range actions {
				if actions[i].ID == "package-api-v2.0.0-alpha.5" {
					action = &actions[i]
				}
			}
			if got := action != nil; got != test.want {
				t.Fatalf("package API action present = %t, want %t: %#v", got, test.want, actions)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"anduin.NewServer",
				"cacheStore.Close()",
				"storage.AtomicOps",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
			if got := strings.Contains(action.Instructions, "inertia.RendererConfig"); got != test.inertia {
				t.Errorf("Inertia instructions present = %t, want %t", got, test.inertia)
			}
			if got := strings.Contains(action.Instructions, "inertia.SSRRuntimeConfig"); got != test.inertia {
				t.Errorf("Inertia SSR instructions present = %t, want %t", got, test.inertia)
			}
		})
	}
}

func TestStorageTestClusterManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.4 to stable", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, true)
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}

			var action *ManualAction
			for i := range actions {
				if actions[i].ID == "storage-storagetest-v2.0.0-alpha.5" {
					action = &actions[i]
				}
			}
			if got := action != nil; got != test.want {
				t.Fatalf("storage storagetest action present = %t, want %t: %#v", got, test.want, actions)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"pkg/storage/storagetest",
				"storagetest.NewCluster",
				"storagetest.Cluster",
				"storage.NewTestCluster",
				"NewTestDB",
				"pkg/storage " + versions.Storage,
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
		})
	}
}

func TestRoutingHostsManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.4 to stable", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := findManualAction(t, test.from, test.to, "routing-hosts-v2.0.0-alpha.5")
			if got := action != nil; got != test.want {
				t.Fatalf("routing hosts action present = %t, want %t", got, test.want)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"routing.ConfigureHosts",
				"routing.NewHosts(cfg.App.Hosts)",
				"anduin.WithHosts(hosts)",
				"routes.ViteBuild.FullURL(hosts)",
				"services.NewIdentity",
				"BuildPathURL",
				"andurel patterns show composition",
				"go vet ./...",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
		})
	}
}

func TestSessionRevocationManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.4 to stable", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := findManualAction(t, test.from, test.to, "session-revocation-v2.0.0-alpha.5")
			if got := action != nil; got != test.want {
				t.Fatalf("session revocation action present = %t, want %t", got, test.want)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"andurel generate migration add_session_version_to_users",
				"session_version INTEGER NOT NULL DEFAULT 1",
				"andurel sync queries",
				"BumpSessionVersion",
				"middleware.AuthOnly(users)",
				"SessionVersion  int32",
				"andurel patterns show domain",
				"andurel patterns show composition",
				"keeps building without it",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
		})
	}
}

func TestEmailSMTPManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.4 to stable", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := findManualAction(t, test.from, test.to, "email-smtp-v2.0.0-alpha.5")
			if got := action != nil; got != test.want {
				t.Fatalf("email SMTP action present = %t, want %t", got, test.want)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"config.App",
				"newEmailSenders(cfg.Mail, cfg.App)",
				"email.NewSMTP",
				"EMAIL_PROVIDER=smtp",
				"optional",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
		})
	}
}

func TestValidationManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "alpha.4 to alpha.5", from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", want: true},
		{name: "alpha.4 to stable", from: "v2.0.0-alpha.4", to: "v2.0.0", want: true},
		{name: "already on alpha.5", from: "v2.0.0-alpha.5", to: "v2.0.0", want: false},
		{name: "target predates change", from: "v2.0.0-alpha.3", to: "v2.0.0-alpha.4", want: false},
		{name: "development version", from: "dev", to: "v2.0.0", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := findManualAction(t, test.from, test.to, "validation-validate-v2.0.0-alpha.5")
			if got := action != nil; got != test.want {
				t.Fatalf("validation action present = %t, want %t", got, test.want)
			}
			if action == nil {
				return
			}

			for _, want := range []string{
				"validation.Validate",
				"entity.Validate()",
				"domains/user.go",
				"domains/token.go",
				"andurel patterns show domain",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
		})
	}
}

func findManualAction(t *testing.T, from, to, id string) *ManualAction {
	t.Helper()

	actions, err := manualActionsForUpgrade(from, to, true)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
	}

	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}

	return nil
}

// TestAlpha4UpgradeReportsAlpha5ManualActions guards the plan wiring: the
// alpha.5 package and storagetest actions must reach the report, not just
// manualActionsForUpgrade. alpha.4 is the first version that can be upgraded
// past the Go-native rebuild.
func TestAlpha4UpgradeReportsAlpha5ManualActions(t *testing.T) {
	root := newUpgradeFixtureProjectWithConfig(t, layout.ScaffoldConfig{
		ProjectName: "testapp",
		Inertia:     "react",
	})
	lock, err := layout.ReadLockFile(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Version = "v2.0.0-alpha.4"
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatal(err)
	}
	commitUpgradeTree(t, root, "move fixture to alpha.4")

	upgrader, err := NewUpgrader(
		root,
		UpgradeOptions{DryRun: true, TargetVersion: "v2.0.0-alpha.5"},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, action := range report.ManualActions {
		got[action.ID] = true
	}
	for _, want := range []string{
		"package-api-v2.0.0-alpha.5",
		"storage-storagetest-v2.0.0-alpha.5",
		"routing-hosts-v2.0.0-alpha.5",
		"session-revocation-v2.0.0-alpha.5",
		"email-smtp-v2.0.0-alpha.5",
		"validation-validate-v2.0.0-alpha.5",
	} {
		if !got[want] {
			t.Fatalf("alpha.4 to alpha.5 report is missing %q: %#v", want, report.ManualActions)
		}
	}
	if got["inertia-vite-fullurl-v2.0.0"] {
		t.Fatalf("alpha.4 already has the Inertia FullURL change: %#v", report.ManualActions)
	}
}

func TestInertiaManualActionSkippedForNonInertiaProjects(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-alpha", "v2.0.0", false)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
	}
	for _, action := range actions {
		if action.ID == "inertia-vite-fullurl-v2.0.0" {
			t.Fatalf("non-Inertia project emitted the Inertia action: %#v", actions)
		}
	}
}
