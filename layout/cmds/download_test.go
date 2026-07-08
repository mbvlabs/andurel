package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
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
			name:        "mailpit",
			downloader:  ToolDownloader{Name: "mailpit", Module: "github.com/axllent/mailpit", Version: "v1.2.0"},
			goos:        "linux",
			goarch:      "arm64",
			wantURLPart: "mailpit-linux-arm64.tar.gz",
			wantArchive: "tar.gz",
		},
		{
			name:        "dblab",
			downloader:  ToolDownloader{Name: "dblab", Module: "github.com/danvergara/dblab", Version: "v0.9.0"},
			goos:        "darwin",
			goarch:      "arm64",
			wantURLPart: "dblab_0.9.0_darwin_arm64.tar.gz",
			wantArchive: "tar.gz",
		},
		{
			name:        "shadowfax",
			downloader:  ToolDownloader{Name: "shadowfax", Module: "github.com/mbvlabs/shadowfax", Version: "v0.1.0"},
			goos:        "linux",
			goarch:      "amd64",
			wantURLPart: "shadowfax-linux-amd64",
			wantArchive: "binary",
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

func TestDownloadFromURLTemplateAndURL(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		switch r.URL.Path {
		case "/v1.2.3/linux/amd64/tool":
			_, _ = w.Write([]byte("binary content"))
		case "/archive.tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			writeTarGzResponse(t, w, "nested/tool-linux-amd64", "tar content")
		case "/archive.zip":
			w.Header().Set("Content-Type", "application/zip")
			writeZipResponse(t, w, "bin/tool.exe", "zip content")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	binaryDest := filepath.Join(tmpDir, "tool")
	err := DownloadFromURLTemplate(
		"tool",
		"v1.2.3",
		server.URL+"/{{version}}/{{os}}/{{arch}}/tool",
		"",
		"",
		"linux",
		"amd64",
		binaryDest,
	)
	if err != nil {
		t.Fatalf("DownloadFromURLTemplate binary: %v", err)
	}
	assertFileContent(t, binaryDest, "binary content")
	if mode := executableMode(t, binaryDest); mode&0o111 == 0 {
		t.Fatalf("expected executable mode, got %v", mode)
	}

	tarDest := filepath.Join(tmpDir, "tar-tool")
	if err := DownloadFromURL("tool", server.URL+"/archive.tar.gz", "tar.gz", "tool", tarDest); err != nil {
		t.Fatalf("DownloadFromURL tar.gz: %v", err)
	}
	assertFileContent(t, tarDest, "tar content")

	zipDest := filepath.Join(tmpDir, "zip-tool")
	if err := DownloadFromURL("tool", server.URL+"/archive.zip", "zip", "tool", zipDest); err != nil {
		t.Fatalf("DownloadFromURL zip: %v", err)
	}
	assertFileContent(t, zipDest, "zip content")

	if err := DownloadFromURLTemplate("tool", "v1.0.0", "", "binary", "", "linux", "amd64", filepath.Join(tmpDir, "missing")); err == nil {
		t.Fatalf("expected missing urlTemplate error")
	}
	err = DownloadFromURL("tool", server.URL+"/missing", "binary", "tool", filepath.Join(tmpDir, "missing"))
	if err == nil || !strings.Contains(err.Error(), "unexpected status code 404") {
		t.Fatalf("expected status error, got %v", err)
	}
	if len(requested) == 0 || requested[0] != "/v1.2.3/linux/amd64/tool" {
		t.Fatalf("unexpected requested paths: %#v", requested)
	}
}

func TestDownloadGoToolAndTailwindUseResolvedAssets(t *testing.T) {
	originalDownloadFile := downloadFileFunc
	t.Cleanup(func() {
		downloadFileFunc = originalDownloadFile
	})

	var downloadedURLs []string
	downloadFileFunc = func(url, destPath string) error {
		downloadedURLs = append(downloadedURLs, url)
		if strings.HasSuffix(destPath, ".tar.gz") {
			writeTarGz(t, destPath, "bin/templ", "templ binary")
			return nil
		}
		return os.WriteFile(destPath, []byte("plain binary"), 0o644)
	}

	tmpDir := t.TempDir()
	templDest := filepath.Join(tmpDir, "templ")
	if err := DownloadGoTool("templ", "github.com/a-h/templ/cmd/templ", "v0.3.0", "linux", "amd64", templDest); err != nil {
		t.Fatalf("DownloadGoTool templ: %v", err)
	}
	assertFileContent(t, templDest, "templ binary")

	tailwindDest := filepath.Join(tmpDir, "tailwindcli")
	if err := DownloadTailwindCLI("v4.0.0", "darwin", "amd64", tailwindDest); err != nil {
		t.Fatalf("DownloadTailwindCLI: %v", err)
	}
	assertFileContent(t, tailwindDest, "plain binary")

	if len(downloadedURLs) != 2 ||
		!strings.Contains(downloadedURLs[0], "templ_Linux_x86_64.tar.gz") ||
		!strings.Contains(downloadedURLs[1], "tailwindcss-macos-x64") {
		t.Fatalf("unexpected downloaded URLs: %#v", downloadedURLs)
	}

	if err := DownloadGoTool("unknown", "github.com/example/tool", "v1.0.0", "linux", "amd64", filepath.Join(tmpDir, "unknown")); err == nil ||
		!strings.Contains(err.Error(), ErrFailedToGetRleaseURL.Error()) {
		t.Fatalf("expected release URL error, got %v", err)
	}

	downloadFileFunc = func(url, destPath string) error {
		return os.ErrPermission
	}
	if err := DownloadTailwindCLI("v4.0.0", "linux", "arm64", filepath.Join(tmpDir, "blocked")); err == nil ||
		!strings.Contains(err.Error(), "failed to download tailwindcli") {
		t.Fatalf("expected tailwind download error, got %v", err)
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

func TestDownloadHelpersErrorsAndPlatform(t *testing.T) {
	if goos, goarch := GetPlatform(); goos == "" || goarch == "" {
		t.Fatalf("GetPlatform returned empty values: %q/%q", goos, goarch)
	}

	tmpDir := t.TempDir()
	if err := copyFile(filepath.Join(tmpDir, "missing"), filepath.Join(tmpDir, "dest")); err == nil {
		t.Fatalf("expected copy missing source error")
	}
	source := filepath.Join(tmpDir, "source")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := copyFile(source, filepath.Join(tmpDir, "missing", "dest")); err == nil {
		t.Fatalf("expected copy destination error")
	}

	if err := extractTarGz(filepath.Join(tmpDir, "missing.tar.gz"), "tool", filepath.Join(tmpDir, "tool")); err == nil {
		t.Fatalf("expected missing tar.gz error")
	}
	if err := extractZip(filepath.Join(tmpDir, "missing.zip"), "tool", filepath.Join(tmpDir, "tool")); err == nil {
		t.Fatalf("expected missing zip error")
	}
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

func writeTarGzResponse(t *testing.T, w http.ResponseWriter, name, content string) {
	t.Helper()

	gzw := gzip.NewWriter(w)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	header := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write response tar header: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("write response tar content: %v", err)
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

func writeZipResponse(t *testing.T, w http.ResponseWriter, name, content string) {
	t.Helper()

	zw := zip.NewWriter(w)
	defer zw.Close()

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create response zip entry: %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write response zip content: %v", err)
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

func executableMode(t *testing.T, path string) os.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode()
}
