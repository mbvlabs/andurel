package cli

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// routeUsageAnalysis checks frontend modules for full-URL route-helper calls
// that run during module initialization. Generated routes.ts resolves a
// helper's host from configureRouteHosts at call time, so a call evaluated
// while the module initializes freezes an empty or stale origin and can leak it
// across SSR requests. Calls at render/event/function time and explicit
// `.path()` relative helpers are allowed.
//
// The analysis is token-based rather than a TypeScript program: it has no type
// checker and resolves only relative ESM imports. It recognizes the generated
// `routes.<helper>(...)` shape, namespace and named imports, module-scope
// object aliases and destructuring, and one level of locally declared wrapper
// functions. Dynamic property access, tsconfig path aliases, re-exports, and
// cross-module wrappers are outside this analysis; the Inertia runtime's
// request-scoped host context is the authoritative safety net for those cases.
// `andurel verify --help` documents these limits.
func analyzeRouteUsage(rootDir string) (violations []verifyViolation, skippedReason string, applicable bool) {
	routesPath := generatedRoutesJSPath
	frontendRoot := filepath.Join(rootDir, "resources", "js")
	if _, err := os.Stat(frontendRoot); err != nil {
		return nil, "resources/js is missing; there is no frontend module tree to analyze", false
	}
	if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(routesPath))); err != nil {
		return nil, "resources/js/routes.ts is missing; run andurel sync routes before the route-usage check", true
	}
	helpers, err := generatedRouteHelperNames(rootDir, routesPath)
	if err != nil || len(helpers) == 0 {
		return nil, "resources/js/routes.ts has no generated route helpers; run andurel sync routes", true
	}
	violations = make([]verifyViolation, 0)
	walkErr := filepath.WalkDir(frontendRoot, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, current)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == routesPath || !isTypeScriptSource(rel) {
			return nil
		}
		body, readErr := os.ReadFile(current)
		if readErr != nil {
			return readErr
		}
		violations = append(
			violations,
			analyzeRouteUsageFile(rel, string(body), routesPath, helpers)...,
		)
		return nil
	})
	if walkErr != nil {
		return nil, "scan resources/js: " + walkErr.Error(), true
	}
	slices.SortFunc(violations, func(a, b verifyViolation) int {
		if order := strings.Compare(a.File, b.File); order != 0 {
			return order
		}
		return a.Line - b.Line
	})
	return violations, "", true
}

var generatedRouteHelperPattern = regexp.MustCompile(`^\s{2}([A-Za-z_$][A-Za-z0-9_$]*): routeHelper\(\{`)

// generatedRouteHelperNames reads the helper names out of the generated
// routes.ts. Consuming the generated file keeps this rule in step with the
// route generator instead of hard-coding route names here.
func generatedRouteHelperNames(rootDir, routesPath string) (map[string]bool, error) {
	body, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(routesPath)))
	if err != nil {
		return nil, err
	}
	helpers := map[string]bool{}
	for line := range strings.SplitSeq(string(body), "\n") {
		if match := generatedRouteHelperPattern.FindStringSubmatch(line); match != nil {
			helpers[match[1]] = true
		}
	}
	return helpers, nil
}

