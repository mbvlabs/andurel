package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDownloadURL(t *testing.T) {
	template := "https://example.com/{{version}}/{{version_no_v}}/{{os}}/{{arch}}/{{os_capitalized}}/{{arch_x86_64}}/{{os_tailwind}}/{{arch_tailwind}}"

	got := renderDownloadURL(template, "v1.2.3", "darwin", "amd64")
	want := "https://example.com/v1.2.3/1.2.3/darwin/amd64/Darwin/x86_64/macos/x64"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPlatformNormalizationHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "tailwind darwin", got: normalizeTailwindOS("darwin"), want: "macos"},
		{name: "tailwind linux", got: normalizeTailwindOS("linux"), want: "linux"},
		{name: "tailwind amd64", got: normalizeTailwindArch("amd64"), want: "x64"},
		{name: "tailwind arm64", got: normalizeTailwindArch("arm64"), want: "arm64"},
		{name: "map amd64", got: mapArch("amd64"), want: "x86_64"},
		{name: "map arm64", got: mapArch("arm64"), want: "arm64"},
		{name: "map 386", got: mapArch("386"), want: "i386"},
		{name: "map unknown", got: mapArch("riscv64"), want: "riscv64"},
		{name: "capitalize", got: capitalize("linux"), want: "Linux"},
		{name: "capitalize empty", got: capitalize(""), want: ""},
		{name: "repo github", got: extractGitHubRepo("github.com/owner/repo/cmd/tool"), want: "owner/repo"},
		{name: "repo other", got: extractGitHubRepo("example.com/owner/repo"), want: "example.com/owner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, tt.got)
			}
		})
	}
}

func TestToolDownloaderReleaseURLs(t *testing.T) {
	tests := []struct {
		name        string
		downloader  ToolDownloader
		goos        string
		goarch      string
		wantURLPart string
		wantArchive string
		wantErr     string
	}{
		{
			name:        "templ",
			downloader:  ToolDownloader{Name: "templ", Module: "github.com/a-h/templ/cmd/templ", Version: "v0.1.0"},
			goos:        "linux",
			goarch:      "amd64",
			wantURLPart: "templ_Linux_x86_64.tar.gz",
			wantArchive: "tar.gz",
		},
		{
			name:        "goose",
			downloader:  ToolDownloader{Name: "goose", Module: "github.com/pressly/goose/v3/cmd/goose", Version: "v3.1.0"},
			goos:        "darwin",
			goarch:      "arm64",
			wantURLPart: "goose_darwin_arm64",
			wantArchive: "binary",
		},
		{
			name:        "usql",
			downloader:  ToolDownloader{Name: "usql", Module: "github.com/xo/usql", Version: "v0.2.0"},
			goos:        "linux",
			goarch:      "amd64",
			wantURLPart: "usql-0.2.0-linux-amd64.tar.bz2",
			wantArchive: "tar.bz2",
		},
		{
			name:        "unknown",
			downloader:  ToolDownloader{Name: "unknown", Module: "github.com/example/tool", Version: "v1.0.0"},
			goos:        "linux",
			goarch:      "amd64",
			wantErr:     "unknown tool",
			wantArchive: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, archiveType, err := tt.downloader.getReleaseURL(tt.goos, tt.goarch)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("getReleaseURL failed: %v", err)
			}
			if !strings.Contains(url, tt.wantURLPart) {
				t.Fatalf("expected URL %q to contain %q", url, tt.wantURLPart)
			}
			if archiveType != tt.wantArchive {
				t.Fatalf("expected archive %q, got %q", tt.wantArchive, archiveType)
			}
		})
	}
}

func TestExtractBinary(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "tool.bin")
	if err := os.WriteFile(source, []byte("binary"), 0o644); err != nil {
		t.Fatalf("write binary source: %v", err)
	}

	t.Run("binary", func(t *testing.T) {
		dest := filepath.Join(tmpDir, "copied-tool")
		if err := extractBinary(source, "tool", dest, "binary"); err != nil {
			t.Fatalf("extract binary failed: %v", err)
		}
		assertFileContent(t, dest, "binary")
	})

	t.Run("tar.gz", func(t *testing.T) {
		archivePath := filepath.Join(tmpDir, "tool.tar.gz")
		writeTarGz(t, archivePath, "nested/tool-linux-amd64", "tar content")
		dest := filepath.Join(tmpDir, "tar-tool")
		if err := extractBinary(archivePath, "tool", dest, "tar.gz"); err != nil {
			t.Fatalf("extract tar.gz failed: %v", err)
		}
		assertFileContent(t, dest, "tar content")
	})

	t.Run("zip", func(t *testing.T) {
		archivePath := filepath.Join(tmpDir, "tool.zip")
		writeZip(t, archivePath, "bin/tool.exe", "zip content")
		dest := filepath.Join(tmpDir, "zip-tool")
		if err := extractBinary(archivePath, "tool", dest, "zip"); err != nil {
			t.Fatalf("extract zip failed: %v", err)
		}
		assertFileContent(t, dest, "zip content")
	})

	t.Run("missing binary", func(t *testing.T) {
		archivePath := filepath.Join(tmpDir, "missing.tar.gz")
		writeTarGz(t, archivePath, "other", "content")
		err := extractBinary(archivePath, "tool", filepath.Join(tmpDir, "missing-tool"), "tar.gz")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected missing binary error, got %v", err)
		}
	})

	t.Run("unsupported archive", func(t *testing.T) {
		err := extractBinary(source, "tool", filepath.Join(tmpDir, "unsupported"), "rar")
		if err == nil || !strings.Contains(err.Error(), "unsupported archive type") {
			t.Fatalf("expected unsupported archive error, got %v", err)
		}
	})
}

func writeTarGz(t *testing.T, path, name, content string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
}

func writeZip(t *testing.T, path, name, content string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write zip content: %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("expected %q, got %q", want, string(data))
	}
}
