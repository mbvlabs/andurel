package upgrade

import (
	"strings"
	"testing"
)

func TestSessionRecoveryManualActionVersionGate(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{name: "first affected upgrade", from: "v1.5.2", to: "v1.5.3", want: true},
		{name: "skips directly over release", from: "v1.4.0", to: "v1.6.0", want: true},
		{name: "already received note", from: "v1.5.3", to: "v1.6.0", want: false},
		{name: "target predates note", from: "v1.5.1", to: "v1.5.2", want: false},
		{name: "development version", from: "dev", to: "v1.5.3", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions, err := manualActionsForUpgrade(test.from, test.to, "example.com/acme")
			if err != nil {
				t.Fatalf("manualActionsForUpgrade returned an error: %v", err)
			}
			if got := len(actions) > 0; got != test.want {
				t.Fatalf("manual action present = %t, want %t: %#v", got, test.want, actions)
			}
			if !test.want {
				return
			}

			action := actions[0]
			if action.ID != "session-cookie-recovery-v1.5.3" {
				t.Fatalf("manual action ID = %q", action.ID)
			}
			for _, want := range []string{
				"Create router/cookies/session.go",
				`"example.com/acme/config"`,
				"cookies.RecoverInvalidSessions(c)",
				"github.com/gorilla/securecookie v1.1.2",
				"session.Get",
			} {
				if !strings.Contains(action.Instructions, want) {
					t.Errorf("manual action missing %q:\n%s", want, action.Instructions)
				}
			}
			for _, unwanted := range []string{"{{.ModuleName}}", "https://"} {
				if strings.Contains(action.Instructions, unwanted) {
					t.Errorf("manual action contains %q:\n%s", unwanted, action.Instructions)
				}
			}
		})
	}
}
