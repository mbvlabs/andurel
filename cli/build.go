package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/mbvlabs/andurel/v2/layout/versions"
	"github.com/spf13/cobra"
)

// buildArtifact is one compiled process in the build manifest.
type buildArtifact struct {
	Process string `json:"process"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Version string `json:"version"`
}

// buildReport is the artifact manifest printed (or emitted as JSON) after a
// successful build.
// buildReport is the machine-readable result of andurel build.
type buildReport struct {
	Version     string          `json:"version"`
	OutputDir   string          `json:"output_dir"`
	Isolated    bool            `json:"isolated,omitempty"`
	AssetsBuilt bool            `json:"assets_built"`
	Artifacts   []buildArtifact `json:"artifacts"`
	// Recovered lists interrupted publications that were repaired before this
	// build published its own output, for example a backup restored after a
	// SIGKILL between backup and rename.
	Recovered []string `json:"recovered,omitempty"`
}

// buildOptions is one build invocation. An empty OutputDir keeps the legacy
// in-place layout (dist/ and assets/dist in the project); any explicit output
// directory enables a staged, source-tree-isolated build.
type buildOptions struct {
	Version   string
	Only      []string
	OutputDir string
}

// buildProcess is one command directory that compiles to a runnable binary.
type buildProcess struct {
	Name string
}

func newBuildCommand() *cobra.Command {
	var (
		versionFlag   string
		onlyFlag      []string
		outputDirFlag string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build every application process for production",
		Long: `Build every cmd/* process that has a main package, except cmd/seeds, and
compile all assets for production deployment.

This command:
  • Generates Go from .templ files (email + Inertia root)
  • Generates narsilc code when domains/**/*.sql contains SQL files
  • Compiles Tailwind utilities in email templates
  • Builds Vite client assets and the SSR bundle (when Inertia is configured)
  • Builds cmd/app, cmd/queue, cmd/migrate, and cmd/ssr into dist/ (or --output-dir)
  • Injects the version string via ldflags (from --version or git describe)

Client assets and the SSR bundle are embedded in the binaries: assets/assets.go
embeds assets/dist, including assets/dist/ssr/ssr.js, so the app and SSR
processes need no asset files beside the binary at runtime.

Use --only to build a subset, and --json for an artifact manifest.

Use --output-dir to isolate every writable build intermediate. The project
source tree, including assets/dist and generated Go files, is left untouched:
andurel copies the project into a private staging directory, runs templ,
narsilc, email, Vite and Go builds there, and publishes only the finished
binaries. Relative --output-dir paths resolve from the project root. The
manifest reports absolute artifact paths and isolated=true. Two builds of the
same project with different output directories do not share writable paths.

Isolated builds reuse an existing node_modules snapshot instead of running the
package manager against the shared source tree; run your package manager
install command first when dependencies change. Staging directories are removed
on normal success and failure. A SIGKILL can leave an unreferenced temp
directory behind, but it is never used by a later build.

Without --output-dir the historical in-place layout is preserved: binaries in
dist/, Vite output in assets/dist/, tools in bin/. Use that mode when the
development asset tree should be refreshed by the build.

Every process loads .env at startup. Runtime configuration by process:
  • app: database, HTTP/hosts, sessions, cookies, auth, email, cache, queue,
    Inertia, and telemetry configuration
  • queue: database, queue, cache, email, and telemetry configuration
  • migrate: database configuration only
  • ssr: INERTIA_* configuration only, plus a JavaScript runtime on PATH

See andurel patterns show deploy for the start order (migrate → ssr → app +
queue), shared environment, and health checks.`,
		Example: `  andurel build
  andurel build --version 1.2.3
  andurel build --only app,queue
  andurel build --output-dir /tmp/app-build
  andurel build --json`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}

			var report buildReport
			run := func() error {
				var buildErr error
				report, buildErr = buildProject(rootDir, buildOptions{
					Version:   versionFlag,
					Only:      onlyFlag,
					OutputDir: outputDirFlag,
				})
				return buildErr
			}

			if output.SuppressesHumanOutput(opts) {
				if err := runWithOptionalStdoutSilence(true, run); err != nil {
					return err
				}
				return output.OK(
					cmd,
					report,
					buildReportSummary(report),
					output.Breadcrumb{
						Command:     "andurel patterns show deploy",
						Description: "read the production process order and runtime environment",
					},
				)
			}

			if err := run(); err != nil {
				return err
			}
			return writeBuildReport(cmd.OutOrStdout(), report)
		},
	}

	cmd.Flags().
		StringVar(&versionFlag, "version", "", "Set the application version (injected via ldflags)")
	cmd.Flags().
		StringSliceVar(&onlyFlag, "only", nil, "Build only the named processes (comma-separated, e.g. app,queue)")
	cmd.Flags().StringVar(
		&outputDirFlag,
		"output-dir",
		"",
		"Publish binaries into an isolated output directory (relative paths resolve from the project root)",
	)

	return cmd
}

// buildApp keeps the historical in-place build entry point used by tests and
// callers that do not need output isolation.
func buildApp(rootDir string, versionFlag string, only []string) (buildReport, error) {
	return buildProject(rootDir, buildOptions{Version: versionFlag, Only: only})
}

func buildProject(rootDir string, opts buildOptions) (buildReport, error) {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return buildReport{}, fmt.Errorf("failed to read andurel.toml: %w", err)
	}

	processes, err := discoverBuildProcesses(rootDir)
	if err != nil {
		return buildReport{}, err
	}
	selected, err := selectBuildProcesses(processes, opts.Only)
	if err != nil {
		return buildReport{}, err
	}

	output, err := resolveBuildOutputDir(rootDir, opts.OutputDir)
	if err != nil {
		return buildReport{}, err
	}

	appVersion := opts.Version
	if appVersion == "" {
		if v, err := detectGitVersion(rootDir); err == nil {
			appVersion = v
		}
	}

	workRoot := rootDir
	if output.Isolated {
		workRoot, err = stageBuildSource(rootDir, output.AbsPath)
		if err != nil {
			return buildReport{}, err
		}
		defer os.RemoveAll(workRoot)
	}

	report, err := runBuildSteps(workRoot, lock, selected, appVersion, output.Isolated)
	if err != nil {
		return buildReport{}, err
	}
	report.OutputDir = output.DisplayPath
	report.Isolated = output.Isolated

	if output.Isolated {
		if err := publishBuildArtifacts(workRoot, output.AbsPath, selected, &report); err != nil {
			return buildReport{}, err
		}
	}

	return report, nil
}

func runBuildSteps(
	rootDir string,
	lock *layout.AndurelLock,
	selected []buildProcess,
	appVersion string,
	isolated bool,
) (buildReport, error) {
	binDir := filepath.Join(rootDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return buildReport{}, fmt.Errorf("failed to create bin directory: %w", err)
	}
	distDir := filepath.Join(rootDir, "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return buildReport{}, fmt.Errorf("failed to create dist directory: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// 1. Templ generate
	if tool, ok := lock.Tools["templ"]; ok {
		if err := syncSingleToolFunc(rootDir, "templ", tool, goos, goarch); err != nil {
			return buildReport{}, fmt.Errorf("failed to sync templ: %w", err)
		}

		fmt.Println("Generating templ files...")
		templCmd := exec.Command(filepath.Join(binDir, "templ"), templGenerateArgs...)
		templCmd.Dir = rootDir
		templCmd.Stdout = os.Stdout
		templCmd.Stderr = os.Stderr
		if err := templCmd.Run(); err != nil {
			return buildReport{}, fmt.Errorf("templ generation failed: %w", err)
		}
	}

	// 1b. narsilc generate
	if hasQueries, err := cmds.HasQueryFiles(rootDir); err != nil {
		return buildReport{}, fmt.Errorf("check narsilc queries: %w", err)
	} else if hasQueries {
		tool, ok := lock.Tools["narsilc"]
		if !ok {
			return buildReport{}, fmt.Errorf("narsilc queries found but narsilc is not configured in andurel.lock")
		}
		if err := syncSingleToolFunc(rootDir, "narsilc", tool, goos, goarch); err != nil {
			return buildReport{}, fmt.Errorf("failed to sync narsilc: %w", err)
		}

		fmt.Println("Generating narsilc queries...")
		if err := cmds.RunNarsilcGenerate(rootDir); err != nil {
			return buildReport{}, fmt.Errorf("narsilc generation failed: %w", err)
		}
	}

	// 2. Tailwind compiles email CSS. Vite builds the application CSS.
	tailwindTool, ok := lock.Tools["tailwindcli"]
	if !ok {
		tailwindTool = layout.NewBinaryTool("tailwindcli", versions.TailwindCLI)
	}

	if err := syncSingleToolFunc(rootDir, "tailwindcli", tailwindTool, goos, goarch); err != nil {
		return buildReport{}, fmt.Errorf("failed to sync tailwind CLI: %w", err)
	}
	if hasEmailCompilerInputs(rootDir) {
		if err := compileEmailWithTailwind(
			context.Background(),
			rootDir,
			filepath.Join(binDir, "tailwindcli"),
		); err != nil {
			return buildReport{}, err
		}
	}

	// 2.5. Vite build for Inertia frontend. Migrate-only builds do not import
	// the embedded assets package, so they must not touch Vite output.
	assetsBuilt := false
	if lock.ScaffoldConfig != nil && layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) &&
		buildNeedsAssets(selected) {
		installCmd, buildCmd, err := inertiaPackageManagerCommands(
			lock.ScaffoldConfig.PackageManager(),
		)
		if err != nil {
			return buildReport{}, err
		}

		if isolated && buildHasNodeModules(rootDir) {
			fmt.Println("Reusing installed frontend dependencies...")
		} else {
			fmt.Printf("Installing %s dependencies...\n", installCmd.Name)
			install := exec.Command(installCmd.Name, installCmd.Args...)
			install.Dir = rootDir
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err != nil {
				return buildReport{}, fmt.Errorf("%s failed: %w", installCmd.String(), err)
			}
		}

		fmt.Println("Building Vite assets...")
		viteBuild := exec.Command(buildCmd.Name, buildCmd.Args...)
		viteBuild.Dir = rootDir
		viteBuild.Stdout = os.Stdout
		viteBuild.Stderr = os.Stderr
		if err := viteBuild.Run(); err != nil {
			return buildReport{}, fmt.Errorf("%s failed: %w", buildCmd.String(), err)
		}
		assetsBuilt = true
	}

	// 3. go mod download. Workspace mode resolves local use entries through
	// go build; go mod download would try to fetch a workspace module by its
	// required version instead of using the local directory.
	if !buildUsesGoWork(rootDir) {
		fmt.Println("Downloading Go dependencies...")
		dlCmd := exec.Command("go", "mod", "download")
		dlCmd.Dir = rootDir
		dlCmd.Env = buildGoEnv(rootDir)
		dlCmd.Stdout = os.Stdout
		dlCmd.Stderr = os.Stderr
		if err := dlCmd.Run(); err != nil {
			return buildReport{}, fmt.Errorf("go mod download failed: %w", err)
		}
	} else {
		fmt.Println("Using go.work; dependencies resolve during go build...")
	}

	// 4. Build each Go process.
	modulePath, err := extractModulePath(rootDir)
	if err != nil {
		return buildReport{}, fmt.Errorf("failed to determine module path from go.mod: %w", err)
	}

	reportVersion := appVersion
	if reportVersion == "" {
		reportVersion = "dev"
	}

	report := buildReport{
		Version:     reportVersion,
		OutputDir:   "dist",
		AssetsBuilt: assetsBuilt,
		Artifacts:   make([]buildArtifact, 0, len(selected)),
	}

	for _, process := range selected {
		artifacts, err := buildProcessBinary(rootDir, modulePath, process, appVersion)
		if err != nil {
			return buildReport{}, err
		}
		report.Artifacts = append(report.Artifacts, artifacts)
	}

	return report, nil
}

func buildProcessBinary(
	rootDir string,
	modulePath string,
	process buildProcess,
	appVersion string,
) (buildArtifact, error) {
	outputPath := filepath.Join("dist", process.Name)

	args := []string{"build", "-v"}
	var ldflags []string
	if appVersion != "" {
		ldflags = append(ldflags, "-X main.appVersion="+appVersion)
		if modulePath != "" {
			ldflags = append(
				ldflags,
				"-X "+modulePath+"/config.BuildVersion="+appVersion,
			)
		}
	}
	if len(ldflags) > 0 {
		args = append(args, "-ldflags", strings.Join(ldflags, " "))
	}
	args = append(args, "-o", outputPath, "./cmd/"+process.Name)

	fmt.Printf("Building %s...\n", process.Name)
	buildCmd := exec.Command("go", args...)
	buildCmd.Dir = rootDir
	buildCmd.Env = envWithBase(buildGoEnv(rootDir), map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
	})
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return buildArtifact{}, fmt.Errorf("build %s failed: %w", process.Name, err)
	}

	info, err := os.Stat(filepath.Join(rootDir, outputPath))
	if err != nil {
		return buildArtifact{}, fmt.Errorf("stat %s: %w", outputPath, err)
	}

	version := appVersion
	if version == "" {
		version = "dev"
	}

	return buildArtifact{
		Process: process.Name,
		Path:    filepath.ToSlash(outputPath),
		Size:    info.Size(),
		Version: version,
	}, nil
}

func discoverBuildProcesses(rootDir string) ([]buildProcess, error) {
	cmdDir := filepath.Join(rootDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"no buildable processes found under cmd/ (expected main packages besides seeds)",
			)
		}
		return nil, fmt.Errorf("read cmd directory: %w", err)
	}

	processes := make([]buildProcess, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "seeds" {
			continue
		}
		hasMain, err := hasMainPackage(filepath.Join(cmdDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if hasMain {
			processes = append(processes, buildProcess{Name: entry.Name()})
		}
	}

	if len(processes) == 0 {
		return nil, fmt.Errorf("no buildable processes found under cmd/ (expected main packages besides seeds)")
	}

	sort.Slice(processes, func(i, j int) bool { return processes[i].Name < processes[j].Name })
	return processes, nil
}

func hasMainPackage(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
		if file.Name.Name == "main" {
			return true, nil
		}
	}

	return false, nil
}

func selectBuildProcesses(
	processes []buildProcess,
	only []string,
) ([]buildProcess, error) {
	available := make([]string, 0, len(processes))
	known := make(map[string]bool, len(processes))
	for _, process := range processes {
		available = append(available, process.Name)
		known[process.Name] = true
	}
	availableList := strings.Join(available, ", ")

	if len(only) == 0 {
		return processes, nil
	}

	wanted := make(map[string]bool, len(only))
	for _, name := range only {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !known[name] {
			return nil, fmt.Errorf(
				"unknown process %q in --only (available: %s)",
				name,
				availableList,
			)
		}
		wanted[name] = true
	}

	selected := make([]buildProcess, 0, len(wanted))
	for _, process := range processes {
		if wanted[process.Name] {
			selected = append(selected, process)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf(
			"--only selected no processes (available: %s)",
			availableList,
		)
	}

	return selected, nil
}

func buildReportSummary(report buildReport) string {
	noun := "processes"
	if len(report.Artifacts) == 1 {
		noun = "process"
	}
	return fmt.Sprintf(
		"Built %d %s into %s (version %s)",
		len(report.Artifacts),
		noun,
		report.OutputDir,
		report.Version,
	)
}

func writeBuildReport(w io.Writer, report buildReport) error {
	if _, err := fmt.Fprintf(w, "✓ %s\n", buildReportSummary(report)); err != nil {
		return err
	}
	for _, artifact := range report.Artifacts {
		if _, err := fmt.Fprintf(
			w,
			"  %-8s %-16s %s\n",
			artifact.Process,
			artifact.Path,
			formatBuildSize(artifact.Size),
		); err != nil {
			return err
		}
	}
	return nil
}

func formatBuildSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}

	return fmt.Sprintf("%.1f TB", value/unit)
}

type shellCommand struct {
	Name string
	Args []string
}

// String returns the shell command as it should be displayed.
func (c shellCommand) String() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

func inertiaPackageManagerCommands(runtime string) (shellCommand, shellCommand, error) {
	if runtime == "" {
		runtime = "pnpm"
	}

	switch runtime {
	case "npm":
		return shellCommand{Name: "npm", Args: []string{"ci"}},
			shellCommand{Name: "npm", Args: []string{"run", "build"}},
			nil
	case "pnpm":
		return shellCommand{Name: "pnpm", Args: []string{"install", "--frozen-lockfile"}},
			shellCommand{Name: "pnpm", Args: []string{"run", "build"}},
			nil
	case "bun":
		return shellCommand{Name: "bun", Args: []string{"install", "--frozen-lockfile"}},
			shellCommand{Name: "bun", Args: []string{"run", "build"}},
			nil
	default:
		return shellCommand{}, shellCommand{}, fmt.Errorf(
			"invalid JavaScript package manager in andurel.lock: %s - valid options are 'pnpm', 'bun', 'npm'",
			runtime,
		)
	}
}

func detectGitVersion(rootDir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git describe failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func extractModulePath(rootDir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("could not open go.mod: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

func extractModuleName(rootDir string) (string, error) {
	modulePath, err := extractModulePath(rootDir)
	if err != nil {
		return "", err
	}
	parts := strings.Split(modulePath, "/")
	return parts[len(parts)-1], nil
}
