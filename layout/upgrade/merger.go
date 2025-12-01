package upgrade

import (
	"bytes"
	"io"
	"strings"

	"github.com/epiclabs-io/diff3"
)

type MergeResult struct {
	Success      bool
	Content      []byte
	HasConflicts bool
	ConflictInfo string
}

type FileMerger struct{}

func NewFileMerger() *FileMerger {
	return &FileMerger{}
}

func (m *FileMerger) Merge(oldContent, userContent, newContent []byte) (*MergeResult, error) {
	if bytes.Equal(userContent, oldContent) {
		return &MergeResult{
			Success:      true,
			Content:      newContent,
			HasConflicts: false,
		}, nil
	}

	if bytes.Equal(newContent, oldContent) {
		return &MergeResult{
			Success:      true,
			Content:      userContent,
			HasConflicts: false,
		}, nil
	}

	if bytes.Equal(userContent, newContent) {
		return &MergeResult{
			Success:      true,
			Content:      userContent,
			HasConflicts: false,
		}, nil
	}

	return m.performDiff3Merge(oldContent, userContent, newContent)
}

func (m *FileMerger) performDiff3Merge(baseContent, oursContent, theirsContent []byte) (*MergeResult, error) {
	oursReader := bytes.NewReader(oursContent)
	baseReader := bytes.NewReader(baseContent)
	theirsReader := bytes.NewReader(theirsContent)

	result, err := diff3.Merge(oursReader, baseReader, theirsReader, true, "ours", "theirs")
	if err != nil {
		return nil, err
	}

	var mergedBuf bytes.Buffer
	_, err = io.Copy(&mergedBuf, result.Result)
	if err != nil {
		return nil, err
	}

	mergedContent := mergedBuf.Bytes()

	hasConflicts := result.Conflicts

	conflictInfo := ""
	if hasConflicts {
		conflictInfo = "Merge has conflicts - see conflict markers in file"
	}

	return &MergeResult{
		Success:      !hasConflicts,
		Content:      mergedContent,
		HasConflicts: hasConflicts,
		ConflictInfo: conflictInfo,
	}, nil
}

func containsConflictMarkers(content []byte) bool {
	contentStr := string(content)
	return strings.Contains(contentStr, "<<<<<<<") ||
		strings.Contains(contentStr, "=======") ||
		strings.Contains(contentStr, ">>>>>>>")
}
