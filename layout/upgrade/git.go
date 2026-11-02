package upgrade

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitAnalyzer represents git analyzer.
type GitAnalyzer struct {
	projectRoot string
	commands    *commandRecorder
}

// NewGitAnalyzer creates a new git analyzer.
func NewGitAnalyzer(projectRoot string) *GitAnalyzer {
	return &GitAnalyzer{
		projectRoot: projectRoot,
	}
}

// run executes one git command and records the attempt, including failures.
func (g *GitAnalyzer) run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = g.projectRoot
	output, err := cmd.Output()
	g.commands.record(joinCommand(name, args), err)
	return output, err
}

// IsClean reports whether clean.
func (g *GitAnalyzer) IsClean() (bool, error) {
	output, err := g.run("git", "status", "--porcelain")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(exitErr.Stderr), "not a git repository") {
				return false, fmt.Errorf(
					"not a git repository: please initialize git first (git init)",
				)
			}
		}
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(output) == 0, nil
}

func (g *GitAnalyzer) getFirstCommit() (string, error) {
	output, err := g.run("git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get first commit: %w", err)
	}

	commits := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found in repository")
	}

	return commits[0], nil
}

// GetModifiedFiles returns modified files.
func (g *GitAnalyzer) GetModifiedFiles() (map[string]bool, error) {
	firstCommit, err := g.getFirstCommit()
	if err != nil {
		return nil, err
	}

	output, err := g.run("git", "diff", "--name-only", firstCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	modifiedFiles := make(map[string]bool)
	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	for line := range lines {
		if line != "" {
			modifiedFiles[line] = true
		}
	}

	return modifiedFiles, nil
}

// GetFileFromInitialCommit returns file from initial commit.
func (g *GitAnalyzer) GetFileFromInitialCommit(relPath string) ([]byte, error) {
	firstCommit, err := g.getFirstCommit()
	if err != nil {
		return nil, err
	}

	output, err := g.run("git", "show", fmt.Sprintf("%s:%s", firstCommit, relPath))
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrStr := string(exitErr.Stderr)
			if strings.Contains(stderrStr, "does not exist") ||
				strings.Contains(stderrStr, "exists on disk, but not in") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("failed to get file from initial commit: %w", err)
	}

	return output, nil
}
