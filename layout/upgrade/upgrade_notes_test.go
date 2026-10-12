package upgrade

import (
	"strings"
	"testing"
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
				if len(actions) != 0 {
					t.Fatalf("expected no manual actions, got %#v", actions)
				}
				return
			}

			for _, want := range []string{
				"NewInertia",
				"routes.ViteBuild.FullURL()",
				"Path()",
				"gofmt -w internal/runtime/app.go",
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

func TestLatestAlphaUpgradeToStableEmitsNoManualActions(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-alpha.3", "v2.0.0", true)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("latest alpha to stable emitted manual actions: %#v", actions)
	}
}

func TestInertiaManualActionSkippedForNonInertiaProjects(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-alpha", "v2.0.0", false)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("non-Inertia project emitted manual actions: %#v", actions)
	}
}
