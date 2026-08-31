package emailcompiler

import (
	"strconv"
	"strings"
	"testing"

	"github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
)

func TestTransformElementInlinesUtilitiesAndAddsCompatibilityAttributes(t *testing.T) {
	t.Parallel()

	element := &parser.Element{
		Name: "td",
		Attributes: []parser.Attribute{
			newConstantAttribute("class", "layout brand semantic"),
			newConstantAttribute("style", "color: #fff; width: 320px"),
			newConstantAttribute("bgcolor", "#000000"),
		},
	}
	stylesheet := &compiledStylesheet{
		variables: map[string]string{"--brand": "#123"},
		rules: map[string]utilityRule{
			"layout": {
				order: 1,
				declarations: []declaration{
					{name: "width", value: "96px", order: 1},
					{name: "height", value: "48px", order: 1},
					{name: "text-align", value: "center", order: 1},
					{name: "vertical-align", value: "middle", order: 1},
				},
			},
			"brand": {
				order: 2,
				declarations: []declaration{
					{name: "background-color", value: "var(--brand)", order: 2},
				},
			},
		},
	}

	err := transformElement(
		sourceFile{relative: "email/message.templ"},
		element,
		stylesheet,
		newResidualStylesheet(),
	)
	if err != nil {
		t.Fatalf("transformElement() error = %v", err)
	}

	attributes := constantAttributes(element)
	if got, want := attributes["class"], "semantic"; got != want {
		t.Errorf("class = %q, want %q", got, want)
	}
	if got, want := attributes["style"], "height: 48px; text-align: center; vertical-align: middle; background-color: #112233; color: #ffffff; width: 320px;"; got != want {
		t.Errorf("style = %q, want %q", got, want)
	}
	for name, want := range map[string]string{
		"bgcolor": "#112233",
		"width":   "320",
		"height":  "48",
		"align":   "center",
		"valign":  "middle",
	} {
		if got := attributes[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestTransformElementRemovesFullyCompiledClassAttribute(t *testing.T) {
	t.Parallel()

	element := &parser.Element{
		Name:       "p",
		Attributes: []parser.Attribute{newConstantAttribute("class", "text")},
	}
	stylesheet := &compiledStylesheet{
		variables: map[string]string{},
		rules: map[string]utilityRule{
			"text": {
				declarations: []declaration{{name: "font-size", value: "1rem"}},
			},
		},
	}

	if err := transformElement(
		sourceFile{},
		element,
		stylesheet,
		newResidualStylesheet(),
	); err != nil {
		t.Fatalf("transformElement() error = %v", err)
	}
	if class := constantAttribute(element, "class"); class != nil {
		t.Errorf("class attribute was retained with value %q", class.Value)
	}
	if got, want := constantAttribute(element, "style").Value, "font-size: 16px;"; got != want {
		t.Errorf("style = %q, want %q", got, want)
	}
}

func TestTransformTemplateVisitsNestedElements(t *testing.T) {
	t.Parallel()

	template := mustParseTemplate(t, `package email

templ Message() {
	<table><tbody><tr><td class="padded">Hello</td></tr></tbody></table>
}`)
	file := sourceFile{relative: "email/message.templ", template: template}
	stylesheet := &compiledStylesheet{
		variables: map[string]string{},
		rules: map[string]utilityRule{
			"padded": {declarations: []declaration{{name: "padding", value: "1.5rem"}}},
		},
	}

	if err := transformTemplate(file, stylesheet, newResidualStylesheet()); err != nil {
		t.Fatalf("transformTemplate() error = %v", err)
	}

	td := findElement(t, template, "td")
	if got, want := constantAttribute(td, "style").Value, "padding: 24px;"; got != want {
		t.Errorf("nested td style = %q, want %q", got, want)
	}
}

func TestTransformTemplatePropagatesElementErrors(t *testing.T) {
	t.Parallel()

	template := mustParseTemplate(t, `package email

templ Message() {
	<div class="sm:[broken">Hello</div>
}`)
	err := transformTemplate(
		sourceFile{relative: "email/message.templ", template: template},
		emptyStylesheet(),
		newResidualStylesheet(),
	)
	if err == nil ||
		!strings.Contains(
			err.Error(),
			`email/message.templ:3:13: invalid Tailwind class "sm:[broken"`,
		) {
		t.Fatalf("transformTemplate() error = %v", err)
	}
}

func TestAddCompatibilityAttributesByElement(t *testing.T) {
	t.Parallel()

	declarations := []declaration{
		{name: "background-color", value: "#abcdef"},
		{name: "width", value: "600px"},
		{name: "height", value: "40px"},
		{name: "text-align", value: "right"},
		{name: "vertical-align", value: "bottom"},
	}
	tests := []struct {
		tag  string
		want map[string]string
	}{
		{tag: "body", want: map[string]string{"bgcolor": "#abcdef"}},
		{
			tag:  "table",
			want: map[string]string{"bgcolor": "#abcdef", "width": "600", "align": "right"},
		},
		{
			tag: "td",
			want: map[string]string{
				"bgcolor": "#abcdef",
				"width":   "600",
				"height":  "40",
				"align":   "right",
				"valign":  "bottom",
			},
		},
		{
			tag: "th",
			want: map[string]string{
				"bgcolor": "#abcdef",
				"width":   "600",
				"height":  "40",
				"align":   "right",
				"valign":  "bottom",
			},
		},
		{tag: "img", want: map[string]string{"width": "600", "height": "40", "valign": "bottom"}},
		{tag: "div", want: map[string]string{}},
	}

	for _, test := range tests {
		t.Run(test.tag, func(t *testing.T) {
			t.Parallel()
			element := &parser.Element{Name: test.tag}
			addCompatibilityAttributes(element, declarations)
			if got := constantAttributes(element); !equalStringMap(got, test.want) {
				t.Errorf("attributes = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCompatibilityAttributesPreserveUnsafeAndUpdateExistingValues(t *testing.T) {
	t.Parallel()

	element := &parser.Element{
		Name: "td",
		Attributes: []parser.Attribute{
			newConstantAttribute("width", "original"),
			newConstantAttribute("height", "original"),
		},
	}
	addCompatibilityAttributes(element, []declaration{
		{name: "width", value: "50%"},
		{name: "height", value: "calc(40px)"},
		{name: "text-align", value: "center"},
	})
	attributes := constantAttributes(element)
	if got := attributes["width"]; got != "original" {
		t.Errorf("unsafe width replaced existing value with %q", got)
	}
	if got := attributes["height"]; got != "original" {
		t.Errorf("unsafe height replaced existing value with %q", got)
	}
	if got := attributes["align"]; got != "center" {
		t.Errorf("align = %q, want center", got)
	}

	addCompatibilityAttributes(element, []declaration{{name: "width", value: "44px"}})
	if got := constantAttribute(element, "width").Value; got != "44" {
		t.Errorf("existing width = %q, want 44", got)
	}
}

func TestTransformElementCompilesResidualVariants(t *testing.T) {
	t.Parallel()

	element := &parser.Element{
		Name: "a",
		Attributes: []parser.Attribute{
			newConstantAttribute(
				"class",
				"sm:hover:bg-red dark:max-md:focus:bg-red active:bg-red semantic",
			),
		},
	}
	stylesheet := &compiledStylesheet{
		variables: map[string]string{},
		rules: map[string]utilityRule{
			"bg-red": {declarations: []declaration{{name: "background-color", value: "#f00"}}},
		},
	}
	residual := newResidualStylesheet()

	if err := transformElement(sourceFile{}, element, stylesheet, residual); err != nil {
		t.Fatalf("transformElement() error = %v", err)
	}
	if got, want := constantAttribute(
		element,
		"class",
	).Value, "andurel-email-sm-hover-bg-red andurel-email-dark-max-md-focus-bg-red andurel-email-active-bg-red semantic"; got != want {
		t.Errorf("class = %q, want %q", got, want)
	}

	css := residual.String()
	for _, fragment := range []string{
		"/* Generated by andurel email compile. */",
		".andurel-email-active-bg-red:active { background-color: #ff0000 !important; }",
		"@media (prefers-color-scheme: dark) {",
		"@media screen and (max-width: 767px) {",
		".andurel-email-dark-max-md-focus-bg-red:focus { background-color: #ff0000 !important; }",
		"@media screen and (min-width: 640px) {",
		".andurel-email-sm-hover-bg-red:hover { background-color: #ff0000 !important; }",
	} {
		if !strings.Contains(css, fragment) {
			t.Errorf("residual CSS does not contain %q:\n%s", fragment, css)
		}
	}
}

func TestResidualStylesheetSupportsEveryBreakpoint(t *testing.T) {
	t.Parallel()

	residual := newResidualStylesheet()
	if got := residual.String(); got != "" {
		t.Fatalf("empty residual stylesheet = %q, want empty", got)
	}
	rule := utilityRule{declarations: []declaration{{name: "display", value: "block"}}}
	for variant, width := range emailBreakpoints {
		className := variant + ":block"
		if _, err := residual.add(className, []string{variant}, rule, nil); err != nil {
			t.Fatalf("add(%q) error = %v", className, err)
		}
		want := "screen and (min-width: " + strconv.Itoa(width) + "px)"
		if got := residual.rules[className].media; len(got) != 1 || got[0] != want {
			t.Errorf("%s media = %#v, want %q", variant, got, want)
		}
	}
}

func TestResidualStylesheetRejectsUnsupportedVariantsAndClassConflicts(t *testing.T) {
	t.Parallel()

	rule := utilityRule{declarations: []declaration{{name: "display", value: "block"}}}

	t.Run("unsupported variant", func(t *testing.T) {
		residual := newResidualStylesheet()
		_, err := residual.add("print:block", []string{"print"}, rule, nil)
		if err == nil || !strings.Contains(err.Error(), `email variant "print" is not supported`) {
			t.Fatalf("add() error = %v", err)
		}
	})

	t.Run("sanitized class conflict", func(t *testing.T) {
		residual := newResidualStylesheet()
		if _, err := residual.add("hover:bg/red", []string{"hover"}, rule, nil); err != nil {
			t.Fatalf("first add() error = %v", err)
		}
		_, err := residual.add("hover:bg.red", []string{"hover"}, rule, nil)
		if err == nil ||
			!strings.Contains(
				err.Error(),
				`email class "hover:bg.red" conflicts with "hover:bg/red"`,
			) {
			t.Fatalf("second add() error = %v", err)
		}
	})

	t.Run("unresolved declaration", func(t *testing.T) {
		residual := newResidualStylesheet()
		unresolved := utilityRule{
			declarations: []declaration{{name: "color", value: "var(--missing)"}},
		}
		_, err := residual.add("hover:brand", []string{"hover"}, unresolved, nil)
		if err == nil ||
			!strings.Contains(err.Error(), "resolve color: unresolved CSS variable --missing") {
			t.Fatalf("add() error = %v", err)
		}
	})
}

func TestSafeEmailClass(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"sm:hover:bg-red/50": "andurel-email-sm-hover-bg-red-50",
		"dark::focus":        "andurel-email-dark-focus",
		"already_safe":       "andurel-email-already_safe",
		"trailing!!!":        "andurel-email-trailing",
	}
	for input, want := range tests {
		if got := safeEmailClass(input); got != want {
			t.Errorf("safeEmailClass(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTransformElementReportsActionableErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		classValue string
		styleValue string
		stylesheet *compiledStylesheet
		want       string
	}{
		{
			name:       "invalid Tailwind class",
			classValue: "sm:[broken",
			stylesheet: emptyStylesheet(),
			want:       `email/message.templ:7:11: invalid Tailwind class "sm:[broken"`,
		},
		{
			name:       "unsupported variant",
			classValue: "print:block",
			stylesheet: stylesheetWithRule("block", declaration{name: "display", value: "block"}),
			want:       `email/message.templ:7:11: email variant "print" is not supported`,
		},
		{
			name:       "invalid authored style",
			classValue: "block",
			styleValue: "not-a-declaration",
			stylesheet: stylesheetWithRule("block", declaration{name: "display", value: "block"}),
			want:       "email/message.templ:9:3: invalid static style attribute",
		},
		{
			name:       "unresolved variable",
			classValue: "brand",
			stylesheet: stylesheetWithRule(
				"brand",
				declaration{name: "color", value: "var(--missing)"},
			),
			want: "email/message.templ:7:11: resolve color: unresolved CSS variable --missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			class := newConstantAttribute("class", test.classValue)
			class.ValueRange.From = parser.Position{Line: 7, Col: 11}
			element := &parser.Element{Name: "div", Attributes: []parser.Attribute{class}}
			if test.styleValue != "" {
				style := newConstantAttribute("style", test.styleValue)
				style.ValueRange.From = parser.Position{Line: 9, Col: 3}
				element.Attributes = append(element.Attributes, style)
			}

			err := transformElement(
				sourceFile{relative: "email/message.templ"},
				element,
				test.stylesheet,
				newResidualStylesheet(),
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("transformElement() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInjectHeadStyles(t *testing.T) {
	t.Parallel()

	first := mustParseTemplate(t, `package email

templ First() {
	<html><head><title>First</title></head><body></body></html>
}`)
	second := mustParseTemplate(t, `package email

templ Second() {
	<html><head></head><body></body></html>
}`)
	findElement(t, second, "head").Name = "HEAD"
	files := []sourceFile{{template: first}, {template: second}}
	css := ".responsive { display: block !important; }"

	if found := injectHeadStyles(files, css); !found {
		t.Fatal("injectHeadStyles() = false, want true")
	}
	for index, template := range []*parser.TemplateFile{first, second} {
		head := findElement(t, template, "head")
		if len(head.Children) == 0 {
			t.Fatalf("template %d head has no children", index)
		}
		style, ok := head.Children[len(head.Children)-1].(*parser.RawElement)
		if !ok {
			t.Fatalf(
				"template %d last head child = %T, want *parser.RawElement",
				index,
				head.Children[len(head.Children)-1],
			)
		}
		if style.Name != "style" || style.Contents != "\n"+css {
			t.Errorf("template %d style = %#v", index, style)
		}
	}

	withoutHead := mustParseTemplate(t, `package email

templ Fragment() {
	<div>Fragment</div>
}`)
	if found := injectHeadStyles([]sourceFile{{template: withoutHead}}, css); found {
		t.Fatal("injectHeadStyles() without head = true, want false")
	}
}

func mustParseTemplate(t *testing.T, input string) *parser.TemplateFile {
	t.Helper()
	template, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return template
}

func findElement(t *testing.T, template *parser.TemplateFile, name string) *parser.Element {
	t.Helper()
	var found *parser.Element
	walk := visitor.New()
	visitElement := walk.Element
	walk.Element = func(element *parser.Element) error {
		if strings.EqualFold(element.Name, name) && found == nil {
			found = element
		}
		return visitElement(element)
	}
	if err := template.Visit(walk); err != nil {
		t.Fatalf("visit template: %v", err)
	}
	if found == nil {
		t.Fatalf("element %q not found", name)
	}
	return found
}

func constantAttributes(element *parser.Element) map[string]string {
	attributes := make(map[string]string)
	for _, attribute := range element.Attributes {
		constant, ok := attribute.(*parser.ConstantAttribute)
		if ok {
			attributes[strings.ToLower(constant.Key.String())] = constant.Value
		}
	}
	return attributes
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func emptyStylesheet() *compiledStylesheet {
	return &compiledStylesheet{variables: map[string]string{}, rules: map[string]utilityRule{}}
}

func stylesheetWithRule(name string, item declaration) *compiledStylesheet {
	return &compiledStylesheet{
		variables: map[string]string{},
		rules: map[string]utilityRule{
			name: {declarations: []declaration{item}},
		},
	}
}
