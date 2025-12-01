package upgrade

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmezard/go-difflib/difflib"
)

type DiffStatus int

const (
	DiffStatusIdentical DiffStatus = iota
	DiffStatusChanged
	DiffStatusUserModified
	DiffStatusNewFile
	DiffStatusDeletedFile
)

func (s DiffStatus) String() string {
	switch s {
	case DiffStatusIdentical:
		return "identical"
	case DiffStatusChanged:
		return "changed"
	case DiffStatusUserModified:
		return "user-modified"
	case DiffStatusNewFile:
		return "new-file"
	case DiffStatusDeletedFile:
		return "deleted-file"
	default:
		return "unknown"
	}
}

type DiffResult struct {
	Path        string
	Status      DiffStatus
	UnifiedDiff string
}

type FileDiffer struct{}

func NewFileDiffer() *FileDiffer {
	return &FileDiffer{}
}

func (d *FileDiffer) Compare(oldPath, newPath, userPath string) (*DiffResult, error) {
	result := &DiffResult{
		Path: filepath.Base(userPath),
	}

	oldExists := fileExists(oldPath)
	newExists := fileExists(newPath)
	userExists := fileExists(userPath)

	if !userExists {
		if newExists {
			result.Status = DiffStatusNewFile
			return result, nil
		}
		return result, fmt.Errorf("user file does not exist: %s", userPath)
	}

	if !oldExists && !newExists {
		result.Status = DiffStatusIdentical
		return result, nil
	}

	if !oldExists && newExists {
		result.Status = DiffStatusNewFile
		return result, nil
	}

	if oldExists && !newExists {
		result.Status = DiffStatusDeletedFile
		return result, nil
	}

	oldContent, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read old file: %w", err)
	}

	newContent, err := os.ReadFile(newPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read new file: %w", err)
	}

	userContent, err := os.ReadFile(userPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user file: %w", err)
	}

	templateUnchanged := bytes.Equal(oldContent, newContent)
	if templateUnchanged {
		result.Status = DiffStatusIdentical
		return result, nil
	}

	userUnmodified := bytes.Equal(oldContent, userContent)
	if userUnmodified {
		result.Status = DiffStatusChanged
		diff, err := d.generateUnifiedDiff(string(oldContent), string(newContent), oldPath, newPath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate diff: %w", err)
		}
		result.UnifiedDiff = diff
		return result, nil
	}

	result.Status = DiffStatusUserModified
	return result, nil
}

func (d *FileDiffer) CompareWithOriginal(originalContent []byte, newPath, userPath string) (*DiffResult, error) {
	result := &DiffResult{
		Path: filepath.Base(userPath),
	}

	newExists := fileExists(newPath)
	userExists := fileExists(userPath)

	if !userExists {
		return result, fmt.Errorf("user file does not exist: %s", userPath)
	}

	if !newExists {
		result.Status = DiffStatusDeletedFile
		return result, nil
	}

	newContent, err := os.ReadFile(newPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read new file: %w", err)
	}

	userContent, err := os.ReadFile(userPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user file: %w", err)
	}

	templateUnchanged := bytes.Equal(originalContent, newContent)
	if templateUnchanged {
		result.Status = DiffStatusIdentical
		return result, nil
	}

	userUnmodified := bytes.Equal(originalContent, userContent)
	if userUnmodified {
		result.Status = DiffStatusChanged
		diff, err := d.generateUnifiedDiff(string(originalContent), string(newContent), "original", newPath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate diff: %w", err)
		}
		result.UnifiedDiff = diff
		return result, nil
	}

	result.Status = DiffStatusUserModified
	return result, nil
}

func (d *FileDiffer) generateUnifiedDiff(oldContent, newContent, oldPath, newPath string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: oldPath,
		ToFile:   newPath,
		Context:  3,
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", err
	}

	return result, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
