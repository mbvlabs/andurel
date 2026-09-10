package cmds

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseChecksumManifestFormats(t *testing.T) {
	body := strings.Join([]string{
		"# comment",
		strings.Repeat("a", 64) + "  tool-linux-amd64",
		strings.Repeat("b", 64) + " *tool-linux-arm64",
		"SHA256 (./tool-darwin-amd64) = " + strings.Repeat("C", 64),
		"not-a-checksum line",
		"",
	}, "\n")

	got := parseChecksumManifest(body)
	want := map[string]string{
		"tool-linux-amd64":  strings.Repeat("a", 64),
		"tool-linux-arm64":  strings.Repeat("b", 64),
		"tool-darwin-amd64": strings.Repeat("c", 64),
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d entries, want %d: %#v", len(got), len(want), got)
	}
	for name, digest := range want {
		if got[name] != digest {
			t.Fatalf("digest for %s = %q, want %q", name, got[name], digest)
		}
	}
}

func TestResolveURLTemplateChecksumsFromManifest(t *testing.T) {
	linuxAMD := strings.Repeat("1", 64)
	linuxARM := strings.Repeat("2", 64)
	darwinAMD := strings.Repeat("3", 64)
	darwinARM := strings.Repeat("4", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.2.3/checksums.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(strings.Join([]string{
			linuxAMD + "  tool-linux-amd64",
			linuxARM + "  tool-linux-arm64",
			darwinAMD + "  tool-darwin-amd64",
			darwinARM + "  tool-darwin-arm64",
		}, "\n")))
	}))
	t.Cleanup(server.Close)
	originalClient := downloadHTTPClient
	downloadHTTPClient = server.Client()
	t.Cleanup(func() { downloadHTTPClient = originalClient })

	checksums, err := ResolveURLTemplateChecksums(
		"v1.2.3",
		server.URL+"/{{version}}/tool-{{os}}-{{arch}}",
	)
	if err != nil {
		t.Fatalf("ResolveURLTemplateChecksums: %v", err)
	}
	if checksums["linux/amd64"] != linuxAMD ||
		checksums["linux/arm64"] != linuxARM ||
		checksums["darwin/amd64"] != darwinAMD ||
		checksums["darwin/arm64"] != darwinARM {
		t.Fatalf("checksums = %#v", checksums)
	}
}

