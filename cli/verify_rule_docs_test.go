package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
)

// TestVerifyRuleDocsMatchCatalog is the unit-test twin of the docs part of
// `andurel commands --check`: every agent doc's generated rule table matches
// the catalog, and no agent doc names a rule the catalog does not have.
func TestVerifyRuleDocsMatchCatalog(t *testing.T) {
	root := findAndurelFrameworkRoot()
	if root == "" {
		t.Fatal("andurel framework root not found from the test working directory")
	}
	if failures := checkVerifyRuleDocs(root); len(failures) > 0 {
		t.Fatalf("agent docs disagree with the verify rule catalog:\n%s", strings.Join(failures, "\n"))
	}
}

func verifyRulesTestDoc(region string) string {
	return "# Doc\n\n<!-- " + verifyRulesBeginMarker + " -->\n" + region + "<!-- " + verifyRulesEndMarker + " -->\n\nAfter.\n"
}

func TestCheckVerifyRulesDocNamesUnknownAndMissingRules(t *testing.T) {
	rules := layers.Catalog()
	region := renderVerifyRulesMarkdown(rules[1:]) + "| `tx-bypass` | retired |\n"
	failures := checkVerifyRulesDoc(verifyRulesTestDoc(region), rules)
	want := []string{
		`names rule "tx-bypass", which is not in the catalog`,
		`is missing catalog rule "` + rules[0].ID + `"`,
	}
	if !slices.Equal(failures, want) {
		t.Fatalf("failures = %q, want %q", failures, want)
	}
}

func TestCheckVerifyRulesDocReportsStaleSummary(t *testing.T) {
	rules := layers.Catalog()
	region := strings.Replace(renderVerifyRulesMarkdown(rules), rules[0].Summary, "stale", 1)
	failures := checkVerifyRulesDoc(verifyRulesTestDoc(region), rules)
	if !slices.Equal(failures, []string{"verify rule table is out of date"}) {
		t.Fatalf("failures = %q", failures)
	}
	if failures := checkVerifyRulesDoc("# Doc\n", rules); len(failures) != 1 ||
		!strings.Contains(failures[0], verifyRulesBeginMarker) {
		t.Fatalf("missing markers should fail, got %q", failures)
	}
}

func TestReplaceVerifyRulesRegion(t *testing.T) {
	rules := layers.Catalog()
	rendered := renderVerifyRulesMarkdown(rules)
	for name, region := range map[string]string{
		"empty": "",
		"stale": "| `tx-bypass` | retired |\n",
	} {
		t.Run(name, func(t *testing.T) {
			updated, err := replaceVerifyRulesRegion(verifyRulesTestDoc(region), rendered)
			if err != nil {
				t.Fatal(err)
			}
			if updated != verifyRulesTestDoc(rendered) {
				t.Fatalf("updated doc:\n%s", updated)
			}
			if failures := checkVerifyRulesDoc(updated, rules); len(failures) > 0 {
				t.Fatalf("regenerated doc still fails: %q", failures)
			}
		})
	}
}

func TestUnknownRuleMentions(t *testing.T) {
	text := "Run `tx-escape` and `tx-bypass`; see `test-domain`, `db-handle`, and `tx-bypass` again."
	got := unknownRuleMentions(text, layers.Catalog())
	if !slices.Equal(got, []string{"tx-bypass"}) {
		t.Fatalf("unknown mentions = %q", got)
	}
}
