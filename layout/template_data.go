package layout

import (
	"strings"
	"unicode"

	"github.com/mbvlabs/andurel/v2/layout/blueprint"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

const andurelInstallModule = "github.com/mbvlabs/andurel/v2"

// AndurelInstallCommand returns the command that installs a version of the
// Andurel CLI. It is the single source of truth for the install command in the
// scaffold README, the scaffold Welcome pages, and andurel doctor.
func AndurelInstallCommand(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" {
		version = "latest"
	}
	return "go install " + andurelInstallModule + "@" + version
}

// AndurelInstallCommand returns the install command for this scaffold's
// framework version.
func (td *TemplateData) AndurelInstallCommand() string {
	if td == nil {
		return AndurelInstallCommand("")
	}
	return AndurelInstallCommand(td.FrameworkVersion)
}

// TemplateData carries the values available to base templates.
// It wraps a Blueprint for structured data alongside project-level metadata.
type TemplateData struct {
	AppName              string
	ProjectName          string
	ModuleName           string
	Database             string // Always "postgresql"
	GoVersion            string
	SessionKey           string
	SessionEncryptionKey string
	TokenSigningKey      string
	Pepper               string
	RunToolVersion       string // Version of the run built tool
	FrameworkVersion     string // Version of the framework that generated managed files
	Inertia              string // "vue", "react", "svelte", etc. Empty means templ-only
	JavaScriptSSRRuntime string // Seeded into config DefaultInertiaSSRRuntime
	// JavaScriptPackageManager is the selected frontend package manager (pnpm,
	// npm, bun). Empty means the scaffold has no frontend.
	JavaScriptPackageManager string

	// Blueprint holds the structured scaffold configuration
	blueprint *blueprint.Blueprint
}

// DatabaseDialect returns the configured database, which is always PostgreSQL.
func (td *TemplateData) DatabaseDialect() string {
	return "postgresql"
}

// JavaScriptLockfileName is the lockfile the selected package manager writes.
// It is empty when the scaffold has no frontend, so the README never claims a
// lockfile the project does not have.
func (td *TemplateData) JavaScriptLockfileName() string {
	if td == nil {
		return ""
	}

	switch td.JavaScriptPackageManager {
	case "pnpm":
		return "pnpm-lock.yaml"
	case "npm":
		return "package-lock.json"
	case "bun":
		return "bun.lock"
	default:
		return ""
	}
}

// JavaScriptFrozenInstallCommand is the reproducible install command CI uses:
// it fails when the lockfile and package.json disagree instead of rewriting
// the lockfile.
func (td *TemplateData) JavaScriptFrozenInstallCommand() string {
	if td == nil {
		return ""
	}

	switch td.JavaScriptPackageManager {
	case "pnpm":
		return "pnpm install --frozen-lockfile"
	case "npm":
		return "npm ci"
	case "bun":
		return "bun install --frozen-lockfile"
	default:
		return ""
	}
}

// Identifier returns the project name as a lowercase identifier safe for
// database names, connection application names and cache namespaces: anything
// other than a letter or digit becomes an underscore.
func (td *TemplateData) Identifier() string {
	if td == nil {
		return ""
	}

	var b strings.Builder
	for _, r := range strings.ToLower(ToSnakeCase(td.ProjectName)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}

	identifier := strings.Trim(b.String(), "_")
	if identifier == "" {
		return "app"
	}
	if identifier[0] >= '0' && identifier[0] <= '9' {
		return "app_" + identifier
	}

	return identifier
}

// SiteName returns the default display name for the project: the project name
// title-cased, with separators replaced by spaces (andurel-site -> Andurel Site).
func (td *TemplateData) SiteName() string {
	if td == nil {
		return ""
	}

	var b strings.Builder
	capitalize := true
	for _, r := range strings.TrimSpace(td.ProjectName) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			capitalize = true
			b.WriteRune(' ')
			continue
		}
		if capitalize {
			b.WriteRune(unicode.ToUpper(r))
			capitalize = false
			continue
		}
		b.WriteRune(r)
	}

	return strings.Join(strings.Fields(b.String()), " ")
}

// InertiaEntryPoint returns the default Inertia client entry point for the
// configured adapter.
func (td *TemplateData) InertiaEntryPoint() string {
	if td != nil && td.Inertia == "react" {
		return "resources/js/app.tsx"
	}
	return "resources/js/app.ts"
}