func isTypeScriptSource(rel string) bool {
	if strings.HasSuffix(rel, ".d.ts") {
		return false
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// analyzeRouteUsageFile checks one module and returns its
// module-initialization violations.
func analyzeRouteUsageFile(
	file, body, routesPath string,
	helpers map[string]bool,
) []verifyViolation {
	tokens := lexTypeScript(body)
	namespaces, importedHelpers := routeModuleBindings(file, tokens, routesPath, helpers)
	wrappers := moduleWrapperFunctions(tokens, namespaces, importedHelpers, helpers)
	usages := moduleInitializationUsages(tokens, namespaces, importedHelpers, wrappers, helpers)
	violations := make([]verifyViolation, 0, len(usages))
	for _, usage := range usages {
		violations = append(violations, verifyViolation{
			Rule: "frontend-route-usage",
			File: file,
			Line: usage.line,
			Message: fmt.Sprintf(
				"%s runs the full-URL route helper at module initialization; configureRouteHosts runs after boot, so the URL keeps an empty or stale origin (including across SSR requests)",
				usage.description,
			),
			Fix: "call the helper while rendering, in an event handler, or inside a function; use " +
				usage.pathExpression + " for a relative URL",
		})
	}
	return violations
}

// tsTokenKind is the small token vocabulary the route-usage scan needs.
type tsTokenKind int

const (
	tsIdent tsTokenKind = iota
	tsString
	tsNumber
	tsPunct
)

type tsToken struct {
	Kind tsTokenKind
	Text string
	Line int
}

// lexTypeScript tokenizes enough TypeScript/JSX to find identifiers, calls,
// strings, and brace structure. Comments and regex literals are skipped.
// Template interpolations are lexed as code so `${routes.foo()}` is visible.
func lexTypeScript(src string) []tsToken {
	return lexTypeScriptFrom(src, 1)
}

func lexTypeScriptFrom(src string, baseLine int) []tsToken {
	tokens := make([]tsToken, 0)
	line := baseLine
	for index := 0; index < len(src); {
		char := src[index]
		switch {
		case char == '\n':
			line++
			index++
		case char == ' ' || char == '\t' || char == '\r':
			index++
		case char == '/' && index+1 < len(src) && src[index+1] == '/':
			for index < len(src) && src[index] != '\n' {
				index++
			}
		case char == '/' && index+1 < len(src) && src[index+1] == '*':
			index += 2
			for index+1 < len(src) && !(src[index] == '*' && src[index+1] == '/') {
				if src[index] == '\n' {
					line++
				}
				index++
			}
			index += 2
		case char == '\'' || char == '"':
			startLine := line
			quote := char
			start := index + 1
			index++
			for index < len(src) {
				if src[index] == '\\' {
					index += 2
					continue
				}
				if src[index] == quote {
					break
				}
				if src[index] == '\n' {
					line++
				}
				index++
			}
			text := src[start:index]
			if index < len(src) {
				index++
			}
			tokens = append(tokens, tsToken{Kind: tsString, Text: text, Line: startLine})
		case char == '`':
			startLine := line
			index++
			var template strings.Builder
			for index < len(src) {
				if src[index] == '\\' {
					template.WriteByte(src[index])
					if index+1 < len(src) {
						template.WriteByte(src[index+1])
					}
					index += 2
					continue
				}
				if src[index] == '`' {
					index++
					break
				}
				if src[index] == '\n' {
					line++
				}
				template.WriteByte(src[index])
				index++
			}
			for _, interpolation := range templateInterpolations(template.String()) {
				tokens = append(tokens, lexTypeScriptFrom(interpolation, startLine)...)
			}
			tokens = append(tokens, tsToken{Kind: tsString, Text: template.String(), Line: startLine})
		case isIdentifierStart(char):
			start := index
			startLine := line
			for index < len(src) && isIdentifierPart(src[index]) {
				index++
			}
			tokens = append(tokens, tsToken{Kind: tsIdent, Text: src[start:index], Line: startLine})
		case char >= '0' && char <= '9':
			start := index
			for index < len(src) {
				if isIdentifierPart(src[index]) || src[index] == '.' {
					index++
					continue
				}
				break
			}
			tokens = append(tokens, tsToken{Kind: tsNumber, Text: src[start:index], Line: line})
		case char == '/' && regexCanStart(tokens):
			startLine := line
			index++
			inClass := false
			for index < len(src) {
				switch {
				case src[index] == '\\':
					index += 2
					continue
				case src[index] == '[':
					inClass = true
				case src[index] == ']':
					inClass = false
				case src[index] == '/' && !inClass:
					index++
					goto regexDone
				case src[index] == '\n':
					line++
				}
				index++
			}
		regexDone:
			tokens = append(tokens, tsToken{Kind: tsString, Line: startLine})
		default:
			text := string(char)
			if index+1 < len(src) {
				switch two := src[index : index+2]; two {
				case "=>", "==", "!=", "?.", "??", "&&", "||", "<=", ">=", "++", "--", "+=", "-=", "**":
					text = two
					index++
				}
			}
			tokens = append(tokens, tsToken{Kind: tsPunct, Text: text, Line: line})
			index++
		}
	}
	return tokens
}

func isIdentifierStart(char byte) bool {
	return char == '_' || char == '$' ||
		char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func isIdentifierPart(char byte) bool {
	return isIdentifierStart(char) || char >= '0' && char <= '9'
}

// regexCanStart is the usual heuristic: a slash starts a regex literal unless
// the previous token ends an expression.
func regexCanStart(tokens []tsToken) bool {
	if len(tokens) == 0 {
		return true
	}
	last := tokens[len(tokens)-1]
	if last.Kind == tsPunct {
		switch last.Text {
		case ")", "]", "}":
			return false
		}
		return true
	}
	if last.Kind == tsIdent {
		switch last.Text {
		case "return", "typeof", "instanceof", "in", "of", "new", "delete", "void", "case":
			return true
		}
		return false
	}
	return false
}

// templateInterpolations extracts the code inside ${...} segments so
// module-scope calls inside a template literal are still analyzed.
func templateInterpolations(text string) []string {
	interpolations := make([]string, 0)
	for index := 0; index+1 < len(text); index++ {
		if text[index] != '$' || text[index+1] != '{' {
			continue
		}
		depth := 1
		start := index + 2
		cursor := start
		for cursor < len(text) && depth > 0 {
			switch text[cursor] {
			case '{':
				depth++
			case '}':
				depth--
			case '\'', '"', '`':
				quote := text[cursor]
				cursor++
				for cursor < len(text) {
					if text[cursor] == '\\' {
						cursor += 2
						continue
					}
					if text[cursor] == quote {
						break
					}
					cursor++
				}
			}
			cursor++
		}
		if depth == 0 {
			interpolations = append(interpolations, text[start:cursor-1])
			index = cursor - 1
		}
	}
	return interpolations
}

// routeModuleBindings collects local names bound to the generated routes
// module and its helpers. Only relative ESM imports are resolved; tsconfig
// path aliases are documented as unsupported.
func routeModuleBindings(
	file string,
	tokens []tsToken,
	routesPath string,
	helpers map[string]bool,
) (map[string]bool, map[string]bool) {
	namespaces := map[string]bool{}
	importedHelpers := map[string]bool{}
	depth := 0
	for index := 0; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "{", "(", "[":
			depth++
			continue
		case "}", ")", "]":
			depth--
			continue
		}
		if depth != 0 || tokens[index].Text != "import" {
			continue
		}
		clause := make([]tsToken, 0)
		specifier := ""
		for index++; index < len(tokens); index++ {
			if tokens[index].Text == ";" {
				break
			}
			if tokens[index].Text == "from" && index+1 < len(tokens) &&
				tokens[index+1].Kind == tsString {
				specifier = tokens[index+1].Text
				index++
				break
			}
			clause = append(clause, tokens[index])
		}
		// String tokens carry no text; re-read the raw specifier is impossible
		// here, so lexer keeps string text for import specifiers.
		if specifier == "" || !specifierResolvesTo(file, specifier, routesPath) {
			continue
		}
		for position := 0; position < len(clause); position++ {
			switch {
			case clause[position].Text == "*" && position+2 < len(clause) &&
				clause[position+1].Text == "as" && clause[position+2].Kind == tsIdent:
				namespaces[clause[position+2].Text] = true
				position += 2
			case clause[position].Text == "{":
				for position++; position < len(clause) && clause[position].Text != "}"; position++ {
					if clause[position].Kind != tsIdent {
						continue
					}
					importedName := clause[position].Text
					local := importedName
					if position+2 < len(clause) && clause[position+1].Text == "as" &&
						clause[position+2].Kind == tsIdent {
						local = clause[position+2].Text
						position += 2
					}
					switch {
					case importedName == "routes":
						namespaces[local] = true
					case helpers[importedName]:
						importedHelpers[local] = true
					}
				}
			}
		}
	}
	return namespaces, importedHelpers
}

// specifierResolvesTo resolves a relative ESM specifier to the generated
// routes.ts path. Bare package specifiers and tsconfig aliases return false.
func specifierResolvesTo(importer, specifier, routesPath string) bool {
	if !strings.HasPrefix(specifier, ".") {
		return false
	}
	base := path.Clean(path.Join(path.Dir(importer), specifier))
	candidates := []string{
		base,
		base + ".ts",
		base + ".tsx",
		base + "/index.ts",
		base + "/index.tsx",
	}
	if before, ok := strings.CutSuffix(base, ".js"); ok {
		candidates = append(candidates, before+".ts")
	}
	return slices.Contains(candidates, routesPath)
}

type moduleUsage struct {
	line           int
	description    string
	pathExpression string
}

// moduleWrapperFunctions records module-scope functions that call a full-URL
// helper, so a module-scope call to that wrapper is still recognized.
func moduleWrapperFunctions(
	tokens []tsToken,
	namespaces, importedHelpers, helpers map[string]bool,
) map[string]bool {
	wrappers := map[string]bool{}
	depth := 0
	for index := 0; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "{", "(", "[":
			depth++
			continue
		case "}", ")", "]":
			depth--
			continue
		}
		if depth != 0 {
			continue
		}
		if tokens[index].Text != "function" {
			continue
		}
		nameIndex := index + 1
		if nameIndex < len(tokens) && tokens[nameIndex].Text == "*" {
			nameIndex++
		}
		if nameIndex >= len(tokens) || tokens[nameIndex].Kind != tsIdent {
			continue
		}
		name := tokens[nameIndex].Text
		open := nameIndex + 1
		for open < len(tokens) && tokens[open].Text != "(" {
			open++
		}
		if open >= len(tokens) {
			continue
		}
		closeParams := matchingToken(tokens, open)
		if closeParams < 0 {
			continue
		}
		bodyStart := closeParams + 1
		for bodyStart < len(tokens) && tokens[bodyStart].Text != "{" {
			bodyStart++
		}
		if bodyStart >= len(tokens) {
			continue
		}
		bodyEnd := matchingToken(tokens, bodyStart)
		if bodyEnd < 0 {
			continue
		}
		if containsFullHelperCall(tokens[bodyStart+1:bodyEnd], namespaces, importedHelpers, helpers) {
			wrappers[name] = true
		}
		index = bodyEnd
	}
	return wrappers
}

