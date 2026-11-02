package tui

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

const ansiESC = '\x1b'

var (
	styleTime     = lipgloss.NewStyle().Faint(true)
	stylePrefix   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleKey      = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Faint(true)
	styleString   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleNumber   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleBool     = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	stylePunct    = lipgloss.NewStyle().Faint(true)
	styleURL      = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Underline(true)
	styleFile     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleDuration = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleDebug    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleInfo     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleGet      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	stylePost     = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	stylePut      = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleDelete   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	stylePatch    = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
)

var (
	rePrefix   = regexp.MustCompile(`^\[[A-Za-z][\w./:-]*\]`)
	reTime     = regexp.MustCompile(`^(?:\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?|\d{2}:\d{2}:\d{2}(?:\.\d+)?)`)
	reLevel    = regexp.MustCompile(`^(?i)(?:DEBUG|INFO|WARN(?:ING)?|ERROR|FATAL|TRACE|DBG|INF|WRN|ERR|FTL)\b`)
	reHTTP     = regexp.MustCompile(`^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\b`)
	reKeyEq    = regexp.MustCompile(`^[A-Za-z_][\w.-]*=`)
	reString   = regexp.MustCompile(`^"(?:\\.|[^"\\])*"`)
	reURL      = regexp.MustCompile(`^https?://[^\s]+`)
	reDuration = regexp.MustCompile(`^\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h)\b`)
	reGoPos    = regexp.MustCompile(`^(?:\.?/?[\w./-]+\.go):\d+(?::\d+)?`)
	reStatus   = regexp.MustCompile(`^[1-5]\d{2}\b`)
	reNumber   = regexp.MustCompile(`^-?\d+(?:\.\d+)?\b`)
)

// HighlightLines colors each log line for the TUI viewport.
func HighlightLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = HighlightLine(line)
	}
	return strings.Join(out, "\n")
}

// HighlightLine adds syntax highlighting. Lines that already contain ANSI are left alone.
func HighlightLine(line string) string {
	if line == "" || strings.IndexByte(line, ansiESC) >= 0 {
		return line
	}
	if brace := strings.Index(line, "{"); brace >= 0 {
		payload := strings.TrimSpace(line[brace:])
		if json.Valid([]byte(payload)) {
			return line[:brace] + highlightJSON(payload)
		}
	}
	return highlightLogText(line)
}

func highlightLogText(line string) string {
	var b strings.Builder
	b.Grow(len(line) * 2)
	i := 0
	sawHTTP := false
	for i < len(line) {
		rest := line[i:]
		if m := rePrefix.FindString(rest); m != "" {
			b.WriteString(stylePrefix.Render(m))
			i += len(m)
			continue
		}
		if m := reTime.FindString(rest); m != "" {
			b.WriteString(styleTime.Render(m))
			i += len(m)
			continue
		}
		if m := reLevel.FindString(rest); m != "" {
			b.WriteString(levelStyle(m).Render(m))
			i += len(m)
			continue
		}
		if m := reHTTP.FindString(rest); m != "" {
			b.WriteString(httpStyle(m).Render(m))
			i += len(m)
			sawHTTP = true
			continue
		}
		if sawHTTP {
			if m := reStatus.FindString(rest); m != "" && (i == 0 || isBoundary(line[i-1])) {
				b.WriteString(statusStyle(m).Render(m))
				i += len(m)
				continue
			}
		}
		if m := reURL.FindString(rest); m != "" {
			b.WriteString(styleURL.Render(m))
			i += len(m)
			continue
		}
		if m := reGoPos.FindString(rest); m != "" {
			b.WriteString(styleFile.Render(m))
			i += len(m)
			continue
		}
		if m := reDuration.FindString(rest); m != "" {
			b.WriteString(styleDuration.Render(m))
			i += len(m)
			continue
		}
		if m := reString.FindString(rest); m != "" {
			b.WriteString(styleString.Render(m))
			i += len(m)
			continue
		}
		if m := reKeyEq.FindString(rest); m != "" {
			key := m[:len(m)-1]
			b.WriteString(styleKey.Render(key))
			b.WriteString(stylePunct.Render("="))
			i += len(m)
			continue
		}
		if m := reNumber.FindString(rest); m != "" && (i == 0 || isBoundary(line[i-1])) {
			b.WriteString(styleNumber.Render(m))
			i += len(m)
			continue
		}
		r, size := utf8.DecodeRuneInString(rest)
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

func highlightJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	i := 0
	expectKey := false
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			b.WriteByte(c)
			i++
		case '{':
			expectKey = true
			b.WriteString(stylePunct.Render("{"))
			i++
		case '[':
			expectKey = false
			b.WriteString(stylePunct.Render("["))
			i++
		case '}':
			expectKey = false
			b.WriteString(stylePunct.Render("}"))
			i++
		case ']':
			expectKey = false
			b.WriteString(stylePunct.Render("]"))
			i++
		case ',':
			expectKey = true
			b.WriteString(stylePunct.Render(","))
			i++
		case ':':
			expectKey = false
			b.WriteString(stylePunct.Render(":"))
			i++
		case '"':
			str, n := scanJSONString(s[i:])
			if expectKey {
				b.WriteString(styleKey.Render(str))
			} else {
				b.WriteString(styleString.Render(str))
			}
			i += n
		default:
			if lit, n, kind := scanJSONLiteral(s[i:]); n > 0 {
				switch kind {
				case "bool", "null":
					b.WriteString(styleBool.Render(lit))
				default:
					b.WriteString(styleNumber.Render(lit))
				}
				i += n
				continue
			}
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

func scanJSONString(s string) (string, int) {
	if len(s) == 0 || s[0] != '"' {
		return s, len(s)
	}
	esc := false
	for i := 1; i < len(s); i++ {
		if esc {
			esc = false
			continue
		}
		if s[i] == '\\' {
			esc = true
			continue
		}
		if s[i] == '"' {
			return s[:i+1], i + 1
		}
	}
	return s, len(s)
}

func scanJSONLiteral(s string) (string, int, string) {
	switch {
	case strings.HasPrefix(s, "true"):
		return "true", 4, "bool"
	case strings.HasPrefix(s, "false"):
		return "false", 5, "bool"
	case strings.HasPrefix(s, "null"):
		return "null", 4, "null"
	}
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	start := i
	for i < len(s) && (s[i] == '.' || s[i] == 'e' || s[i] == 'E' || s[i] == '+' || s[i] == '-' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == start {
		return "", 0, ""
	}
	return s[:i], i, "number"
}

func levelStyle(level string) lipgloss.Style {
	switch strings.ToUpper(level) {
	case "DEBUG", "DBG", "TRACE":
		return styleDebug
	case "WARN", "WARNING", "WRN":
		return styleWarn
	case "ERROR", "ERR", "FATAL", "FTL":
		return styleError
	default:
		return styleInfo
	}
}

func httpStyle(method string) lipgloss.Style {
	switch method {
	case "POST":
		return stylePost
	case "PUT":
		return stylePut
	case "PATCH":
		return stylePatch
	case "DELETE":
		return styleDelete
	default:
		return styleGet
	}
}

func statusStyle(code string) lipgloss.Style {
	switch code[0] {
	case '2':
		return styleInfo
	case '3':
		return styleDebug
	case '4':
		return styleWarn
	default:
		return styleError
	}
}

func isBoundary(b byte) bool {
	return b == ' ' || b == '\t' || b == '"' || b == '=' || b == ':' || b == ',' || b == '[' || b == '{'
}
