// Package patterns embeds the read-only Andurel pattern library.
//
// Patterns describe the complete shape of a primitive — the files to write,
// the placement rules, and a definition-of-done checklist — so an agent can
// author application-owned code by hand instead of running a code-writing
// generator. Patterns never write files.
package patterns

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

//go:embed assets/*.md
var assets embed.FS

// File is one reference file rendered by a pattern.
type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Content  string `json:"content"`
}

// Pattern is the complete, machine-readable description of a primitive.
type Pattern struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Files     []File   `json:"files"`
	Rules     []string `json:"rules"`
	Checklist []string `json:"checklist"`

	markdown string
}

// Markdown returns the raw embedded Markdown for human rendering.
func (p Pattern) Markdown() string { return p.markdown }

var (
	once    sync.Once
	all     []Pattern
	byID    map[string]Pattern
	loadErr error
)

func load() {
	once.Do(func() {
		entries, err := fs.ReadDir(assets, "assets")
		if err != nil {
			loadErr = err
			return
		}
		byID = make(map[string]Pattern, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".md") {
				continue
			}
			raw, err := fs.ReadFile(assets, "assets/"+name)
			if err != nil {
				loadErr = err
				return
			}
			pattern := parseMarkdown(strings.TrimSuffix(name, ".md"), string(raw))
			all = append(all, pattern)
			byID[pattern.ID] = pattern
		}
		sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	})
}

// Err reports any failure to load the embedded pattern library.
func Err() error {
	load()
	return loadErr
}

// All returns every pattern in stable alphabetical order.
func All() []Pattern {
	load()
	return append([]Pattern(nil), all...)
}

// IDs returns every pattern id in stable alphabetical order.
func IDs() []string {
	load()
	ids := make([]string, 0, len(all))
	for _, pattern := range all {
		ids = append(ids, pattern.ID)
	}
	return ids
}

// Get returns the pattern with the given id.
func Get(id string) (Pattern, bool) {
	load()
	pattern, ok := byID[id]
	return pattern, ok
}

// parseMarkdown reads the small, stable Markdown contract used by the embedded
// assets:
//
//	# Title
//	<summary>
//	## Files
//	### path/to/file.go
//	```go
//	...
//	```
//	## Rules
//	- rule
//	## Checklist
//	- [ ] item
func parseMarkdown(id, raw string) Pattern {
	pattern := Pattern{
		ID:        id,
		Files:     []File{},
		Rules:     []string{},
		Checklist: []string{},
		markdown:  raw,
	}

	section := ""
	summary := make([]string, 0)
	filePath := ""
	fileLang := ""
	fileBody := make([]string, 0)
	inFence := false

	flushFile := func() {
		if filePath == "" {
			return
		}
		content := strings.Join(fileBody, "\n")
		content = strings.TrimRight(content, "\n")
		if content != "" {
			content += "\n"
		}
		pattern.Files = append(pattern.Files, File{
			Path:     filePath,
			Language: fileLang,
			Content:  content,
		})
		filePath, fileLang, fileBody = "", "", nil
	}

	for line := range strings.SplitSeq(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, "```") {
				inFence = false
				continue
			}
			fileBody = append(fileBody, line)
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "### ") && section == "files":
			flushFile()
			filePath = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
		case strings.HasPrefix(trimmed, "## "):
			flushFile()
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
		case strings.HasPrefix(trimmed, "# ") && pattern.Title == "":
			pattern.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		case strings.HasPrefix(trimmed, "```"):
			fileLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			inFence = true
		default:
			switch section {
			case "":
				summary = append(summary, strings.TrimRight(line, " \t"))
			case "rules":
				if item, ok := bullet(trimmed); ok {
					pattern.Rules = append(pattern.Rules, item)
				}
			case "checklist":
				if item, ok := bullet(trimmed); ok {
					pattern.Checklist = append(pattern.Checklist, stripCheckbox(item))
				}
			}
		}
	}
	flushFile()
	pattern.Summary = strings.TrimSpace(strings.Join(summary, "\n"))
	return pattern
}

func bullet(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "- "):
		return strings.TrimSpace(strings.TrimPrefix(line, "- ")), true
	case strings.HasPrefix(line, "* "):
		return strings.TrimSpace(strings.TrimPrefix(line, "* ")), true
	}
	return "", false
}

func stripCheckbox(item string) string {
	item = strings.TrimSpace(strings.TrimPrefix(item, "[ ]"))
	item = strings.TrimSpace(strings.TrimPrefix(item, "[x]"))
	item = strings.TrimSpace(strings.TrimPrefix(item, "[X]"))
	return item
}
