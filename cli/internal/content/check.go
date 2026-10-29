package content

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// projectTree is the deterministic working-tree view used by the content
// checks. Paths are project-root-relative, slash-separated, and compared
// case-sensitively so results do not depend on the host filesystem.
type projectTree struct {
	files map[string]bool
	dirs  map[string]bool
	paths []string
}

func walkProject(rootDir string) (projectTree, error) {
	tree := projectTree{files: map[string]bool{}, dirs: map[string]bool{}}
	err := filepath.WalkDir(rootDir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == rootDir {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == ".git" || name == "node_modules" || name == "bin" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(rootDir, current)
			if relErr != nil {
				return relErr
			}
			tree.dirs[filepath.ToSlash(rel)] = true
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, current)
		if relErr != nil {
			return relErr
		}
		tree.files[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		return tree, err
	}
	tree.paths = make([]string, 0, len(tree.files))
	for file := range tree.files {
		tree.paths = append(tree.paths, file)
	}
	sort.Strings(tree.paths)
	return tree, nil
}

// matchGlob matches a root-relative glob where '*' and '?' stay inside one
// path segment and '**' matches any number of segments.
func matchGlob(pattern, name string) bool {
	patternSegments := strings.Split(pattern, "/")
	nameSegments := strings.Split(name, "/")
	var match func(pattern, name []string) bool
	match = func(pattern, name []string) bool {
		for len(pattern) > 0 {
			if pattern[0] == "**" {
				for skip := 0; skip <= len(name); skip++ {
					if match(pattern[1:], name[skip:]) {
						return true
					}
				}
				return false
			}
			if len(name) == 0 {
				return false
			}
			ok, err := path.Match(pattern[0], name[0])
			if err != nil || !ok {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
		return len(name) == 0
	}
	return match(patternSegments, nameSegments)
}

// Check runs every declared content invariant.
func Check(rootDir string, cfg *Config) []Diagnostic {
	return CheckSelected(rootDir, cfg, nil)
}

// CheckSelected runs only the selected content rules; an empty selection runs
// every rule. Configuration is always validated by Load before this point. A
// rule that is not selected must not execute its built-in or a declared
// subprocess checker: selecting content-link must never run a stateful
// content-checker and then discard its failure.
func CheckSelected(rootDir string, cfg *Config, selectedRules []string) []Diagnostic {
	rootDir = cfg.resolvedRoot(rootDir)
	diagnostics := make([]Diagnostic, 0)
	needTree := contentRuleSelected(selectedRules, RuleCatalog) ||
		contentRuleSelected(selectedRules, RuleSlug) ||
		contentRuleSelected(selectedRules, RuleRequired) ||
		contentRuleSelected(selectedRules, RuleLink)
	if needTree {
		tree, err := walkProject(rootDir)
		if err != nil {
			return []Diagnostic{{
				Rule:    RuleConfig,
				Message: "scan project tree: " + err.Error(),
				Fix:     "make the project tree readable",
			}}
		}
		documents := includedDocuments(cfg, tree)
		// checkCatalog reports both catalog-target and slug diagnostics; the
		// caller filters any rule it did not select.
		if contentRuleSelected(selectedRules, RuleCatalog) ||
			contentRuleSelected(selectedRules, RuleSlug) {
			diagnostics = append(diagnostics, checkCatalog(cfg, tree, documents)...)
		}
		if contentRuleSelected(selectedRules, RuleRequired) {
			diagnostics = append(diagnostics, checkRequired(cfg, tree)...)
		}
		if contentRuleSelected(selectedRules, RuleLink) {
			diagnostics = append(diagnostics, checkLinks(rootDir, cfg, tree, documents)...)
		}
	}
	if contentRuleSelected(selectedRules, RuleChecker) {
		diagnostics = append(diagnostics, checkDeclaredCheckers(rootDir, cfg)...)
	}
	return diagnostics
}

func contentRuleSelected(selectedRules []string, rule string) bool {
	return len(selectedRules) == 0 || slices.Contains(selectedRules, rule)
}

// resolvedRoot returns the canonical project root Load recorded, or the given
// root for a Config built outside Load.
func (c *Config) resolvedRoot(rootDir string) string {
	if c.rootDir != "" {
		return c.rootDir
	}
	if resolved, err := resolveProjectRoot(rootDir); err == nil {
		return resolved
	}
	return rootDir
}

func includedDocuments(cfg *Config, tree projectTree) []string {
	documents := make([]string, 0)
	for _, file := range tree.paths {
		if !strings.EqualFold(path.Ext(file), ".md") {
			continue
		}
		included := false
		for _, pattern := range cfg.Include {
			if matchGlob(pattern, file) {
				included = true
				break
			}
		}
		if !included || excluded(cfg, file) {
			continue
		}
		documents = append(documents, file)
	}
	return documents
}

func excluded(cfg *Config, file string) bool {
	for _, pattern := range cfg.Exclude {
		if matchGlob(pattern, file) {
			return true
		}
	}
	return false
}

func checkCatalog(cfg *Config, tree projectTree, documents []string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	documentSet := map[string]bool{}
	for _, document := range documents {
		documentSet[document] = true
	}
	seenSlugs := map[string]CatalogEntry{}
	for _, entry := range cfg.Catalog {
		slug := strings.TrimSpace(entry.Slug)
		target := filepath.ToSlash(strings.TrimSpace(entry.File))
		if slug == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleSlug,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry for %q has an empty slug", entry.File),
				Fix:     "give the entry a stable non-empty slug",
			})
		} else if cfg.slugPattern != nil && !cfg.slugPattern.MatchString(slug) {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleSlug,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog slug %q does not match %s", slug, cfg.slugPattern.String()),
				Fix:     "rename the slug to match [slugs].pattern",
			})
		}
		if slug != "" {
			if previous, ok := seenSlugs[slug]; ok {
				diagnostics = append(diagnostics, Diagnostic{
					Rule:    RuleSlug,
					File:    ManifestName,
					Message: fmt.Sprintf("duplicate catalog slug %q: %s and %s", slug, previous.File, entry.File),
					Fix:     "give each catalog entry a unique slug",
				})
			} else {
				seenSlugs[slug] = entry
			}
		}
		if target == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleCatalog,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry %q has no file", slug),
				Fix:     "set file to the Markdown document that serves the slug",
			})
			continue
		}
		if !strings.EqualFold(path.Ext(target), ".md") {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleCatalog,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry %q targets %q, which is not a Markdown file", slug, target),
				Fix:     "catalog entries must resolve to a .md file",
			})
			continue
		}
		if !tree.files[target] {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleCatalog,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry %q targets missing file %s", slug, target),
				Fix:     "add the Markdown file or correct the catalog entry's file path",
			})
			continue
		}
		if excluded(cfg, target) {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleCatalog,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry %q targets excluded file %s", slug, target),
				Fix:     "remove the exclusion or point the catalog entry at an included file",
			})
			continue
		}
		if !documentSet[target] {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleCatalog,
				File:    ManifestName,
				Message: fmt.Sprintf("catalog entry %q targets %s, which is outside include %v", slug, target, cfg.Include),
				Fix:     "add the file's directory to include or move the document under an included root",
			})
		}
	}
	return diagnostics
}

