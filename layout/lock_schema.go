package layout

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const currentLockSchemaVersion = 1

var requiredChecksumPlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func isSupportedChecksumPlatform(platform string) bool {
	return slices.Contains(requiredChecksumPlatforms, platform)
}

func decodeAndValidateLock(data []byte) (*AndurelLock, error) {
	var header struct {
		SchemaVersion *int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}

	if header.SchemaVersion == nil {
		return nil, fmt.Errorf("andurel.lock schemaVersion is required")
	}
	if *header.SchemaVersion > currentLockSchemaVersion {
		return nil, fmt.Errorf(
			"andurel.lock schemaVersion %d is newer than this CLI supports; upgrade Andurel to read it",
			*header.SchemaVersion,
		)
	}
	if *header.SchemaVersion != currentLockSchemaVersion {
		return nil, fmt.Errorf("unsupported andurel.lock schemaVersion %d", *header.SchemaVersion)
	}

	var lock AndurelLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	if err := validateSchema1Lock(&lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

func validateSchema1Lock(lock *AndurelLock) error {
	if lock == nil {
		return fmt.Errorf("lock is required")
	}
	if lock.SchemaVersion != currentLockSchemaVersion {
		return fmt.Errorf("schemaVersion must be %d", currentLockSchemaVersion)
	}
	if strings.TrimSpace(lock.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if lock.Tools == nil {
		return fmt.Errorf("tools is required")
	}

	for name, tool := range lock.Tools {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("tool name is required")
		}
		if err := validateSchema1Tool(name, tool); err != nil {
			return err
		}
	}

	for name, extension := range lock.Extensions {
		if strings.TrimSpace(name) == "" || extension == nil || strings.TrimSpace(extension.AppliedAt) == "" {
			return fmt.Errorf("extension %q must have appliedAt", name)
		}
	}
	if lock.ScaffoldConfig != nil {
		if strings.TrimSpace(lock.ScaffoldConfig.ProjectName) == "" {
			return fmt.Errorf("scaffoldConfig.projectName is required")
		}
		if strings.TrimSpace(lock.ScaffoldConfig.Database) == "" {
			return fmt.Errorf("scaffoldConfig.database is required")
		}
	}
	if lock.DatabaseConfig != nil && strings.TrimSpace(lock.DatabaseConfig.NullType) == "" {
		return fmt.Errorf("databaseConfig.nullType is required")
	}
	return nil
}

func validateSchema1Tool(name string, tool *Tool) error {
	prefix := fmt.Sprintf("tool %q", name)
	if tool == nil {
		return fmt.Errorf("%s is required", prefix)
	}
	if strings.TrimSpace(tool.Version) == "" {
		return fmt.Errorf("%s.version is required", prefix)
	}
	if strings.TrimSpace(tool.Path) == "" && tool.Download == nil {
		return fmt.Errorf("%s requires path or download metadata", prefix)
	}
	if tool.VersionCheck == nil {
		return fmt.Errorf("%s.versionCheck is required", prefix)
	}
	if len(tool.VersionCheck.Args) == 0 {
		return fmt.Errorf("%s.versionCheck.args must not be empty", prefix)
	}
	for index, arg := range tool.VersionCheck.Args {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("%s.versionCheck.args[%d] must not be empty", prefix, index)
		}
	}
	if tool.VersionCheck.Regexp != "" {
		if _, err := regexp.Compile(tool.VersionCheck.Regexp); err != nil {
			return fmt.Errorf("%s.versionCheck.regexp is invalid: %w", prefix, err)
		}
	}
	if tool.Download != nil {
		if err := validateSchema1Download(prefix, tool.Download); err != nil {
			return err
		}
	}
	return nil
}

func validateSchema1Download(prefix string, download *ToolDownload) error {
	parsed, err := url.Parse(download.URLTemplate)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s.download.urlTemplate must be an HTTPS URL", prefix)
	}
	if strings.TrimSpace(download.BinaryName) == "" {
		return fmt.Errorf("%s.download.binaryName is required", prefix)
	}
	switch download.Archive {
	case "binary", "tar.bz2", "tar.gz":
	default:
		return fmt.Errorf("%s.download.archive must be binary, tar.bz2, or tar.gz", prefix)
	}
	for _, platform := range requiredChecksumPlatforms {
		digest, ok := download.SHA256[platform]
		if !ok {
			return fmt.Errorf("%s.download.sha256 is missing %s", prefix, platform)
		}
		if !sha256Pattern.MatchString(digest) {
			return fmt.Errorf("%s.download.sha256[%q] must be a 64-character SHA-256 digest", prefix, platform)
		}
	}
	if len(download.SHA256) != len(requiredChecksumPlatforms) {
		return fmt.Errorf("%s.download.sha256 must contain exactly four supported platforms", prefix)
	}
	return nil
}
