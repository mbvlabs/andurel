package emailcompiler

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
)

const authoredStyleOrder = int(^uint(0) >> 2)

var emailBreakpoints = map[string]int{
	"sm":  640,
	"md":  768,
	"lg":  1024,
	"xl":  1280,
	"2xl": 1536,
}

type residualRule struct {
	originalClass string
	className     string
	declarations  []declaration
	media         []string
	pseudos       []string
}

type residualStylesheet struct {
	rules       map[string]residualRule
	classOwners map[string]string
}

func newResidualStylesheet() *residualStylesheet {
	return &residualStylesheet{
		rules:       make(map[string]residualRule),
		classOwners: make(map[string]string),
	}
}

func transformTemplate(
	file sourceFile,
	stylesheet *compiledStylesheet,
	residual *residualStylesheet,
) error {
	walk := visitor.New()
	visitElement := walk.Element
	walk.Element = func(element *parser.Element) error {
		if err := transformElement(file, element, stylesheet, residual); err != nil {
			return err
		}
		return visitElement(element)
	}
	return file.template.Visit(walk)
}

func transformElement(
	file sourceFile,
	element *parser.Element,
	stylesheet *compiledStylesheet,
	residual *residualStylesheet,
) error {
	classAttribute := constantAttribute(element, "class")
	if classAttribute == nil {
		return nil
	}

	var inlineRules []utilityRule
	var outputClasses []string
	for className := range strings.FieldsSeq(classAttribute.Value) {
		variants, base, err := splitVariants(className)
		if err != nil {
			return compilerError(file.relative, classAttribute.ValueRange.From, err.Error())
		}
		rule, found := stylesheet.rules[base]
		if !found {
			outputClasses = append(outputClasses, className)
			continue
		}
		if len(variants) == 0 {
			inlineRules = append(inlineRules, rule)
			continue
		}

		compiledClass, err := residual.add(className, variants, rule, stylesheet.variables)
		if err != nil {
			return compilerError(file.relative, classAttribute.ValueRange.From, err.Error())
		}
		outputClasses = append(outputClasses, compiledClass)
	}

	if len(inlineRules) == 0 {
		setClassAttribute(element, classAttribute, outputClasses)
		return nil
	}

	sort.SliceStable(inlineRules, func(i, j int) bool {
		return inlineRules[i].order < inlineRules[j].order
	})
	groups := make([][]declaration, 0, len(inlineRules)+1)
	for _, rule := range inlineRules {
		groups = append(groups, rule.declarations)
	}

	styleAttribute := constantAttribute(element, "style")
	if styleAttribute != nil {
		authored, declarationOnly, err := parseDeclarationBlock(
			styleAttribute.Value,
			authoredStyleOrder,
		)
		if err != nil || !declarationOnly {
			return compilerError(
				file.relative,
				styleAttribute.ValueRange.From,
				"invalid static style attribute",
			)
		}
		groups = append(groups, authored)
	}

	resolved, err := resolveDeclarations(mergeDeclarations(groups...), stylesheet.variables)
	if err != nil {
		return compilerError(file.relative, classAttribute.ValueRange.From, err.Error())
	}
	styleValue := strings.ReplaceAll(serializeDeclarations(resolved), `"`, "&quot;")
	if styleAttribute == nil {
		styleAttribute = newConstantAttribute("style", styleValue)
		element.Attributes = append(element.Attributes, styleAttribute)
	} else {
		styleAttribute.Value = styleValue
	}
	setClassAttribute(element, classAttribute, outputClasses)
	addCompatibilityAttributes(element, resolved)
	return nil
}

func setClassAttribute(
	element *parser.Element,
	classAttribute *parser.ConstantAttribute,
	classes []string,
) {
	if len(classes) > 0 {
		classAttribute.Value = strings.Join(classes, " ")
		return
	}
	for index, attribute := range element.Attributes {
		if attribute == classAttribute {
			element.Attributes = append(element.Attributes[:index], element.Attributes[index+1:]...)
			return
		}
	}
}

func constantAttribute(element *parser.Element, name string) *parser.ConstantAttribute {
	for _, attribute := range element.Attributes {
		constant, ok := attribute.(*parser.ConstantAttribute)
		if ok && attributeKeyEqual(constant.Key, name) {
			return constant
		}
	}
	return nil
}

func newConstantAttribute(name, value string) *parser.ConstantAttribute {
	return &parser.ConstantAttribute{
		Key:   parser.ConstantAttributeKey{Name: name},
		Value: value,
	}
}

func serializeDeclarations(declarations []declaration) string {
	var output strings.Builder
	for index, declaration := range declarations {
		if index > 0 {
			output.WriteString("; ")
		}
		output.WriteString(declaration.name)
		output.WriteString(": ")
		output.WriteString(declaration.value)
		if declaration.important {
			output.WriteString(" !important")
		}
	}
	if len(declarations) > 0 {
		output.WriteByte(';')
	}
	return output.String()
}