// moduleInitializationUsages walks top-level declarations and flags full-URL
// helper calls evaluated by the initializer. Function bodies are deferred and
// skipped; a deferred arrow initializer only marks a wrapper.
func moduleInitializationUsages(
	tokens []tsToken,
	namespaces, importedHelpers, wrappers, helpers map[string]bool,
) []moduleUsage {
	usages := make([]moduleUsage, 0)
	depth := 0
	for index := 0; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "{", "(", "[":
			depth++
			continue
		case "}", ")", "]":
			depth--
			continue
		}
		if depth != 0 {
			continue
		}
		switch tokens[index].Text {
		case "const", "let", "var":
			index = analyzeModuleDeclarators(
				tokens, index, namespaces, importedHelpers, wrappers, helpers, &usages,
			)
		case "function":
			open := index
			for open < len(tokens) && tokens[open].Text != "{" {
				open++
			}
			if open < len(tokens) {
				if close := matchingToken(tokens, open); close >= 0 {
					index = close
				}
			}
		}
	}
	return usages
}

// analyzeModuleDeclarators analyzes each declarator in a const/let/var
// statement and returns the index of its last token.
func analyzeModuleDeclarators(
	tokens []tsToken,
	start int,
	namespaces, importedHelpers, wrappers, helpers map[string]bool,
	usages *[]moduleUsage,
) int {
	index := start + 1
	for index < len(tokens) {
		// const { homePage } = routes
		if tokens[index].Text == "{" {
			close := matchingToken(tokens, index)
			if close < 0 {
				return len(tokens) - 1
			}
			if close+2 < len(tokens) && tokens[close+1].Text == "=" &&
				tokens[close+2].Kind == tsIdent && namespaces[tokens[close+2].Text] {
				for _, name := range destructuredRouteNames(tokens[index+1:close], helpers) {
					importedHelpers[name] = true
				}
			}
			return initializerStatementEnd(tokens, close+1)
		}
		if tokens[index].Kind != tsIdent {
			return index
		}
		name := tokens[index].Text
		equals := findTopLevelToken(tokens, index+1, "=")
		if equals < 0 {
			return index
		}
		initStart := equals + 1
		initEnd := initializerEnd(tokens, initStart)
		initializer := tokens[initStart:initEnd]
		if len(initializer) == 1 && initializer[0].Kind == tsIdent && namespaces[initializer[0].Text] {
			namespaces[name] = true
		}
		if body, isArrow := arrowInitializerBody(initializer, namespaces, importedHelpers, helpers); isArrow {
			if containsFullHelperCall(body, namespaces, importedHelpers, helpers) {
				wrappers[name] = true
			}
		} else if len(initializer) > 0 && initializer[0].Text == "function" {
			if containsFullHelperCall(initializer, namespaces, importedHelpers, helpers) {
				wrappers[name] = true
			}
		} else {
			*usages = append(*usages, initializerUsages(
				initializer, namespaces, importedHelpers, wrappers, helpers,
			)...)
		}
		index = initEnd
		if index < len(tokens) && tokens[index].Text == "," {
			index++
			continue
		}
		if index < len(tokens) && tokens[index].Text == ";" {
			return index
		}
		return index - 1
	}
	return len(tokens) - 1
}

