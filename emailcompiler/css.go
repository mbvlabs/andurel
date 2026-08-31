package emailcompiler

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type declaration struct {
	name      string
	value     string
	important bool
	order     int
}

type utilityRule struct {
	className    string
	declarations []declaration
	order        int
}

type compiledStylesheet struct {
	variables map[string]string
	rules     map[string]utilityRule
}

func parseStylesheet(css string) (*compiledStylesheet, error) {
	stylesheet := &compiledStylesheet{
		variables: make(map[string]string),
		rules:     make(map[string]utilityRule),
	}
	order := 0
	if err := parseCSSBlocks(css, stylesheet, &order); err != nil {
		return nil, err
	}
	return stylesheet, nil
}

func parseCSSBlocks(css string, stylesheet *compiledStylesheet, order *int) error {
	for offset := 0; offset < len(css); {
		offset = skipCSSSpaceAndComments(css, offset)
		if offset >= len(css) {
			return nil
		}

		preludeStart := offset
		terminator, next, err := findCSSPreludeTerminator(css, offset)
		if err != nil {
			return err
		}
		prelude := strings.TrimSpace(css[preludeStart:terminator])
		if next == ';' {
			offset = terminator + 1
			continue
		}

		blockEnd, err := findMatchingBrace(css, terminator)
		if err != nil {
			return fmt.Errorf("%s: %w", prelude, err)
		}
		block := css[terminator+1 : blockEnd]
		offset = blockEnd + 1

		if strings.HasPrefix(prelude, "@") {
			if after, ok := strings.CutPrefix(prelude, "@property"); ok {
				propertyName := strings.TrimSpace(after)
				declarations, declarationOnly, err := parseDeclarationBlock(block, *order)
				if err != nil {
					return fmt.Errorf("%s: %w", prelude, err)
				}
				if declarationOnly {
					for _, item := range declarations {
						if item.name == "initial-value" {
							stylesheet.variables[propertyName] = item.value
						}
					}
				}
				continue
			}
			if err := parseCSSBlocks(block, stylesheet, order); err != nil {
				return err
			}
			continue
		}

		declarations, declarationOnly, err := parseDeclarationBlock(block, *order)
		if err != nil {
			return fmt.Errorf("selector %s: %w", prelude, err)
		}
		if !declarationOnly {
			continue
		}
		*order = *order + 1

		if selectorDefinesVariables(prelude) {
			for _, declaration := range declarations {
				if strings.HasPrefix(declaration.name, "--") {
					stylesheet.variables[declaration.name] = declaration.value
				}
			}
		}

		for _, selector := range splitCSSList(prelude) {
			className, ok := classFromSimpleSelector(selector)
			if !ok {
				continue
			}
			stylesheet.rules[className] = utilityRule{
				className:    className,
				declarations: declarations,
				order:        *order,
			}
		}
	}
	return nil
}

func skipCSSSpaceAndComments(css string, offset int) int {
	for offset < len(css) {
		if offset+1 < len(css) && css[offset:offset+2] == "/*" {
			end := strings.Index(css[offset+2:], "*/")
			if end < 0 {
				return len(css)
			}
			offset += end + 4
			continue
		}
		if css[offset] == ' ' || css[offset] == '\n' || css[offset] == '\r' || css[offset] == '\t' {
			offset++
			continue
		}
		return offset
	}
	return offset
}

