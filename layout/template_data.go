package layout

import (
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/blueprint"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

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

	// Blueprint holds the structured scaffold configuration
	blueprint *blueprint.Blueprint
}

// DatabaseDialect returns the configured database, which is always PostgreSQL.
func (td *TemplateData) DatabaseDialect() string {
	return "postgresql"
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
