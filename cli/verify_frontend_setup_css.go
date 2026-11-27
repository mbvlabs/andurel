package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// verifyFrontendCSS follows static local module imports, not text matches or
// dynamic imports in arbitrary control flow. Unknown module forms block proof
// instead of claiming that an unreachable stylesheet is wired into the app.
func verifyFrontendCSS(root string, entries []string) ([]verifyViolation, string) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, "cannot resolve project CSS entry directory"
	}
	base := filepath.Join(root, "css", "base.css")
	if info, err := os.Stat(base); err != nil || info.IsDir() {
		return []verifyViolation{frontendViolation("css/base.css", "the shared CSS entry is missing or unreadable", "Create root-level css/base.css and import it from the frontend entry.")}, ""
	}
	var violations []verifyViolation
	var blocked []string
	for _, entry := range entries {
		if !filepath.IsAbs(entry) {
			entry = filepath.Join(root, entry)
		}
		seen := map[string]bool{}
		var uncertain bool
		var visit func(string) bool
		visit = func(file string) bool {
			file = filepath.Clean(file)
			if file == base {
				return true
			}
			if seen[file] {
				return false
			}
			seen[file] = true
			switch filepath.Ext(file) {
			case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".mts", ".cjs", ".cts":
			default:
				// CSS imports are terminal unless they are the root entry.
				// Component-file scripts need adapter parsing to prove transitively.
				if filepath.Ext(file) != ".css" {
					uncertain = true
				}
				return false
			}
			src, err := os.ReadFile(file)
			if err != nil {
				uncertain = true
				return false
			}
			tokens := lexTypeScript(string(src))
			for i := 0; i+1 < len(tokens); i++ {
				if tokens[i].Text == "import" && tokens[i+1].Text == "(" {
					uncertain = true
				}
			}
			for _, specifier := range frontendStaticImports(tokens) {
				if strings.ContainsAny(specifier, "?\\") {
					uncertain = true
					continue
				}
				target := ""
				switch {
				case strings.HasPrefix(specifier, "."):
					target = filepath.Join(filepath.Dir(file), filepath.FromSlash(specifier))
				case strings.HasPrefix(specifier, "/"):
					target = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(specifier, "/")))
				default:
					// tsconfig paths do not prove that Vite resolves an alias.
					// Only the actual resolver can prove these imports; bare
					// dependencies and aliases cannot establish our CSS entry.
					uncertain = true
					continue
				}
				candidates := []string{target}
				if filepath.Ext(target) == "" {
					for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".mts"} {
						candidates = append(candidates, target+ext, filepath.Join(target, "index"+ext))
					}
				}
				found := false
				for _, candidate := range candidates {
					if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
						found = true
						if visit(candidate) {
							return true
						}
						break
					}
				}
				if !found {
					uncertain = true
				}
			}
			return false
		}
		if visit(entry) {
			continue
		}
		rel, _ := filepath.Rel(root, entry)
		rel = filepath.ToSlash(rel)
		if uncertain {
			blocked = append(blocked, rel+": cannot prove a static import path to css/base.css; import the shared entry directly or through local JS/TS modules")
		} else {
			violations = append(violations, frontendViolation(rel, "frontend entry does not import root css/base.css", "Import css/base.css from this entry, directly or through a static local module import."))
		}
	}
	return violations, strings.Join(blocked, "; ")
}

func frontendStaticImports(tokens []tsToken) []string {
	var imports []string
	depth := 0
	for i, tok := range tokens {
		if tok.Text == "{" {
			depth++
		}
		if tok.Text == "}" {
			depth--
		}
		if depth != 0 || tok.Kind != tsIdent || (tok.Text != "import" && tok.Text != "export") || i+1 >= len(tokens) {
			continue
		}
		next := tokens[i+1]
		if next.Text == "type" || next.Text == "(" || next.Text == "." {
			continue
		}
		if tok.Text == "export" && next.Text != "{" && next.Text != "*" {
			continue
		}
		if next.Text == "{" {
			allTypes := true
			for j := i + 2; j < len(tokens) && tokens[j].Text != "}"; {
				if tokens[j].Text != "type" {
					allTypes = false
					break
				}
				for j < len(tokens) && tokens[j].Text != "," && tokens[j].Text != "}" {
					j++
				}
				if j < len(tokens) && tokens[j].Text == "," {
					j++
				}
			}
			if allTypes {
				continue
			}
		}
		if tok.Text == "import" && next.Kind == tsString {
			imports = append(imports, next.Text)
			continue
		}
		if next.Text != "{" && next.Text != "*" && next.Kind != tsIdent {
			continue
		}
		for j := i + 1; j+1 < len(tokens); j++ {
			if tokens[j].Text == ";" || (j > i+1 && tokens[j].Line > tok.Line && tokens[j].Text == "import") {
				break
			}
			if tokens[j].Text == "from" && tokens[j+1].Kind == tsString {
				imports = append(imports, tokens[j+1].Text)
				break
			}
		}
	}
	return imports
}