func checkRequired(cfg *Config, tree projectTree) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, required := range cfg.Required {
		required = filepath.ToSlash(strings.TrimSpace(required))
		if tree.files[required] || tree.dirs[required] {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Rule:    RuleRequired,
			File:    ManifestName,
			Message: "required path is missing: " + required,
			Fix:     "create the required file or directory, or remove it from the required list",
		})
	}
	return diagnostics
}

func checkLinks(
	rootDir string,
	cfg *Config,
	tree projectTree,
	documents []string,
) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	anchorCache := map[string]map[string]bool{}
	for _, document := range documents {
		body, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(document)))
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Rule:    RuleConfig,
				File:    document,
				Message: "read document: " + err.Error(),
				Fix:     "make the document readable",
			})
			continue
		}
		for _, link := range scanDocument(string(body)) {
			diagnostics = append(
				diagnostics,
				checkLink(rootDir, cfg, tree, document, link, anchorCache)...,
			)
		}
	}
	return diagnostics
}

type documentLink struct {
	Line   int
	Target string
}

func checkLink(
	rootDir string,
	cfg *Config,
	tree projectTree,
	document string,
	link documentLink,
	anchorCache map[string]map[string]bool,
) []Diagnostic {
	target := strings.TrimSpace(link.Target)
	add := func(message, fix string) []Diagnostic {
		return []Diagnostic{{
			Rule:    RuleLink,
			File:    document,
			Line:    link.Line,
			Message: message,
			Fix:     fix,
		}}
	}
	if target == "" {
		return add("link target is empty", "give the link a local path or remove it")
	}
	if strings.HasPrefix(target, "#") {
		if cfg.Links.Anchors != "check" {
			return nil
		}
		if !anchorExists(rootDir, tree, document, target, anchorCache) {
			return add(
				fmt.Sprintf("anchor %q does not exist in %s", target, document),
				"add the heading or correct the anchor",
			)
		}
		return nil
	}
	if isExternalTarget(target) {
		return nil
	}
	if strings.HasPrefix(target, "/") {
		// Site-absolute URLs are routing concerns outside the project tree;
		// the content stage does not guess a site root.
		return nil
	}
	pathPart, fragment, hasFragment := splitTarget(target)
	if pathPart == "" {
		return nil
	}
	decoded, err := url.PathUnescape(pathPart)
	if err != nil {
		return add(fmt.Sprintf("link target %q is not valid percent-encoding", target), "fix the percent-encoding in the link")
	}
	resolved := path.Clean(path.Join(path.Dir(document), decoded))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return add(fmt.Sprintf("link target %q escapes the project root", target), "keep local links inside the project")
	}
	isDirectory := strings.HasSuffix(decoded, "/") || tree.dirs[resolved]
	if isDirectory {
		index := path.Join(resolved, "index.md")
		readme := path.Join(resolved, "README.md")
		if !tree.files[index] && !tree.files[readme] {
			return add(
				fmt.Sprintf("link target %q resolves to a directory without index.md or README.md", target),
				"add an index.md/README.md or link to the file directly",
			)
		}
		if hasFragment && cfg.Links.Anchors == "check" {
			anchorDocument := index
			if !tree.files[index] {
				anchorDocument = readme
			}
			if !anchorExists(rootDir, tree, anchorDocument, "#"+fragment, anchorCache) {
				return add(
					fmt.Sprintf("anchor %q does not exist in %s", "#"+fragment, anchorDocument),
					"add the heading or correct the anchor",
				)
			}
		}
		return nil
	}
	if !tree.files[resolved] {
		return add(
			fmt.Sprintf("link target %q resolves to missing file %s", target, resolved),
			"create the file or correct the link target",
		)
	}
	if hasFragment && cfg.Links.Anchors == "check" && strings.EqualFold(path.Ext(resolved), ".md") {
		if !anchorExists(rootDir, tree, resolved, "#"+fragment, anchorCache) {
			return add(
				fmt.Sprintf("anchor %q does not exist in %s", "#"+fragment, resolved),
				"add the heading or correct the anchor",
			)
		}
	}
	return nil
}