func findCSSPreludeTerminator(css string, offset int) (index int, terminator byte, err error) {
	var quote byte
	parenDepth := 0
	for i := offset; i < len(css); i++ {
		char := css[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{', ';':
			if parenDepth == 0 {
				return i, char, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("unterminated CSS prelude at byte %d", offset)
}

func findMatchingBrace(css string, open int) (int, error) {
	depth := 0
	var quote byte
	for i := open; i < len(css); i++ {
		char := css[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated CSS block at byte %d", open)
}

func parseDeclarationBlock(block string, order int) ([]declaration, bool, error) {
	if containsTopLevelBrace(block) {
		return nil, false, nil
	}
	parts := splitCSSTopLevel(block, ';')
	declarations := make([]declaration, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colon := findTopLevelCharacter(part, ':')
		if colon < 0 {
			return nil, false, fmt.Errorf("invalid declaration %q", part)
		}
		name := strings.TrimSpace(part[:colon])
		value := strings.TrimSpace(part[colon+1:])
		important := false
		if strings.HasSuffix(strings.ToLower(value), "!important") {
			important = true
			value = strings.TrimSpace(value[:len(value)-len("!important")])
		}
		declarations = append(declarations, declaration{
			name:      name,
			value:     value,
			important: important,
			order:     order,
		})
	}
	return declarations, true, nil
}

func containsTopLevelBrace(input string) bool {
	return findTopLevelCharacter(input, '{') >= 0
}

func findTopLevelCharacter(input string, target byte) int {
	var quote byte
	parenDepth := 0
	bracketDepth := 0
	for i := 0; i < len(input); i++ {
		char := input[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\\' && i+1 < len(input) {
			i++
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		default:
			if char == target && parenDepth == 0 && bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitCSSTopLevel(input string, separator byte) []string {
	var parts []string
	start := 0
	var quote byte
	parenDepth := 0
	bracketDepth := 0
	for i := 0; i < len(input); i++ {
		char := input[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\\' && i+1 < len(input) {
			i++
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		default:
			if char == separator && parenDepth == 0 && bracketDepth == 0 {
				parts = append(parts, input[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, input[start:])
}

func splitCSSList(input string) []string {
	return splitCSSTopLevel(input, ',')
}

func selectorDefinesVariables(selector string) bool {
	for _, part := range splitCSSList(selector) {
		part = strings.TrimSpace(part)
		if part == ":root" || part == ":host" {
			return true
		}
	}
	return false
}

func classFromSimpleSelector(selector string) (string, bool) {
	selector = strings.TrimSpace(selector)
	if !strings.HasPrefix(selector, ".") {
		return "", false
	}
	encoded := selector[1:]
	if encoded == "" {
		return "", false
	}
	className, err := unescapeCSSIdentifier(encoded)
	if err != nil {
		return "", false
	}
	return className, true
}

func unescapeCSSIdentifier(input string) (string, error) {
	var output strings.Builder
	for i := 0; i < len(input); {
		if input[i] != '\\' {
			output.WriteByte(input[i])
			i++
			continue
		}
		i++
		if i >= len(input) {
			return "", errorsNew("trailing CSS escape")
		}
		start := i
		for i < len(input) && i-start < 6 && isHex(input[i]) {
			i++
		}
		if i > start {
			value, err := strconv.ParseInt(input[start:i], 16, 32)
			if err != nil || !utf8.ValidRune(rune(value)) {
				return "", errorsNew("invalid CSS escape")
			}
			output.WriteRune(rune(value))
			if i < len(input) && isCSSSpace(input[i]) {
				i++
			}
			continue
		}
		output.WriteByte(input[i])
		i++
	}
	return output.String(), nil
}

func errorsNew(message string) error {
	return fmt.Errorf("%s", message)
}

func isHex(char byte) bool {
	return char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F'
}

func isCSSSpace(char byte) bool {
	return char == ' ' || char == '\n' || char == '\r' || char == '\t' || char == '\f'
}

func splitVariants(className string) ([]string, string, error) {
	var parts []string
	start := 0
	bracketDepth := 0
	parenDepth := 0
	var quote byte
	for i := 0; i < len(className); i++ {
		char := className[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case ':':
			if bracketDepth == 0 && parenDepth == 0 {
				parts = append(parts, className[start:i])
				start = i + 1
			}
		}
	}
	if bracketDepth != 0 || parenDepth != 0 || quote != 0 {
		return nil, "", fmt.Errorf("invalid Tailwind class %q", className)
	}
	base := className[start:]
	if base == "" {
		return nil, "", fmt.Errorf("invalid Tailwind class %q", className)
	}
	return parts, base, nil
}

func resolveDeclarations(
	declarations []declaration,
	globalVariables map[string]string,
) ([]declaration, error) {
	variables := make(map[string]string, len(globalVariables)+len(declarations))
	maps.Copy(variables, globalVariables)
	for _, item := range declarations {
		if strings.HasPrefix(item.name, "--") {
			variables[item.name] = item.value
		}
	}

	resolved := make([]declaration, 0, len(declarations))
	for _, item := range declarations {
		if strings.HasPrefix(item.name, "--") {
			continue
		}
		value, err := resolveCSSValue(item.value, variables, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", item.name, err)
		}
		resolved = append(resolved, declaration{
			name:      item.name,
			value:     normalizeEmailValue(value),
			important: item.important,
			order:     item.order,
		})
	}
	return resolved, nil
}

func resolveCSSValue(
	value string,
	variables map[string]string,
	stack map[string]bool,
) (string, error) {
	if stack == nil {
		stack = make(map[string]bool)
	}
	for {
		start := strings.Index(value, "var(")
		if start < 0 {
			break
		}
		end, err := matchingParen(value, start+3)
		if err != nil {
			return "", err
		}
		content := value[start+4 : end]
		parts := splitCSSTopLevel(content, ',')
		name := strings.TrimSpace(parts[0])
		replacement, ok := variables[name]
		if ok && replacement != "initial" {
			if stack[name] {
				return "", fmt.Errorf("cyclic CSS variable %s", name)
			}
			stack[name] = true
			replacement, err = resolveCSSValue(replacement, variables, stack)
			delete(stack, name)
			if err != nil {
				return "", err
			}
		} else if len(parts) > 1 {
			replacement = strings.TrimSpace(strings.Join(parts[1:], ","))
			replacement, err = resolveCSSValue(replacement, variables, stack)
			if err != nil {
				return "", err
			}
		} else {
			return "", fmt.Errorf("unresolved CSS variable %s", name)
		}
		value = value[:start] + replacement + value[end+1:]
	}
	return simplifyCalc(value), nil
}

func matchingParen(value string, open int) (int, error) {
	depth := 0
	var quote byte
	for i := open; i < len(value); i++ {
		char := value[i]
		if quote != 0 {
			if char == '\\' {
				i++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errorsNew("unterminated CSS function")
}

var simpleCalcPattern = regexp.MustCompile(
	`calc\(\s*(-?[0-9]*\.?[0-9]+)([a-zA-Z%]*)\s*([*/])\s*(-?[0-9]*\.?[0-9]+)([a-zA-Z%]*)\s*\)`,
)

func simplifyCalc(value string) string {
	for {
		match := simpleCalcPattern.FindStringSubmatchIndex(value)
		if match == nil {
			return value
		}
		left, _ := strconv.ParseFloat(value[match[2]:match[3]], 64)
		leftUnit := value[match[4]:match[5]]
		operator := value[match[6]:match[7]]
		right, _ := strconv.ParseFloat(value[match[8]:match[9]], 64)
		rightUnit := value[match[10]:match[11]]
		unit := leftUnit
		var result float64
		if operator == "*" {
			result = left * right
			if unit == "" {
				unit = rightUnit
			}
		} else {
			if right == 0 || rightUnit != "" {
				return value
			}
			result = left / right
		}
		replacement := formatCSSNumber(result) + unit
		value = value[:match[0]] + replacement + value[match[1]:]
	}
}

var remPattern = regexp.MustCompile(`(-?[0-9]*\.?[0-9]+)rem\b`)

var oklchPattern = regexp.MustCompile(
	`oklch\(\s*([0-9]*\.?[0-9]+)%?\s+([0-9]*\.?[0-9]+)\s+([0-9]*\.?[0-9]+)(?:\s*/\s*([0-9]*\.?[0-9]+)%?)?\s*\)`,
)

var shortHexPattern = regexp.MustCompile(
	`(?i)(^|[^#a-z0-9])#([0-9a-f])([0-9a-f])([0-9a-f])([^0-9a-f]|$)`,
)

func normalizeEmailValue(value string) string {
	value = remPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := remPattern.FindStringSubmatch(match)
		number, _ := strconv.ParseFloat(parts[1], 64)
		return formatCSSNumber(number*16) + "px"
	})
	value = oklchPattern.ReplaceAllStringFunc(value, oklchToHex)
	value = shortHexPattern.ReplaceAllString(value, `${1}#${2}${2}${3}${3}${4}${4}${5}`)
	return value
}

func oklchToHex(value string) string {
	parts := oklchPattern.FindStringSubmatch(value)
	if len(parts) == 0 {
		return value
	}
	l, _ := strconv.ParseFloat(parts[1], 64)
	if strings.Contains(parts[1], ".") && l > 1 || strings.Contains(value, "%") {
		l /= 100
	}
	c, _ := strconv.ParseFloat(parts[2], 64)
	h, _ := strconv.ParseFloat(parts[3], 64)
	a := c * math.Cos(h*math.Pi/180)
	b := c * math.Sin(h*math.Pi/180)

	lRoot := l + 0.3963377774*a + 0.2158037573*b
	mRoot := l - 0.1055613458*a - 0.0638541728*b
	sRoot := l - 0.0894841775*a - 1.291485548*b
	lLinear := lRoot * lRoot * lRoot
	mLinear := mRoot * mRoot * mRoot
	sLinear := sRoot * sRoot * sRoot
	r := 4.0767416621*lLinear - 3.3077115913*mLinear + 0.2309699292*sLinear
	g := -1.2684380046*lLinear + 2.6097574011*mLinear - 0.3413193965*sLinear
	bl := -0.0041960863*lLinear - 0.7034186147*mLinear + 1.707614701*sLinear
	r = linearToSRGB(r)
	g = linearToSRGB(g)
	bl = linearToSRGB(bl)

	if parts[4] != "" {
		alpha, _ := strconv.ParseFloat(parts[4], 64)
		if strings.Contains(value, "%") && alpha > 1 {
			alpha /= 100
		}
		return fmt.Sprintf(
			"rgba(%d, %d, %d, %s)",
			colorByte(r),
			colorByte(g),
			colorByte(bl),
			formatCSSNumber(alpha),
		)
	}
	return fmt.Sprintf("#%02x%02x%02x", colorByte(r), colorByte(g), colorByte(bl))
}

func linearToSRGB(value float64) float64 {
	if value <= 0.0031308 {
		return 12.92 * value
	}
	return 1.055*math.Pow(value, 1/2.4) - 0.055
}

func colorByte(value float64) int {
	return int(math.Round(math.Max(0, math.Min(1, value)) * 255))
}

func formatCSSNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func mergeDeclarations(groups ...[]declaration) []declaration {
	merged := make(map[string]declaration)
	for _, group := range groups {
		for _, declaration := range group {
			current, exists := merged[declaration.name]
			if exists && current.important && !declaration.important {
				continue
			}
			merged[declaration.name] = declaration
		}
	}
	result := make([]declaration, 0, len(merged))
	for _, declaration := range merged {
		result = append(result, declaration)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].order != result[j].order {
			return result[i].order < result[j].order
		}
		return result[i].name < result[j].name
	})
	return result
}
