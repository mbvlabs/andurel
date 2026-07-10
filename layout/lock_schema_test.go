package layout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeSchema1AndValidateCompleteLock(t *testing.T) {
	data := mustMarshalLock(t, validSchema1Lock())
	lock, err := decodeAndValidateLock(data)
	if err != nil {
		t.Fatalf("decodeAndValidateLock: %v", err)
	}
	if lock.SchemaVersion != 1 || lock.Version != "v9.8.7" {
		t.Fatalf("unexpected versions: schema=%d framework=%q", lock.SchemaVersion, lock.Version)
	}
	if lock.Tools["templ"].Download.SHA256["linux/amd64"] == "" {
		t.Fatalf("missing decoded checksum")
	}
}

func TestDecodeLockSchemaSelectionIsIndependentOfFrameworkVersion(t *testing.T) {
	schemaOne := validSchema1Lock()
	schemaOne.Version = "v99.0.0"
	decoded, err := decodeAndValidateLock(mustMarshalLock(t, schemaOne))
	if err != nil {
		t.Fatalf("schema selection must be independent of framework version: %v", err)
	}
	if decoded.Version != "v99.0.0" || decoded.SchemaVersion != 1 {
		t.Fatalf("independent versions were not preserved: %#v", decoded)
	}
}

func TestDecodeLockRejectsMissingMalformedAndFutureSchemas(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "missing schema", data: `{"version":"v1.0.0","tools":{}}`, want: "schemaVersion is required"},
		{name: "schema wrong type", data: `{"schemaVersion":"1","version":"v1.0.0","tools":{}}`, want: "cannot unmarshal"},
		{name: "schema zero", data: `{"schemaVersion":0,"version":"v1.0.0","tools":{}}`, want: "unsupported"},
		{name: "future schema", data: `{"schemaVersion":2,"version":"v1.0.0","tools":{}}`, want: "upgrade Andurel"},
		{name: "malformed JSON", data: `{`, want: "unexpected end"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeAndValidateLock([]byte(test.data))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateSchema1RequiredFields(t *testing.T) {
	if err := validateSchema1Lock(nil); err == nil || !strings.Contains(err.Error(), "lock is required") {
		t.Fatalf("nil lock error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*AndurelLock)
		want   string
	}{
		{name: "version", mutate: func(lock *AndurelLock) { lock.Version = "" }, want: "version is required"},
		{name: "tools", mutate: func(lock *AndurelLock) { lock.Tools = nil }, want: "tools is required"},
		{name: "tool name", mutate: func(lock *AndurelLock) { lock.Tools[""] = lock.Tools["templ"] }, want: "tool name is required"},
		{name: "tool", mutate: func(lock *AndurelLock) { lock.Tools["templ"] = nil }, want: "is required"},
		{name: "tool version", mutate: func(lock *AndurelLock) { lock.Tools["templ"].Version = "" }, want: ".version is required"},
		{name: "tool location", mutate: func(lock *AndurelLock) { lock.Tools["templ"].Download = nil }, want: "requires path or download"},
		{name: "version check", mutate: func(lock *AndurelLock) { lock.Tools["templ"].VersionCheck = nil }, want: ".versionCheck is required"},
		{name: "version args", mutate: func(lock *AndurelLock) { lock.Tools["templ"].VersionCheck.Args = nil }, want: ".args must not be empty"},
		{name: "empty version arg", mutate: func(lock *AndurelLock) { lock.Tools["templ"].VersionCheck.Args[0] = "" }, want: "args[0]"},
		{name: "download url", mutate: func(lock *AndurelLock) { lock.Tools["templ"].Download.URLTemplate = "http://example.com/tool" }, want: "HTTPS"},
		{name: "archive", mutate: func(lock *AndurelLock) { lock.Tools["templ"].Download.Archive = "zip" }, want: "archive must"},
		{name: "binary name", mutate: func(lock *AndurelLock) { lock.Tools["templ"].Download.BinaryName = "" }, want: "binaryName is required"},
		{name: "scaffold project", mutate: func(lock *AndurelLock) { lock.ScaffoldConfig.ProjectName = "" }, want: "projectName"},
		{name: "scaffold database", mutate: func(lock *AndurelLock) { lock.ScaffoldConfig.Database = "" }, want: "scaffoldConfig.database"},
		{name: "database null type", mutate: func(lock *AndurelLock) { lock.DatabaseConfig.NullType = "" }, want: "databaseConfig.nullType"},
		{name: "extension name", mutate: func(lock *AndurelLock) { lock.Extensions[""] = lock.Extensions["example"] }, want: "must have appliedAt"},
		{name: "extension value", mutate: func(lock *AndurelLock) { lock.Extensions["example"] = nil }, want: "must have appliedAt"},
		{name: "extension applied at", mutate: func(lock *AndurelLock) { lock.Extensions["example"].AppliedAt = "" }, want: "appliedAt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := validSchema1Lock()
			test.mutate(lock)
			if err := validateSchema1Lock(lock); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateSchema1ChecksumsAndVersionExpression(t *testing.T) {
	for _, platform := range requiredChecksumPlatforms {
		t.Run("missing "+platform, func(t *testing.T) {
			lock := validSchema1Lock()
			delete(lock.Tools["templ"].Download.SHA256, platform)
			if err := validateSchema1Lock(lock); err == nil || !strings.Contains(err.Error(), platform) {
				t.Fatalf("missing platform error = %v", err)
			}
		})
	}

	lock := validSchema1Lock()
	lock.Tools["templ"].Download.SHA256["linux/amd64"] = "xyz"
	if err := validateSchema1Lock(lock); err == nil || !strings.Contains(err.Error(), "64-character") {
		t.Fatalf("malformed digest error = %v", err)
	}

	lock = validSchema1Lock()
	lock.Tools["templ"].Download.SHA256["freebsd/amd64"] = strings.Repeat("a", 64)
	if err := validateSchema1Lock(lock); err == nil || !strings.Contains(err.Error(), "exactly four") {
		t.Fatalf("extra platform error = %v", err)
	}

	lock = validSchema1Lock()
	lock.Tools["templ"].VersionCheck.Regexp = "["
	if err := validateSchema1Lock(lock); err == nil || !strings.Contains(err.Error(), "regexp is invalid") {
		t.Fatalf("regexp error = %v", err)
	}

	lock = validSchema1Lock()
	lock.Tools["templ"].VersionCheck.Regexp = ""
	if err := validateSchema1Lock(lock); err != nil {
		t.Fatalf("omitted regexp should use generic expression: %v", err)
	}
}

func TestReadLockFileValidatesAfterSchemaFirstDecode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "andurel.lock"), mustMarshalLock(t, validSchema1Lock()), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLockFile(root); err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}
}

func TestDefaultToolCatalogCoversEverySupportedPlatform(t *testing.T) {
	for name, version := range defaultToolVersions {
		t.Run(name, func(t *testing.T) {
			download, ok := getDefaultToolDownloadForVersion(name, version)
			if !ok {
				t.Fatalf("missing catalog release for %s %s", name, version)
			}
			for _, platform := range requiredChecksumPlatforms {
				digest, ok := download.SHA256[platform]
				if !ok || !sha256Pattern.MatchString(digest) {
					t.Fatalf("missing valid checksum for %s %s on %s", name, version, platform)
				}
			}
		})
	}
}

func validSchema1Lock() *AndurelLock {
	return &AndurelLock{
		SchemaVersion: 1,
		Version:       "v9.8.7",
		Extensions: map[string]*Extension{
			"example": {AppliedAt: "2026-01-01T00:00:00Z"},
		},
		Tools: map[string]*Tool{
			"templ": {
				Version: "v0.3.1020",
				Download: &ToolDownload{
					URLTemplate: "https://example.com/{{version}}/templ.tar.gz",
					Archive:     "tar.gz",
					BinaryName:  "templ",
					SHA256: map[string]string{
						"linux/amd64":  strings.Repeat("1", 64),
						"linux/arm64":  strings.Repeat("2", 64),
						"darwin/amd64": strings.Repeat("3", 64),
						"darwin/arm64": strings.Repeat("4", 64),
					},
				},
				VersionCheck: &VersionCheck{Args: []string{"--version"}, Regexp: `v?([0-9]+\.[0-9]+\.[0-9]+)`},
			},
		},
		ScaffoldConfig: &ScaffoldConfig{ProjectName: "app", Database: "postgresql"},
		DatabaseConfig: &DatabaseConfig{NullType: "sql.Null"},
	}
}

func mustMarshalLock(t *testing.T, lock *AndurelLock) []byte {
	t.Helper()
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
