// Package cmds holds commands being used for scaffolding
package cmds

import (
	"archive/tar"
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	return SetupTailwindWithVersion(targetDir, "v4.1.17")
}

func SetupTailwindWithVersion(targetDir, version string) error {
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

	resp, err := http.Get(downloadURL)
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
	var arch string
	switch runtime.GOOS {
	case "darwin":
		arch = "macos-x64"
	case "linux":
		arch = "linux-x64"
	case "windows":
		arch = "windows-x64.exe"
	default:
		arch = "linux-x64"
	}

	return fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/download/%s/tailwindcss-%s", version, arch)
}

func SetupMailHog(targetDir string) error {
	return SetupMailHogWithVersion(targetDir, "v1.0.1")
}

func SetupMailHogWithVersion(targetDir, version string) error {
	binPath := filepath.Join(targetDir, "bin", "mailhog")

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("MailHog binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL := getMailHogDownloadURL(version)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download MailHog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download MailHog: status %d", resp.StatusCode)
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

func getMailHogDownloadURL(version string) string {
	var platform string
	switch runtime.GOOS {
	case "darwin":
		platform = "MailHog_darwin_amd64"
	case "linux":
		platform = "MailHog_linux_amd64"
	case "windows":
		platform = "MailHog_windows_amd64.exe"
	default:
		platform = "MailHog_linux_amd64"
	}

	return fmt.Sprintf("https://github.com/mailhog/MailHog/releases/download/%s/%s", version, platform)
}

func SetupUsql(targetDir string) error {
	return SetupUsqlWithVersion(targetDir, "v0.19.26")
}

func SetupUsqlWithVersion(targetDir, version string) error {
	binPath := filepath.Join(targetDir, "bin", "usql")

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("usql binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL := getUsqlDownloadURL(version)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download usql: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download usql: status %d", resp.StatusCode)
	}

	bzr := bzip2.NewReader(resp.Body)
	tr := tar.NewReader(bzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "usql" {
			out, err := os.Create(binPath)
			if err != nil {
				return fmt.Errorf("failed to create binary file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("failed to write binary: %w", err)
			}

			if err := os.Chmod(binPath, 0o755); err != nil {
				return fmt.Errorf("failed to make binary executable: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("usql binary not found in archive")
}

func getUsqlDownloadURL(version string) string {
	var platform string
	switch runtime.GOOS {
	case "darwin":
		platform = "darwin-amd64"
	case "linux":
		platform = "linux-amd64"
	case "windows":
		platform = "windows-amd64"
	default:
		platform = "linux-amd64"
	}

	return fmt.Sprintf("https://github.com/xo/usql/releases/download/%s/usql-%s-%s.tar.bz2", version, version, platform)
}