// Slug returns a lowercase, hyphenated project name matching the scaffold's
// telemetry service-name default.
func (td *TemplateData) Slug() string {
	if td == nil {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(td.ProjectName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

// GetModuleName returns the module name for the project.
func (td *TemplateData) GetModuleName() string {
	if td == nil {
		return ""
	}

	return td.ModuleName
}

// GetInertia returns the inertia adapter for the project, if any.
func (td *TemplateData) GetInertia() string {
	if td == nil {
		return ""
	}

	return td.Inertia
}

// EmailPackageVersion returns the standalone email module version verified with
// this framework release.
func (td *TemplateData) EmailPackageVersion() string {
	return versions.Email
}

// AnduinPackageVersion returns the standalone anduin HTTP module version
// verified with this framework release.
func (td *TemplateData) AnduinPackageVersion() string {
	return versions.Anduin
}

// AuthPackageVersion returns the standalone auth module version verified with
// this framework release.
func (td *TemplateData) AuthPackageVersion() string {
	return versions.Auth
}

// EnvPackageVersion returns the standalone env module version verified with
// this framework release.
func (td *TemplateData) EnvPackageVersion() string {
	return versions.Env
}

// CachePackageVersion returns the standalone cache module version verified with
// this framework release.
func (td *TemplateData) CachePackageVersion() string {
	return versions.Cache
}

// InertiaPackageVersion returns the standalone Inertia module version verified
// with this framework release.
func (td *TemplateData) InertiaPackageVersion() string {
	return versions.Inertia
}

// KiksPackageVersion returns the standalone kiks module version verified with
// this framework release.
func (td *TemplateData) KiksPackageVersion() string {
	return versions.Kiks
}

// LifecyclePackageVersion returns the standalone lifecycle module version
// verified with this framework release.
func (td *TemplateData) LifecyclePackageVersion() string {
	return versions.Lifecycle
}

// RoutingPackageVersion returns the standalone routing module version verified
// with this framework release.
func (td *TemplateData) RoutingPackageVersion() string {
	return versions.Routing
}

// StoragePackageVersion returns the standalone storage module version verified
// with this framework release.
func (td *TemplateData) StoragePackageVersion() string {
	return versions.Storage
}

// TelemetryPackageVersion returns the standalone telemetry module version
// verified with this framework release.
func (td *TemplateData) TelemetryPackageVersion() string {
	return versions.Telemetry
}

// ValidationPackageVersion returns the standalone validation module version
// verified with this framework release.
func (td *TemplateData) ValidationPackageVersion() string {
	return versions.Validation
}

// NarsilcPackageVersion returns the narsilc module version required by
// generated query code.
func (td *TemplateData) NarsilcPackageVersion() string {
	return versions.NarsilcModule
}

// IsSupportedInertiaAdapter reports whether adapter names a frontend adapter
// Andurel knows how to scaffold.
func IsSupportedInertiaAdapter(adapter string) bool {
	switch adapter {
	case "vue", "react", "svelte":
		return true
	default:
		return false
	}
}

// IsSupportedJavaScriptRuntime reports whether runtime is a known
// JavaScript package manager.
func IsSupportedJavaScriptRuntime(runtime string) bool {
	switch runtime {
	case "npm", "pnpm", "bun":
		return true
	default:
		return false
	}
}

// IsSupportedJavaScriptSSRRuntime reports whether runtime is a known
// JavaScript executable for Inertia SSR.
func IsSupportedJavaScriptSSRRuntime(runtime string) bool {
	switch runtime {
	case "node", "bun":
		return true
	default:
		return false
	}
}

// Blueprint returns the underlying blueprint. If not yet initialized, creates
// a new one.
func (td *TemplateData) Blueprint() *blueprint.Blueprint {
	if td == nil {
		return nil
	}

	if td.blueprint == nil {
		td.blueprint = blueprint.New()
	}

	return td.blueprint
}

// SetBlueprint sets the blueprint for this template data.
func (td *TemplateData) SetBlueprint(bp *blueprint.Blueprint) {
	if td != nil {
		td.blueprint = bp
	}
}

// Builder returns a builder wrapping the template data's blueprint.
func (td *TemplateData) Builder() *blueprint.Builder {
	if td == nil {
		return nil
	}

	return blueprint.NewBuilder(td.Blueprint())
}
