package cmds

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	originalClient := downloadHTTPClient
	downloadHTTPClient = server.Client()
	t.Cleanup(func() { downloadHTTPClient = originalClient })

	tmpDir := t.TempDir()
	binaryDest := filepath.Join(tmpDir, "tool")
	err := DownloadVerifiedFromURLTemplate(
		"tool",
		"v1.2.3",
		server.URL+"/{{version}}/{{os}}/{{arch}}/tool",
		"",
		"",
		"linux",
		"amd64",
		binaryDest,
		sha256Hex("binary content"),
	)
	if err != nil {
		t.Fatalf("DownloadFromURLTemplate binary: %v", err)
	}
	assertFileContent(t, binaryDest, "binary content")
	if mode := executableMode(t, binaryDest); mode&0o111 == 0 {
		t.Fatalf("expected executable mode, got %v", mode)
	}

	tarDest := filepath.Join(tmpDir, "tar-tool")
	archivePath := filepath.Join(tmpDir, "expected.tar.gz")
	writeTarGz(t, archivePath, "nested/tool-linux-amd64", "tar content")
	archiveDigest := sha256File(t, archivePath)
	if err := DownloadVerifiedFromURL("tool", server.URL+"/archive.tar.gz", "tar.gz", "tool-linux-amd64", tarDest, archiveDigest); err != nil {
		t.Fatalf("DownloadFromURL tar.gz: %v", err)
	}
	assertFileContent(t, tarDest, "tar content")

	zipDest := filepath.Join(tmpDir, "zip-tool")
	zipPath := filepath.Join(tmpDir, "expected.zip")
	writeZip(t, zipPath, "bin/tool.exe", "zip content")
	zipDigest := sha256File(t, zipPath)
	if err := DownloadVerifiedFromURL("tool", server.URL+"/archive.zip", "zip", "tool.exe", zipDest, zipDigest); err != nil {
		t.Fatalf("DownloadFromURL zip: %v", err)
	}
	assertFileContent(t, zipDest, "zip content")

	if err := DownloadFromURLTemplate("tool", "v1.0.0", "", "binary", "", "linux", "amd64", filepath.Join(tmpDir, "missing")); err == nil {
		t.Fatalf("expected missing urlTemplate error")
	}
	err = DownloadVerifiedFromURL("tool", server.URL+"/missing", "binary", "tool", filepath.Join(tmpDir, "missing"), strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "unexpected status code 404") {
		t.Fatalf("expected status error, got %v", err)
	}
	if len(requested) == 0 || requested[0] != "/v1.2.3/linux/amd64/tool" {
		t.Fatalf("unexpected requested paths: %#v", requested)
	}
}

func TestDownloadGoToolAndTailwindUseResolvedAssets(t *testing.T) {
	originalDownloadVerified := downloadVerifiedFunc
	t.Cleanup(func() {
		downloadVerifiedFunc = originalDownloadVerified
	})

	var downloadedURLs []string
	downloadVerifiedFunc = func(name, sourceURL, archiveType, binaryName, destPath, digest string) error {
		downloadedURLs = append(downloadedURLs, sourceURL)
		if archiveType == "tar.gz" {
			return os.WriteFile(destPath, []byte("templ binary"), 0o755)
		}
		return os.WriteFile(destPath, []byte("plain binary"), 0o644)
	}

	tmpDir := t.TempDir()
	templDest := filepath.Join(tmpDir, "templ")
	if err := DownloadVerifiedGoTool("templ", "github.com/a-h/templ/cmd/templ", "v0.3.0", "linux", "amd64", templDest, strings.Repeat("1", 64)); err != nil {
		t.Fatalf("DownloadGoTool templ: %v", err)
	}
	assertFileContent(t, templDest, "templ binary")

	tailwindDest := filepath.Join(tmpDir, "tailwindcli")
	if err := DownloadVerifiedTailwindCLI("v4.0.0", "darwin", "amd64", tailwindDest, strings.Repeat("2", 64)); err != nil {
		t.Fatalf("DownloadTailwindCLI: %v", err)
	}
	assertFileContent(t, tailwindDest, "plain binary")

	if len(downloadedURLs) != 2 ||
		!strings.Contains(downloadedURLs[0], "templ_Linux_x86_64.tar.gz") ||
		!strings.Contains(downloadedURLs[1], "tailwindcss-macos-x64") {
		t.Fatalf("unexpected downloaded URLs: %#v", downloadedURLs)
	}

	if err := DownloadVerifiedGoTool("unknown", "github.com/example/tool", "v1.0.0", "linux", "amd64", filepath.Join(tmpDir, "unknown"), strings.Repeat("3", 64)); err == nil ||
		!strings.Contains(err.Error(), ErrFailedToGetRleaseURL.Error()) {
		t.Fatalf("expected release URL error, got %v", err)
	}

	downloadVerifiedFunc = func(name, sourceURL, archiveType, binaryName, destPath, digest string) error {
		return os.ErrPermission
	}
	if err := DownloadVerifiedTailwindCLI("v4.0.0", "linux", "arm64", filepath.Join(tmpDir, "blocked"), strings.Repeat("4", 64)); err == nil ||
		!strings.Contains(err.Error(), "failed to download tailwindcli") {
		t.Fatalf("expected tailwind download error, got %v", err)
	}
}

func TestUnverifiedDownloadWrappersRejectMissingDigest(t *testing.T) {
	if err := DownloadGoTool("templ", "github.com/a-h/templ/cmd/templ", "v0.3.0", "linux", "amd64", filepath.Join(t.TempDir(), "templ")); err == nil ||
		!strings.Contains(err.Error(), "valid SHA-256 digest is required") {
		t.Fatalf("DownloadGoTool error = %v", err)
	}
	if err := DownloadTailwindCLI("v4.0.0", "linux", "amd64", filepath.Join(t.TempDir(), "tailwindcli")); err == nil ||
		!strings.Contains(err.Error(), "valid SHA-256 digest is required") {
		t.Fatalf("DownloadTailwindCLI error = %v", err)
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
		writeTarGz(t, archivePath, "nested/tool", "tar content")
		dest := filepath.Join(tmpDir, "tar-tool")
		if err := extractBinary(archivePath, "tool", dest, "tar.gz"); err != nil {
			t.Fatalf("extract tar.gz failed: %v", err)
		}
		assertFileContent(t, dest, "tar content")
	})

	t.Run("zip", func(t *testing.T) {
		archivePath := filepath.Join(tmpDir, "tool.zip")
		writeZip(t, archivePath, "bin/tool", "zip content")
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
	gzw := gzip.NewWriter(file)
	tw := tar.NewWriter(gzw)

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
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tar.gz: %v", err)
	}
}

func writeTarGzResponse(t *testing.T, w http.ResponseWriter, name, content string) {
	t.Helper()

	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)

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
	if err := tw.Close(); err != nil {
		t.Fatalf("close response tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close response gzip writer: %v", err)
	}
}

func writeZip(t *testing.T, path, name, content string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(file)

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write zip content: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func writeZipResponse(t *testing.T, w http.ResponseWriter, name, content string) {
	t.Helper()

	zw := zip.NewWriter(w)

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create response zip entry: %v", err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write response zip content: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close response zip writer: %v", err)
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

func sha256Hex(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read digest input: %v", err)
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
