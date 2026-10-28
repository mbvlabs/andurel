// Package versions defines default versions for tools installed in generated projects.
package versions

const (
	// Templ is the default templ tool version.
	Templ = "v0.3.1020"
	// Golines is the pinned golines formatter version used by CI and local tooling.
	Golines = "v0.13.0"
	// Goimports is the pinned goimports version used by CI and local tooling.
	Goimports = "v0.44.0"
	// Goose is the default goose migration tool version.
	Goose = "v3.27.1"
	// Mailpit is the default Mailpit tool version.
	Mailpit = "v1.30.2"
	// Usql is the default usql tool version.
	Usql = "v0.21.4"
	// Dblab is the default dblab tool version.
	Dblab = "v0.40.2"
	// TailwindCLI is the default Tailwind CSS CLI version.
	TailwindCLI = "v4.3.2"
	// Shadowfax is the default Shadowfax runner version.
	Shadowfax = "v0.12.0"
	// Narsilc is the default narsilc tool version.
	Narsilc = "v0.6.2"
	// NarsilcModule is the Go module version required by generated query code.
	NarsilcModule = "v0.6.2"
	// NarsilcModulePath is the module path of the narsilc query code generator.
	NarsilcModulePath = "github.com/mbvlabs/narsilc"
	// Anduin is the standalone Andurel HTTP module version verified with this framework.
	Anduin = "v0.6.1"
	// Auth is the standalone Andurel auth module version verified with this framework.
	Auth = "v0.2.1"
	// Cache is the standalone Andurel cache module version verified with this framework.
	Cache = "v0.2.1"
	// Kiks is the standalone Andurel cookies and sessions module version verified with this framework.
	Kiks = "v0.6.0"
	// Lifecycle is the standalone Andurel process lifecycle module version verified with this framework.
	Lifecycle = "v0.1.1"
	// Inertia is the standalone Andurel Inertia module version verified with this framework.
	Inertia = "v0.12.1"
	// Email is the standalone Andurel email module version verified with this framework.
	Email = "v0.6.0"
	// Env is the standalone Andurel environment module version verified with this framework.
	Env = "v0.1.1"
	// Routing is the standalone Andurel routing module version verified with this framework.
	Routing = "v0.7.0"
	// Storage is the standalone Andurel storage module version verified with this framework.
	Storage = "v0.13.1"
	// Telemetry is the standalone Andurel telemetry module version verified with this framework.
	Telemetry = "v0.2.2"
	// Validation is the standalone Andurel validation module version verified with this framework.
	Validation = "v0.3.1"

	// PkgPrefix is the module path prefix for standalone Andurel packages.
	PkgPrefix = "github.com/mbvlabs/andurel/pkg/"
)

var verifiedPackageVersions = map[string]string{
	PkgPrefix + "anduin":     Anduin,
	PkgPrefix + "auth":       Auth,
	PkgPrefix + "cache":      Cache,
	PkgPrefix + "email":      Email,
	PkgPrefix + "env":        Env,
	PkgPrefix + "inertia":    Inertia,
	PkgPrefix + "kiks":       Kiks,
	PkgPrefix + "lifecycle":  Lifecycle,
	PkgPrefix + "routing":    Routing,
	PkgPrefix + "storage":    Storage,
	PkgPrefix + "telemetry":  Telemetry,
	PkgPrefix + "validation": Validation,
}

// PackageVersion returns the version of a standalone Andurel package verified
// with this framework release.
func PackageVersion(modulePath string) (string, bool) {
	version, ok := verifiedPackageVersions[modulePath]
	return version, ok
}
