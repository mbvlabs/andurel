// Package layers implements the typed persistence and layer-boundary checks
// behind `andurel verify`.
//
// It loads the project with go/packages and runs go/analysis analyzers
// in-process. The rule catalog below is the single source for the rules that
// `verify --help` and `commands --json` advertise.
package layers

// Rule is one verification rule. ID is the value reported in a violation's
// rule field; Layer is the kind of check ("format", "build", "layers",
// "frontend", or "generated").
type Rule struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Fix     string `json:"fix"`
	Layer   string `json:"layer"`
}

// RuleLayerLayers identifies the type-based layer analyzers.
const RuleLayerLayers = "layers"

var catalog = []Rule{
	{
		ID:      "gofmt",
		Summary: "every Go file is gofmt-clean",
		Fix:     "gofmt -w <file>",
		Layer:   "format",
	},
	{
		ID:      "go fix",
		Summary: "go fix reports no automatic rewrites",
		Fix:     "go fix ./...",
		Layer:   "build",
	},
	{
		ID:      "queries-import",
		Summary: "only domains/** imports domains/internal/queries",
		Fix:     "call a method on a domains receiver instead of generated queries",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "db-handle",
		Summary: "outside domains no SQL methods or SQL drivers; Connection/Transaction are persistence wiring",
		Fix:     "move the database access into a domains method and call it on a receiver",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "tx-unwrap",
		Summary: "storage.Unwrap is referenced only under domains/**",
		Fix:     "bind a domain receiver with receiver.InTx(tx) instead of unwrapping the transaction",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "tx-escape",
		Summary: "a storage.AtomicOps callback uses only dependencies bound inside the transaction",
		Fix:     "bind the dependency at the top of the callback (dep := value.InTx(tx)) and use the bound value",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "tx-public-call",
		Summary: "a storage.AtomicOps callback does not call an exported method on its own receiver",
		Fix:     "move the shared logic into an unexported helper that takes bound dependencies as parameters",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "tx-layer",
		Summary: "storage.AtomicOps and storage.Transactor are used only in services, domains, and internal/runtime",
		Fix:     "move transaction orchestration into a service; run 'andurel patterns show service'",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "domain-transport",
		Summary: "domains/** imports no HTTP, Inertia, routing, handler, middleware, or route package",
		Fix:     "keep domains free of transport packages; move the adapter to handlers or services",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "entity-columns",
		Summary: "every automatic generated query column maps to an exported andurel tag on its domain entity",
		Fix:     "add an exported field tagged andurel:\"<column>\", or narrow the query columns and run 'andurel sync queries'",
		Layer:   RuleLayerLayers,
	},
	{
		ID:      "frontend",
		Summary: "the package.json typecheck script passes when the scaffold defines one",
		Fix:     "fix the reported TypeScript errors and re-run the project package manager's typecheck script",
		Layer:   "frontend",
	},
	{
		ID:      "generated",
		Summary: "generated templ, email, queries, routes, and payload output is up to date",
		Fix:     "run the owning andurel sync command",
		Layer:   "generated",
	},
	{
		ID:      "missing-tool",
		Summary: "a required generation binary is installed; run 'andurel tool sync'",
		Fix:     "andurel tool sync",
		Layer:   "generated",
	},
}

// Catalog returns the ordered verification rule catalog. The slice is a copy;
// callers cannot reorder the shared source of truth.
func Catalog() []Rule {
	return append([]Rule(nil), catalog...)
}

// RuleByID looks up a rule by its ID.
func RuleByID(id string) (Rule, bool) {
	for _, rule := range catalog {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

// LayerRules returns only the rules implemented by the typed layer analyzers.
func LayerRules() []Rule {
	rules := make([]Rule, 0, len(catalog))
	for _, rule := range catalog {
		if rule.Layer == RuleLayerLayers {
			rules = append(rules, rule)
		}
	}
	return rules
}

func fixFor(ruleID string) string {
	rule, ok := RuleByID(ruleID)
	if !ok {
		return ""
	}
	return rule.Fix
}
