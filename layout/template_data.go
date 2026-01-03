package layout

import (
	"github.com/mbvlabs/andurel/layout/blueprint"
	"github.com/mbvlabs/andurel/layout/extensions"
)

// TemplateData carries the values available to base templates and extension
// contributions. It wraps a Blueprint for structured data alongside
// project-level metadata.
type TemplateData struct {
	AppName              string
	ProjectName          string
	ModuleName           string
	Database             string // Always "postgresql"
	CSSFramework         string
	GoVersion            string
	SessionKey           string
	SessionEncryptionKey string
	TokenSigningKey      string
	Pepper               string
	Extensions           []string
	RunToolVersion       string // Version of the run built tool

	// Blueprint holds the structured scaffold configuration
	blueprint *blueprint.Blueprint
}

// DatabaseDialect returns the configured database, which is always PostgreSQL.
func (td *TemplateData) DatabaseDialect() string {
	return "postgresql"
}

// GetModuleName returns the module name for the project.
func (td *TemplateData) GetModuleName() string {
	if td == nil {
		return ""
	}

	return td.ModuleName
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

// Builder returns a builder adapter wrapping the template data's blueprint.
// The return type satisfies the extensions.Builder interface.
func (td *TemplateData) Builder() *blueprint.Builder {
	if td == nil {
		return nil
	}

	return blueprint.NewBuilder(td.Blueprint())
}

var _ extensions.TemplateData = (*TemplateData)(nil)
