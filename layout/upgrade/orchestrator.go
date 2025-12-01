package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/layout"
)

type UpgradeOptions struct {
	DryRun        bool
	Auto          bool
	TargetVersion string
}

type Upgrader struct {
	projectRoot string
	lock        *layout.AndurelLock
	git         *GitAnalyzer
	generator   *TemplateGenerator
	differ      *FileDiffer
	merger      *FileMerger
	opts        UpgradeOptions
}

type UpgradeReport struct {
	FromVersion     string
	ToVersion       string
	BackupRef       string
	FilesAnalyzed   int
	FilesUnchanged  int
	FilesReplaced   int
	FilesMerged     int
	FilesConflicted int
	FilesSkipped    int
	ConflictFiles   []string
	Success         bool
	Error           error
}

type FileAction struct {
	RelativePath string
	Action       ActionType
	Reason       string
	DiffResult   *DiffResult
}

type ActionType int

const (
	ActionSkip ActionType = iota
	ActionReplace
	ActionMerge
	ActionConflict
)

func (a ActionType) String() string {
	switch a {
	case ActionSkip:
		return "skip"
	case ActionReplace:
		return "replace"
	case ActionMerge:
		return "merge"
	case ActionConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

func NewUpgrader(projectRoot string, opts UpgradeOptions) (*Upgrader, error) {
	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	return &Upgrader{
		projectRoot: projectRoot,
		lock:        lock,
		git:         NewGitAnalyzer(projectRoot),
		generator:   NewTemplateGenerator(opts.TargetVersion),
		differ:      NewFileDiffer(),
		merger:      NewFileMerger(),
		opts:        opts,
	}, nil
}

func (u *Upgrader) Execute() (*UpgradeReport, error) {
	report := &UpgradeReport{
		FromVersion:   u.lock.TemplateVersion,
		ToVersion:     u.opts.TargetVersion,
		ConflictFiles: []string{},
	}

	if err := u.validatePreconditions(); err != nil {
		report.Error = err
		return report, err
	}

	if u.lock.TemplateVersion == u.opts.TargetVersion {
		return report, fmt.Errorf("project is already at version %s", u.opts.TargetVersion)
	}

	if !u.opts.DryRun {
		fmt.Printf("Creating backup...\n")
		backupRef, err := u.git.CreateBackup()
		if err != nil {
			report.Error = err
			return report, fmt.Errorf("failed to create backup: %w", err)
		}
		report.BackupRef = backupRef
		fmt.Printf("✓ Created backup: %s\n", backupRef)
	}

	if u.lock.ScaffoldConfig == nil {
		return report, fmt.Errorf("lock file missing scaffold config - cannot determine original project settings")
	}

	fmt.Printf("Generating fresh templates...\n")
	shadowDir, err := u.generator.Generate(*u.lock.ScaffoldConfig, u.projectRoot)
	if err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to generate shadow templates: %w", err)
	}
	defer u.generator.Cleanup(shadowDir)
	fmt.Printf("✓ Generated templates in %s\n", shadowDir)

	modifiedFiles, err := u.git.GetModifiedFiles()
	if err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to get modified files: %w", err)
	}

	fmt.Printf("Analyzing files...\n")
	actions, err := u.analyzeFiles(shadowDir, modifiedFiles)
	if err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to analyze files: %w", err)
	}

	report.FilesAnalyzed = len(actions)
	u.categorizeActions(actions, report)

	if !u.opts.Auto && !u.opts.DryRun {
		fmt.Printf("\nChanges to apply:\n")
		fmt.Printf("  • %d files: Unchanged (skipped)\n", report.FilesSkipped)
		fmt.Printf("  • %d files: Safe to update (will replace)\n", report.FilesReplaced)
		fmt.Printf("  • %d files: Will attempt merge\n", report.FilesMerged)
		fmt.Printf("\n")

		if !u.confirmApply() {
			return report, fmt.Errorf("upgrade cancelled by user")
		}
	}

	if u.opts.DryRun {
		fmt.Printf("\n[DRY RUN] Would apply:\n")
		fmt.Printf("  • %d files: Unchanged (skip)\n", report.FilesSkipped)
		fmt.Printf("  • %d files: Safe to update (replace)\n", report.FilesReplaced)
		fmt.Printf("  • %d files: Attempt merge\n", report.FilesMerged)
		report.Success = true
		return report, nil
	}

	fmt.Printf("Applying changes...\n")
	if err := u.applyChanges(shadowDir, actions, report); err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to apply changes: %w", err)
	}

	u.lock.TemplateVersion = u.opts.TargetVersion
	if err := u.lock.WriteLockFile(u.projectRoot); err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to update lock file: %w", err)
	}
	fmt.Printf("✓ Updated andurel.lock\n")

	if err := u.runPostUpgradeHooks(); err != nil {
		fmt.Printf("⚠ Warning: post-upgrade hooks failed: %v\n", err)
	}

	if len(report.ConflictFiles) > 0 {
		fmt.Printf("\n⚠ %d file(s) need manual review:\n", len(report.ConflictFiles))
		for _, f := range report.ConflictFiles {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Printf("\nResolve conflicts and commit the changes.\n")
	} else {
		fmt.Printf("\n✓ Upgrade complete! Project is now at version %s\n", u.opts.TargetVersion)
		report.Success = true
	}

	return report, nil
}

func (u *Upgrader) validatePreconditions() error {
	if _, err := os.Stat(u.projectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", u.projectRoot)
	}

	if _, err := u.git.IsClean(); err != nil {
		return fmt.Errorf("git validation failed: %w", err)
	}

	if u.lock == nil {
		return fmt.Errorf("andurel.lock file not found or invalid")
	}

	if u.lock.TemplateVersion == "" {
		return fmt.Errorf("lock file missing template version - please manually set it")
	}

	return nil
}

