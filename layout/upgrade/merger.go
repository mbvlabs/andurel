package upgrade

import (
	"bytes"
	"fmt"
	"strings"
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

	oldLines := strings.Split(string(oldContent), "\n")
	userLines := strings.Split(string(userContent), "\n")
	newLines := strings.Split(string(newContent), "\n")

	merged, conflicts := m.merge3Way(oldLines, userLines, newLines)

	result := &MergeResult{
		Success:      len(conflicts) == 0,
		Content:      []byte(strings.Join(merged, "\n")),
		HasConflicts: len(conflicts) > 0,
	}

	if len(conflicts) > 0 {
		result.ConflictInfo = fmt.Sprintf("%d conflict(s) detected", len(conflicts))
	}

	return result, nil
}

func (m *FileMerger) merge3Way(oldLines, userLines, newLines []string) ([]string, []conflictRegion) {
	var result []string
	var conflicts []conflictRegion

	oldLen := len(oldLines)
	userLen := len(userLines)
	newLen := len(newLines)

	maxLen := max(oldLen, max(userLen, newLen))

	i := 0
	for i < maxLen {
		if i >= oldLen && i >= userLen && i >= newLen {
			break
		}

		oldLine := getLineAt(oldLines, i)
		userLine := getLineAt(userLines, i)
		newLine := getLineAt(newLines, i)

		if oldLine == userLine && userLine == newLine {
			result = append(result, oldLine)
			i++
			continue
		}

		if oldLine == userLine && userLine != newLine {
			result = append(result, newLine)
			i++
			continue
		}

		if oldLine == newLine && userLine != oldLine {
			result = append(result, userLine)
			i++
			continue
		}

		conflictStart := i
		conflictEnd := i + 1

		for conflictEnd < maxLen {
			nextOld := getLineAt(oldLines, conflictEnd)
			nextUser := getLineAt(userLines, conflictEnd)
			nextNew := getLineAt(newLines, conflictEnd)

			if nextOld == nextUser && nextUser == nextNew {
				break
			}

			if (nextOld == nextUser && nextUser != nextNew) ||
				(nextOld == nextNew && nextUser != nextOld) {
				break
			}

			conflictEnd++
		}

		userConflictLines := extractLines(userLines, conflictStart, conflictEnd)
		newConflictLines := extractLines(newLines, conflictStart, conflictEnd)

		result = append(result, "<<<<<<< Current (Your changes)")
		result = append(result, userConflictLines...)
		result = append(result, "=======")
		result = append(result, newConflictLines...)
		result = append(result, ">>>>>>> Template (New version)")

		conflicts = append(conflicts, conflictRegion{
			start: conflictStart,
			end:   conflictEnd,
		})

		i = conflictEnd
	}

	return result, conflicts
}

func getLineAt(lines []string, index int) string {
	if index >= len(lines) {
		return ""
	}
	return lines[index]
}

func extractLines(lines []string, start, end int) []string {
	if start >= len(lines) {
		return []string{}
	}

	actualEnd := end
	if actualEnd > len(lines) {
		actualEnd = len(lines)
	}

	return lines[start:actualEnd]
}

type conflictRegion struct {
	start int
	end   int
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
