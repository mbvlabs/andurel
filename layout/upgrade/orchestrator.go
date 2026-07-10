package upgrade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mbvlabs/andurel/layout"
	"golang.org/x/mod/semver"
)

// UpgradeOptions configures upgrade behavior.
type UpgradeOptions struct {
	DryRun        bool
	Auto          bool
	TargetVersion string
}

// Upgrader represents upgrader.
type Upgrader struct {
	projectRoot      string
	lock             *layout.AndurelLock
	git              *GitAnalyzer
	generator        *TemplateGenerator
	opts             UpgradeOptions
	sourceLockSchema int
	transaction      *transactionRuntime
}

// UpgradeReport represents upgrade report.
type UpgradeReport struct {
	FromVersion   string
	ToVersion     string
	FilesReplaced int
	FilesRemoved  int

	ToolsAdded   int
	ToolsRemoved int
	ToolsUpdated int

	AddedTools          []string
	RemovedTools        []string
	UpdatedTools        []string
	ToolMetadataChanges []string
	ReplacedFiles       []string
	RemovedFiles        []string
	LockMigrations      []string
	FrameworkMigrations []string
	Conflicts           []string
	Diffs               []FileDiff
	DirtyWorktree       bool

	Success bool
	Error   error
}

// NewUpgrader creates a new upgrader.
func NewUpgrader(projectRoot string, opts UpgradeOptions) (*Upgrader, error) {
	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	sourceSchema, err := readSourceLockSchema(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect lock schema: %w", err)
	}
	return &Upgrader{
		projectRoot:      projectRoot,
		lock:             lock,
		git:              NewGitAnalyzer(projectRoot),
		generator:        NewTemplateGenerator(opts.TargetVersion),
		opts:             opts,
		sourceLockSchema: sourceSchema,
	}, nil
}

// Execute performs the execute operation.
func (u *Upgrader) Execute() (*UpgradeReport, error) {
	lock, err := layout.ReadLockFile(u.projectRoot)
	if err != nil {
		return &UpgradeReport{}, fmt.Errorf("failed to refresh lock file: %w", err)
	}
	sourceSchema, err := readSourceLockSchema(u.projectRoot)
	if err != nil {
		return &UpgradeReport{}, err
	}
	working := *u
	working.lock = lock
	working.sourceLockSchema = sourceSchema
	if err := working.validatePreconditions(); err != nil {
		report := &UpgradeReport{FromVersion: lock.Version, ToVersion: u.opts.TargetVersion, Error: err}
		return report, err
	}
	if lock.ScaffoldConfig == nil {
		err := fmt.Errorf("lock file missing scaffold config - cannot determine original project settings")
		return &UpgradeReport{FromVersion: lock.Version, ToVersion: u.opts.TargetVersion, Error: err}, err
	}
	clean, err := working.git.IsClean()
	if err != nil {
		return &UpgradeReport{}, fmt.Errorf("git validation failed: %w", err)
	}
	plan, err := working.buildPlan(!clean)
	if err != nil {
		return &UpgradeReport{FromVersion: lock.Version, ToVersion: u.opts.TargetVersion, Error: err}, err
	}
	report := plan.cloneReport()
	if len(plan.conflicts) > 0 {
		printUpgradePlan(report, u.opts.DryRun)
		if u.opts.DryRun {
			return report, nil
		}
		err := fmt.Errorf("upgrade has %d conflict(s); no files were written", len(plan.conflicts))
		report.Error = err
		return report, err
	}
	if u.opts.DryRun {
		report.Success = true
		printUpgradePlan(report, true)
		return report, nil
	}
	if err := working.applyPlan(plan); err != nil {
		report.Error = err
		return report, err
	}
	report.Success = true
	printUpgradePlan(report, false)
	return report, nil
}

