package upgrade

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
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

func (g *GitAnalyzer) CreateBackup() (string, error) {
	clean, err := g.IsClean()
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("andurel-upgrade-backup-%s", timestamp)
	commitMsg := fmt.Sprintf("andurel upgrade backup - %s", timestamp)

	if !clean {
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = g.projectRoot
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to stage changes: %w", err)
		}

		cmd = exec.Command("git", "commit", "-m", commitMsg)
		cmd.Dir = g.projectRoot
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create backup commit: %w", err)
		}
	}

	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = g.projectRoot
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create backup branch: %w", err)
	}

	return branchName, nil
}

func (g *GitAnalyzer) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
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

	cmd := exec.Command("git", "diff", "--name-only", fmt.Sprintf("%s..HEAD", firstCommit))
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

func (g *GitAnalyzer) RestoreBackup(backupRef string) error {
	if backupRef == "" {
		return fmt.Errorf("backup reference is empty")
	}

	clean, err := g.IsClean()
	if err != nil {
		return err
	}

	if !clean {
		cmd := exec.Command("git", "reset", "--hard")
		cmd.Dir = g.projectRoot
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to reset working directory: %w", err)
		}
	}

	cmd := exec.Command("git", "reset", "--hard", backupRef)
	cmd.Dir = g.projectRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}
