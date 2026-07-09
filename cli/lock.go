package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/spf13/cobra"
)

type managedTool struct {
	Name        string
	Source      string
	Version     string
	Description string
}

var managedTools = []managedTool{
	{Name: "templ", Source: "github.com/a-h/templ/cmd/templ", Version: versions.Templ, Description: "Templ templating engine"},
	{Name: "goose", Source: "github.com/pressly/goose/v3/cmd/goose", Version: versions.Goose, Description: "Database migrations"},
	{Name: "mailpit", Source: "github.com/axllent/mailpit", Version: versions.Mailpit, Description: "Email testing"},
	{Name: "usql", Source: "github.com/xo/usql", Version: versions.Usql, Description: "Universal SQL CLI"},
	{Name: "dblab", Source: "github.com/danvergara/dblab", Version: versions.Dblab, Description: "Database UI"},
	{Name: "shadowfax", Source: "github.com/mbvlabs/shadowfax", Version: versions.Shadowfax, Description: "Shadowfax dev server"},
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

Examples:
  andurel tool set-version templ 0.3.977
  andurel tool set-version tailwindcli 4.1.18
  andurel tool set-version shadowfax 0.1.3 \
    --sha256 linux/amd64=<hex> --sha256 linux/arm64=<hex> \
    --sha256 darwin/amd64=<hex> --sha256 darwin/arm64=<hex>

This updates andurel.lock and syncs the tool binary to bin/.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			version := args[1]

			projectRoot, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}

			return setVersion(projectRoot, toolName, version, checksumArguments...)
		},
	}
	cmd.Flags().StringArrayVar(
		&checksumArguments,
		"sha256",
		nil,
		"SHA-256 digest as os/arch=hex; repeat for all four supported platforms",
	)
	return cmd
}

func setVersion(projectRoot, toolName, version string, checksumArguments ...string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty\n\nExample: andurel tool set-version %s 1.0.0", toolName)
	}

	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	versionWithV := "v" + version
	managed, managedToolExists := managedToolByName[toolName]
	if !managedToolExists {
		return unknownToolError(toolName)
	}

	download, hasCatalogRelease := layout.GetDefaultToolDownload(toolName)
	if !hasCatalogRelease {
		return fmt.Errorf("tool %s has no download catalog entry", toolName)
	}
	checksums, err := parseChecksumArguments(checksumArguments)
	if err != nil {
		return err
	}
	if len(checksums) == 0 {
		catalogVersion := managed.Version
		if catalogVersion == "" || versionWithV != catalogVersion {
			return fmt.Errorf(
				"custom version %s for %s requires four repeated --sha256 os/arch=hex arguments",
				versionWithV,
				toolName,
			)
		}
	} else {
		download.SHA256 = checksums
	}

	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("andurel.lock not found. Are you in an andurel project?")
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
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
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	if err := installToolVersionAndLockFunc(projectRoot, toolName, lock.Tools[toolName], lock, runtime.GOOS, runtime.GOARCH); err != nil {
		return err
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", toolName, versionWithV)
	return nil
}

func installToolVersionAndLock(projectRoot, toolName string, tool *layout.Tool, lock *layout.AndurelLock, goos, goarch string) error {
	stagingDir, err := os.MkdirTemp(projectRoot, ".andurel-lock-*")
	if err != nil {
		return fmt.Errorf("failed to create lock staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)
	if err := lock.WriteLockFile(stagingDir); err != nil {
		return fmt.Errorf("failed to stage updated lock file: %w", err)
	}

	fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", toolName, tool.Version, goos, goarch)
	candidatePath, err := prepareToolCandidate(projectRoot, toolName, tool, goos, goarch)
	if err != nil {
		return err
	}
	defer os.Remove(candidatePath)

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
	defer os.Remove(backupPath)

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
		return errors.Join(fmt.Errorf("failed to atomically replace %s: %w", toolName, err), restoreBinary())
	}
	stagedLockPath := filepath.Join(stagingDir, "andurel.lock")
	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if err := os.Rename(stagedLockPath, lockPath); err != nil {
		return errors.Join(fmt.Errorf("failed to atomically update andurel.lock: %w", err), restoreBinary())
	}
	if hadExistingBinary {
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("failed to remove binary backup: %w", err)
		}
	}
	return nil
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
			return nil, fmt.Errorf("invalid --sha256 %q; expected os/arch=64-character-hex", argument)
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
