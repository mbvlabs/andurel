package upgrade

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitAnalyzer struct {
	projectRoot string
}

func NewGitAnalyzer(projectRoot string) *GitAnalyzer {
	return &GitAnalyzer{
		projectRoot: projectRoot,
	}
}

func (g *GitAnalyzer) IsClean() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.projectRoot

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(exitErr.Stderr), "not a git repository") {
				return false, fmt.Errorf("not a git repository: please initialize git first (git init)")
			}
		}
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(output) == 0, nil
}


func (g *GitAnalyzer) getFirstCommit() (string, error) {
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = g.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get first commit: %w", err)
	}

	commits := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found in repository")
	}

	return commits[0], nil
}

func (g *GitAnalyzer) GetModifiedFiles() (map[string]bool, error) {
	firstCommit, err := g.getFirstCommit()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "diff", "--name-only", firstCommit)
	cmd.Dir = g.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	modifiedFiles := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" {
			modifiedFiles[line] = true
		}
	}

	return modifiedFiles, nil
}

func (g *GitAnalyzer) GetFileFromInitialCommit(relPath string) ([]byte, error) {
	firstCommit, err := g.getFirstCommit()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", firstCommit, relPath))
	cmd.Dir = g.projectRoot

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrStr := string(exitErr.Stderr)
			if strings.Contains(stderrStr, "does not exist") || strings.Contains(stderrStr, "exists on disk, but not in") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to get file from initial commit: %w", err)
	}

	return output, nil
}
