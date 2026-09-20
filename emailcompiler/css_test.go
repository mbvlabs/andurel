package emailcompiler

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestParseStylesheet(t *testing.T) {
	css := `
		/* generated utilities */
		@charset "UTF-8";
		@property --spacing {
			syntax: "<length>";
			inherits: false;
			initial-value: 0.25rem;
		}
		:root, :host {
			--brand: oklch(50% 0.1 120);
		}
		.block, .sm\:block {
			display: block;
			color: var(--brand) !IMPORTANT;
		}
		@media (width >= 40rem) {
			.hover\:underline:hover { text-decoration-line: underline; }
		}
		@supports (display: grid) {
			.wrapper { .nested { display: grid; } }
		}
	`

	stylesheet, err := parseStylesheet(css)
	if err != nil {
		t.Fatalf("parseStylesheet() error = %v", err)
	}

	wantVariables := map[string]string{
		"--spacing": "0.25rem",
		"--brand":   "oklch(50% 0.1 120)",
	}
	if !reflect.DeepEqual(stylesheet.variables, wantVariables) {
		t.Fatalf("variables = %#v, want %#v", stylesheet.variables, wantVariables)
	}

	for _, className := range []string{"block", "sm:block", "hover:underline:hover"} {
		if _, ok := stylesheet.rules[className]; !ok {
			t.Errorf("rules missing %q: %#v", className, stylesheet.rules)
		}
	}
	if _, ok := stylesheet.rules["wrapper"]; ok {
		t.Error("nested rule block should not be treated as declarations")
	}

	block := stylesheet.rules["block"]
	if block.order != 2 {
		t.Errorf("block order = %d, want 2", block.order)
	}
	wantDeclarations := []declaration{
		{name: "display", value: "block", order: 1},
		{name: "color", value: "var(--brand)", important: true, order: 1},
	}
	if !reflect.DeepEqual(block.declarations, wantDeclarations) {
		t.Errorf("block declarations = %#v, want %#v", block.declarations, wantDeclarations)
	}
}

