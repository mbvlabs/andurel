// Package cmds holds commands being used for scaffolding
package cmds

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/versions"
)

func RunGoModTidy(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = absTargetDir

	return cmd.Run()
}

func RunGoFmt(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func RunGoFmtPath(targetDir, path string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "fmt", path)
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func RunGolines(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command("golines", "-w", "-m", "100", ".")
	cmd.Dir = absTargetDir
	return cmd.Run()
}

func RunTemplGenerate(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command(
		"go",
		"run",
		"github.com/a-h/templ/cmd/templ@"+versions.Templ,
		"generate",
		"./views",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}

func RunTemplFmt(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
	cmd := exec.Command(
		"go",
		"run",
		"github.com/a-h/templ/cmd/templ@"+versions.Templ,
		"fmt",
		"views",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
	// }
	//
	// templBin := filepath.Join(absTargetDir, "bin", "templ")
	// cmd := exec.Command(templBin, "fmt", "views")
	// cmd.Dir = absTargetDir
	// return cmd.Run()
}

func RunSqlcGenerate(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
	cmd := exec.Command(
		"go",
		"run",
		"github.com/sqlc-dev/sqlc/cmd/sqlc@"+versions.Sqlc,
		"generate",
		"-f",
		"database/sqlc.yaml",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
	// }

	// sqlcBin := filepath.Join(absTargetDir, "bin", "sqlc")
	// cmd := exec.Command(sqlcBin, "generate", "-f", "database/sqlc.yaml")
	// cmd.Dir = absTargetDir
	// return cmd.Run()
}

func RunGooseFix(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
	cmd := exec.Command(
		"go",
		"run",
		"github.com/pressly/goose/v3/cmd/goose@"+versions.Goose,
		"-dir",
		"database/migrations",
		"fix",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
	// }
	//
	// gooseBin := filepath.Join(absTargetDir, "bin", "goose")
	// cmd := exec.Command(gooseBin, "-dir", "database/migrations", "fix")
	// cmd.Dir = absTargetDir
	// return cmd.Run()
}

// func SetupTailwind(targetDir string) error {
// 	return SetupTailwindWithVersion(targetDir, "v4.1.17", 10*time.Second)
// }
//
// func SetupTailwindWithVersion(targetDir, version string, timeout time.Duration) error {
// 	absTargetDir, err := filepath.Abs(targetDir)
// 	if err != nil {
// 		return fmt.Errorf("failed to get absolute path: %w", err)
// 	}
// 	binPath := filepath.Join(absTargetDir, "bin", "tailwindcli")
//
// 	if _, err := os.Stat(binPath); err == nil {
// 		fmt.Printf("Tailwind binary already exists at: %s\n", binPath)
// 		return nil
// 	}
//
// 	binDir := filepath.Join(absTargetDir, "bin")
// 	if err := os.MkdirAll(binDir, 0o755); err != nil {
// 		return fmt.Errorf("failed to create bin directory: %w", err)
// 	}
//
// 	downloadURL := getTailwindDownloadURL(version)
//
// 	client := &http.Client{
// 		Timeout: timeout,
// 	}
// 	resp, err := client.Get(downloadURL)
// 	if err != nil {
// 		return fmt.Errorf("failed to download Tailwind: %w", err)
// 	}
// 	defer resp.Body.Close()
//
// 	if resp.StatusCode != http.StatusOK {
// 		return fmt.Errorf("failed to download Tailwind: status %d", resp.StatusCode)
// 	}
//
// 	out, err := os.Create(binPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create binary file: %w", err)
// 	}
// 	defer out.Close()
//
// 	if _, err := io.Copy(out, resp.Body); err != nil {
// 		return fmt.Errorf("failed to write binary: %w", err)
// 	}
//
// 	if err := os.Chmod(binPath, 0o755); err != nil {
// 		return fmt.Errorf("failed to make binary executable: %w", err)
// 	}
//
// 	return nil
// }

// func getTailwindDownloadURL(version string) string {
// 	arch := "x64"
// 	if runtime.GOARCH == "arm64" {
// 		arch = "arm64"
// 	}
//
// 	var platform string
// 	switch runtime.GOOS {
// 	case "darwin":
// 		platform = fmt.Sprintf("macos-%s", arch)
// 	case "linux":
// 		platform = fmt.Sprintf("linux-%s", arch)
// 	case "windows":
// 		platform = fmt.Sprintf("windows-%s.exe", arch)
// 	default:
// 		platform = fmt.Sprintf("linux-%s", arch)
// 	}
//
// 	return fmt.Sprintf(
// 		"https://github.com/tailwindlabs/tailwindcss/releases/download/%s/tailwindcss-%s",
// 		version,
// 		platform,
// 	)
// }