func addCompatibilityAttributes(element *parser.Element, declarations []declaration) {
	properties := make(map[string]string, len(declarations))
	for _, declaration := range declarations {
		properties[declaration.name] = declaration.value
	}

	tag := strings.ToLower(element.Name)
	if oneOf(tag, "body", "table", "td", "th") {
		setCompatibilityAttribute(element, "bgcolor", properties["background-color"])
	}
	if oneOf(tag, "table", "td", "th", "img") {
		setCompatibilityAttribute(element, "width", attributeDimension(properties["width"]))
	}
	if oneOf(tag, "td", "th", "img") {
		setCompatibilityAttribute(element, "height", attributeDimension(properties["height"]))
	}
	if oneOf(tag, "table", "td", "th") {
		setCompatibilityAttribute(element, "align", properties["text-align"])
	}
	if oneOf(tag, "td", "th", "img") {
		setCompatibilityAttribute(element, "valign", properties["vertical-align"])
	}
}

func setCompatibilityAttribute(element *parser.Element, name, value string) {
	if value == "" || strings.ContainsAny(value, "()%") {
		return
	}
	if existing := constantAttribute(element, name); existing != nil {
		existing.Value = value
		return
	}
	element.Attributes = append(element.Attributes, newConstantAttribute(name, value))
}

func attributeDimension(value string) string {
	return strings.TrimSuffix(value, "px")
}

func oneOf(value string, candidates ...string) bool {
	return slices.Contains(candidates, value)
}

func (stylesheet *residualStylesheet) add(
	originalClass string,
	variants []string,
	rule utilityRule,
	variables map[string]string,
) (string, error) {
	className := safeEmailClass(originalClass)
	if owner, exists := stylesheet.classOwners[className]; exists && owner != originalClass {
		return "", fmt.Errorf(
			"email class %q conflicts with %q after compilation",
			originalClass,
			owner,
		)
	}
	stylesheet.classOwners[className] = originalClass

	resolved, err := resolveDeclarations(rule.declarations, variables)
	if err != nil {
		return "", err
	}
	for index := range resolved {
		resolved[index].important = true
	}
	residual := residualRule{
		originalClass: originalClass,
		className:     className,
		declarations:  resolved,
	}
	for _, variant := range variants {
		switch variant {
		case "hover", "focus", "active":
			residual.pseudos = append(residual.pseudos, ":"+variant)
		case "dark":
			residual.media = append(residual.media, "(prefers-color-scheme: dark)")
		default:
			if width, ok := emailBreakpoints[variant]; ok {
				residual.media = append(
					residual.media,
					fmt.Sprintf("screen and (min-width: %dpx)", width),
				)
				continue
			}
			if after, ok := strings.CutPrefix(variant, "max-"); ok {
				if width, ok := emailBreakpoints[after]; ok {
					residual.media = append(
						residual.media,
						fmt.Sprintf("screen and (max-width: %dpx)", width-1),
					)
					continue
				}
			}
			return "", fmt.Errorf("email variant %q is not supported", variant)
		}
	}
	stylesheet.rules[originalClass] = residual
	return className, nil
}

func safeEmailClass(className string) string {
	var output strings.Builder
	output.WriteString("andurel-email-")
	lastWasDash := false
	for _, char := range className {
		valid := unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-'
		if valid {
			output.WriteRune(char)
			lastWasDash = false
			continue
		}
		if !lastWasDash {
			output.WriteByte('-')
			lastWasDash = true
		}
	}
	return strings.TrimSuffix(output.String(), "-")
}

func (stylesheet *residualStylesheet) String() string {
	if len(stylesheet.rules) == 0 {
		return ""
	}
	keys := make([]string, 0, len(stylesheet.rules))
	for key := range stylesheet.rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var output strings.Builder
	output.WriteString("/* Generated by andurel email compile. */\n")
	for _, key := range keys {
		rule := stylesheet.rules[key]
		indent := ""
		for _, media := range rule.media {
			output.WriteString(indent)
			output.WriteString("@media ")
			output.WriteString(media)
			output.WriteString(" {\n")
			indent += "  "
		}
		output.WriteString(indent)
		output.WriteByte('.')
		output.WriteString(rule.className)
		output.WriteString(strings.Join(rule.pseudos, ""))
		output.WriteString(" { ")
		output.WriteString(serializeDeclarations(rule.declarations))
		output.WriteString(" }\n")
		for range rule.media {
			indent = strings.TrimSuffix(indent, "  ")
			output.WriteString(indent)
			output.WriteString("}\n")
		}
	}
	return output.String()
}

func injectHeadStyles(files []sourceFile, css string) bool {
	foundHead := false
	for _, file := range files {
		walk := visitor.New()
		visitElement := walk.Element
		walk.Element = func(element *parser.Element) error {
			if strings.EqualFold(element.Name, "head") {
				element.Children = append(element.Children, &parser.RawElement{
					Name:     "style",
					Contents: "\n" + css,
				})
				foundHead = true
			}
			return visitElement(element)
		}
		_ = file.template.Visit(walk)
	}
	return foundHead
}
