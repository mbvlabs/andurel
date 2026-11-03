package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/versions"
	"github.com/spf13/cobra"
)

type managedTool struct {
	Name        string
	Source      string
	Version     string
	Description string
}

var managedTools = []managedTool{
	{
		Name:        "narsilc",
		Version:     versions.Narsilc,
		Description: "Typed SQL compiler with caller-owned result structs",
	},
	{
		Name:        "goose",
		Source:      "github.com/pressly/goose/v3/cmd/goose",
		Version:     versions.Goose,
		Description: "Database migrations",
	},
	{
		Name:        "mailpit",
		Source:      "github.com/axllent/mailpit",
		Version:     versions.Mailpit,
		Description: "Email testing",
	},
	{
		Name:        "usql",
		Source:      "github.com/xo/usql",
		Version:     versions.Usql,
		Description: "Universal SQL CLI",
	},
	{
		Name:        "dblab",
		Source:      "github.com/danvergara/dblab",
		Version:     versions.Dblab,
		Description: "Database UI",
	},
	{Name: "tailwindcli", Version: versions.TailwindCLI, Description: "Tailwind CSS CLI"},
}

var managedToolByName = func() map[string]managedTool {
	m := make(map[string]managedTool, len(managedTools))
	for _, tool := range managedTools {
		m[tool.Name] = tool
	}
	return m
}()