func destructuredRouteNames(tokens []tsToken, helpers map[string]bool) []string {
	names := make([]string, 0)
	for index := 0; index < len(tokens); index++ {
		if tokens[index].Kind != tsIdent {
			continue
		}
		local := tokens[index].Text
		if index+2 < len(tokens) && tokens[index+1].Text == ":" &&
			tokens[index+2].Kind == tsIdent {
			local = tokens[index+2].Text
			index += 2
		}
		if helpers[local] {
			names = append(names, local)
		}
	}
	return names
}

// initializerEnd finds the end of one declarator initializer: a comma or
// semicolon at the initializer's base depth, or a new top-level statement.
func initializerEnd(tokens []tsToken, start int) int {
	parens, brackets, braces := 0, 0, 0
	startLine := 0
	if start < len(tokens) {
		startLine = tokens[start].Line
	}
	for index := start; index < len(tokens); index++ {
		token := tokens[index]
		switch token.Text {
		case "(":
			parens++
		case ")":
			if parens > 0 {
				parens--
			}
		case "[":
			brackets++
		case "]":
			if brackets > 0 {
				brackets--
			}
		case "{":
			braces++
		case "}":
			if braces > 0 {
				braces--
			}
		case ",", ";":
			if parens == 0 && brackets == 0 && braces == 0 {
				return index
			}
		case "const", "let", "var", "function", "class", "import", "export", "interface", "type":
			if parens == 0 && brackets == 0 && braces == 0 && token.Line > startLine {
				return index
			}
		}
	}
	return len(tokens)
}