func TestResolveURLTemplateChecksumsHashesMissingManifestEntries(t *testing.T) {
	linuxAMD := strings.Repeat("1", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.2.3/checksums.txt":
			_, _ = w.Write([]byte(linuxAMD + "  tool-linux-amd64\n"))
		case "/v1.2.3/tool-linux-arm64":
			_, _ = w.Write([]byte("arm-linux"))
		case "/v1.2.3/tool-darwin-amd64":
			_, _ = w.Write([]byte("amd-darwin"))
		case "/v1.2.3/tool-darwin-arm64":
			_, _ = w.Write([]byte("arm-darwin"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	originalClient := downloadHTTPClient
	downloadHTTPClient = server.Client()
	t.Cleanup(func() { downloadHTTPClient = originalClient })

	checksums, err := ResolveURLTemplateChecksums(
		"v1.2.3",
		server.URL+"/{{version}}/tool-{{os}}-{{arch}}",
	)
	if err != nil {
		t.Fatalf("ResolveURLTemplateChecksums: %v", err)
	}
	if checksums["linux/amd64"] != linuxAMD {
		t.Fatalf("manifest digest = %q", checksums["linux/amd64"])
	}
	if checksums["linux/arm64"] != sha256Hex("arm-linux") ||
		checksums["darwin/amd64"] != sha256Hex("amd-darwin") ||
		checksums["darwin/arm64"] != sha256Hex("arm-darwin") {
		t.Fatalf("hashed checksums = %#v", checksums)
	}
}

func TestResolveURLTemplateChecksumsHashesWhenManifestMissing(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v9.9.9/tool-linux-amd64":
			_, _ = w.Write([]byte("linux-amd"))
		case "/v9.9.9/tool-linux-arm64":
			_, _ = w.Write([]byte("linux-arm"))
		case "/v9.9.9/tool-darwin-amd64":
			_, _ = w.Write([]byte("darwin-amd"))
		case "/v9.9.9/tool-darwin-arm64":
			_, _ = w.Write([]byte("darwin-arm"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	originalClient := downloadHTTPClient
	downloadHTTPClient = server.Client()
	t.Cleanup(func() { downloadHTTPClient = originalClient })

	checksums, err := ResolveURLTemplateChecksums(
		"v9.9.9",
		server.URL+"/{{version}}/tool-{{os}}-{{arch}}",
	)
	if err != nil {
		t.Fatalf("ResolveURLTemplateChecksums: %v", err)
	}
	if checksums["linux/amd64"] != sha256Hex("linux-amd") ||
		checksums["linux/arm64"] != sha256Hex("linux-arm") ||
		checksums["darwin/amd64"] != sha256Hex("darwin-amd") ||
		checksums["darwin/arm64"] != sha256Hex("darwin-arm") {
		t.Fatalf("checksums = %#v", checksums)
	}
}

func TestResolveURLTemplateChecksumsFromSha256sums(t *testing.T) {
	linuxAMD := strings.Repeat("1", 64)
	linuxARM := strings.Repeat("2", 64)
	darwinAMD := strings.Repeat("3", 64)
	darwinARM := strings.Repeat("4", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4.1.18/sha256sums.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(strings.Join([]string{
			linuxAMD + "  tool-linux-amd64",
			linuxARM + "  tool-linux-arm64",
			darwinAMD + "  tool-darwin-amd64",
			darwinARM + "  tool-darwin-arm64",
		}, "\n")))
	}))
	t.Cleanup(server.Close)
	originalClient := downloadHTTPClient
	downloadHTTPClient = server.Client()
	t.Cleanup(func() { downloadHTTPClient = originalClient })

	checksums, err := ResolveURLTemplateChecksums(
		"v4.1.18",
		server.URL+"/{{version}}/tool-{{os}}-{{arch}}",
	)
	if err != nil {
		t.Fatalf("ResolveURLTemplateChecksums: %v", err)
	}
	if checksums["linux/amd64"] != linuxAMD || checksums["darwin/arm64"] != darwinARM {
		t.Fatalf("checksums = %#v", checksums)
	}
}

func TestGitHubReleaseDigestHelpers(t *testing.T) {
	apiURL, ok := githubReleaseAPIURL(
		"https://github.com/sqlc-dev/sqlc/releases/download/v1.31.1/sqlc_1.31.1_linux_amd64.tar.gz",
	)
	if !ok || apiURL != "https://api.github.com/repos/sqlc-dev/sqlc/releases/tags/v1.31.1" {
		t.Fatalf("api URL = %q, ok=%t", apiURL, ok)
	}
	if _, ok := githubReleaseAPIURL("https://example.invalid/v1.0.0/tool"); ok {
		t.Fatal("non-GitHub URL should not map to the GitHub API")
	}

	got := parseGitHubReleaseDigests([]byte(`{
		"assets": [
			{"name": "tool-linux-amd64", "digest": "sha256:` + strings.Repeat("a", 64) + `"},
			{"name": "tool-linux-arm64", "digest": "SHA256:` + strings.Repeat("B", 64) + `"},
			{"name": "ignored", "digest": "md5:deadbeef"}
		]
	}`))
	if got["tool-linux-amd64"] != strings.Repeat("a", 64) ||
		got["tool-linux-arm64"] != strings.Repeat("b", 64) ||
		len(got) != 2 {
		t.Fatalf("parsed GitHub digests = %#v", got)
	}
}

func TestResolveURLTemplateChecksumsRejectsInvalidInput(t *testing.T) {
	if _, err := ResolveURLTemplateChecksums("", "https://example.invalid/{{version}}"); err == nil {
		t.Fatal("expected missing version error")
	}
	if _, err := ResolveURLTemplateChecksums("v1.0.0", ""); err == nil {
		t.Fatal("expected missing template error")
	}
	if _, err := ResolveURLTemplateChecksums("v1.0.0", "http://example.invalid/{{version}}"); err == nil ||
		!strings.Contains(err.Error(), "must use HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}
