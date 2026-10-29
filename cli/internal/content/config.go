// Package content implements the declared content invariants behind
// `andurel verify --only content`.
//
// The declaration is a standalone andurel.content.toml at the project root,
// not a table in andurel.toml: andurel.toml is rewritten by tool sync and
// upgrade, so a hand-maintained content table there would be lost.
package content

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Rule IDs reported by this package. The verify rule catalog advertises the
// same IDs, and a test keeps the two lists in step.
const (
	RuleConfig   = "content-config"
	RuleCatalog  = "content-catalog"
	RuleSlug     = "content-slug"
	RuleRequired = "content-required"
	RuleLink     = "content-link"
	RuleChecker  = "content-checker"
)

// RuleIDs returns the rules the content stage can report.
func RuleIDs() []string {
	return []string{
		RuleConfig,
		RuleCatalog,
		RuleSlug,
		RuleRequired,
		RuleLink,
		RuleChecker,
	}
}

// ManifestName is the project-root content declaration file.
const ManifestName = "andurel.content.toml"

// SchemaVersion is the only supported manifest version.
const SchemaVersion = 1

// DefaultSlugPattern is the documented default: kebab-case slug segments.
const DefaultSlugPattern = "^[a-z0-9]+(?:-[a-z0-9]+)*$"

const (
	defaultCheckerTimeout = 120 * time.Second
	maxCheckerTimeout     = 600 * time.Second
)

// Diagnostic is one actionable content finding. Rule is a stable catalog ID.
type Diagnostic struct {
	Rule    string
	File    string
	Line    int
	Message string
	Fix     string
}

// Config is the parsed and validated andurel.content.toml.
type Config struct {
	SchemaVersion int            `toml:"schemaVersion"`
	Include       []string       `toml:"include"`
	Exclude       []string       `toml:"exclude"`
	Required      []string       `toml:"required"`
	CatalogFile   string         `toml:"catalogFile"`
	Catalog       []CatalogEntry `toml:"catalog"`
	Slugs         SlugConfig     `toml:"slugs"`
	Links         LinkConfig     `toml:"links"`
	Checkers      []Checker      `toml:"checkers"`

	slugPattern *regexp.Regexp
}

// CatalogEntry maps a stable slug to the Markdown file that serves it.
type CatalogEntry struct {
	Slug string `toml:"slug"`
	File string `toml:"file"`
}

// SlugConfig configures the allowed slug shape.
type SlugConfig struct {
	// Pattern is a full-match regular expression. Empty disables the check;
	// the documented default is DefaultSlugPattern.
	Pattern string `toml:"pattern"`
}

// LinkConfig configures local link checking. External URLs are never fetched.
type LinkConfig struct {
	// Anchors is "ignore" (default) or "check".
	Anchors string `toml:"anchors"`
	// External is documentation only; the only supported value is "ignore".
	External string `toml:"external"`
}

// Checker is one project-declared deterministic checker. Run is an argv array
// executed without a shell; Dir is relative to the project root.
type Checker struct {
	Name           string   `toml:"name"`
	Run            []string `toml:"run"`
	Dir            string   `toml:"dir"`
	TimeoutSeconds int      `toml:"timeoutSeconds"`
}

type catalogFileDocument struct {
	SchemaVersion int            `toml:"schemaVersion"`
	Entries       []CatalogEntry `toml:"entry"`
}

// Load reads and validates the content manifest. A missing manifest returns
// (nil, nil): the content stage is not configured, not broken.
func Load(rootDir string) (*Config, []Diagnostic) {
	path := filepath.Join(rootDir, ManifestName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []Diagnostic{{
			Rule:    RuleConfig,
			File:    ManifestName,
			Message: "read " + ManifestName + ": " + err.Error(),
			Fix:     "make " + ManifestName + " readable",
		}}
	}
	var cfg Config
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, []Diagnostic{configDecodeDiagnostic(err)}
	}
	if diagnostics := cfg.validate(); len(diagnostics) > 0 {
		return nil, diagnostics
	}
	if err := cfg.loadCatalogFile(rootDir); err != nil {
		return nil, []Diagnostic{*err}
	}
	return &cfg, nil
}

func configDecodeDiagnostic(err error) Diagnostic {
	diagnostic := Diagnostic{
		Rule:    RuleConfig,
		File:    ManifestName,
		Message: "parse " + ManifestName + ": unknown key or wrong value type: " + err.Error(),
		Fix:     "fix the reported TOML syntax or unknown key; the schema is documented in 'andurel verify --help'",
	}
	if decodeErr, ok := errors.AsType[*toml.DecodeError](err); ok {
		row, _ := decodeErr.Position()
		diagnostic.Line = row
	}
	return diagnostic
}

