// Package upgrade plans and applies transactional upgrades to generated projects.
package upgrade

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"golang.org/x/mod/semver"
)

// UpgradeOptions configures upgrade behavior.
type UpgradeOptions struct {
	DryRun        bool
	Auto          bool
	Repair        bool
	TargetVersion string
}

// Upgrader represents upgrader.
type Upgrader struct {
	projectRoot string
	lock        *layout.AndurelLock
	git         *GitAnalyzer
	generator   *TemplateGenerator
	opts        UpgradeOptions
	transaction *transactionRuntime
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
	Diffs               []FileDiff
	ManualActions       []ManualAction `json:"manual_actions,omitempty"`
	DirtyWorktree       bool
	AlreadyCurrent      bool
	RepairAvailable     bool
	RepairApplied       bool

	Success bool
	Error   error
}

// NewUpgrader creates a new upgrader.
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

// Execute performs the execute operation.
func (u *Upgrader) Execute() (*UpgradeReport, error) {
	lock, err := layout.ReadLockFile(u.projectRoot)
	if err != nil {
		return &UpgradeReport{}, fmt.Errorf("failed to refresh lock file: %w", err)
	}
	working := *u
	working.lock = lock
	if lock.Version == u.opts.TargetVersion {
		return working.executeSameVersion()
	}
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
	printUpgradeStart(os.Stdout, lock.Version, u.opts.TargetVersion)
	plan, err := working.buildPlan(!clean)
	if err != nil {
		return &UpgradeReport{FromVersion: lock.Version, ToVersion: u.opts.TargetVersion, Error: err}, err
	}
	report := plan.cloneReport()
	if u.opts.DryRun {
		report.Success = true
		printUpgradeDryRun(os.Stdout, report)
		return report, nil
	}
	if err := working.applyPlan(plan); err != nil {
		report.Error = err
		return report, err
	}
	report.Success = true
	printUpgradeSuccess(os.Stdout, report)
	return report, nil
}

