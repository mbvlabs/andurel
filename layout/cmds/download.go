package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type ToolDownloader struct {
	Name    string
	Module  string
	Version string
}

func DownloadGoTool(name, module, version, goos, goarch, destPath string) error {
	downloader := &ToolDownloader{
		Name:    name,
		Module:  module,
		Version: version,
	}

	url, archiveType, err := downloader.getReleaseURL(goos, goarch)
	if err != nil {
		return fmt.Errorf("failed to get release URL for %s: %w", name, err)
	}

	tmpDir, err := os.MkdirTemp("", "andurel-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, fmt.Sprintf("%s.%s", name, archiveType))

	if err := downloadFile(url, archivePath); err != nil {
		return fmt.Errorf("failed to download %s: %w", name, err)
	}

	if err := extractBinary(archivePath, name, destPath, archiveType); err != nil {
		return fmt.Errorf("failed to extract %s: %w", name, err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	return nil
}

func (d *ToolDownloader) getReleaseURL(goos, goarch string) (string, string, error) {
	repo := extractGitHubRepo(d.Module)

	switch d.Name {
	case "templ":
		os := capitalize(goos)
		arch := mapArch(goarch)
		if goos == "windows" {
			return fmt.Sprintf("https://github.com/%s/releases/download/%s/templ_%s_%s.zip",
				repo, d.Version, os, arch), "zip", nil
		}
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/templ_%s_%s.tar.gz",
			repo, d.Version, os, arch), "tar.gz", nil

	case "sqlc":
		os := goos
		arch := goarch
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/sqlc_%s_%s_%s.tar.gz",
			repo, d.Version, d.Version[1:], os, arch), "tar.gz", nil

	case "goose":
		os := goos
		arch := mapArch(goarch)
		ext := ""
		if goos == "windows" {
			ext = ".exe"
		}
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/goose_%s_%s%s",
			repo, d.Version, os, arch, ext), "binary", nil

	case "mailpit":
		os := goos
		arch := goarch
		if goos == "windows" {
			return fmt.Sprintf("https://github.com/%s/releases/download/%s/mailpit-%s-%s.zip",
				repo, d.Version, os, arch), "zip", nil
		}
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/mailpit-%s-%s.tar.gz",
			repo, d.Version, os, arch), "tar.gz", nil

	case "usql":
		os := goos
		arch := goarch
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/usql-%s-%s-%s.tar.bz2",
			repo, d.Version, d.Version[1:], os, arch), "tar.bz2", nil

	case "dblab":
		os := goos
		arch := goarch
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/dblab_%s_%s_%s.tar.gz",
			repo, d.Version, d.Version[1:], os, arch), "tar.gz", nil

	default:
		return "", "", fmt.Errorf("unknown tool: %s", d.Name)
	}
}

func DownloadTailwindCLI(version, goos, goarch, destPath string) error {
	osName := goos
	if osName == "darwin" {
		osName = "macos"
	}

	arch := goarch
	if arch == "amd64" {
		arch = "x64"
	}

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	url := fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/download/%s/tailwindcss-%s-%s%s",
		version, osName, arch, ext)

	if err := downloadFile(url, destPath); err != nil {
		return fmt.Errorf("failed to download tailwindcli: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	return nil
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func extractBinary(archivePath, binaryName, destPath, archiveType string) error {
	switch archiveType {
	case "tar.gz":
		return extractTarGz(archivePath, binaryName, destPath)
	case "tar.bz2":
		return extractTarBz2(archivePath, binaryName, destPath)
	case "zip":
		return extractZip(archivePath, binaryName, destPath)
	case "binary":
		return copyFile(archivePath, destPath)
	default:
		return fmt.Errorf("unsupported archive type: %s", archiveType)
	}
}

func extractTarGz(archivePath, binaryName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
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

		if header.Typeflag == tar.TypeReg {
			baseName := filepath.Base(header.Name)
			if baseName == binaryName || baseName == binaryName+".exe" || strings.HasPrefix(baseName, binaryName) {
				out, err := os.Create(destPath)
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer out.Close()

				if _, err := io.Copy(out, tr); err != nil {
					return fmt.Errorf("failed to extract binary: %w", err)
				}

				return nil
			}
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractTarBz2(archivePath, binaryName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	bzr := bzip2.NewReader(f)
	tr := tar.NewReader(bzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			baseName := filepath.Base(header.Name)
			if baseName == binaryName || baseName == binaryName+".exe" || strings.HasPrefix(baseName, binaryName) {
				out, err := os.Create(destPath)
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer out.Close()

				if _, err := io.Copy(out, tr); err != nil {
					return fmt.Errorf("failed to extract binary: %w", err)
				}

				return nil
			}
		}
	}

	return fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractZip(archivePath, binaryName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		if baseName == binaryName || baseName == binaryName+".exe" || strings.HasPrefix(baseName, binaryName) {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("failed to extract binary: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("binary %s not found in zip", binaryName)
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return goarch
	}
}

func GetPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

func extractGitHubRepo(module string) string {
	module = strings.TrimPrefix(module, "github.com/")
	parts := strings.Split(module, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return module
}