func (c *Config) validate() []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	add := func(message, fix string) {
		diagnostics = append(diagnostics, Diagnostic{
			Rule:    RuleConfig,
			File:    ManifestName,
			Message: message,
			Fix:     fix,
		})
	}
	if c.SchemaVersion != SchemaVersion {
		add(
			fmt.Sprintf("schemaVersion is %d; only %d is supported", c.SchemaVersion, SchemaVersion),
			fmt.Sprintf("set schemaVersion = %d", SchemaVersion),
		)
	}
	if len(c.Include) == 0 {
		add("include declares no Markdown globs", `add include = ["content/**/*.md"] or the roots this project publishes`)
	}
	for _, pattern := range append(append([]string{}, c.Include...), c.Exclude...) {
		if err := validateGlob(pattern); err != nil {
			add(fmt.Sprintf("invalid glob %q: %s", pattern, err), "use a root-relative glob such as content/**/*.md")
		}
	}
	for _, required := range c.Required {
		if err := validateRelativePath(required); err != nil {
			add(fmt.Sprintf("invalid required path %q: %s", required, err), "use a project-root-relative path without '..'")
		}
	}
	if c.CatalogFile != "" {
		if err := validateRelativePath(c.CatalogFile); err != nil {
			add(fmt.Sprintf("invalid catalogFile %q: %s", c.CatalogFile, err), "use a project-root-relative path without '..'")
		}
	}
	switch c.Slugs.Pattern {
	case "":
	default:
		pattern, err := regexp.Compile(c.Slugs.Pattern)
		if err != nil {
			add("slugs.pattern is not a valid regular expression: "+err.Error(), "fix [slugs].pattern or leave it empty to disable the shape check")
		} else {
			c.slugPattern = pattern
		}
	}
	if c.slugPattern == nil && c.Slugs.Pattern == "" {
		c.slugPattern = regexp.MustCompile(DefaultSlugPattern)
	}
	switch c.Links.Anchors {
	case "", "ignore":
		c.Links.Anchors = "ignore"
	case "check":
	default:
		add(
			fmt.Sprintf("links.anchors is %q; only \"ignore\" or \"check\" is supported", c.Links.Anchors),
			`set links.anchors = "ignore" or links.anchors = "check"`,
		)
	}
	switch c.Links.External {
	case "", "ignore":
		c.Links.External = "ignore"
	default:
		add(
			fmt.Sprintf("links.external is %q; network link checking is not supported", c.Links.External),
			`network link checking is not part of the deterministic default; leave links.external = "ignore"`,
		)
	}
	seenChecker := map[string]bool{}
	for index, checker := range c.Checkers {
		label := fmt.Sprintf("checkers[%d]", index)
		if strings.TrimSpace(checker.Name) == "" {
			add(label+" has no name", "give every [[checkers]] entry a unique name")
		} else if seenChecker[checker.Name] {
			add(fmt.Sprintf("duplicate checker name %q", checker.Name), "give every [[checkers]] entry a unique name")
		}
		seenChecker[checker.Name] = true
		if len(checker.Run) == 0 || strings.TrimSpace(checker.Run[0]) == "" {
			add(label+" has no run argv", `set run = ["go", "run", "./internal/docscheck"]`)
		}
		if checker.Dir != "" {
			if err := validateRelativePath(checker.Dir); err != nil {
				add(fmt.Sprintf("checker %q has invalid dir %q: %s", checker.Name, checker.Dir, err), "use a directory inside the project root")
			}
		}
		if checker.TimeoutSeconds < 0 || time.Duration(checker.TimeoutSeconds)*time.Second > maxCheckerTimeout {
			add(
				fmt.Sprintf("checker %q timeoutSeconds must be between 1 and %d", checker.Name, int(maxCheckerTimeout/time.Second)),
				"use a bounded timeout; the default is 120 seconds",
			)
		}
	}
	return diagnostics
}

func (c *Config) loadCatalogFile(rootDir string) *Diagnostic {
	if c.CatalogFile == "" {
		return nil
	}
	path := filepath.Join(rootDir, filepath.FromSlash(c.CatalogFile))
	data, err := os.ReadFile(path)
	if err != nil {
		return &Diagnostic{
			Rule:    RuleConfig,
			File:    c.CatalogFile,
			Message: "read catalog file: " + err.Error(),
			Fix:     "create the catalog file named by catalogFile, or remove the catalogFile entry",
		}
	}
	var doc catalogFileDocument
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		diagnostic := configDecodeDiagnostic(err)
		diagnostic.File = c.CatalogFile
		return &diagnostic
	}
	if doc.SchemaVersion != SchemaVersion {
		return &Diagnostic{
			Rule:    RuleConfig,
			File:    c.CatalogFile,
			Message: fmt.Sprintf("schemaVersion is %d; only %d is supported", doc.SchemaVersion, SchemaVersion),
			Fix:     fmt.Sprintf("set schemaVersion = %d", SchemaVersion),
		}
	}
	c.Catalog = append(c.Catalog, doc.Entries...)
	return nil
}

func validateRelativePath(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("path is empty")
	}
	if strings.HasPrefix(value, "/") || filepath.IsAbs(value) {
		return errors.New("path must be relative to the project root")
	}
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return errors.New("path escapes the project root")
	}
	return nil
}

func validateGlob(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return errors.New("glob is empty")
	}
	if strings.HasPrefix(pattern, "/") || filepath.IsAbs(pattern) {
		return errors.New("glob must be relative to the project root")
	}
	for segment := range strings.SplitSeq(pattern, "/") {
		if segment == ".." {
			return errors.New("glob escapes the project root")
		}
	}
	return nil
}

// checkerTimeout returns the bounded timeout for one checker.
func (c Checker) checkerTimeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return defaultCheckerTimeout
	}
	timeout := time.Duration(c.TimeoutSeconds) * time.Second
	if timeout > maxCheckerTimeout {
		return maxCheckerTimeout
	}
	return timeout
}
