package upgrade

import (
	"slices"
	"testing"

	"github.com/mbvlabs/andurel/v2/layout"
)

// TestPreReleaseUpgradesOnlyEmitApprovedRcOneBreakingActions guards the policy
// that upgrade notes only exist between proper releases, with the deliberate
// rc.1 exception: removing compiled templ roots and pkg/email is a pre-rc
// breaking change, so those two notices are actionable and approved.
func TestPreReleaseUpgradesOnlyEmitApprovedRcOneBreakingActions(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		inertia  bool
		want     []string
	}{
		{from: "v2.0.0-alpha.4", to: "v2.0.0-alpha.5", inertia: true},
		{from: "v2.0.0-alpha.4", to: "v2.0.0-beta.5", inertia: true},
		{from: "v2.0.0-beta.4", to: "v2.0.0-beta.5", inertia: false},
		{from: "v2.0.0-beta.5", to: "v2.0.0-beta.6", inertia: true},
		{
			from:    "v2.0.0-beta.5",
			to:      "v2.0.0-rc.1",
			inertia: true,
			want:    []string{templRootActionID, postmandMigrationActionID},
		},
		{
			from:    "v2.0.0-beta.7",
			to:      "v2.0.0-rc.1",
			inertia: true,
			want:    []string{templRootActionID, postmandMigrationActionID},
		},
		{
			from:    "v2.0.0-beta.5",
			to:      "v2.0.0",
			inertia: true,
			want:    []string{templRootActionID, postmandMigrationActionID},
		},
		{
			from:    "v2.0.0-beta.5",
			to:      "v2.0.0-rc.1",
			inertia: false,
			want:    []string{postmandMigrationActionID},
		},
	} {
		actions, err := ManualActionsBetween(tc.from, tc.to, tc.inertia)
		if err != nil {
			t.Fatalf("ManualActionsBetween(%s, %s): %v", tc.from, tc.to, err)
		}
		if got := actionIDs(actions); !slices.Equal(got, tc.want) {
			t.Fatalf("ManualActionsBetween(%s, %s) = %#v, want %#v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestRcOneManualActionsOnlyApplyToLegacyProjects(t *testing.T) {
	writeState := func(t *testing.T, root string, goMod string, files map[string]string) {
		t.Helper()
		mustWriteTestFile(t, root, "go.mod", []byte(goMod))
		for rel, content := range files {
			mustWriteTestFile(t, root, rel, []byte(content))
		}
	}
	postmandMod := "module testapp\n\ngo 1.27.1\n\nrequire github.com/mbvlabs/andurel/pkg/postmand v0.6.1\n"
	emailMod := "module testapp\n\ngo 1.27.1\n\nrequire github.com/mbvlabs/andurel/pkg/email v0.6.1\n"
	migratedRoot := map[string]string{
		"resources/root.html": "<!doctype html>\n",
		"resources/root.go":   "package resources\n",
	}
	legacyRoot := map[string]string{
		"resources/root.templ":    "package resources\n",
		"resources/root_templ.go": "package resources\n",
	}

	for _, tc := range []struct {
		name  string
		goMod string
		files map[string]string
		want  []string
	}{
		{
			name:  "already migrated",
			goMod: postmandMod,
			files: migratedRoot,
			want:  nil,
		},
		{
			name:  "legacy root and pkg/email",
			goMod: emailMod,
			files: legacyRoot,
			want:  []string{templRootActionID, postmandMigrationActionID},
		},
		{
			name:  "root migrated only",
			goMod: emailMod,
			files: migratedRoot,
			want:  []string{postmandMigrationActionID},
		},
		{
			name:  "pkg/email migrated only",
			goMod: postmandMod,
			files: legacyRoot,
			want:  []string{templRootActionID},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeState(t, root, tc.goMod, tc.files)

			actions, err := ManualActionsForProject(
				root,
				"v2.0.0-beta.5",
				"v2.0.0-rc.1",
				true,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := actionIDs(actions); !slices.Equal(got, tc.want) {
				t.Fatalf("actions = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func actionIDs(actions []ManualAction) []string {
	ids := make([]string, 0, len(actions))
	for _, action := range actions {
		ids = append(ids, action.ID)
	}
	return ids
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
// dry-run report between pre-releases carries an empty action list unless it
// crosses the approved rc.1 breaking changes. The alpha.4 fixture predates
// rc.1, so it has no actions.
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
