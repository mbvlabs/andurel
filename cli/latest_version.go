package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const andurelLatestVersionURL = "https://proxy.golang.org/github.com/mbvlabs/andurel/@latest"

var (
	andurelVersionHTTPClient       = &http.Client{Timeout: 3 * time.Second}
	lookupLatestAndurelVersionFunc = lookupLatestAndurelVersion
)

func lookupLatestAndurelVersion(ctx context.Context) (string, error) {
	return fetchLatestAndurelVersion(ctx, andurelVersionHTTPClient, andurelLatestVersionURL)
}

func fetchLatestAndurelVersion(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create latest version request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "andurel-version-check")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("check latest Andurel version: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return "", fmt.Errorf("check latest Andurel version: unexpected HTTP status %s", response.Status)
	}

	var info struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&info); err != nil {
		return "", fmt.Errorf("decode latest Andurel version: %w", err)
	}

	version, ok := canonicalAndurelVersion(info.Version)
	if !ok {
		return "", fmt.Errorf("latest Andurel version %q is not valid semantic versioning", info.Version)
	}
	if semver.Prerelease(version) != "" {
		return "", fmt.Errorf("latest Andurel version %q is not a stable release", info.Version)
	}

	return version, nil
}

func canonicalAndurelVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	version = semver.Canonical(version)
	return version, version != ""
}

func newerAndurelVersion(currentVersion, latestVersion string) bool {
	current, currentOK := canonicalAndurelVersion(currentVersion)
	latest, latestOK := canonicalAndurelVersion(latestVersion)
	return currentOK && latestOK && semver.Compare(latest, current) > 0
}

func andurelInstallCommand(version string) string {
	return "go install github.com/mbvlabs/andurel@" + version
}
