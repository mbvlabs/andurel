package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffoldTestProject creates a real project in a temp directory using
// layout.Scaffold and returns the project directory path. It sets
// ANDUREL_TEST_MODE=true for deterministic migration timestamps.
func scaffoldTestProject(t *testing.T, extensions []string, diMode, cssFramework string) string {
	t.Helper()

	t.Setenv("ANDUREL_TEST_MODE", "true")

	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "testapp")

	if err := Scaffold(projectDir, "testapp", "postgresql", cssFramework, "test", extensions, diMode, "", ""); err != nil {
		t.Fatalf("failed to scaffold project: %v", err)
	}

	return projectDir
}

func fileExists(t *testing.T, dir, path string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

func fileContains(t *testing.T, dir, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("expected %s to contain %q", path, expected)
	}
}

func readFileContent(t *testing.T, dir, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(content)
}

// --- parseGoMod tests ---

func TestParseGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module github.com/example/myapp\n\ngo 1.26.0\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	module, goVer, err := parseGoMod(tmpDir)
	if err != nil {
		t.Fatalf("parseGoMod failed: %v", err)
	}

	if module != "github.com/example/myapp" {
		t.Fatalf("expected module github.com/example/myapp, got %s", module)
	}
	if goVer != "1.26.0" {
		t.Fatalf("expected go version 1.26.0, got %s", goVer)
	}
}

func TestParseGoMod_MissingModule(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("go 1.26.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	_, _, err := parseGoMod(tmpDir)
	if err == nil {
		t.Fatalf("expected error for missing module declaration")
	}
}

func TestParseGoMod_FallbackGoVersion(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module github.com/example/myapp\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	_, goVer, err := parseGoMod(tmpDir)
	if err != nil {
		t.Fatalf("parseGoMod failed: %v", err)
	}
	if goVer != goVersion {
		t.Fatalf("expected fallback go version %s, got %s", goVersion, goVer)
	}
}

// --- readSecrets tests ---

func TestReadSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	envContent := "SESSION_KEY=abc123\nSESSION_ENCRYPTION_KEY=def456\nTOKEN_SIGNING_KEY=ghi789\nPEPPER=jkl012\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.example"), []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to write .env.example: %v", err)
	}

	secrets := readSecrets(tmpDir)

	if secrets["SESSION_KEY"] != "abc123" {
		t.Fatalf("expected SESSION_KEY=abc123, got %s", secrets["SESSION_KEY"])
	}
	if secrets["SESSION_ENCRYPTION_KEY"] != "def456" {
		t.Fatalf("expected SESSION_ENCRYPTION_KEY=def456, got %s", secrets["SESSION_ENCRYPTION_KEY"])
	}
	if secrets["TOKEN_SIGNING_KEY"] != "ghi789" {
		t.Fatalf("expected TOKEN_SIGNING_KEY=ghi789, got %s", secrets["TOKEN_SIGNING_KEY"])
	}
	if secrets["PEPPER"] != "jkl012" {
		t.Fatalf("expected PEPPER=jkl012, got %s", secrets["PEPPER"])
	}
}

func TestReadSecrets_FallsBackToEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envContent := "SESSION_KEY=fromenv\nPEPPER=pepperfromenv\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	secrets := readSecrets(tmpDir)

	if secrets["SESSION_KEY"] != "fromenv" {
		t.Fatalf("expected SESSION_KEY=fromenv, got %s", secrets["SESSION_KEY"])
	}
	if secrets["PEPPER"] != "pepperfromenv" {
		t.Fatalf("expected PEPPER=pepperfromenv, got %s", secrets["PEPPER"])
	}
}

func TestReadSecrets_NoEnvFiles(t *testing.T) {
	tmpDir := t.TempDir()
	secrets := readSecrets(tmpDir)

	if len(secrets) != 0 {
		t.Fatalf("expected empty secrets map, got %v", secrets)
	}
}

// --- LoadProjectContext tests ---

func TestLoadProjectContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, []string{"docker"}, "uberfx", "tailwind")

	td, lock, err := LoadProjectContext(projectDir)
	if err != nil {
		t.Fatalf("LoadProjectContext failed: %v", err)
	}

	if td.ProjectName != "testapp" {
		t.Fatalf("expected ProjectName testapp, got %s", td.ProjectName)
	}
	if td.Database != "postgresql" {
		t.Fatalf("expected Database postgresql, got %s", td.Database)
	}
	if td.CSSFramework != "tailwind" {
		t.Fatalf("expected CSSFramework tailwind, got %s", td.CSSFramework)
	}
	if td.DIMode != "uberfx" {
		t.Fatalf("expected DIMode uberfx, got %s", td.DIMode)
	}
	if td.ModuleName == "" {
		t.Fatalf("expected non-empty ModuleName")
	}
	if td.GoVersion == "" {
		t.Fatalf("expected non-empty GoVersion")
	}
	if td.SessionKey == "" {
		t.Fatalf("expected non-empty SessionKey (read from .env.example)")
	}
	if td.Pepper == "" {
		t.Fatalf("expected non-empty Pepper (read from .env.example)")
	}
	if len(td.Extensions) != 1 || td.Extensions[0] != "docker" {
		t.Fatalf("expected Extensions [docker], got %v", td.Extensions)
	}
	if lock == nil {
		t.Fatalf("expected non-nil lock")
	}
	if _, exists := lock.Extensions["docker"]; !exists {
		t.Fatalf("expected docker in lock extensions")
	}
}

func TestLoadProjectContext_RebuildsBlueprintWithExistingExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, []string{"aws-ses"}, "uberfx", "tailwind")

	td, _, err := LoadProjectContext(projectDir)
	if err != nil {
		t.Fatalf("LoadProjectContext failed: %v", err)
	}

	bp := td.Blueprint()
	if bp == nil {
		t.Fatalf("expected non-nil blueprint")
	}

	foundAwsSes := false
	for _, field := range bp.Config.Fields {
		if field.Name == "AwsSes" {
			foundAwsSes = true
			break
		}
	}
	if !foundAwsSes {
		t.Fatalf("expected AwsSes in blueprint config fields after re-applying aws-ses")
	}

	foundAwsRegion := false
	for _, envVar := range bp.Config.EnvVars {
		if envVar.Key == "AWS_REGION" {
			foundAwsRegion = true
			break
		}
	}
	if !foundAwsRegion {
		t.Fatalf("expected AWS_REGION in blueprint env vars after re-applying aws-ses")
	}
}

func TestLoadProjectContext_MissingScaffoldConfig(t *testing.T) {
	tmpDir := t.TempDir()
	lock := NewAndurelLock("test")
	if err := lock.WriteLockFile(tmpDir); err != nil {
		t.Fatalf("failed to write lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	_, _, err := LoadProjectContext(tmpDir)
	if err == nil {
		t.Fatalf("expected error for missing ScaffoldConfig")
	}
	if !strings.Contains(err.Error(), "scaffoldConfig") {
		t.Fatalf("expected scaffoldConfig error, got: %v", err)
	}
}

// --- ApplyExtension tests ---

func TestApplyExtension_Docker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	applied, err := ApplyExtension(projectDir, "docker")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "docker" {
		t.Fatalf("expected [docker], got %v", applied)
	}

	fileExists(t, projectDir, "Dockerfile")
	fileExists(t, projectDir, ".dockerignore")

	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("failed to read lock: %v", err)
	}
	if _, exists := lock.Extensions["docker"]; !exists {
		t.Fatalf("expected docker in lock extensions")
	}
}

func TestApplyExtension_AwsSes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	applied, err := ApplyExtension(projectDir, "aws-ses")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "aws-ses" {
		t.Fatalf("expected [aws-ses], got %v", applied)
	}

	fileExists(t, projectDir, "clients/email/aws_ses.go")
	fileExists(t, projectDir, "config/aws_ses.go")

	// Verify blueprint was updated: config.go should contain AwsSes field
	fileContains(t, projectDir, "config/config.go", "AwsSes")

	// Verify .env.example was updated with AWS env vars
	fileContains(t, projectDir, ".env.example", "AWS_REGION")

	// Verify lock file
	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("failed to read lock: %v", err)
	}
	if _, exists := lock.Extensions["aws-ses"]; !exists {
		t.Fatalf("expected aws-ses in lock extensions")
	}
}

func TestApplyExtension_CssComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	applied, err := ApplyExtension(projectDir, "css-components")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "css-components" {
		t.Fatalf("expected [css-components], got %v", applied)
	}

	// Tailwind CSS framework → tailwind component files
	fileExists(t, projectDir, "css/themes.css")
	fileExists(t, projectDir, "css/utilities.css")
	fileExists(t, projectDir, "css/components.css")
	fileExists(t, projectDir, "views/components/toast.templ")

	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("failed to read lock: %v", err)
	}
	if _, exists := lock.Extensions["css-components"]; !exists {
		t.Fatalf("expected css-components in lock extensions")
	}
}