// isExternalTarget reports URLs the stage never validates: URI schemes and
// protocol-relative hosts are not fetched by the deterministic default.
func isExternalTarget(target string) bool {
	if strings.HasPrefix(target, "//") {
		return true
	}
	colon := strings.IndexByte(target, ':')
	if colon <= 0 {
		return false
	}
	scheme := target[:colon]
	for index, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case index > 0 && r >= '0' && r <= '9', index > 0 && (r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}

func splitTarget(target string) (pathPart, fragment string, hasFragment bool) {
	if before, after, ok := strings.Cut(target, "#"); ok {
		target = before
		fragment = after
		hasFragment = true
	}
	if before, _, ok := strings.Cut(target, "?"); ok {
		target = before
	}
	return target, fragment, hasFragment
}

func anchorExists(
	rootDir string,
	tree projectTree,
	document, fragment string,
	cache map[string]map[string]bool,
) bool {
	anchors, ok := cache[document]
	if !ok {
		body, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(document)))
		if err != nil {
			return false
		}
		anchors = documentAnchors(string(body))
		cache[document] = anchors
	}
	return anchors[strings.TrimPrefix(fragment, "#")]
}

var (
	atxHeadingPattern  = regexp.MustCompile(`^ {0,3}#{1,6}[ \t]+(.*?)[ \t]*#*[ \t]*$`)
	referenceLinkStart = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:[ \t]*(\S+)`)
)

// documentAnchors collects GitHub-style heading anchors from a Markdown
// document. Only ATX headings are recognized; explicit HTML anchors are not.
func documentAnchors(body string) map[string]bool {
	anchors := map[string]bool{}
	seen := map[string]int{}
	for _, line := range markdownLines(body) {
		match := atxHeadingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		slug := headingSlug(match[1])
		if slug == "" {
			continue
		}
		count := seen[slug]
		seen[slug] = count + 1
		if count > 0 {
			slug = fmt.Sprintf("%s-%d", slug, count)
		}
		anchors[slug] = true
	}
	return anchors
}

func headingSlug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// scanDocument extracts inline links/images and reference definitions outside
// fenced code blocks and inline code spans. HTML links and multiline inline
// link destinations are outside the supported subset; the help documents that
// limit rather than silently pretending to check them.
func scanDocument(body string) []documentLink {
	links := make([]documentLink, 0)
	fenceChar := byte(0)
	fenceLen := 0
	for index, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceChar != 0 {
			if isFenceClose(trimmed, fenceChar, fenceLen) {
				fenceChar = 0
				fenceLen = 0
			}
			continue
		}
		if char, length, ok := fenceOpen(trimmed); ok {
			fenceChar = char
			fenceLen = length
			continue
		}
		content := stripInlineCode(line)
		if match := referenceLinkStart.FindStringSubmatch(content); match != nil {
			links = append(links, documentLink{Line: index + 1, Target: match[1]})
		}
		links = append(links, inlineLinks(content, index+1)...)
	}
	return links
}

// markdownLines is scanDocument's non-fenced line view, used for anchors.
func markdownLines(body string) []string {
	lines := make([]string, 0)
	fenceChar := byte(0)
	fenceLen := 0
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceChar != 0 {
			if isFenceClose(trimmed, fenceChar, fenceLen) {
				fenceChar = 0
				fenceLen = 0
			}
			continue
		}
		if char, length, ok := fenceOpen(trimmed); ok {
			fenceChar = char
			fenceLen = length
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func fenceOpen(trimmed string) (byte, int, bool) {
	for _, char := range []byte{'`', '~'} {
		length := 0
		for length < len(trimmed) && trimmed[length] == char {
			length++
		}
		if length >= 3 {
			return char, length, true
		}
	}
	return 0, 0, false
}