func newSetVersionCommand() *cobra.Command {
	var checksumArguments []string
	cmd := &cobra.Command{
		Use:     "set-version <tool> <version>",
		Aliases: []string{"sv"},
		Short:   "Set a specific version for a tool",
		Long: `Set the version of a tool and update it.

The tool entry in andurel.lock controls where binaries are downloaded from.
The version should be specified WITHOUT the "v" prefix.

Andurel resolves SHA-256 digests for the requested version automatically from
the tool's GitHub release checksums, or by hashing the published artifacts,
and records them in andurel.lock. Pass --sha256 only to pin known digests.

Examples:
  andurel tool set-version goose 3.27.1
  andurel tool set-version tailwindcli 4.1.18
  andurel tool set-version narsilc 0.6.4 \
    --sha256 linux/amd64=<hex> --sha256 linux/arm64=<hex> \
    --sha256 darwin/amd64=<hex> --sha256 darwin/arm64=<hex>

This updates andurel.lock and syncs the tool binary to bin/.

For narsilc, this command also compares the narsilc runtime required by go.mod
with the requested generator version. A mismatch is reported with the exact
go get and go mod tidy remediation; set-version never edits go.mod or
go.sum. Replaced or unreadable module data is reported as unverifiable.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			version := args[1]

			projectRoot, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}

			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				return setVersion(projectRoot, toolName, version, checksumArguments...)
			}

			var report toolVersionReport
			if err := runWithOptionalStdoutSilence(true, func() error {
				var runErr error
				report, runErr = setVersionAndReport(
					projectRoot,
					toolName,
					version,
					checksumArguments...,
				)
				return runErr
			}); err != nil {
				return err
			}
			return output.OK(
				cmd,
				report,
				setVersionSummary(report),
				setVersionBreadcrumbs(report)...,
			)
		},
	}
	cmd.Flags().StringArrayVar(
		&checksumArguments,
		"sha256",
		nil,
		"optional SHA-256 digest as os/arch=hex; omit to resolve digests for the requested version",
	)
	return cmd
}

type toolVersionReport struct {
	Action          string                `json:"action"`
	Tool            string                `json:"tool"`
	Version         string                `json:"version"`
	PreviousVersion string                `json:"previous_version,omitempty"`
	NarsilcRuntime  *narsilcRuntimeReport `json:"narsilc_runtime,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
	NextSteps       []string              `json:"next_steps,omitempty"`
}

func setVersion(projectRoot, toolName, version string, checksumArguments ...string) error {
	_, err := setVersionAndReport(projectRoot, toolName, version, checksumArguments...)
	return err
}

func setVersionAndReport(
	projectRoot, toolName, version string,
	checksumArguments ...string,
) (report toolVersionReport, err error) {
	report = toolVersionReport{
		Action:  "set-version",
		Tool:    toolName,
		Version: strings.TrimSpace(version),
	}
	if version == "" {
		return report, fmt.Errorf(
			"version cannot be empty\n\nExample: andurel tool set-version %s 1.0.0",
			toolName,
		)
	}

	version = strings.TrimPrefix(version, "v")
	versionWithV := "v" + version
	report.Version = versionWithV
	managed, managedToolExists := managedToolByName[toolName]
	if !managedToolExists {
		return report, unknownToolError(toolName)
	}

	download, hasCatalogRelease := layout.GetDefaultToolDownload(toolName)
	if !hasCatalogRelease {
		return report, fmt.Errorf("tool %s has no download catalog entry", toolName)
	}
	checksums, err := parseChecksumArguments(checksumArguments)
	if err != nil {
		return report, err
	}

	lockPath := filepath.Join(projectRoot, layout.ProjectTomlName)
	if _, err := os.Stat(lockPath); err != nil {
		return report, fmt.Errorf("andurel.toml not found. Are you in an andurel project?")
	}

	if len(checksums) > 0 {
		download.SHA256 = checksums
	} else {
		fmt.Printf("Resolving SHA-256 digests for %s %s...\n", toolName, versionWithV)
		resolved, resolveErr := resolveToolChecksumsFunc(versionWithV, download.URLTemplate)
		if resolveErr != nil {
			return report, fmt.Errorf(
				"failed to resolve SHA-256 digests for %s %s: %w\n\nYou can supply known digests with repeated --sha256 os/arch=hex arguments",
				toolName,
				versionWithV,
				resolveErr,
			)
		}
		if !hasCompleteChecksums(resolved) {
			return report, fmt.Errorf(
				"resolved SHA-256 digests for %s %s are incomplete\n\nYou can supply known digests with repeated --sha256 os/arch=hex arguments",
				toolName,
				versionWithV,
			)
		}
		download.SHA256 = resolved
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return report, fmt.Errorf("failed to read lock file: %w", err)
	}

	tool, exists := lock.Tools[toolName]
	if !exists {
		if managed.Source != "" {
			tool = layout.NewGoTool(toolName, extractSourcePath(managed.Source), versionWithV)
		} else {
			tool = layout.NewBinaryTool(toolName, versionWithV)
		}
		lock.AddTool(toolName, tool)
	} else {
		report.PreviousVersion = tool.Version
		copyOfTool := *tool
		tool = &copyOfTool
		tool.Version = versionWithV
		if tool.VersionCheck == nil {
			if vc, ok := layout.GetDefaultToolVersionCheck(toolName); ok {
				tool.VersionCheck = vc
			}
		}
	}
	tool.Download = download
	if tool.VersionCheck == nil {
		if vc, ok := layout.GetDefaultToolVersionCheck(toolName); ok {
			tool.VersionCheck = vc
		}
	}
	lock.AddTool(toolName, tool)

	fmt.Printf("Setting %s to version %s...\n", toolName, versionWithV)

	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return report, fmt.Errorf("failed to create bin directory: %w", err)
	}

	if err := installToolVersionAndLockFunc(
		projectRoot,
		toolName,
		lock.Tools[toolName],
		lock,
		runtime.GOOS,
		runtime.GOARCH,
	); err != nil {
		return report, err
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", toolName, versionWithV)

	if toolName == "narsilc" {
		state := inspectNarsilcRuntime(projectRoot, versionWithV)
		runtimeReport := state.report()
		report.NarsilcRuntime = &runtimeReport
		if warning := state.warning(); warning != "" {
			report.Warnings = append(report.Warnings, warning)
			report.NextSteps = append(report.NextSteps, state.remediation()...)
			fmt.Printf("\n⚠ %s\n", warning)
			if steps := state.remediation(); len(steps) > 0 {
				fmt.Println("  Run:")
				for _, step := range steps {
					fmt.Printf("    %s\n", step)
				}
			}
		}
	}
	return report, nil
}

func setVersionSummary(report toolVersionReport) string {
	summary := fmt.Sprintf("Updated %s to %s", report.Tool, report.Version)
	if report.NarsilcRuntime == nil {
		return summary
	}
	switch report.NarsilcRuntime.Status {
	case narsilcRuntimeMismatch:
		return fmt.Sprintf(
			"%s; narsilc runtime requires %s",
			summary,
			report.NarsilcRuntime.RuntimeVersion,
		)
	case narsilcRuntimeReplaced:
		return summary + "; narsilc runtime replacement could not be verified"
	case narsilcRuntimeUnverifiable:
		return summary + "; narsilc runtime could not be verified"
	default:
		return summary
	}
}

func setVersionBreadcrumbs(report toolVersionReport) []output.Breadcrumb {
	breadcrumbs := make([]output.Breadcrumb, 0, len(report.NextSteps))
	for _, step := range report.NextSteps {
		description := ""
		switch {
		case strings.HasPrefix(step, "go get "):
			description = "Align the narsilc runtime with the configured tool version"
		case step == "go mod tidy":
			description = "Refresh module checksums after changing the narsilc runtime"
		}
		breadcrumbs = append(breadcrumbs, output.Breadcrumb{
			Command:     step,
			Description: description,
		})
	}
	return breadcrumbs
}

func installToolVersionAndLock(
	projectRoot, toolName string,
	tool *layout.Tool,
	lock *layout.AndurelLock,
	goos, goarch string,
) (err error) {
	stagingDir, err := os.MkdirTemp(projectRoot, ".andurel-lock-*")
	if err != nil {
		return fmt.Errorf("failed to create lock staging directory: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stagingDir)) }()
	if err := lock.WriteLockFile(stagingDir); err != nil {
		return fmt.Errorf("failed to stage updated lock file: %w", err)
	}

	fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", toolName, tool.Version, goos, goarch)
	candidatePath, err := prepareToolCandidate(projectRoot, toolName, tool, goos, goarch)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, removeIfExists(candidatePath)) }()

	binPath := filepath.Join(projectRoot, "bin", toolName)
	backup, err := os.CreateTemp(filepath.Dir(binPath), ".andurel-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create binary backup path: %w", err)
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("failed to close binary backup path: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to prepare binary backup path: %w", err)
	}
	defer func() { err = errors.Join(err, removeIfExists(backupPath)) }()

	hadExistingBinary := false
	if _, err := os.Stat(binPath); err == nil {
		hadExistingBinary = true
		if err := os.Rename(binPath, backupPath); err != nil {
			return fmt.Errorf("failed to preserve existing %s binary: %w", toolName, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect existing %s binary: %w", toolName, err)
	}

	restoreBinary := func() error {
		removeErr := os.Remove(binPath)
		if os.IsNotExist(removeErr) {
			removeErr = nil
		}
		if hadExistingBinary {
			return errors.Join(removeErr, os.Rename(backupPath, binPath))
		}
		return removeErr
	}

	if err := os.Rename(candidatePath, binPath); err != nil {
		return errors.Join(
			fmt.Errorf("failed to atomically replace %s: %w", toolName, err),
			restoreBinary(),
		)
	}
	for _, name := range []string{layout.ProjectTomlName, layout.ProjectLockName} {
		stagedPath := filepath.Join(stagingDir, name)
		destPath := filepath.Join(projectRoot, name)
		if err := os.Rename(stagedPath, destPath); err != nil {
			return errors.Join(
				fmt.Errorf("failed to atomically update %s: %w", name, err),
				restoreBinary(),
			)
		}
	}
	if hadExistingBinary {
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("failed to remove binary backup: %w", err)
		}
	}
	return nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func hasCompleteChecksums(checksums map[string]string) bool {
	if len(checksums) != 4 {
		return false
	}
	for _, platform := range []string{
		"linux/amd64",
		"linux/arm64",
		"darwin/amd64",
		"darwin/arm64",
	} {
		if len(checksums[platform]) != 64 {
			return false
		}
	}
	return true
}

func parseChecksumArguments(arguments []string) (map[string]string, error) {
	if len(arguments) == 0 {
		return nil, nil
	}
	required := map[string]bool{
		"linux/amd64":  false,
		"linux/arm64":  false,
		"darwin/amd64": false,
		"darwin/arm64": false,
	}
	checksums := make(map[string]string, len(arguments))
	for _, argument := range arguments {
		platform, digest, ok := strings.Cut(argument, "=")
		if !ok || platform == "" || digest == "" {
			return nil, fmt.Errorf(
				"invalid --sha256 %q; expected os/arch=64-character-hex",
				argument,
			)
		}
		if _, ok := required[platform]; !ok {
			return nil, fmt.Errorf("unsupported --sha256 platform %q", platform)
		}
		if _, duplicate := checksums[platform]; duplicate {
			return nil, fmt.Errorf("duplicate --sha256 platform %q", platform)
		}
		if len(digest) != 64 {
			return nil, fmt.Errorf("invalid SHA-256 digest for %s", platform)
		}
		for _, character := range digest {
			if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
				return nil, fmt.Errorf("invalid SHA-256 digest for %s", platform)
			}
		}
		checksums[platform] = strings.ToLower(digest)
		required[platform] = true
	}
	for platform, present := range required {
		if !present {
			return nil, fmt.Errorf("missing --sha256 for %s", platform)
		}
	}
	return checksums, nil
}

func unknownToolError(toolName string) error {
	allNames := make([]string, 0, len(managedTools))
	for _, tool := range managedTools {
		allNames = append(allNames, tool.Name)
	}
	sort.Strings(allNames)

	return fmt.Errorf(
		"unknown tool: %s\n\nSupported tools:\n  %s\n\nRun 'andurel tool set-version --help' for more information",
		toolName,
		strings.Join(allNames, ", "),
	)
}

func extractSourcePath(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return source
}
