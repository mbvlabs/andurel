package upgrade

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

// TestPreReleaseUpgradesEmitNoManualActions guards the policy that upgrade
// notes only exist between proper releases.
func TestPreReleaseUpgradesEmitNoManualActions(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		inertia  bool
	}{
		{from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", inertia: true},
		{from: "v2.0.0-alpha.4", to: "v2.0.0-beta.5", inertia: true},
		{from: "v2.0.0-beta.4", to: "v2.0.0-beta.5", inertia: false},
		{from: "v2.0.0-beta.5", to: "v2.0.0", inertia: true},
	} {
		actions, err := ManualActionsBetween(tc.from, tc.to, tc.inertia)
		if err != nil {
			t.Fatalf("ManualActionsBetween(%s, %s): %v", tc.from, tc.to, err)
		}
		if len(actions) != 0 {
			t.Fatalf("ManualActionsBetween(%s, %s) = %#v, want none", tc.from, tc.to, actions)
		}
	}
}

func TestManualActionsBetweenRejectsInvalidRanges(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
	}{
		{name: "invalid from", from: "not-a-version", to: "v2.0.0-beta.5"},
		{name: "invalid to", from: "v2.0.0-beta.4", to: "not-a-version"},
		{name: "same version", from: "v2.0.0-beta.5", to: "v2.0.0-beta.5"},
		{name: "from newer", from: "v2.0.0-beta.5", to: "v2.0.0-beta.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ManualActionsBetween(tc.from, tc.to, true); err == nil {
				t.Fatalf("ManualActionsBetween(%s, %s) succeeded, want an error", tc.from, tc.to)
			}
		})
	}
}

// TestPreReleaseUpgradeReportHasNoManualActions guards the plan wiring: a
// dry-run report between pre-releases carries an empty action list.
func TestPreReleaseUpgradeReportHasNoManualActions(t *testing.T) {
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
		UpgradeOptions{DryRun: true, TargetVersion: "v2.0.0-beta.5"},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := upgrader.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ManualActions) != 0 {
		t.Fatalf("alpha.4 to beta.5 report = %#v, want no manual actions", report.ManualActions)
	}
}