func isFenceClose(trimmed string, char byte, length int) bool {
	count := 0
	for count < len(trimmed) && trimmed[count] == char {
		count++
	}
	if count < length {
		return false
	}
	return strings.TrimSpace(trimmed[count:]) == ""
}

// stripInlineCode removes `code` spans so link-shaped text inside them is not
// mistaken for a link.
func stripInlineCode(line string) string {
	var b strings.Builder
	for index := 0; index < len(line); {
		if line[index] != '`' {
			b.WriteByte(line[index])
			index++
			continue
		}
		run := 0
		for index+run < len(line) && line[index+run] == '`' {
			run++
		}
		closing := strings.Index(line[index+run:], strings.Repeat("`", run))
		if closing < 0 {
			b.WriteString(line[index:])
			break
		}
		index += run + closing + run
	}
	return b.String()
}

// inlineLinks extracts [text](target) and ![alt](target) destinations with
// balanced parentheses. Lines are the supported unit: a destination split
// across lines is not recognized.
func inlineLinks(line string, lineNumber int) []documentLink {
	links := make([]documentLink, 0)
	for index := 0; index < len(line); index++ {
		if line[index] != ']' || index+1 >= len(line) || line[index+1] != '(' {
			continue
		}
		target, end, ok := parseLinkDestination(line[index+2:])
		if !ok {
			continue
		}
		links = append(links, documentLink{Line: lineNumber, Target: target})
		index += 1 + end
	}
	return links
}

// parseLinkDestination parses a destination and optional title, returning the
// destination and the index just past the closing parenthesis.
func parseLinkDestination(rest string) (string, int, bool) {
	index := 0
	for index < len(rest) && (rest[index] == ' ' || rest[index] == '\t') {
		index++
	}
	if index >= len(rest) {
		return "", 0, false
	}
	destination := ""
	if rest[index] == '<' {
		close := strings.IndexByte(rest[index+1:], '>')
		if close < 0 {
			return "", 0, false
		}
		destination = rest[index+1 : index+1+close]
		index += close + 2
	} else {
		start := index
		depth := 0
		destinationSet := false
	scanDestination:
		for index < len(rest) {
			switch rest[index] {
			case '\\':
				if index+1 < len(rest) {
					index += 2
					continue
				}
				index++
			case '(':
				depth++
				index++
			case ')':
				if depth == 0 {
					destination = rest[start:index]
					return strings.TrimSpace(destination), index + 1, true
				}
				depth--
				index++
			case ' ', '\t':
				if depth == 0 {
					destination = rest[start:index]
					destinationSet = true
					break scanDestination
				}
				index++
			default:
				index++
			}
		}
		if !destinationSet {
			destination = rest[start:]
		}
	}
	for index < len(rest) && (rest[index] == ' ' || rest[index] == '\t') {
		index++
	}
	if index < len(rest) {
		switch rest[index] {
		case '"', '\'':
			quote := rest[index]
			close := strings.IndexByte(rest[index+1:], quote)
			if close < 0 {
				return "", 0, false
			}
			index += close + 2
		case '(':
			close := strings.IndexByte(rest[index+1:], ')')
			if close < 0 {
				return "", 0, false
			}
			index += close + 2
		}
	}
	for index < len(rest) && (rest[index] == ' ' || rest[index] == '\t') {
		index++
	}
	if index >= len(rest) || rest[index] != ')' {
		return "", 0, false
	}
	return strings.TrimSpace(destination), index + 1, true
}