func TestParseStylesheetErrors(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want string
	}{
		{name: "unterminated prelude", css: ".block", want: "unterminated CSS prelude"},
		{
			name: "unterminated block",
			css:  ".block { color: red",
			want: ".block: unterminated CSS block",
		},
		{
			name: "invalid rule declaration",
			css:  ".block { color }",
			want: `selector .block: invalid declaration "color"`,
		},
		{
			name: "invalid property declaration",
			css:  "@property --size { invalid }",
			want: `@property --size: invalid declaration "invalid"`,
		},
		{
			name: "invalid nested rule",
			css:  "@media screen { .block { invalid } }",
			want: `selector .block: invalid declaration "invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseStylesheet(tt.css)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseStylesheet() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCSSScanningHelpers(t *testing.T) {
	t.Run("skip whitespace and comments", func(t *testing.T) {
		input := " \n/* one */\t/* two */value"
		if got := skipCSSSpaceAndComments(input, 0); got != strings.Index(input, "value") {
			t.Errorf("skipCSSSpaceAndComments() = %d, want %d", got, strings.Index(input, "value"))
		}
		if got := skipCSSSpaceAndComments("/* unfinished", 0); got != len("/* unfinished") {
			t.Errorf("unfinished comment offset = %d", got)
		}
	})

	t.Run("find prelude terminator", func(t *testing.T) {
		input := `.example:not([title="a;b{"]) { color: red }`
		index, terminator, err := findCSSPreludeTerminator(input, 0)
		if err != nil {
			t.Fatalf("findCSSPreludeTerminator() error = %v", err)
		}
		if terminator != '{' || index != strings.Index(input, "{ color") {
			t.Errorf("terminator = (%d, %q)", index, terminator)
		}

		index, terminator, err = findCSSPreludeTerminator(`@import url("a;b");`, 0)
		if err != nil || terminator != ';' || index != len(`@import url("a;b")`) {
			t.Errorf("at-rule terminator = (%d, %q, %v)", index, terminator, err)
		}
		escapedQuote := `.example[title="a\"{b"] { color: red }`
		index, terminator, err = findCSSPreludeTerminator(escapedQuote, 0)
		if err != nil || terminator != '{' || index != strings.Index(escapedQuote, "{ color") {
			t.Errorf("escaped quote terminator = (%d, %q, %v)", index, terminator, err)
		}
		if _, _, err := findCSSPreludeTerminator(".missing", 0); err == nil {
			t.Error("expected unterminated prelude error")
		}
	})

	t.Run("find matching brace", func(t *testing.T) {
		input := `{ content: "}"; nested { value: '{'; } }tail`
		got, err := findMatchingBrace(input, 0)
		if err != nil {
			t.Fatalf("findMatchingBrace() error = %v", err)
		}
		if got != strings.Index(input, "}tail") {
			t.Errorf("findMatchingBrace() = %d, want %d", got, strings.Index(input, "}tail"))
		}
		escapedQuote := `{ content: "a\"}b"; }tail`
		got, err = findMatchingBrace(escapedQuote, 0)
		if err != nil || got != strings.Index(escapedQuote, "}tail") {
			t.Errorf("escaped quote brace = (%d, %v)", got, err)
		}
		if _, err := findMatchingBrace("{ open", 0); err == nil {
			t.Error("expected unterminated block error")
		}
	})
}

func TestParseDeclarationBlock(t *testing.T) {
	tests := []struct {
		name            string
		block           string
		want            []declaration
		declarationOnly bool
		wantError       string
	}{
		{
			name:  "values retain protected separators",
			block: `content: "a:b;c"; background: rgb(1, 2, 3); color: red !important;`,
			want: []declaration{
				{name: "content", value: `"a:b;c"`, order: 7},
				{name: "background", value: "rgb(1, 2, 3)", order: 7},
				{name: "color", value: "red", important: true, order: 7},
			},
			declarationOnly: true,
		},
		{name: "empty", block: " ; ", declarationOnly: true, want: []declaration{}},
		{name: "nested block", block: ".child { color: red; }", declarationOnly: false},
		{name: "invalid", block: "display", wantError: `invalid declaration "display"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, declarationOnly, err := parseDeclarationBlock(tt.block, 7)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDeclarationBlock() error = %v", err)
			}
			if declarationOnly != tt.declarationOnly {
				t.Errorf("declarationOnly = %v, want %v", declarationOnly, tt.declarationOnly)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("declarations = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTopLevelHelpers(t *testing.T) {
	input := `one,"two,three",func(four,five),[six,seven],eight\,nine,ten`
	want := []string{"one", `"two,three"`, "func(four,five)", "[six,seven]", `eight\,nine`, "ten"}
	if got := splitCSSTopLevel(input, ','); !reflect.DeepEqual(got, want) {
		t.Errorf("splitCSSTopLevel() = %#v, want %#v", got, want)
	}
	escapedQuoteList := `"one\",two",three`
	if got := splitCSSTopLevel(
		escapedQuoteList,
		',',
	); !reflect.DeepEqual(
		got,
		[]string{`"one\",two"`, "three"},
	) {
		t.Errorf("splitCSSTopLevel() escaped quote = %#v", got)
	}
	if got := splitCSSList(
		".one, :is(.two,.three), .four",
	); !reflect.DeepEqual(
		got,
		[]string{".one", " :is(.two,.three)", " .four"},
	) {
		t.Errorf("splitCSSList() = %#v", got)
	}

	tests := []struct {
		input  string
		target byte
		want   int
	}{
		{input: `a:b`, target: ':', want: 1},
		{input: `a\:b`, target: ':', want: -1},
		{input: `fn(a:b):c`, target: ':', want: 7},
		{input: `[a:b]:c`, target: ':', want: 5},
		{input: `"a:b":c`, target: ':', want: 5},
		{input: `"a\":b":c`, target: ':', want: 7},
	}
	for _, tt := range tests {
		if got := findTopLevelCharacter(tt.input, tt.target); got != tt.want {
			t.Errorf(
				"findTopLevelCharacter(%q, %q) = %d, want %d",
				tt.input,
				tt.target,
				got,
				tt.want,
			)
		}
	}

	for _, selector := range []string{":root", ".utility, :host", ":root, .utility"} {
		if !selectorDefinesVariables(selector) {
			t.Errorf("selectorDefinesVariables(%q) = false", selector)
		}
	}
	if selectorDefinesVariables("body, .utility") {
		t.Error("body selector should not define global variables")
	}
	if !containsTopLevelBrace("rule { color: red }") || containsTopLevelBrace(`fn("{")`) {
		t.Error("containsTopLevelBrace() did not respect nesting")
	}
}

func TestCSSIdentifierHandling(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError string
	}{
		{name: "plain", input: "block", want: "block"},
		{name: "escaped punctuation", input: `sm\:block`, want: "sm:block"},
		{name: "hex escape and consumed space", input: `\41 bc`, want: "Abc"},
		{name: "six digit hex", input: `\000041B`, want: "AB"},
		{name: "trailing escape", input: `broken\`, wantError: "trailing CSS escape"},
		{name: "invalid rune", input: `\110000`, wantError: "invalid CSS escape"},
		{name: "surrogate", input: `\D800`, wantError: "invalid CSS escape"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unescapeCSSIdentifier(tt.input)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("unescapeCSSIdentifier() = (%q, %v), want %q", got, err, tt.want)
			}
		})
	}

	selectors := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: `.sm\:block`, want: "sm:block", ok: true},
		{input: "  .block  ", want: "block", ok: true},
		{input: "body", ok: false},
		{input: ".", ok: false},
		{input: `.broken\`, ok: false},
	}
	for _, tt := range selectors {
		got, ok := classFromSimpleSelector(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf(
				"classFromSimpleSelector(%q) = (%q, %v), want (%q, %v)",
				tt.input,
				got,
				ok,
				tt.want,
				tt.ok,
			)
		}
	}

	for _, char := range []byte{'0', '9', 'a', 'f', 'A', 'F'} {
		if !isHex(char) {
			t.Errorf("isHex(%q) = false", char)
		}
	}
	if isHex('g') {
		t.Error("isHex('g') = true")
	}
	for _, char := range []byte{' ', '\n', '\r', '\t', '\f'} {
		if !isCSSSpace(char) {
			t.Errorf("isCSSSpace(%q) = false", char)
		}
	}
	if isCSSSpace('x') {
		t.Error("isCSSSpace('x') = true")
	}
}

func TestSplitVariants(t *testing.T) {
	tests := []struct {
		name      string
		className string
		variants  []string
		base      string
		wantError bool
	}{
		{name: "base", className: "block", base: "block"},
		{
			name:      "variants",
			className: "sm:hover:block",
			variants:  []string{"sm", "hover"},
			base:      "block",
		},
		{
			name:      "arbitrary value",
			className: `sm:bg-[url("https://example.test/a:b")]`,
			variants:  []string{"sm"},
			base:      `bg-[url("https://example.test/a:b")]`,
		},
		{
			name:      "arbitrary variant",
			className: "[@media(width:600px)]:block",
			variants:  []string{"[@media(width:600px)]"},
			base:      "block",
		},
		{
			name:      "escaped quote",
			className: `hover:bg-["a\"b:c"]`,
			variants:  []string{"hover"},
			base:      `bg-["a\"b:c"]`,
		},
		{name: "unclosed bracket", className: "bg-[red", wantError: true},
		{name: "unclosed paren", className: "supports-(display:grid", wantError: true},
		{name: "unclosed quote", className: `bg-["red]`, wantError: true},
		{name: "empty base", className: "hover:", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variants, base, err := splitVariants(tt.className)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || !reflect.DeepEqual(variants, tt.variants) || base != tt.base {
				t.Errorf(
					"splitVariants() = (%#v, %q, %v), want (%#v, %q, nil)",
					variants,
					base,
					err,
					tt.variants,
					tt.base,
				)
			}
		})
	}
}

func TestResolveDeclarations(t *testing.T) {
	declarations := []declaration{
		{name: "--size", value: "2rem", order: 1},
		{name: "width", value: "calc(var(--size) * 2)", important: true, order: 2},
		{name: "color", value: "var(--missing, #abc)", order: 3},
	}
	got, err := resolveDeclarations(declarations, map[string]string{"--size": "1rem"})
	if err != nil {
		t.Fatalf("resolveDeclarations() error = %v", err)
	}
	want := []declaration{
		{name: "width", value: "64px", important: true, order: 2},
		{name: "color", value: "#aabbcc", order: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveDeclarations() = %#v, want %#v", got, want)
	}

	_, err = resolveDeclarations([]declaration{{name: "color", value: "var(--missing)"}}, nil)
	if err == nil ||
		!strings.Contains(err.Error(), "resolve color: unresolved CSS variable --missing") {
		t.Errorf("resolveDeclarations() error = %v", err)
	}
}

func TestResolveCSSValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		variables map[string]string
		want      string
		wantError string
	}{
		{name: "unchanged", value: "red", want: "red"},
		{
			name:      "nested variables",
			value:     "var(--a)",
			variables: map[string]string{"--a": "var(--b)", "--b": "2rem"},
			want:      "2rem",
		},
		{name: "fallback", value: "var(--missing, rgb(1, 2, 3))", want: "rgb(1, 2, 3)"},
		{
			name:      "initial uses fallback",
			value:     "var(--a, blue)",
			variables: map[string]string{"--a": "initial"},
			want:      "blue",
		},
		{
			name:      "multiple replacements and calc",
			value:     "calc(var(--n) * 2) var(--unit)",
			variables: map[string]string{"--n": "3px", "--unit": "solid"},
			want:      "6px solid",
		},
		{
			name:      "cycle",
			value:     "var(--a)",
			variables: map[string]string{"--a": "var(--b)", "--b": "var(--a)"},
			wantError: "cyclic CSS variable --a",
		},
		{name: "missing", value: "var(--a)", wantError: "unresolved CSS variable --a"},
		{
			name:      "missing fallback variable",
			value:     "var(--a, var(--b))",
			wantError: "unresolved CSS variable --b",
		},
		{name: "unterminated", value: "var(--a", wantError: "unterminated CSS function"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCSSValue(tt.value, tt.variables, nil)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("resolveCSSValue() = (%q, %v), want %q", got, err, tt.want)
			}
		})
	}
}

