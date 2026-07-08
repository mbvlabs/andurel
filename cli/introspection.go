package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

type projectInfo struct {
	Root               string                 `json:"root"`
	Module             string                 `json:"module,omitempty"`
	GoVersion          string                 `json:"go_version,omitempty"`
	AndurelVersion     string                 `json:"andurel_version,omitempty"`
	ScaffoldConfig     *layout.ScaffoldConfig `json:"scaffold_config,omitempty"`
	DatabaseConfig     *layout.DatabaseConfig `json:"database_config,omitempty"`
	Extensions         []extensionInfo        `json:"extensions"`
	Tools              []toolInfo             `json:"tools"`
	ConfigPath         string                 `json:"config_path,omitempty"`
	UserConfigPath     string                 `json:"user_config_path,omitempty"`
	UserCacheDirectory string                 `json:"user_cache_directory,omitempty"`
}

type projectItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
}

type extensionInfo struct {
	Name      string `json:"name"`
	AppliedAt string `json:"applied_at,omitempty"`
	Available bool   `json:"available,omitempty"`
}

type toolInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Source     string `json:"source,omitempty"`
	Path       string `json:"path,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
	Installed  bool   `json:"installed"`
}

func newProjectInfoCommand() *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect Andurel project metadata",
		Long:  "Inspect project metadata derived from go.mod, andurel.lock, and Andurel config files.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectInfo(cmd)
		},
	}
	setAgentMetadata(projectCmd, "introspection", "Read-only project metadata. Prefer this before generation.")

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show project metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectInfo(cmd)
		},
	}
	setAgentMetadata(infoCmd, "introspection", "Read-only project metadata. Prefer this before generation.")
	projectCmd.AddCommand(infoCmd)
	return projectCmd
}

func runProjectInfo(cmd *cobra.Command) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	info, err := collectProjectInfo(rootDir)
	if err != nil {
		return err
	}
	return output.OK(cmd, info, "Project info loaded")
}

func collectProjectInfo(rootDir string) (projectInfo, error) {
	module, goVersion, _ := readGoModMetadata(rootDir)
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return projectInfo{}, err
	}
	userConfig, _ := userConfigPath()
	userCache, _ := userCacheDir()
	info := projectInfo{
		Root:               rootDir,
		Module:             module,
		GoVersion:          goVersion,
		AndurelVersion:     lock.Version,
		ScaffoldConfig:     lock.ScaffoldConfig,
		DatabaseConfig:     lock.DatabaseConfig,
		Extensions:         extensionInfos(lock),
		Tools:              toolInfos(rootDir, lock),
		ConfigPath:         filepath.Join(rootDir, ".andurel", "config.json"),
		UserConfigPath:     userConfig,
		UserCacheDirectory: userCache,
	}
	return info, nil
}

func newRoutesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "List route manifest",
		Long: `List route metadata extracted from router/routes/*.go.

The manifest reports actual URL paths, route names, parameter names and
types, and the Go source location for each route variable. In this command's
output, path means the route URL path. The declaring Go file is reported as
source_file in structured output.`,
		Example: `  andurel routes
  andurel routes --json
  andurel routes --jq .routes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			manifest, err := collectRouteManifest(rootDir)
			if err != nil {
				return err
			}
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				if opts.Quiet {
					return nil
				}
				return renderRouteManifestHuman(cmd.OutOrStdout(), manifest)
			}

			summary := fmt.Sprintf("Listed %d routes", len(manifest.Routes))
			if len(manifest.Skipped) > 0 {
				summary = fmt.Sprintf("%s (%d skipped)", summary, len(manifest.Skipped))
			}
			return output.OK(cmd, manifest, summary)
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only route manifest with actual URL paths, route names, params, and source files.")
	return cmd
}

func newModelsCommand() *cobra.Command {
	return readOnlyListCommand("models", "List model files", "models", ".go")
}

func newMigrationsCommand() *cobra.Command {
	return readOnlyListCommand("migrations", "List migration files", filepath.Join("database", "migrations"), ".sql")
}

func newControllersCommand() *cobra.Command {
	return readOnlyListCommand("controllers", "List controller files", "controllers", ".go")
}

func newViewsCommand() *cobra.Command {
	return readOnlyListCommand("views", "List view templates", "views", ".templ")
}

func newJobsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List job and worker files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items := []projectItem{}
			jobs, _ := listProjectFiles(rootDir, filepath.Join("queue", "jobs"), ".go", "job")
			workers, _ := listProjectFiles(rootDir, "queue", ".go", "worker")
			items = append(items, jobs...)
			items = append(items, workers...)
			sortProjectItems(items)
			return output.OK(cmd, items, "Listed jobs")
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only list of queue job and worker files.")
	return cmd
}

func readOnlyListCommand(use, short, dir, ext string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items, err := listProjectFiles(rootDir, dir, ext, strings.TrimSuffix(use, "s"))
			if err != nil {
				return err
			}
			return output.OK(cmd, items, short)
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only project shape inspection.")
	return cmd
}

func listProjectFiles(rootDir, relDir, ext, kind string) ([]projectItem, error) {
	base := filepath.Join(rootDir, relDir)
	if _, err := os.Stat(base); err != nil {
		return []projectItem{}, nil
	}
	items := []projectItem{}
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if ext != "" && filepath.Ext(path) != ext {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		items = append(items, projectItem{
			Name: name,
			Path: filepath.ToSlash(rel),
			Kind: kind,
		})
		return nil
	})
	sortProjectItems(items)
	return items, err
}

func sortProjectItems(items []projectItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
}

func readGoModMetadata(rootDir string) (module, goVersion string, err error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", "", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "module":
			module = fields[1]
		case "go":
			goVersion = fields[1]
		}
	}
	return module, goVersion, nil
}

func extensionInfos(lock *layout.AndurelLock) []extensionInfo {
	infos := []extensionInfo{}
	if lock == nil {
		return infos
	}
	for name, ext := range lock.Extensions {
		appliedAt := ""
		if ext != nil {
			appliedAt = ext.AppliedAt
		}
		infos = append(infos, extensionInfo{Name: name, AppliedAt: appliedAt})
	}
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

func toolInfos(rootDir string, lock *layout.AndurelLock) []toolInfo {
	infos := []toolInfo{}
	if lock == nil {
		return infos
	}
	for name, tool := range lock.Tools {
		info := toolInfo{Name: name}
		if tool != nil {
			info.Version = tool.Version
			info.Source = tool.Source
			info.Path = tool.Path
		}
		if info.Path != "" {
			info.BinaryPath = info.Path
		} else {
			info.BinaryPath = filepath.ToSlash(filepath.Join("bin", name))
		}
		if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(info.BinaryPath))); err == nil {
			info.Installed = true
		}
		infos = append(infos, info)
	}
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}
