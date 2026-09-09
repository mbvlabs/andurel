package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var (
	andurelVersionHTTPClient       = &http.Client{Timeout: 3 * time.Second}
	lookupLatestAndurelVersionFunc = lookupLatestAndurelVersion
	lookupLatestModuleVersionFunc  = lookupLatestModuleVersion
)

func lookupLatestAndurelVersion(ctx context.Context) (string, error) {
	return lookupLatestModuleVersion(ctx, "github.com/mbvlabs/andurel")
}

func lookupLatestModuleVersion(ctx context.Context, modulePath string) (string, error) {
	escaped, err := module.EscapePath(modulePath)
	if err != nil {
		return "", fmt.Errorf("encode module path %q: %w", modulePath, err)
	}
	return fetchLatestAndurelVersion(
		ctx,
		andurelVersionHTTPClient,
		"https://proxy.golang.org/"+escaped+"/@latest",
	)
}

func fetchLatestAndurelVersion(
	ctx context.Context,
	client *http.Client,
	endpoint string,
) (version string, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create latest version request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "andurel-version-check")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("check latest module version: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			version = ""
			err = errors.Join(
				err,
				fmt.Errorf("close latest module version response: %w", closeErr),
			)
		}
	}()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return "", fmt.Errorf(
			"check latest module version: unexpected HTTP status %s",
			response.Status,
		)
	}

	var info struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&info); err != nil {
		return "", fmt.Errorf("decode latest module version: %w", err)
	}

	version, ok := canonicalAndurelVersion(info.Version)
	if !ok {
		return "", fmt.Errorf(
			"latest module version %q is not valid semantic versioning",
			info.Version,
		)
	}
	if semver.Prerelease(version) != "" {
		return "", fmt.Errorf("latest module version %q is not a stable release", info.Version)
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
