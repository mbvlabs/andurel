package sqlcgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// modelGroup is the set of queries the plugin will emit into one file.
type modelGroup struct {
	Model   string // e.g. "Server" — exactly as written in -- model:
	Queries []*plugin.Query
}

// modelAnnotation is the comment directive the plugin requires on every query.
const modelAnnotation = "model:"

// groupQueriesByModel reads each query's comments, expects exactly one
// "model:" directive, and buckets queries by that value. The result is sorted
// by model name for deterministic file emission.
func groupQueriesByModel(queries []*plugin.Query) ([]modelGroup, error) {
	byModel := map[string][]*plugin.Query{}
	for _, q := range queries {
		model, err := extractModelAnnotation(q)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.GetName(), err)
		}
		byModel[model] = append(byModel[model], q)
	}

	groups := make([]modelGroup, 0, len(byModel))
	for model, qs := range byModel {
		sort.SliceStable(qs, func(i, j int) bool { return qs[i].GetName() < qs[j].GetName() })
		groups = append(groups, modelGroup{Model: model, Queries: qs})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Model < groups[j].Model })
	return groups, nil
}

// extractModelAnnotation finds the single "model: <Name>" directive in a
// query's comments. Both leading "-- " and a leading "--" are stripped — sqlc
// strips one or the other depending on version. Whitespace is trimmed.
func extractModelAnnotation(q *plugin.Query) (string, error) {
	var found string
	for _, raw := range q.GetComments() {
		line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "--"))
		if !strings.HasPrefix(line, modelAnnotation) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, modelAnnotation))
		if value == "" {
			return "", fmt.Errorf("empty %s directive", modelAnnotation)
		}
		if found != "" && found != value {
			return "", fmt.Errorf("conflicting %s directives: %q vs %q", modelAnnotation, found, value)
		}
		found = value
	}
	if found == "" {
		return "", fmt.Errorf("missing required %q comment", "-- "+modelAnnotation+" <Name>")
	}
	return found, nil
}