func (u *Upgrader) validatePreconditions() error {
	if _, err := os.Stat(u.projectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", u.projectRoot)
	}

	if u.lock == nil {
		return fmt.Errorf("andurel.lock file not found or invalid")
	}

	if u.lock.Version == "" {
		return fmt.Errorf("lock file missing template version - please manually set it")
	}

	clean, err := u.git.IsClean()
	if err != nil {
		return fmt.Errorf("git validation failed: %w", err)
	}
	if !clean && !u.opts.DryRun {
		return fmt.Errorf("worktree is dirty; commit or stash changes before a real upgrade")
	}

	return nil
}

func readSourceLockSchema(projectRoot string) (int, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "andurel.lock"))
	if err != nil {
		return 0, err
	}
	var header struct {
		SchemaVersion *int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return 0, err
	}
	if header.SchemaVersion == nil {
		return 0, nil
	}
	return *header.SchemaVersion, nil
}

func printUpgradePlan(report *UpgradeReport, dryRun bool) {
	if dryRun {
		fmt.Printf("\n[DRY RUN] dirty worktree: %t\n", report.DirtyWorktree)
		if report.DirtyWorktree {
			fmt.Printf("[DRY RUN] WARNING: worktree is dirty; planning only is permitted\n")
		}
	}
	printList := func(label string, values []string) {
		fmt.Printf("%s:\n", label)
		for _, value := range values {
			fmt.Printf("  %s\n", value)
		}
	}
	printList("File replacements", report.ReplacedFiles)
	printList("File deletions", report.RemovedFiles)
	printList("Framework migrations", report.FrameworkMigrations)
	printList("Lock migrations", report.LockMigrations)
	tools := append(append([]string{}, report.AddedTools...), report.UpdatedTools...)
	tools = append(tools, report.RemovedTools...)
	tools = append(tools, report.ToolMetadataChanges...)
	sort.Strings(tools)
	printList("Tool metadata changes", tools)
	printList("Conflicts", report.Conflicts)
	if dryRun {
		fmt.Printf("Unified diffs:\n")
		for _, diff := range report.Diffs {
			fmt.Print(diff.Diff)
		}
	}
}

func (u *Upgrader) obsoleteManagedInternalFiles() []string {
	expected := make(map[string]struct{})
	for _, file := range layout.GetInternalFrameworkFiles(u.lock.ScaffoldConfig) {
		expected[file.TargetPath] = struct{}{}
	}

	var obsolete []string
	for _, file := range layout.GetAllManagedInternalFrameworkFiles() {
		if _, ok := expected[file.TargetPath]; ok {
			continue
		}

		fullPath := filepath.Join(u.projectRoot, file.TargetPath)
		if _, err := os.Stat(fullPath); err == nil {
			obsolete = append(obsolete, file.TargetPath)
		}
	}

	return obsolete
}

// ToolSyncResult represents the result of synchronizing tools
type ToolSyncResult struct {
	Added    []string
	Removed  []string
	Updated  []string
	Metadata []string
}

// syncToolsToFrameworkVersion synchronizes the lock file's tools with the target framework version
// This ensures new tools are added, obsolete tools are removed, and existing tools are updated
func (u *Upgrader) syncToolsToFrameworkVersion() (*ToolSyncResult, error) {
	return syncTools(u.lock)
}

