// Package versions defines default versions for tools installed in generated projects.
package versions

const (
	// Templ is the default templ tool version.
	Templ = "v0.3.1020"
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
	Shadowfax = "v0.10.0"
	// Sqlc is the default sqlc tool version.
	Sqlc = "v1.31.1"
	// Hypermedia is the standalone Andurel hypermedia module version verified with this framework.
	Hypermedia = "v0.2.3"
	// Inertia is the standalone Andurel Inertia module version verified with this framework.
	Inertia = "v0.5.0"
	// Email is the standalone Andurel email module version verified with this framework.
	Email = "v0.3.2"
	// Routing is the standalone Andurel routing module version verified with this framework.
	Routing = "v0.2.1"
	// Server is the standalone Andurel server module version verified with this framework.
	Server = "v0.3.2"
	// Storage is the standalone Andurel storage module version verified with this framework.
	Storage = "v0.7.1"
	// Telemetry is the standalone Andurel telemetry module version verified with this framework.
	Telemetry = "v0.1.0"
	// Validation is the standalone Andurel validation module version verified with this framework.
	Validation = "v0.1.3"

	// PkgPrefix is the module path prefix for standalone Andurel packages.
	PkgPrefix = "github.com/mbvlabs/andurel/pkg/"
)

var verifiedPackageVersions = map[string]string{
	PkgPrefix + "email":      Email,
	PkgPrefix + "hypermedia": Hypermedia,
	PkgPrefix + "inertia":    Inertia,
	PkgPrefix + "routing":    Routing,
	PkgPrefix + "server":     Server,
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