func (u *Upgrader) analyzeFiles(shadowDir string, modifiedFiles map[string]bool) ([]*FileAction, error) {
	var actions []*FileAction

	projectName := filepath.Base(u.projectRoot)
	shadowProjectDir := filepath.Join(shadowDir, projectName)

	err := filepath.Walk(shadowProjectDir, func(shadowPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(shadowProjectDir, shadowPath)
		if err != nil {
			return err
		}

		if u.shouldSkipFile(relPath) {
			return nil
		}

		userPath := filepath.Join(u.projectRoot, relPath)

		action := &FileAction{
			RelativePath: relPath,
		}

		userExists := fileExists(userPath)
		if !userExists {
			action.Action = ActionSkip
			action.Reason = "new file in template (not in user project)"
			actions = append(actions, action)
			return nil
		}

		oldTemplatePath := userPath
		newTemplatePath := shadowPath

		diffResult, err := u.differ.Compare(oldTemplatePath, newTemplatePath, userPath)
		if err != nil {
			return fmt.Errorf("failed to compare %s: %w", relPath, err)
		}

		action.DiffResult = diffResult

		switch diffResult.Status {
		case DiffStatusIdentical:
			action.Action = ActionSkip
			action.Reason = "template unchanged"

		case DiffStatusChanged:
			if modifiedFiles[relPath] {
				action.Action = ActionMerge
				action.Reason = "template changed, user modified"
			} else {
				action.Action = ActionReplace
				action.Reason = "template changed, user unmodified"
			}

		case DiffStatusUserModified:
			action.Action = ActionMerge
			action.Reason = "both template and user modified"

		case DiffStatusNewFile:
			action.Action = ActionSkip
			action.Reason = "new file"

		case DiffStatusDeletedFile:
			action.Action = ActionSkip
			action.Reason = "deleted from template"
		}

		actions = append(actions, action)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return actions, nil
}

func (u *Upgrader) shouldSkipFile(relPath string) bool {
	skipPatterns := []string{
		"andurel.lock",
		".git/",
		"bin/",
		".env",
		"go.mod",
		"go.sum",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(relPath, pattern) {
			return true
		}
	}

	return false
}

func (u *Upgrader) categorizeActions(actions []*FileAction, report *UpgradeReport) {
	for _, action := range actions {
		switch action.Action {
		case ActionSkip:
			report.FilesSkipped++
		case ActionReplace:
			report.FilesReplaced++
		case ActionMerge:
			report.FilesMerged++
		}
	}
}

func (u *Upgrader) confirmApply() bool {
	fmt.Printf("Apply these changes? [Y/n] ")
	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "" || response == "y" || response == "yes"
}

func (u *Upgrader) applyChanges(shadowDir string, actions []*FileAction, report *UpgradeReport) error {
	projectName := filepath.Base(u.projectRoot)
	shadowProjectDir := filepath.Join(shadowDir, projectName)

	for _, action := range actions {
		if action.Action == ActionSkip {
			continue
		}

		shadowPath := filepath.Join(shadowProjectDir, action.RelativePath)
		userPath := filepath.Join(u.projectRoot, action.RelativePath)

		switch action.Action {
		case ActionReplace:
			if err := u.replaceFile(shadowPath, userPath); err != nil {
				return fmt.Errorf("failed to replace %s: %w", action.RelativePath, err)
			}

		case ActionMerge:
			if err := u.mergeFile(shadowPath, userPath, action, report); err != nil {
				return fmt.Errorf("failed to merge %s: %w", action.RelativePath, err)
			}
		}
	}

	fmt.Printf("✓ Applied %d changes\n", report.FilesReplaced+report.FilesMerged)
	return nil
}

func (u *Upgrader) replaceFile(shadowPath, userPath string) error {
	content, err := os.ReadFile(shadowPath)
	if err != nil {
		return fmt.Errorf("failed to read shadow file: %w", err)
	}

	if err := os.WriteFile(userPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write user file: %w", err)
	}

	return nil
}

func (u *Upgrader) mergeFile(shadowPath, userPath string, action *FileAction, report *UpgradeReport) error {
	oldContent, err := os.ReadFile(userPath)
	if err != nil {
		return fmt.Errorf("failed to read old content: %w", err)
	}

	userContent := oldContent

	newContent, err := os.ReadFile(shadowPath)
	if err != nil {
		return fmt.Errorf("failed to read new content: %w", err)
	}

	mergeResult, err := u.merger.Merge(oldContent, userContent, newContent)
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	if err := os.WriteFile(userPath, mergeResult.Content, 0644); err != nil {
		return fmt.Errorf("failed to write merged content: %w", err)
	}

	if mergeResult.HasConflicts {
		report.FilesConflicted++
		report.ConflictFiles = append(report.ConflictFiles, action.RelativePath)
	}

	return nil
}

func (u *Upgrader) runPostUpgradeHooks() error {
	hooks := []struct {
		name string
		cmd  *exec.Cmd
	}{
		{
			name: "go mod tidy",
			cmd:  exec.Command("go", "mod", "tidy"),
		},
		{
			name: "templ generate",
			cmd:  exec.Command("templ", "generate"),
		},
		{
			name: "go fmt",
			cmd:  exec.Command("go", "fmt", "./..."),
		},
	}

	fmt.Printf("\nRunning post-upgrade hooks...\n")
	for _, hook := range hooks {
		fmt.Printf("  • %s...\n", hook.name)
		hook.cmd.Dir = u.projectRoot

		if err := hook.cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", hook.name, err)
		}
	}

	fmt.Printf("✓ Post-upgrade hooks completed\n")
	return nil
}
