package layout

import "github.com/mbvlabs/andurel/layout/templatedata"

// TemplateData is the scaffold template context shared with extensions.
// The concrete implementation lives in the templatedata subpackage to avoid
// import cycles while keeping the type exported from layout.
type TemplateData = templatedata.TemplateData