func TestApplyExtension_AlreadyApplied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, []string{"docker"}, "uberfx", "tailwind")

	_, err := ApplyExtension(projectDir, "docker")
	if err == nil {
		t.Fatalf("expected error for already applied extension")
	}
	if !strings.Contains(err.Error(), "already applied") {
		t.Fatalf("expected 'already applied' error, got: %v", err)
	}
}

func TestApplyExtension_UnknownExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	_, err := ApplyExtension(projectDir, "nonexistent")
	if err == nil {
		t.Fatalf("expected error for unknown extension")
	}
	if !strings.Contains(err.Error(), "unknown extension") {
		t.Fatalf("expected 'unknown extension' error, got: %v", err)
	}
}

func TestApplyExtension_PreservesExistingExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	// Scaffold with aws-ses already applied
	projectDir := scaffoldTestProject(t, []string{"aws-ses"}, "uberfx", "tailwind")

	// Add docker extension
	applied, err := ApplyExtension(projectDir, "docker")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "docker" {
		t.Fatalf("expected [docker], got %v", applied)
	}

	// Verify aws-ses files still exist
	fileExists(t, projectDir, "clients/email/aws_ses.go")
	fileExists(t, projectDir, "config/aws_ses.go")

	// Verify docker files were created
	fileExists(t, projectDir, "Dockerfile")
	fileExists(t, projectDir, ".dockerignore")

	// Verify config.go still has AwsSes (blueprint preserved from existing extension)
	fileContains(t, projectDir, "config/config.go", "AwsSes")

	// Verify .env.example still has AWS_REGION
	fileContains(t, projectDir, ".env.example", "AWS_REGION")

	// Verify lock file has both extensions
	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("failed to read lock: %v", err)
	}
	if _, exists := lock.Extensions["aws-ses"]; !exists {
		t.Fatalf("expected aws-ses to remain in lock extensions")
	}
	if _, exists := lock.Extensions["docker"]; !exists {
		t.Fatalf("expected docker in lock extensions")
	}
}

func TestApplyExtension_PreservesSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	// Read original secrets from .env.example
	originalEnv := readFileContent(t, projectDir, ".env.example")
	originalSessionKey := extractEnvValue(originalEnv, "SESSION_KEY")
	originalPepper := extractEnvValue(originalEnv, "PEPPER")

	if originalSessionKey == "" {
		t.Fatalf("expected non-empty SESSION_KEY in scaffolded .env.example")
	}
	if originalPepper == "" {
		t.Fatalf("expected non-empty PEPPER in scaffolded .env.example")
	}

	// Apply an extension (which re-renders .env.example)
	_, err := ApplyExtension(projectDir, "aws-ses")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	// Read re-rendered .env.example
	newEnv := readFileContent(t, projectDir, ".env.example")
	newSessionKey := extractEnvValue(newEnv, "SESSION_KEY")
	newPepper := extractEnvValue(newEnv, "PEPPER")

	if newSessionKey != originalSessionKey {
		t.Fatalf("SESSION_KEY changed: was %q, now %q", originalSessionKey, newSessionKey)
	}
	if newPepper != originalPepper {
		t.Fatalf("PEPPER changed: was %q, now %q", originalPepper, newPepper)
	}
}

func TestApplyExtension_UberFxMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold test in short mode")
	}
	projectDir := scaffoldTestProject(t, nil, "uberfx", "tailwind")

	applied, err := ApplyExtension(projectDir, "aws-ses")
	if err != nil {
		t.Fatalf("ApplyExtension failed: %v", err)
	}

	if len(applied) != 1 || applied[0] != "aws-ses" {
		t.Fatalf("expected [aws-ses], got %v", applied)
	}

	fileExists(t, projectDir, "clients/email/aws_ses.go")
	fileExists(t, projectDir, "config/aws_ses.go")
	fileContains(t, projectDir, "config/config.go", "AwsSes")
	fileContains(t, projectDir, ".env.example", "AWS_REGION")

	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("failed to read lock: %v", err)
	}
	if _, exists := lock.Extensions["aws-ses"]; !exists {
		t.Fatalf("expected aws-ses in lock extensions")
	}
}

// extractEnvValue extracts the value for a given key from KEY=VALUE format text.
func extractEnvValue(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}
