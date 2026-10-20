package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
)

// The verify rule list in the agent docs is generated from layers.Catalog()
// into a marked region. Markdown files mark it with HTML comments; agents.tmpl
// marks it with template comments so the scaffolded AGENTS.md carries no
// markers. Both markers sit on their own line and the generated table sits
// between them.
const (
	verifyRulesBeginMarker = "andurel:verify-rules:begin"
	verifyRulesEndMarker   = "andurel:verify-rules:end"
	verifyRulesSyncCommand = "just docs-sync"
)

// verifyRuleDocPaths are the framework docs that embed the generated rule
// table, relative to the framework root.
var verifyRuleDocPaths = []string{
	"layout/templates/agents.tmpl",
	"skills/andurel/SKILL.md",
	"skills/andurel/references/cli-catalog.md",
}

// verifyRuleMentionGlobs are the agent-facing docs scanned for rule-shaped
// names that are not in the catalog. They include the generated docs above.
var verifyRuleMentionGlobs = []string{
	"README.md",
	"layout/templates/*.tmpl",
	"skills/andurel/*.md",
	"skills/andurel/references/*.md",
	"cli/patterns/assets/*.md",
}

var (
	verifyRuleTableRowPattern = regexp.MustCompile("^\\| `([^`]+)` \\|")
	backtickedNamePattern     = regexp.MustCompile("`([a-z]+(?:-[a-z]+)+)`")
)

// renderVerifyRulesMarkdown renders the rule catalog as the Markdown table
// that fills every verify-rules region.
func renderVerifyRulesMarkdown(rules []layers.Rule) string {
	var b strings.Builder
	b.WriteString("| Rule | Checks |\n")
	b.WriteString("| --- | --- |\n")
	for _, rule := range rules {
		fmt.Fprintf(&b, "| `%s` | %s |\n", rule.ID, escapeMarkdownCell(rule.Summary))
	}
	return b.String()
}

// verifyRulesRegion locates the generated region: the lines strictly between
// the begin and end marker lines. It returns the byte offsets of the region.
func verifyRulesRegion(text string) (start, end int, err error) {
	begin := strings.Index(text, verifyRulesBeginMarker)
	if begin < 0 {
		return 0, 0, fmt.Errorf("missing %s marker", verifyRulesBeginMarker)
	}
	if strings.Count(text, verifyRulesBeginMarker) > 1 {
		return 0, 0, fmt.Errorf("more than one %s marker", verifyRulesBeginMarker)
	}
	newline := strings.IndexByte(text[begin:], '\n')
	if newline < 0 {
		return 0, 0, fmt.Errorf("missing %s marker", verifyRulesEndMarker)
	}
	start = begin + newline + 1
	endMarker := strings.Index(text[start:], verifyRulesEndMarker)
	if endMarker < 0 {
		return 0, 0, fmt.Errorf("missing %s marker after the begin marker", verifyRulesEndMarker)
	}
	end = start + strings.LastIndexByte(text[start:start+endMarker], '\n') + 1
	return start, end, nil
}

// replaceVerifyRulesRegion returns text with its verify-rules region replaced
// by rendered.
func replaceVerifyRulesRegion(text, rendered string) (string, error) {
	start, end, err := verifyRulesRegion(text)
	if err != nil {
		return "", err
	}
	return text[:start] + rendered + text[end:], nil
}

// checkVerifyRulesDoc compares one doc's generated region with the catalog.
// It names unknown and missing rules explicitly and reports any other
// difference as an out-of-date region.
func checkVerifyRulesDoc(text string, rules []layers.Rule) []string {
	start, end, err := verifyRulesRegion(text)
	if err != nil {
		return []string{err.Error()}
	}
	region := text[start:end]
	failures := make([]string, 0)
	documented := map[string]bool{}
	for line := range strings.SplitSeq(region, "\n") {
		match := verifyRuleTableRowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		documented[match[1]] = true
		if _, ok := layers.RuleByID(match[1]); !ok {
			failures = append(failures, fmt.Sprintf("names rule %q, which is not in the catalog", match[1]))
		}
	}
	for _, rule := range rules {
		if !documented[rule.ID] {
			failures = append(failures, fmt.Sprintf("is missing catalog rule %q", rule.ID))
		}
	}
	if len(failures) == 0 && region != renderVerifyRulesMarkdown(rules) {
		failures = append(failures, "verify rule table is out of date")
	}
	return failures
}

// unknownRuleMentions returns backticked rule-shaped names that are not in the
// catalog. A name is rule-shaped when it is hyphenated and shares its first
// segment with a hyphenated catalog rule (tx-, db-, and so on), which catches
// retired or misspelled rule names without flagging ordinary identifiers.
func unknownRuleMentions(text string, rules []layers.Rule) []string {
	known := map[string]bool{}
	prefixes := map[string]bool{}
	for _, rule := range rules {
		known[rule.ID] = true
		if head, _, ok := strings.Cut(rule.ID, "-"); ok {
			prefixes[head] = true
		}
	}
	unknown := make([]string, 0)
	for _, match := range backtickedNamePattern.FindAllStringSubmatch(text, -1) {
		name := match[1]
		head, _, _ := strings.Cut(name, "-")
		if prefixes[head] && !known[name] && !slices.Contains(unknown, name) {
			unknown = append(unknown, name)
		}
	}
	return unknown
}

// checkVerifyRuleDocs reports every agent doc under frameworkRoot that
// disagrees with the rule catalog.
func checkVerifyRuleDocs(frameworkRoot string) []string {
	rules := layers.Catalog()
	failures := make([]string, 0)
	for _, rel := range verifyRuleDocPaths {
		body, err := os.ReadFile(filepath.Join(frameworkRoot, filepath.FromSlash(rel)))
		if err != nil {
			failures = append(failures, rel+": "+err.Error())
			continue
		}
		for _, failure := range checkVerifyRulesDoc(string(body), rules) {
			failures = append(
				failures,
				fmt.Sprintf("%s: %s; run '%s'", rel, failure, verifyRulesSyncCommand),
			)
		}
	}
	for _, pattern := range verifyRuleMentionGlobs {
		matches, err := filepath.Glob(filepath.Join(frameworkRoot, filepath.FromSlash(pattern)))
		if err != nil {
			failures = append(failures, pattern+": "+err.Error())
			continue
		}
		for _, path := range matches {
			body, err := os.ReadFile(path)
			if err != nil {
				failures = append(failures, path+": "+err.Error())
				continue
			}
			rel, err := filepath.Rel(frameworkRoot, path)
			if err != nil {
				rel = path
			}
			for _, name := range unknownRuleMentions(string(body), rules) {
				failures = append(
					failures,
					fmt.Sprintf("%s: names rule %q, which is not in the verify rule catalog", filepath.ToSlash(rel), name),
				)
			}
		}
	}
	return failures
}

// writeVerifyRuleDocs regenerates the verify-rules region of every doc under
// frameworkRoot and returns the docs it changed.
func writeVerifyRuleDocs(frameworkRoot string) ([]string, error) {
	rendered := renderVerifyRulesMarkdown(layers.Catalog())
	changed := make([]string, 0)
	for _, rel := range verifyRuleDocPaths {
		path := filepath.Join(frameworkRoot, filepath.FromSlash(rel))
		body, err := os.ReadFile(path)
		if err != nil {
			return changed, err
		}
		updated, err := replaceVerifyRulesRegion(string(body), rendered)
		if err != nil {
			return changed, fmt.Errorf("%s: %w", rel, err)
		}
		if updated == string(body) {
			continue
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return changed, err
		}
		changed = append(changed, rel)
	}
	return changed, nil
}