func syncTools(lock *layout.AndurelLock) (*ToolSyncResult, error) {
	result := &ToolSyncResult{
		Added:    []string{},
		Removed:  []string{},
		Updated:  []string{},
		Metadata: []string{},
	}

	if lock.Tools == nil {
		lock.Tools = make(map[string]*layout.Tool)
	}

	if existingTool, ok := lock.Tools["run"]; ok && existingTool.Path != "" {
		delete(lock.Tools, "run")
		result.Removed = append(result.Removed, "run")
	}

	// Get expected tools based on the scaffold config
	expectedTools := layout.GetExpectedTools(lock.ScaffoldConfig)

	// Step 1: Add new tools and update existing ones
	expectedNames := make([]string, 0, len(expectedTools))
	for toolName := range expectedTools {
		expectedNames = append(expectedNames, toolName)
	}
	sort.Strings(expectedNames)
	for _, toolName := range expectedNames {
		expectedTool := expectedTools[toolName]
		existingTool, exists := lock.Tools[toolName]

		if !exists {
			// Tool doesn't exist in lock file - add it
			lock.Tools[toolName] = expectedTool
			result.Added = append(
				result.Added,
				fmt.Sprintf("%s (%s)", toolName, getToolVersion(expectedTool)),
			)
		} else if shouldUpdateTool(existingTool, expectedTool) {
			// Tool exists but needs update
			if existingTool.Path != "" {
				// Update version and path for built tools
				existingTool.Version = expectedTool.Version
				existingTool.Path = expectedTool.Path
				result.Updated = append(result.Updated, fmt.Sprintf("%s: %s", toolName, expectedTool.Version))
			} else {
				// Update version and source metadata for versioned tools.
				existingTool.Version = expectedTool.Version
				existingTool.Source = expectedTool.Source
				existingTool.Download = expectedTool.Download
				existingTool.VersionCheck = expectedTool.VersionCheck
				result.Updated = append(result.Updated, fmt.Sprintf("%s: %s", toolName, getToolVersion(expectedTool)))
			}
			lock.Tools[toolName] = existingTool
		} else if existingTool.Path == "" {
			// Keep source metadata aligned even when version does not change.
			metadataChanged := false
			if existingTool.Source != expectedTool.Source {
				existingTool.Source = expectedTool.Source
				metadataChanged = true
			}
			if existingTool.Download == nil && expectedTool.Download != nil {
				existingTool.Download = expectedTool.Download
				metadataChanged = true
			}
			if existingTool.VersionCheck == nil && expectedTool.VersionCheck != nil {
				existingTool.VersionCheck = expectedTool.VersionCheck
				metadataChanged = true
			} else if existingTool.VersionCheck != nil && expectedTool.VersionCheck != nil &&
				existingTool.VersionCheck.Regexp == "" && expectedTool.VersionCheck.Regexp != "" {
				existingTool.VersionCheck = expectedTool.VersionCheck
				metadataChanged = true
			}
			if metadataChanged {
				lock.Tools[toolName] = existingTool
				result.Metadata = append(result.Metadata, toolName+" metadata")
			}
		}
	}

	// Step 2: Remove obsolete tools (tools in lock but not in expected)
	// Only remove framework-managed tools, preserve user-added custom tools
	existingNames := make([]string, 0, len(lock.Tools))
	for toolName := range lock.Tools {
		existingNames = append(existingNames, toolName)
	}
	sort.Strings(existingNames)
	for _, toolName := range existingNames {
		if _, shouldExist := expectedTools[toolName]; !shouldExist {
			if isFrameworkManagedTool(toolName, lock.Tools[toolName]) {
				delete(lock.Tools, toolName)
				result.Removed = append(result.Removed, toolName)
			}
		}
	}

	return result, nil
}

// isFrameworkManagedTool determines if a tool is managed by the framework
// vs a user-added custom tool. Framework tools have known sources and patterns.
func isFrameworkManagedTool(name string, tool *layout.Tool) bool {
	// Check if it's in the known default tools list.
	for _, defaultTool := range layout.DefaultGoTools {
		if defaultTool.Name == name {
			return true
		}
	}

	if name == "tailwindcli" {
		return true
	}

	// Unknown tool - assume user-added, don't remove
	return false
}

// shouldUpdateTool determines if a tool needs its version updated
// Only upgrades tools, never downgrades (preserves user's manual version bumps)
func shouldUpdateTool(existing, expected *layout.Tool) bool {
	// For legacy "built" tools, update if the version or path has changed.
	if existing.Path != "" || expected.Path != "" {
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
