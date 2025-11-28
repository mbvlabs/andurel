// Package cmds holds commands being used for scaffolding
package cmds

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func RunGoModTidy(targetDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunGoFmt(targetDir string) error {
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunGoRunBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/run", "cmd/run/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunGoMigrationBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/migration", "cmd/migration/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunConsoleBin(targetDir string) error {
	cmd := exec.Command("go", "build", "-o", "bin/console", "cmd/console/main.go")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunTemplGenerate(targetDir string) error {
	cmd := exec.Command("go", "tool", "templ", "generate", "./views")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunTemplFmt(targetDir string) error {
	cmd := exec.Command("go", "tool", "templ", "fmt", "views")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunSqlcGenerate(targetDir string) error {
	cmd := exec.Command("go", "tool", "sqlc", "generate", "-f", "database/sqlc.yaml")
	cmd.Dir = targetDir
	return cmd.Run()
}

func RunGooseFix(targetDir string) error {
	cmd := exec.Command("go", "tool", "goose", "-dir", "database/migrations", "fix")
	cmd.Dir = targetDir
	return cmd.Run()
}

func SetupTailwind(targetDir string) error {
	return SetupTailwindWithVersion(targetDir, "v4.1.17", 10*time.Second)
}

func SetupTailwindWithVersion(targetDir, version string, timeout time.Duration) error {
	binPath := filepath.Join(targetDir, "bin", "tailwindcli")

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("Tailwind binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL := getTailwindDownloadURL(version)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download Tailwind: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Tailwind: status %d", resp.StatusCode)
	}

	out, err := os.Create(binPath)
	if err != nil {
		return fmt.Errorf("failed to create binary file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

func getTailwindDownloadURL(version string) string {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	var platform string
	switch runtime.GOOS {
	case "darwin":
		platform = fmt.Sprintf("macos-%s", arch)
	case "linux":
		platform = fmt.Sprintf("linux-%s", arch)
	case "windows":
		platform = fmt.Sprintf("windows-%s.exe", arch)
	default:
		platform = fmt.Sprintf("linux-%s", arch)
	}

	return fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/download/%s/tailwindcss-%s", version, platform)
}