func TestMatchingParen(t *testing.T) {
	tests := []struct {
		value string
		open  int
		want  int
	}{
		{value: "fn(one)", open: 2, want: 6},
		{value: `fn("a)", nested(x))tail`, open: 2, want: 18},
		{value: `fn('a\'b')tail`, open: 2, want: 9},
	}
	for _, tt := range tests {
		got, err := matchingParen(tt.value, tt.open)
		if err != nil || got != tt.want {
			t.Errorf("matchingParen(%q) = (%d, %v), want %d", tt.value, got, err, tt.want)
		}
	}
	if _, err := matchingParen("fn(open", 2); err == nil {
		t.Error("expected unterminated function error")
	}
}

func TestSimplifyCalc(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "calc(2rem * 3)", want: "6rem"},
		{input: "calc(2 * 3px)", want: "6px"},
		{input: "calc(9px / 3)", want: "3px"},
		{input: "calc(-.5rem * 2)", want: "-1rem"},
		{input: "calc(2 * 3) calc(8px / 4)", want: "6 2px"},
		{input: "calc(9px / 0)", want: "calc(9px / 0)"},
		{input: "calc(9px / 3px)", want: "calc(9px / 3px)"},
		{input: "calc(1px + 2px)", want: "calc(1px + 2px)"},
	}
	for _, tt := range tests {
		if got := simplifyCalc(tt.input); got != tt.want {
			t.Errorf("simplifyCalc(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeEmailValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "1rem -0.25rem .5rem", want: "16px -4px 8px"},
		{input: "#abc", want: "#aabbcc"},
		{input: "1px solid #AbC; color:#123", want: "1px solid #AAbbCC; color:#112233"},
		{input: "url(#abc)", want: "url(#aabbcc)"},
		{input: "#abcdef", want: "#abcdef"},
		{input: "oklch(0 0 0)", want: "#000000"},
		{input: "oklch(1 0 0)", want: "#ffffff"},
		{input: "oklch(100% 0 0 / 50%)", want: "rgba(255, 255, 255, 0.5)"},
	}
	for _, tt := range tests {
		if got := normalizeEmailValue(tt.input); got != tt.want {
			t.Errorf("normalizeEmailValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
	if got := oklchToHex("not-a-color"); got != "not-a-color" {
		t.Errorf("oklchToHex() = %q", got)
	}
}

func TestNumericHelpers(t *testing.T) {
	floatTests := []struct {
		input float64
		want  string
	}{
		{input: 2, want: "2"},
		{input: 1.25, want: "1.25"},
		{input: 1.0000001, want: "1"},
		{input: -0.5, want: "-0.5"},
	}
	for _, tt := range floatTests {
		if got := formatCSSNumber(tt.input); got != tt.want {
			t.Errorf("formatCSSNumber(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}

	byteTests := []struct {
		input float64
		want  int
	}{
		{input: -1, want: 0},
		{input: 0, want: 0},
		{input: 0.5, want: 128},
		{input: 1, want: 255},
		{input: 2, want: 255},
	}
	for _, tt := range byteTests {
		if got := colorByte(tt.input); got != tt.want {
			t.Errorf("colorByte(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}

	if got := linearToSRGB(0.003); math.Abs(got-0.03876) > 0.000001 {
		t.Errorf("linearToSRGB() linear branch = %v", got)
	}
	if got := linearToSRGB(1); math.Abs(got-1) > 0.000001 {
		t.Errorf("linearToSRGB() power branch = %v", got)
	}
}

func TestMergeDeclarations(t *testing.T) {
	first := []declaration{
		{name: "color", value: "red", important: true, order: 3},
		{name: "display", value: "block", order: 2},
		{name: "border", value: "none", order: 3},
	}
	second := []declaration{
		{name: "color", value: "blue", order: 4},
		{name: "display", value: "flex", important: true, order: 4},
		{name: "border", value: "1px", important: true, order: 3},
	}
	want := []declaration{
		{name: "border", value: "1px", important: true, order: 3},
		{name: "color", value: "red", important: true, order: 3},
		{name: "display", value: "flex", important: true, order: 4},
	}
	if got := mergeDeclarations(first, second); !reflect.DeepEqual(got, want) {
		t.Errorf("mergeDeclarations() = %#v, want %#v", got, want)
	}
	if got := mergeDeclarations(); len(got) != 0 {
		t.Errorf("mergeDeclarations() = %#v, want empty", got)
	}
}
