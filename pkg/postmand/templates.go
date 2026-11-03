package postmand

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"
)

const (
	htmlTemplateSuffix  = ".html.tmpl"
	textTemplateSuffix  = ".txt.tmpl"
	htmlTemplatePattern = "*" + htmlTemplateSuffix
	textTemplatePattern = "*" + textTemplateSuffix
)

// Templates renders the html/text template pairs exported by React Email.
// Uses text/template on purpose: html/template strips the <!--[if mso]> comments
// React Email emits for Outlook; HTML placeholders carry an explicit `| html` pipe.
type Templates struct {
	html      *template.Template
	text      *template.Template
	htmlFiles map[string]string
	textFiles map[string]string
}

// ParseTemplates parses dir/*.html.tmpl and dir/*.txt.tmpl from fsys. Both
// template kinds are required: an email without an HTML or a text body is a
// scaffold mistake, not a render-time surprise.
func ParseTemplates(fsys fs.FS, dir string) (*Templates, error) {
	if fsys == nil {
		return nil, errors.New("postmand: parse templates: nil filesystem")
	}

	htmlTemplates, htmlFiles, err := parseTemplateKind(fsys, dir, htmlTemplatePattern, "HTML")
	if err != nil {
		return nil, err
	}

	textTemplates, textFiles, err := parseTemplateKind(fsys, dir, textTemplatePattern, "text")
	if err != nil {
		return nil, err
	}

	if err := validateTemplatePairs(htmlFiles, textFiles); err != nil {
		return nil, err
	}

	return &Templates{
		html:      htmlTemplates,
		text:      textTemplates,
		htmlFiles: htmlFiles,
		textFiles: textFiles,
	}, nil
}

// MustParseTemplates is ParseTemplates that panics on error. It is for
// templates committed to the binary with //go:embed, where a parse failure is
// a build-time bug.
func MustParseTemplates(fsys fs.FS, dir string) *Templates {
	templates, err := ParseTemplates(fsys, dir)
	if err != nil {
		panic(err)
	}

	return templates
}

// HTML executes "<name>.html.tmpl" with data.
func (t *Templates) HTML(name string, data any) (string, error) {
	if t == nil {
		return "", errors.New("postmand: HTML: nil Templates")
	}

	return t.execute(t.html, t.htmlFiles, "HTML", name, data)
}

// Text executes "<name>.txt.tmpl" with data.
func (t *Templates) Text(name string, data any) (string, error) {
	if t == nil {
		return "", errors.New("postmand: Text: nil Templates")
	}

	return t.execute(t.text, t.textFiles, "text", name, data)
}

func (t *Templates) execute(
	templates *template.Template,
	files map[string]string,
	kind string,
	name string,
	data any,
) (string, error) {
	file, ok := files[name]
	if !ok {
		available := make([]string, 0, len(files))
		for availableName := range files {
			available = append(available, availableName)
		}
		sort.Strings(available)

		return "", fmt.Errorf(
			"postmand: unknown %s template %q; available templates: %s",
			kind,
			name,
			strings.Join(available, ", "),
		)
	}

	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, file, data); err != nil {
		return "", fmt.Errorf("postmand: render %s template %q: %w", kind, name, err)
	}

	return body.String(), nil
}

// validateTemplatePairs rejects a template set where an HTML template has no
// text counterpart or the other way around. React Email always exports both,
// so an unpaired file means the export or the committed tree is incomplete.
func validateTemplatePairs(htmlFiles, textFiles map[string]string) error {
	missingText := missingTemplateNames(htmlFiles, textFiles)
	missingHTML := missingTemplateNames(textFiles, htmlFiles)
	if len(missingText) == 0 && len(missingHTML) == 0 {
		return nil
	}

	problems := make([]string, 0, len(missingText)+len(missingHTML))
	for _, name := range missingText {
		problems = append(
			problems,
			fmt.Sprintf(
				"%s has HTML but no text template (%s)",
				name,
				name+textTemplateSuffix,
			),
		)
	}
	for _, name := range missingHTML {
		problems = append(
			problems,
			fmt.Sprintf(
				"%s has text but no HTML template (%s)",
				name,
				name+htmlTemplateSuffix,
			),
		)
	}

	return fmt.Errorf("postmand: template pairs mismatch: %s", strings.Join(problems, ", "))
}

// missingTemplateNames returns the sorted names present in files but absent
// from counterparts.
func missingTemplateNames(files, counterparts map[string]string) []string {
	missing := make([]string, 0)
	for name := range files {
		if _, ok := counterparts[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	return missing
}

func parseTemplateKind(
	fsys fs.FS,
	dir string,
	pattern string,
	kind string,
) (*template.Template, map[string]string, error) {
	patternPath := pattern
	if cleaned := path.Clean(strings.TrimSpace(dir)); cleaned != "." && cleaned != "" {
		patternPath = path.Join(cleaned, pattern)
	}

	matches, err := fs.Glob(fsys, patternPath)
	if err != nil {
		return nil, nil, fmt.Errorf("postmand: list %s templates %q: %w", kind, patternPath, err)
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf(
			"postmand: no %s templates matching %q; export them with `andurel sync emails`",
			kind,
			patternPath,
		)
	}

	suffix := strings.TrimPrefix(pattern, "*")
	files := make(map[string]string, len(matches))
	for _, match := range matches {
		file := path.Base(match)
		files[strings.TrimSuffix(file, suffix)] = file
	}

	templates, err := template.New("postmand-"+kind).
		Option("missingkey=error").
		ParseFS(fsys, patternPath)
	if err != nil {
		return nil, nil, fmt.Errorf("postmand: parse %s templates: %w", kind, err)
	}

	return templates, files, nil
}