// checkDeclaredCheckers runs each selected [[checkers]] entry with
// exec.Command: no shell, no interpolation, bounded timeout, and cwd confined
// to the resolved project root.
func checkDeclaredCheckers(rootDir string, cfg *Config) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, checker := range cfg.Checkers {
		diagnostics = append(diagnostics, runChecker(rootDir, cfg, checker)...)
	}
	return diagnostics
}

func runChecker(rootDir string, cfg *Config, checker Checker) []Diagnostic {
	dir := cfg.resolvedRoot(rootDir)
	if checker.Dir != "" {
		resolved, err := resolveConfinedPath(dir, checker.Dir)
		if err != nil {
			return []Diagnostic{{
				Rule:    RuleConfig,
				File:    ManifestName,
				Message: fmt.Sprintf("checker %q dir %q: %v", checker.Name, checker.Dir, err),
				Fix:     "use a checker dir inside the project root and do not symlink it outside",
			}}
		}
		dir = resolved
	}
	ctx, cancel := context.WithTimeout(context.Background(), checker.checkerTimeout())
	defer cancel()
	// #nosec G204 -- the argv array is the project's explicit declaration; it
	// is executed without a shell and never interpolated.
	cmd := exec.CommandContext(ctx, checker.Run[0], checker.Run[1:]...)
	cmd.Dir = dir
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return []Diagnostic{{
			Rule:    RuleChecker,
			File:    ManifestName,
			Message: fmt.Sprintf("checker %q timed out after %s", checker.Name, checker.checkerTimeout()),
			Fix:     "make the checker finish in time or raise timeoutSeconds (max 600)",
		}}
	}
	output := strings.TrimSpace(combined.String())
	if output == "" {
		output = err.Error()
	}
	diagnostics := make([]Diagnostic, 0)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		diagnostic := Diagnostic{
			Rule:    RuleChecker,
			File:    ManifestName,
			Message: fmt.Sprintf("checker %q: %s", checker.Name, line),
			Fix:     "fix the checker's reported diagnostics or its run/dir/timeoutSeconds entry",
		}
		if file, lineNumber, message, ok := parseCheckerDiagnostic(line); ok {
			diagnostic.File = file
			diagnostic.Line = lineNumber
			diagnostic.Message = fmt.Sprintf("checker %q: %s", checker.Name, message)
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Rule:    RuleChecker,
			File:    ManifestName,
			Message: fmt.Sprintf("checker %q failed: %s", checker.Name, err),
			Fix:     "fix the checker's reported diagnostics",
		})
	}
	slices.SortStableFunc(diagnostics, func(a, b Diagnostic) int {
		if order := strings.Compare(a.File, b.File); order != 0 {
			return order
		}
		return a.Line - b.Line
	})
	return diagnostics
}

// parseCheckerDiagnostic recognizes the Go-style "path:line: message" shape so
// checker findings carry a file and line in verify output.
func parseCheckerDiagnostic(line string) (string, int, string, bool) {
	first := strings.Index(line, ":")
	if first <= 0 {
		return "", 0, "", false
	}
	rest := line[first+1:]
	second := strings.Index(rest, ":")
	if second <= 0 {
		return "", 0, "", false
	}
	lineNumber := 0
	for _, digit := range rest[:second] {
		if digit < '0' || digit > '9' {
			return "", 0, "", false
		}
		lineNumber = lineNumber*10 + int(digit-'0')
	}
	if lineNumber == 0 {
		return "", 0, "", false
	}
	message := strings.TrimSpace(rest[second+1:])
	if message == "" {
		return "", 0, "", false
	}
	return line[:first], lineNumber, message, true
}
