package upgrade

import (
	"bytes"
	"strings"
	"testing"
)

func TestMerger_ConflictMarkerFormat(t *testing.T) {
	merger := NewFileMerger()

	oldContent := []byte("line1\nline2\nline3\n")
	userContent := []byte("line1\nUSER_CHANGE\nline3\n")
	newContent := []byte("line1\nTEMPLATE_CHANGE\nline3\n")

	result, err := merger.Merge(oldContent, userContent, newContent)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !result.HasConflicts {
		t.Fatal("Expected conflicts but got none")
	}

	output := string(result.Content)
	t.Logf("Merged output:\n%s", output)

	if !strings.Contains(output, "<<<<<<<") {
		t.Error("Missing opening conflict marker")
	}
	if !strings.Contains(output, "=======") {
		t.Error("Missing middle conflict marker")
	}
	if !strings.Contains(output, ">>>>>>>") {
		t.Error("Missing closing conflict marker")
	}

	if !strings.Contains(output, "ours") {
		t.Error("Missing 'ours' label in conflict markers")
	}
	if !strings.Contains(output, "theirs") {
		t.Error("Missing 'theirs' label in conflict markers")
	}

	if strings.Contains(output, "USER_CHANGE") && strings.Contains(output, "TEMPLATE_CHANGE") {
		t.Log("Both conflicting changes are present in output")
	} else {
		t.Error("One or both conflicting changes are missing from output")
	}
}

func TestMerger_NoConflictWhenUserUnchanged(t *testing.T) {
	merger := NewFileMerger()

	oldContent := []byte("line1\nline2\nline3\n")
	userContent := []byte("line1\nline2\nline3\n")
	newContent := []byte("line1\nTEMPLATE_CHANGE\nline3\n")

	result, err := merger.Merge(oldContent, userContent, newContent)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if result.HasConflicts {
		t.Error("Should not have conflicts when user didn't change anything")
	}

	if !bytes.Equal(result.Content, newContent) {
		t.Error("Should return new template content when user unchanged")
	}
}

func TestMerger_NoConflictWhenTemplateUnchanged(t *testing.T) {
	merger := NewFileMerger()

	oldContent := []byte("line1\nline2\nline3\n")
	userContent := []byte("line1\nUSER_CHANGE\nline3\n")
	newContent := []byte("line1\nline2\nline3\n")

	result, err := merger.Merge(oldContent, userContent, newContent)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if result.HasConflicts {
		t.Error("Should not have conflicts when template didn't change")
	}

	if !bytes.Equal(result.Content, userContent) {
		t.Error("Should return user content when template unchanged")
	}
}

func TestContainsConflictMarkers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no markers",
			content:  "just some regular content",
			expected: false,
		},
		{
			name:     "has opening marker",
			content:  "some content\n<<<<<<<\nmore content",
			expected: true,
		},
		{
			name:     "has middle marker",
			content:  "some content\n=======\nmore content",
			expected: true,
		},
		{
			name:     "has closing marker",
			content:  "some content\n>>>>>>>\nmore content",
			expected: true,
		},
		{
			name: "has full conflict",
			content: `some content
<<<<<<< HEAD
user change
=======
template change
>>>>>>> theirs
more content`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsConflictMarkers([]byte(tt.content))
			if result != tt.expected {
				t.Errorf("containsConflictMarkers() = %v, want %v", result, tt.expected)
			}
		})
	}
}
