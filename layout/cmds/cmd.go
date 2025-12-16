// Package cmds holds commands being used for scaffolding
package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

func SetupMailpit(targetDir string) error {
	return SetupMailpitWithVersion(targetDir, "v1.27.11", 10*time.Second)
}

func SetupMailpitWithVersion(targetDir, version string, timeout time.Duration) error {
	binPath := filepath.Join(targetDir, "bin", "mailpit")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("Mailpit binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL := getMailpitDownloadURL(version)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download Mailpit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Mailpit: status %d", resp.StatusCode)
	}

	if runtime.GOOS == "windows" {
		return extractMailpitFromZip(resp.Body, binPath)
	}

	return extractMailpitFromTarGz(resp.Body, binPath)
}

func extractMailpitFromTarGz(r io.Reader, binPath string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "mailpit" {
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

	return fmt.Errorf("mailpit binary not found in archive")
}

func extractMailpitFromZip(r io.Reader, binPath string) error {
	tmpFile, err := os.CreateTemp("", "mailpit-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	tmpFile.Close()

	zr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "mailpit.exe" {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer rc.Close()

			out, err := os.Create(binPath)
			if err != nil {
				return fmt.Errorf("failed to create binary file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("failed to write binary: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("mailpit.exe not found in zip archive")
}

func getMailpitDownloadURL(version string) string {
	var platform string
	var ext string
	switch runtime.GOOS {
	case "darwin":
		platform = "mailpit-darwin-amd64"
		ext = "tar.gz"
	case "linux":
		platform = "mailpit-linux-amd64"
		ext = "tar.gz"
	case "windows":
		platform = "mailpit-windows-amd64"
		ext = "zip"
	default:
		platform = "mailpit-linux-amd64"
		ext = "tar.gz"
	}

	return fmt.Sprintf("https://github.com/axllent/mailpit/releases/download/%s/%s.%s", version, platform, ext)
}

func SetupDblab(targetDir string) error {
	return SetupDblabWithVersion(targetDir, "v0.34.2", 10*time.Second)
}

func SetupDblabWithVersion(targetDir, version string, timeout time.Duration) error {
	binPath := filepath.Join(targetDir, "bin", "dblab")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("dblab binary already exists at: %s\n", binPath)
		return nil
	}

	binDir := filepath.Join(targetDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	downloadURL := getDblabDownloadURL(version)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download dblab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download dblab: status %d", resp.StatusCode)
	}

	return extractDblabFromTarGz(resp.Body, binPath)
}

func extractDblabFromTarGz(r io.Reader, binPath string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	binaryName := "dblab"
	if runtime.GOOS == "windows" {
		binaryName = "dblab.exe"
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == binaryName {
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

	return fmt.Errorf("dblab binary not found in archive")
}

func getDblabDownloadURL(version string) string {
	var platform string
	var arch string
	switch runtime.GOOS {
	case "darwin":
		platform = "darwin"
		arch = "amd64"
	case "linux":
		platform = "linux"
		arch = "amd64"
	case "windows":
		platform = "windows"
		arch = "amd64"
	default:
		platform = "linux"
		arch = "amd64"
	}

	versionWithoutV := version
	if len(version) > 0 && version[0] == 'v' {
		versionWithoutV = version[1:]
	}

	return fmt.Sprintf("https://github.com/danvergara/dblab/releases/download/%s/dblab_%s_%s_%s.tar.gz", version, versionWithoutV, platform, arch)
}
