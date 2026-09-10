package cmds

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

const (
	maxChecksumManifestSize int64 = 1 << 20
	maxGitHubReleaseSize    int64 = 4 << 20
	andurelUserAgent              = "andurel"
)

var (
	checksumPlatforms = []struct {
		key    string
		goos   string
		goarch string
	}{
		{key: "linux/amd64", goos: "linux", goarch: "amd64"},
		{key: "linux/arm64", goos: "linux", goarch: "arm64"},
		{key: "darwin/amd64", goos: "darwin", goarch: "amd64"},
		{key: "darwin/arm64", goos: "darwin", goarch: "arm64"},
	}
	bsdChecksumPattern = regexp.MustCompile(
		`(?i)^SHA256 \((.+)\) = ([0-9a-f]{64})$`,
	)
	checksumManifestNames = []string{
		"checksums.txt",
		"sha256sums.txt",
		"SHA256SUMS",
	}
)

// ResolveURLTemplateChecksums returns SHA-256 digests for every supported
// platform of a versioned download URL template. It prefers a checksums.txt,
// sha256sums.txt, or SHA256SUMS file next to the artifacts, then GitHub release
// asset digests, and hashes any remaining artifacts over HTTPS.
func ResolveURLTemplateChecksums(version, urlTemplate string) (map[string]string, error) {
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("version is required")
	}
	if strings.TrimSpace(urlTemplate) == "" {
		return nil, fmt.Errorf("download urlTemplate is required")
	}

	resolvedURLs := make(map[string]string, len(checksumPlatforms))
	for _, platform := range checksumPlatforms {
		resolved := renderDownloadURL(urlTemplate, version, platform.goos, platform.goarch)
		if strings.Contains(resolved, "{{") {
			return nil, fmt.Errorf(
				"download urlTemplate still contains placeholders after rendering",
			)
		}
		parsed, err := url.Parse(resolved)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("download URL for %s must use HTTPS", platform.key)
		}
		resolvedURLs[platform.key] = resolved
	}

	checksums := make(map[string]string, len(checksumPlatforms))
	firstURL := firstMapValue(resolvedURLs)
	applyChecksumManifest(checksums, resolvedURLs, loadChecksumManifest(firstURL))
	if !hasCompleteResolvedChecksums(checksums) {
		applyChecksumManifest(checksums, resolvedURLs, loadGitHubReleaseDigests(firstURL))
	}

	for _, platform := range checksumPlatforms {
		if checksums[platform.key] != "" {
			continue
		}
		digest, err := hashHTTPS(resolvedURLs[platform.key])
		if err != nil {
			return nil, fmt.Errorf("failed to hash %s: %w", platform.key, err)
		}
		checksums[platform.key] = digest
	}

	return checksums, nil
}

func applyChecksumManifest(
	checksums map[string]string,
	resolvedURLs map[string]string,
	manifest map[string]string,
) {
	if len(manifest) == 0 {
		return
	}
	for platform, assetURL := range resolvedURLs {
		if checksums[platform] != "" {
			continue
		}
		digest, ok := manifestDigestForURL(manifest, assetURL)
		if !ok {
			continue
		}
		checksums[platform] = digest
	}
}

func hasCompleteResolvedChecksums(checksums map[string]string) bool {
	if len(checksums) != len(checksumPlatforms) {
		return false
	}
	for _, platform := range checksumPlatforms {
		if checksums[platform.key] == "" {
			return false
		}
	}
	return true
}

func firstMapValue(values map[string]string) string {
	for _, platform := range checksumPlatforms {
		if value := values[platform.key]; value != "" {
			return value
		}
	}
	return ""
}

func loadGitHubReleaseDigests(assetURL string) map[string]string {
	apiURL, ok := githubReleaseAPIURL(assetURL)
	if !ok {
		return nil
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return nil
	}
	request.Header.Set("User-Agent", andurelUserAgent)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := downloadHTTPClient.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	var body bytes.Buffer
	if _, err := copyBounded(
		&body,
		response.Body,
		maxGitHubReleaseSize,
		"GitHub release",
	); err != nil {
		return nil
	}
	return parseGitHubReleaseDigests(body.Bytes())
}

func githubReleaseAPIURL(assetURL string) (string, bool) {
	parsed, err := url.Parse(assetURL)
	if err != nil || parsed.Host != "github.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 6 || parts[2] != "releases" || parts[3] != "download" {
		return "", false
	}
	return "https://api.github.com/repos/" + parts[0] + "/" + parts[1] +
		"/releases/tags/" + url.PathEscape(parts[4]), true
}

func parseGitHubReleaseDigests(body []byte) map[string]string {
	var release struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil
	}
	checksums := make(map[string]string)
	for _, asset := range release.Assets {
		name := checksumFileName(asset.Name)
		digest := strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:")
		if name == "" || !validSHA256(digest) {
			continue
		}
		checksums[name] = digest
	}
	if len(checksums) == 0 {
		return nil
	}
	return checksums
}

func loadChecksumManifest(assetURL string) map[string]string {
	for _, manifestURL := range checksumManifestURLs(assetURL) {
		body, err := readHTTPS(manifestURL, maxChecksumManifestSize, "checksum manifest")
		if err != nil {
			continue
		}
		parsed := parseChecksumManifest(string(body))
		if len(parsed) > 0 {
			return parsed
		}
	}
	return nil
}

func checksumManifestURLs(assetURL string) []string {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return nil
	}
	dir := path.Dir(parsed.Path)
	if dir == "." || dir == "/" {
		return nil
	}
	urls := make([]string, 0, len(checksumManifestNames))
	for _, name := range checksumManifestNames {
		item := *parsed
		item.Path = path.Join(dir, name)
		item.RawQuery = ""
		item.Fragment = ""
		urls = append(urls, item.String())
	}
	return urls
}

func parseChecksumManifest(body string) map[string]string {
	checksums := make(map[string]string)
	for line := range strings.SplitSeq(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if matches := bsdChecksumPattern.FindStringSubmatch(line); len(matches) == 3 {
			name := checksumFileName(matches[1])
			digest := strings.ToLower(matches[2])
			if name == "" || !validSHA256(digest) {
				continue
			}
			checksums[name] = digest
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !validSHA256(fields[0]) {
			continue
		}
		name := checksumFileName(fields[1])
		if name == "" {
			continue
		}
		checksums[name] = strings.ToLower(fields[0])
	}
	return checksums
}

func checksumFileName(value string) string {
	name := strings.TrimPrefix(value, "*")
	name = strings.ReplaceAll(name, "\\", "/")
	return path.Base(name)
}

func manifestDigestForURL(manifest map[string]string, assetURL string) (string, bool) {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return "", false
	}
	digest, ok := manifest[path.Base(parsed.Path)]
	return digest, ok && validSHA256(digest)
}

func hashHTTPS(sourceURL string) (string, error) {
	body, err := openHTTPS(sourceURL, "archive")
	if err != nil {
		return "", err
	}
	defer body.Close()
	hash := sha256.New()
	if _, err := copyBounded(hash, body, maxArchiveSize, "archive"); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readHTTPS(sourceURL string, limit int64, description string) ([]byte, error) {
	body, err := openHTTPS(sourceURL, description)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var buf bytes.Buffer
	if _, err := copyBounded(&buf, body, limit, description); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func openHTTPS(sourceURL, description string) (io.ReadCloser, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("%s URL must use HTTPS", description)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	response, err := downloadHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d for %s", response.StatusCode, sourceURL)
	}
	return response.Body, nil
}
