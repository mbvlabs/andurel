package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"golang.org/x/mod/semver"
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

	ToolsAdded   int
	ToolsRemoved int
	ToolsUpdated int

	AddedTools    []string
	RemovedTools  []string
	UpdatedTools  []string
	ReplacedFiles []string

	Success bool
	Error   error
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

		// Preview tool changes
		fmt.Printf("\n[DRY RUN] Tool changes:\n")
		toolSyncPreview, _ := u.syncToolsToFrameworkVersion()
		if len(toolSyncPreview.Added) > 0 {
			fmt.Printf("  Would add:\n")
			for _, tool := range toolSyncPreview.Added {
				fmt.Printf("    + %s\n", tool)
			}
		}
		if len(toolSyncPreview.Updated) > 0 {
			fmt.Printf("  Would update:\n")
			for _, tool := range toolSyncPreview.Updated {
				fmt.Printf("    ↑ %s\n", tool)
			}
		}
		if len(toolSyncPreview.Removed) > 0 {
			fmt.Printf("  Would remove:\n")
			for _, tool := range toolSyncPreview.Removed {
				fmt.Printf("    - %s\n", tool)
			}
		}

		report.Success = true
		return report, nil
	}

	// Replace framework files
	fmt.Printf("Replacing framework files...\n")
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

	// Synchronize tools with target version
	fmt.Printf("Synchronizing tools with framework version...\n")
	toolSyncResult, err := u.syncToolsToFrameworkVersion()
	if err != nil {
		fmt.Printf("⚠ Warning: failed to synchronize tools: %v\n", err)
	} else {
		// Report added tools
		if len(toolSyncResult.Added) > 0 {
			fmt.Printf("  Added:\n")
			for _, tool := range toolSyncResult.Added {
				fmt.Printf("    + %s\n", tool)
			}
			report.ToolsAdded = len(toolSyncResult.Added)
			report.AddedTools = toolSyncResult.Added
		}

		// Report updated tools
		if len(toolSyncResult.Updated) > 0 {
			fmt.Printf("  Updated:\n")
			for _, tool := range toolSyncResult.Updated {
				fmt.Printf("    ↑ %s\n", tool)
			}
			report.ToolsUpdated = len(toolSyncResult.Updated)
			report.UpdatedTools = toolSyncResult.Updated
		}

		// Report removed tools
		if len(toolSyncResult.Removed) > 0 {
			fmt.Printf("  Removed:\n")
			for _, tool := range toolSyncResult.Removed {
				fmt.Printf("    - %s\n", tool)
			}
			report.ToolsRemoved = len(toolSyncResult.Removed)
			report.RemovedTools = toolSyncResult.Removed
		}

		// Clean up obsolete binaries
		if len(toolSyncResult.Removed) > 0 && !u.opts.DryRun {
			if err := u.cleanupObsoleteBinaries(toolSyncResult.Removed); err != nil {
				fmt.Printf("⚠ Warning: failed to clean up binaries: %v\n", err)
			}
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
	if report.ToolsAdded > 0 {
		fmt.Printf("  • %d tools added\n", report.ToolsAdded)
	}
	if report.ToolsUpdated > 0 {
		fmt.Printf("  • %d tools updated\n", report.ToolsUpdated)
	}
	if report.ToolsRemoved > 0 {
		fmt.Printf("  • %d tools removed\n", report.ToolsRemoved)
	}

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

// ToolSyncResult represents the result of synchronizing tools
type ToolSyncResult struct {
	Added   []string
	Removed []string
	Updated []string
}

// syncToolsToFrameworkVersion synchronizes the lock file's tools with the target framework version
// This ensures new tools are added, obsolete tools are removed, and existing tools are updated
func (u *Upgrader) syncToolsToFrameworkVersion() (*ToolSyncResult, error) {
	result := &ToolSyncResult{
		Added:   []string{},
		Removed: []string{},
		Updated: []string{},
	}

	if existingTool, ok := u.lock.Tools["run"]; ok && existingTool.Source == "built" {
		delete(u.lock.Tools, "run")
		result.Removed = append(result.Removed, "run")
	}

	// Get expected tools based on the scaffold config
	expectedTools := layout.GetExpectedTools(u.lock.ScaffoldConfig)

	// Step 1: Add new tools and update existing ones
	for toolName, expectedTool := range expectedTools {
		existingTool, exists := u.lock.Tools[toolName]

		if !exists {
			// Tool doesn't exist in lock file - add it
			u.lock.Tools[toolName] = expectedTool
			result.Added = append(
				result.Added,
				fmt.Sprintf("%s (%s)", toolName, getToolVersion(expectedTool)),
			)
		} else if shouldUpdateTool(existingTool, expectedTool) {
			// Tool exists but needs update
			if existingTool.Source == "built" {
				// Update version and path for built tools
				existingTool.Version = expectedTool.Version
				existingTool.Path = expectedTool.Path
				result.Updated = append(result.Updated, fmt.Sprintf("%s: %s", toolName, expectedTool.Version))
			} else {
				// Update version for versioned tools
				existingTool.Version = expectedTool.Version
				result.Updated = append(result.Updated, fmt.Sprintf("%s: %s", toolName, getToolVersion(expectedTool)))
			}
			u.lock.Tools[toolName] = existingTool
		}
	}

	// Step 2: Remove obsolete tools (tools in lock but not in expected)
	// Only remove framework-managed tools, preserve user-added custom tools
	for toolName := range u.lock.Tools {
		if _, shouldExist := expectedTools[toolName]; !shouldExist {
			if isFrameworkManagedTool(toolName, u.lock.Tools[toolName]) {
				delete(u.lock.Tools, toolName)
				result.Removed = append(result.Removed, toolName)
			}
		}
	}

	return result, nil
}

// isFrameworkManagedTool determines if a tool is managed by the framework
// vs a user-added custom tool. Framework tools have known sources and patterns.
func isFrameworkManagedTool(name string, tool *layout.Tool) bool {
	// All "go" source tools with github.com modules are framework-managed
	if tool.Source == "go" && strings.Contains(tool.Module, "github.com") {
		// Check if it's in the known default tools list
		for _, defaultTool := range layout.DefaultGoTools {
			if defaultTool.Name == name {
				return true
			}
		}
	}

	// Binary tools (tailwindcli)
	if tool.Source == "binary" && name == "tailwindcli" {
		return true
	}

	// Unknown tool - assume user-added, don't remove
	return false
}

// shouldUpdateTool determines if a tool needs its version updated
// Only upgrades tools, never downgrades (preserves user's manual version bumps)
func shouldUpdateTool(existing, expected *layout.Tool) bool {
	// Only update if sources match
	if existing.Source != expected.Source {
		return false
	}

	// For "built" tools, update if the version or path has changed
	if existing.Source == "built" {
		return existing.Version != expected.Version || existing.Path != expected.Path
	}

	// Don't update if versions are the same
	if existing.Version == expected.Version {
		return false
	}

	// Use semver comparison to only upgrade, never downgrade
	// semver.Compare returns:
	//   -1 if v1 < v2
	//    0 if v1 == v2
	//   +1 if v1 > v2
	// We only update if expected > existing (comparison returns +1)
	cmp := semver.Compare(expected.Version, existing.Version)
	return cmp > 0
}

// getToolVersion safely extracts version from a tool
func getToolVersion(tool *layout.Tool) string {
	return tool.Version
}

// cleanupObsoleteBinaries removes binary files for tools that no longer exist in the lock file
func (u *Upgrader) cleanupObsoleteBinaries(removedTools []string) error {
	if len(removedTools) == 0 {
		return nil
	}

	binDir := filepath.Join(u.projectRoot, "bin")

	// Check if bin directory exists
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		// No bin directory, nothing to clean
		return nil
	}

	for _, toolName := range removedTools {
		binPath := filepath.Join(binDir, toolName)

		// Check if binary exists
		if _, err := os.Stat(binPath); err == nil {
			// Binary exists, remove it
			if err := os.Remove(binPath); err != nil {
				// Log warning but don't fail upgrade
				fmt.Printf("  ⚠ Warning: failed to remove obsolete binary %s: %v\n", toolName, err)
			} else {
				fmt.Printf("  ✓ Removed obsolete binary: %s\n", toolName)
			}
		}
	}

	return nil
}
