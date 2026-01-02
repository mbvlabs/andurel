package upgrade

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/layout/versions"
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
	opts        UpgradeOptions
}

type UpgradeReport struct {
	FromVersion   string
	ToVersion     string
	FilesReplaced int
	ToolsUpdated  int
	ReplacedFiles []string
	UpdatedTools  []string
	Success       bool
	Error         error
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
		opts:        opts,
	}, nil
}

func (u *Upgrader) Execute() (*UpgradeReport, error) {
	report := &UpgradeReport{
		FromVersion:   u.lock.Version,
		ToVersion:     u.opts.TargetVersion,
		ReplacedFiles: []string{},
		UpdatedTools:  []string{},
	}

	if err := u.validatePreconditions(); err != nil {
		report.Error = err
		return report, err
	}

	if u.lock.Version == u.opts.TargetVersion {
		return report, fmt.Errorf("project is already at version %s", u.opts.TargetVersion)
	}

	if u.lock.ScaffoldConfig == nil {
		return report, fmt.Errorf(
			"lock file missing scaffold config - cannot determine original project settings",
		)
	}

	// Check if internal/andurel directory exists
	internalAndurelPath := filepath.Join(u.projectRoot, "internal", "andurel")
	if _, err := os.Stat(internalAndurelPath); os.IsNotExist(err) {
		return report, fmt.Errorf("internal/andurel directory not found - nothing to upgrade")
	}

	fmt.Printf(
		"Upgrading framework from %s to %s...\n",
		u.lock.Version,
		u.opts.TargetVersion,
	)

	// Render framework templates
	fmt.Printf("Rendering framework templates...\n")
	renderedTemplates, err := u.generator.RenderFrameworkTemplates(*u.lock.ScaffoldConfig)
	if err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to render framework templates: %w", err)
	}

	if u.opts.DryRun {
		fmt.Printf("\n[DRY RUN] Would replace:\n")
		for path := range renderedTemplates {
			fmt.Printf("  • %s\n", path)
		}
		fmt.Printf("\nWould update tool versions in andurel.lock\n")
		report.Success = true
		return report, nil
	}

	// Replace framework files
	fmt.Printf("Replacing framework files in internal/andurel...\n")
	for targetPath, content := range renderedTemplates {
		fullPath := filepath.Join(u.projectRoot, targetPath)

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			report.Error = err
			return report, fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			report.Error = err
			return report, fmt.Errorf("failed to write %s: %w", targetPath, err)
		}

		report.FilesReplaced++
		report.ReplacedFiles = append(report.ReplacedFiles, targetPath)
		fmt.Printf("  ✓ %s\n", targetPath)
	}

	// Format the upgraded framework files
	fmt.Printf("Formatting framework files...\n")
	if err := cmds.RunGoFmtPath(u.projectRoot, "./internal/andurel/..."); err != nil {
		fmt.Printf("⚠ Warning: failed to format files: %v\n", err)
	} else {
		fmt.Printf("  ✓ Formatted internal/andurel\n")
	}

	// Update tool versions
	fmt.Printf("Updating tool versions...\n")
	updatedTools, err := u.updateToolVersions()
	if err != nil {
		fmt.Printf("⚠ Warning: failed to update tool versions: %v\n", err)
	} else {
		report.ToolsUpdated = len(updatedTools)
		report.UpdatedTools = updatedTools
		for _, tool := range updatedTools {
			fmt.Printf("  ✓ %s\n", tool)
		}
	}

	// Update template version in lock file
	u.lock.Version = u.opts.TargetVersion
	if err := u.lock.WriteLockFile(u.projectRoot); err != nil {
		report.Error = err
		return report, fmt.Errorf("failed to update lock file: %w", err)
	}
	fmt.Printf("✓ Updated andurel.lock\n")

	fmt.Printf("\n✓ Upgrade complete! Project is now at version %s\n", u.opts.TargetVersion)
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  • %d framework files replaced\n", report.FilesReplaced)
	fmt.Printf("  • %d tool versions updated\n", report.ToolsUpdated)
	fmt.Printf("\nNote: Run 'andurel sync' to download updated tool binaries if needed.\n")

	report.Success = true
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

	if u.lock.Version == "" {
		return fmt.Errorf("lock file missing template version - please manually set it")
	}

	return nil
}

// updateToolVersions updates tool versions in the lock file to the latest versions
func (u *Upgrader) updateToolVersions() ([]string, error) {
	var updatedTools []string

	// Define the latest tool versions
	latestVersions := map[string]string{
		"templ":   versions.Templ,
		"sqlc":    versions.Sqlc,
		"goose":   versions.Goose,
		"air":     versions.Air,
		"mailpit": versions.Mailpit,
		"usql":    versions.Usql,
	}

	for toolName, latestVersion := range latestVersions {
		if tool, exists := u.lock.Tools[toolName]; exists {
			if tool.Version != latestVersion {
				tool.Version = latestVersion
				u.lock.Tools[toolName] = tool
				updatedTools = append(updatedTools, fmt.Sprintf("%s: %s", toolName, latestVersion))
			}
		}
	}

	return updatedTools, nil
}
