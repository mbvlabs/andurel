package upgrade

import (
	"testing"
)

func TestManualActionDetectorsCoverEveryAction(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-alpha.0", "v2.0.0", true)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade: %v", err)
	}
	detectors := make(map[string]bool, len(manualActionDetectors))
	for _, detector := range manualActionDetectors {
		if detectors[detector.id] {
			t.Fatalf("duplicate detector for %s", detector.id)
		}
		detectors[detector.id] = true
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		seen[action.ID] = true
		if !detectors[action.ID] {
			t.Errorf("manual action %s has no detector", action.ID)
		}
	}
	for id := range detectors {
		if !seen[id] {
			t.Errorf("detector %s does not match an emitted manual action", id)
		}
	}
}

func TestCheckManualActionsWithoutVersion(t *testing.T) {
	if checks := CheckManualActions(t.TempDir(), "", "v2.0.0-beta.4", true); len(checks) != 0 {
		t.Fatalf("checks without a project version = %#v, want none", checks)
	}
	if checks := CheckManualActions(t.TempDir(), "not-a-version", "v2.0.0-beta.4", true); len(checks) != 0 {
		t.Fatalf("checks with an invalid project version = %#v, want none", checks)
	}
}
