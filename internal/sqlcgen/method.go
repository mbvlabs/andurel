package sqlcgen

import (
	"strings"

	"github.com/jinzhu/inflection"

	"github.com/mbvlabs/andurel/pkg/naming"
)

// queryVerbs is the ordered set of name-prefix verbs the plugin recognises.
// Order matters: longer-prefixed verbs (none here) would need to come first.
var queryVerbs = []string{"Query", "Count", "Insert", "Update", "Delete", "Upsert"}

// deriveMethodName converts an sqlc query name into the public method name
// the plugin will emit on the model's zero-sized base struct. The mapping is
// designed to produce Rails-ergonomic names for the canonical CRUD shape,
// while preserving suffixes (e.g. "BySlug") for non-canonical lookups.
//
// Examples for model "Server":
//
//	QueryServerByID  :one  -> Find
//	QueryServers     :many -> All
//	InsertServer     :one  -> Insert
//	UpdateServer     :one  -> Update
//	DeleteServer     :exec -> Delete
//	UpsertServer     :one  -> Upsert
//	CountServers     :one  -> Count
//	QueryServerBySlug:one  -> FindBySlug
//	QueryServersByStatus :many -> AllByStatus
func deriveMethodName(queryName, modelSingular, cmd string) string {
	verb, rest := splitVerb(queryName)
	plural := inflection.Plural(modelSingular)

	suffix := stripModelName(rest, modelSingular, plural)

	switch {
	case verb == "Query" && suffix == "":
		// QueryServer / QueryServers -> Find or All depending on cardinality.
		if cmd == ":many" {
			return "All"
		}
		return "Find"
	case verb == "Query" && suffix == "ByID":
		return "Find"
	case verb == "Query" && strings.HasPrefix(suffix, "By"):
		if cmd == ":many" {
			return "All" + suffix
		}
		return "Find" + suffix
	case verb == "Count" && suffix == "":
		return "Count"
	case verb == "Count" && strings.HasPrefix(suffix, "By"):
		return "Count" + suffix
	case (verb == "Insert" || verb == "Update" || verb == "Delete" || verb == "Upsert") && suffix == "":
		return verb
	}

	// Fallback: keep the verb plus whatever remained after the model name.
	// Guarantees we never emit an empty identifier.
	if verb == "" {
		return naming.ToPascalCase(queryName)
	}
	if suffix == "" {
		return verb
	}
	return verb + suffix
}

// splitVerb returns (verb, rest) where verb is one of queryVerbs if the name
// starts with one, otherwise ("", queryName).
func splitVerb(name string) (string, string) {
	for _, v := range queryVerbs {
		if strings.HasPrefix(name, v) {
			return v, name[len(v):]
		}
	}
	return "", name
}

// stripModelName removes a leading singular or plural model name from rest.
// Plural is checked first so that "Servers" doesn't get stripped as "Server"
// followed by a stray "s".
func stripModelName(rest, singular, plural string) string {
	if plural != "" && strings.HasPrefix(rest, plural) {
		return rest[len(plural):]
	}
	if singular != "" && strings.HasPrefix(rest, singular) {
		return rest[len(singular):]
	}
	return rest
}
