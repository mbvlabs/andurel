package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/spf13/cobra"
)

func newBuildCommand() *cobra.Command {
	var versionFlag string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the application for production",
		Long: `Build the application binary and compile all assets for production deployment.

This command:
  • Downloads templ and generates views
  • Builds Tailwind CSS
  • Builds Vite assets (if Inertia is configured)
  • Downloads Go dependencies
  • Compiles the application binary as a static Linux binary
  • Injects a version string via ldflags (from --version or git describe)`,
		Example: `  andurel build
  andurel build --version 1.2.3`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			return buildApp(rootDir, versionFlag)
		},
	}

	cmd.Flags().StringVar(&versionFlag, "version", "", "Set the application version (injected via ldflags)")

	return cmd
}

func buildApp(rootDir string, versionFlag string) error {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return fmt.Errorf("failed to read andurel.lock: %w", err)
	}

	binDir := filepath.Join(rootDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// 1. Templ generate
	if tool, ok := lock.Tools["templ"]; ok {
		if err := syncSingleToolFunc(rootDir, "templ", tool, goos, goarch); err != nil {
			return fmt.Errorf("failed to sync templ: %w", err)
		}

		fmt.Println("Generating templ views...")
		templCmd := exec.Command(filepath.Join(binDir, "templ"), "generate")
		templCmd.Dir = rootDir
		templCmd.Stdout = os.Stdout
		templCmd.Stderr = os.Stderr
		if err := templCmd.Run(); err != nil {
			return fmt.Errorf("templ generation failed: %w", err)
		}
	}

	// 2. Tailwind minify
	tailwindTool, ok := lock.Tools["tailwindcli"]
	if !ok {
		tailwindTool = layout.NewBinaryTool("tailwindcli", versions.TailwindCLI)
	}

	if err := syncSingleToolFunc(rootDir, "tailwindcli", tailwindTool, goos, goarch); err != nil {
		return fmt.Errorf("failed to sync tailwind CLI: %w", err)
	}

	fmt.Println("Building tailwind CSS...")
	twCmd := exec.Command(
		filepath.Join(binDir, "tailwindcli"),
		"-i", "./css/base.css",
		"-o", "./assets/css/style.css",
		"--minify",
	)
	twCmd.Dir = rootDir
	twCmd.Stdout = os.Stdout
	twCmd.Stderr = os.Stderr
	if err := twCmd.Run(); err != nil {
		return fmt.Errorf("tailwind build failed: %w", err)
	}

	// 2.5. Vite build for Inertia frontend
	if lock.ScaffoldConfig != nil && layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		installCmd, buildCmd, err := inertiaPackageManagerCommands(lock.ScaffoldConfig.JavaScriptRuntime)
		if err != nil {
			return err
		}

		fmt.Printf("Installing %s dependencies...\n", installCmd.Name)
		install := exec.Command(installCmd.Name, installCmd.Args...)
		install.Dir = rootDir
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", installCmd.String(), err)
		}

		fmt.Println("Building Vite assets...")
		viteBuild := exec.Command(buildCmd.Name, buildCmd.Args...)
		viteBuild.Dir = rootDir
		viteBuild.Stdout = os.Stdout
		viteBuild.Stderr = os.Stderr
		if err := viteBuild.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", buildCmd.String(), err)
		}
	}

	// 3. go mod download
	fmt.Println("Downloading Go dependencies...")
	dlCmd := exec.Command("go", "mod", "download")
	dlCmd.Dir = rootDir
	dlCmd.Stdout = os.Stdout
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		return fmt.Errorf("go mod download failed: %w", err)
	}

	// 4. Build Go app
	binName, err := extractModuleName(rootDir)
	if err != nil {
		return fmt.Errorf("failed to determine binary name from go.mod: %w", err)
	}

	appVersion := versionFlag
	if appVersion == "" {
		if v, err := detectGitVersion(rootDir); err == nil {
			appVersion = v
		}
	}

	args := []string{"build", "-v"}
	if appVersion != "" {
		args = append(args, "-ldflags", fmt.Sprintf("-X main.appVersion=%s", appVersion))
	}
	args = append(args, "-o", binName, "./cmd/app")

	fmt.Printf("Building %s...\n", binName)
	buildCmd := exec.Command("go", args...)
	buildCmd.Dir = rootDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if appVersion != "" {
		fmt.Printf("✓ Build complete: %s (%s)\n", binName, appVersion)
	} else {
		fmt.Printf("✓ Build complete: %s\n", binName)
	}
	return nil
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
		runtime = "npm"
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
	case "yarn":
		return shellCommand{Name: "yarn", Args: []string{"install", "--frozen-lockfile"}},
			shellCommand{Name: "yarn", Args: []string{"build"}},
			nil
	default:
		return shellCommand{}, shellCommand{}, fmt.Errorf(
			"invalid JavaScript runtime in andurel.lock: %s - valid options are 'npm', 'pnpm', 'bun', 'yarn'",
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

func extractModuleName(rootDir string) (string, error) {
	f, err := os.Open(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("could not open go.mod: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			parts := strings.Split(modulePath, "/")
			return parts[len(parts)-1], nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}