func (u *Upgrader) executeSameVersion() (*UpgradeReport, error) {
	if u.lock.ScaffoldConfig == nil {
		err := fmt.Errorf("lock file missing scaffold config - cannot verify framework files")
		return &UpgradeReport{FromVersion: u.lock.Version, ToVersion: u.opts.TargetVersion, Error: err}, err
	}
	clean, err := u.git.IsClean()
	if err != nil {
		return &UpgradeReport{}, fmt.Errorf("git validation failed: %w", err)
	}
	plan, err := u.buildRepairPlan(!clean)
	if err != nil {
		return &UpgradeReport{FromVersion: u.lock.Version, ToVersion: u.opts.TargetVersion, Error: err}, err
	}
	report := plan.cloneReport()
	if len(plan.files) == 0 {
		report.AlreadyCurrent = true
		report.Success = true
		printUpgradeAlreadyCurrent(os.Stdout, u.lock.Version, u.opts.DryRun)
		return report, nil
	}
	report.RepairAvailable = true
	if !u.opts.Repair || !u.opts.Auto {
		printFrameworkDrift(os.Stdout, report, u.opts.DryRun)
	}
	if u.opts.DryRun || !u.opts.Repair {
		report.Success = true
		return report, nil
	}
	if !clean {
		err := fmt.Errorf("worktree is dirty; commit or stash changes before repairing framework files")
		report.Error = err
		return report, err
	}
	if err := u.applyPlan(plan); err != nil {
		report.Error = err
		return report, err
	}
	report.RepairApplied = true
	report.Success = true
	printUpgradeSuccess(os.Stdout, report)
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

func printUpgradeStart(writer io.Writer, fromVersion, toVersion string) {
	output := newPresentationWriter(writer)
	output.printf("Upgrading framework from %s to %s...\n", fromVersion, toVersion)
	output.println("Rendering framework templates...")
}

func printUpgradeAlreadyCurrent(writer io.Writer, version string, dryRun bool) {
	output := newPresentationWriter(writer)
	if dryRun {
		output.printf("[DRY RUN] Project is already at version %s. No files would be changed.\n", version)
		return
	}
	output.printf("✓ Project is already at version %s. Nothing to upgrade.\n", version)
}

func printFrameworkDrift(writer io.Writer, report *UpgradeReport, dryRun bool) {
	output := newPresentationWriter(writer)
	prefix := ""
	if dryRun {
		prefix = "[DRY RUN] "
	}
	output.printf("%sProject is already at version %s.\n", prefix, report.ToVersion)
	output.printf("%sUnexpected changes were found in framework-owned files:\n", prefix)
	for _, path := range report.ReplacedFiles {
		output.printf("  ! %s\n", path)
	}
	for _, path := range report.RemovedFiles {
		output.printf("  ! %s (obsolete)\n", path)
	}
	if report.DirtyWorktree {
		output.printf("%sCommit or stash your changes before restoring these files.\n", prefix)
	}
}

func printUpgradeSuccess(writer io.Writer, report *UpgradeReport) {
	output := newPresentationWriter(writer)
	if len(report.ReplacedFiles) > 0 {
		output.println("Replacing framework files...")
		for _, path := range report.ReplacedFiles {
			output.printf("  ✓ %s\n", path)
		}
	}
	if len(report.RemovedFiles) > 0 {
		output.println("Removing obsolete internal package files...")
		for _, path := range report.RemovedFiles {
			output.printf("  - %s\n", path)
		}
	}
	printToolChanges(output, report, false)

	lockChanged := report.FromVersion != report.ToVersion || hasToolChanges(report)
	if lockChanged {
		output.println("✓ Updated andurel.lock")
	}
	printManualActions(output, report.ManualActions)
	if len(report.ReplacedFiles) == 0 && len(report.RemovedFiles) == 0 && !lockChanged {
		output.println("✓ Project is already up to date")
		return
	}
}

func printUpgradeDryRun(writer io.Writer, report *UpgradeReport) {
	output := newPresentationWriter(writer)
	output.println("\n[DRY RUN] No files will be changed.")
	if report.DirtyWorktree {
		output.println("[DRY RUN] Warning: the worktree is dirty; planning only is permitted.")
	}
	if len(report.ReplacedFiles) > 0 {
		output.println("\n[DRY RUN] Would replace framework files:")
		for _, path := range report.ReplacedFiles {
			output.printf("  • %s\n", path)
		}
	}
	if len(report.RemovedFiles) > 0 {
		output.println("\n[DRY RUN] Would remove obsolete internal package files:")
		for _, path := range report.RemovedFiles {
			output.printf("  - %s\n", path)
		}
	}
	printToolChanges(output, report, true)
	if report.FromVersion != report.ToVersion || hasToolChanges(report) {
		output.println("\n[DRY RUN] Would update andurel.lock")
	}
	printManualActions(output, report.ManualActions)
}

func printManualActions(output *presentationWriter, actions []ManualAction) {
	if len(actions) == 0 {
		return
	}

	output.println("\nManual action required after this upgrade:")
	for _, action := range actions {
		output.printf("\n%s\n\n%s\n", action.Title, strings.TrimSpace(action.Instructions))
	}
}

func printToolChanges(output *presentationWriter, report *UpgradeReport, dryRun bool) {
	if !hasToolChanges(report) {
		return
	}
	label := "Updating managed tool metadata..."
	if dryRun {
		label = "\n[DRY RUN] Tool changes:"
	}
	output.println(label)
	printToolGroup(output, "Added", report.AddedTools)
	printToolGroup(output, "Updated", report.UpdatedTools)
	printToolGroup(output, "Removed", report.RemovedTools)
	printToolGroup(output, "Metadata", report.ToolMetadataChanges)
}

func printToolGroup(output *presentationWriter, label string, values []string) {
	if len(values) == 0 {
		return
	}
	output.printf("  %s:\n", label)
	for _, value := range values {
		output.printf("    %s\n", value)
	}
}

// presentationWriter keeps terminal output best-effort. An output failure must
// never turn a completed filesystem transaction into a failed upgrade.
type presentationWriter struct {
	writer io.Writer
	failed bool
}

func newPresentationWriter(writer io.Writer) *presentationWriter {
	return &presentationWriter{writer: writer}
}

func (w *presentationWriter) printf(format string, args ...any) {
	if w.failed {
		return
	}
	_, err := fmt.Fprintf(w.writer, format, args...)
	w.failed = err != nil
}

func (w *presentationWriter) println(args ...any) {
	if w.failed {
		return
	}
	_, err := fmt.Fprintln(w.writer, args...)
	w.failed = err != nil
}

func hasToolChanges(report *UpgradeReport) bool {
	return len(report.AddedTools) > 0 || len(report.UpdatedTools) > 0 ||
		len(report.RemovedTools) > 0 || len(report.ToolMetadataChanges) > 0
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

const redundantDefaultVersionCheckRegexp = `v?([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)`

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
				fmt.Sprintf("%s (%s)", toolName, expectedTool.Version),
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
				result.Updated = append(result.Updated, fmt.Sprintf("%s: %s", toolName, expectedTool.Version))
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
			} else if existingTool.VersionCheck != nil && expectedTool.VersionCheck != nil {
				switch {
				case existingTool.VersionCheck.Regexp == redundantDefaultVersionCheckRegexp &&
					expectedTool.VersionCheck.Regexp == "":
					existingTool.VersionCheck.Regexp = ""
					metadataChanged = true
				case existingTool.VersionCheck.Regexp == "" && expectedTool.VersionCheck.Regexp != "":
					existingTool.VersionCheck = expectedTool.VersionCheck
					metadataChanged = true
				}
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