func initializerStatementEnd(tokens []tsToken, start int) int {
	end := initializerEnd(tokens, start)
	if end < len(tokens) && tokens[end].Text == "," {
		end++
	}
	return end
}

// arrowInitializerBody reports whether the initializer is a deferred arrow
// function and returns its body tokens. A bare arrow is treated as deferred:
// its body runs when the function is called, not at module initialization.
func arrowInitializerBody(
	initializer []tsToken,
	namespaces, importedHelpers, helpers map[string]bool,
) ([]tsToken, bool) {
	if len(initializer) == 0 {
		return nil, false
	}
	arrow := findTopLevelToken(initializer, 0, "=>")
	if arrow < 0 {
		return nil, false
	}
	return initializer[arrow+1:], true
}

// containsFullHelperCall reports whether a token slice calls a full-URL helper
// directly. It does not recurse into nested function bodies.
func containsFullHelperCall(
	tokens []tsToken,
	namespaces, importedHelpers, helpers map[string]bool,
) bool {
	for index := 0; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "function":
			open := index
			for open < len(tokens) && tokens[open].Text != "{" {
				open++
			}
			if open < len(tokens) {
				if close := matchingToken(tokens, open); close >= 0 {
					index = close
				}
			}
			continue
		case "=>":
			index = skipArrowBody(tokens, index)
			continue
		}
		if helperCallAt(tokens, index, namespaces, importedHelpers, helpers) {
			return true
		}
	}
	return false
}

// initializerUsages finds full-URL helper calls executed while the module
// initializes. Nested function bodies and arrow bodies are deferred and
// skipped.
func initializerUsages(
	tokens []tsToken,
	namespaces, importedHelpers, wrappers, helpers map[string]bool,
) []moduleUsage {
	usages := make([]moduleUsage, 0)
	for index := 0; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "function":
			open := index
			for open < len(tokens) && tokens[open].Text != "{" {
				open++
			}
			if open < len(tokens) {
				if close := matchingToken(tokens, open); close >= 0 {
					index = close
				}
			}
			continue
		case "=>":
			index = skipArrowBody(tokens, index)
			continue
		}
		if usage, ok := helperUsageAt(tokens, index, namespaces, importedHelpers, wrappers, helpers); ok {
			usages = append(usages, usage)
		}
	}
	return usages
}

func helperCallAt(
	tokens []tsToken,
	index int,
	namespaces, importedHelpers, helpers map[string]bool,
) bool {
	_, ok := helperUsageAt(tokens, index, namespaces, importedHelpers, nil, helpers)
	return ok
}

// helperUsageAt recognizes a full-URL helper call or a module-scope wrapper
// call at index. It intentionally ignores `.path` and `.host` property access.
func helperUsageAt(
	tokens []tsToken,
	index int,
	namespaces, importedHelpers, wrappers, helpers map[string]bool,
) (moduleUsage, bool) {
	if tokens[index].Kind != tsIdent {
		return moduleUsage{}, false
	}
	name := tokens[index].Text
	if namespaces[name] && index+3 < len(tokens) && tokens[index+1].Text == "." &&
		tokens[index+2].Kind == tsIdent && helpers[tokens[index+2].Text] {
		helper := tokens[index+2].Text
		if tokens[index+3].Text == "." && index+4 < len(tokens) &&
			(tokens[index+4].Text == "path" || tokens[index+4].Text == "host") {
			return moduleUsage{}, false
		}
		if tokens[index+3].Text == "(" {
			return moduleUsage{
				line:           tokens[index].Line,
				description:    name + "." + helper + "(...)",
				pathExpression: name + "." + helper + ".path(...)",
			}, true
		}
		return moduleUsage{}, false
	}
	if importedHelpers[name] && index+1 < len(tokens) && tokens[index+1].Text == "(" {
		return moduleUsage{
			line:           tokens[index].Line,
			description:    name + "(...)",
			pathExpression: name + ".path(...)",
		}, true
	}
	if wrappers[name] && index+1 < len(tokens) && tokens[index+1].Text == "(" {
		return moduleUsage{
			line:           tokens[index].Line,
			description:    name + "(...) (a module-scope wrapper around a full-URL route helper)",
			pathExpression: "routes.<helper>.path(...)",
		}, true
	}
	return moduleUsage{}, false
}

// skipArrowBody returns the last token of a deferred arrow body. Block bodies
// end at their closing brace; expression bodies end at the enclosing
// comma/brace/bracket boundary.
func skipArrowBody(tokens []tsToken, arrow int) int {
	start := arrow + 1
	if start >= len(tokens) {
		return arrow
	}
	if tokens[start].Text == "{" {
		if close := matchingToken(tokens, start); close >= 0 {
			return close
		}
		return len(tokens) - 1
	}
	parens, brackets, braces := 0, 0, 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].Text {
		case "(":
			parens++
		case ")":
			if parens == 0 && brackets == 0 && braces == 0 {
				return index - 1
			}
			parens--
		case "[":
			brackets++
		case "]":
			if brackets > 0 {
				brackets--
			}
		case "{":
			braces++
		case "}":
			if braces == 0 && parens == 0 && brackets == 0 {
				return index - 1
			}
			braces--
		case ",", ";":
			if parens == 0 && brackets == 0 && braces == 0 {
				return index - 1
			}
		}
	}
	return len(tokens) - 1
}

// matchingToken returns the index of the closing bracket matching the opening
// token at open, or -1.
func matchingToken(tokens []tsToken, open int) int {
	if open < 0 || open >= len(tokens) {
		return -1
	}
	var closeText string
	switch tokens[open].Text {
	case "(":
		closeText = ")"
	case "[":
		closeText = "]"
	case "{":
		closeText = "}"
	default:
		return -1
	}
	depth := 0
	for index := open; index < len(tokens); index++ {
		switch tokens[index].Text {
		case tokens[open].Text:
			depth++
		case closeText:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// findTopLevelToken returns the first index at or after start whose text
// matches want at zero bracket depth.
func findTopLevelToken(tokens []tsToken, start int, want string) int {
	parens, brackets, braces := 0, 0, 0
	for index := start; index < len(tokens); index++ {
		token := tokens[index]
		if parens == 0 && brackets == 0 && braces == 0 && token.Text == want {
			return index
		}
		switch token.Text {
		case "(":
			parens++
		case ")":
			if parens > 0 {
				parens--
			}
		case "[":
			brackets++
		case "]":
			if brackets > 0 {
				brackets--
			}
		case "{":
			braces++
		case "}":
			if braces > 0 {
				braces--
			}
		}
	}
	return -1
}
