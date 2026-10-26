package env

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Load reads KEY=VALUE pairs from filenames and sets them in the process
// environment. Keys already present in the process environment win. With no
// filenames it loads ".env", and a missing default file is not an error.
func Load(filenames ...string) error {
	explicit := len(filenames) > 0
	if !explicit {
		filenames = []string{".env"}
	}

	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			if !explicit && errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return fmt.Errorf("env: load: %w", err)
		}

		values, err := parse(filename, data)
		if err != nil {
			return fmt.Errorf("env: load: %w", err)
		}
		for key, value := range values {
			if _, ok := os.LookupEnv(key); ok {
				continue
			}
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("env: load: %w", err)
			}
		}
	}

	return nil
}

func parse(filename string, data []byte) (map[string]string, error) {
	values := make(map[string]string)
	for index, raw := range strings.Split(string(data), "\n") {
		lineNo := index + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = trimExport(line)

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", filename, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", filename, lineNo)
		}

		value, err := parseValue(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", filename, lineNo, err)
		}
		values[key] = value
	}

	return values, nil
}

func trimExport(line string) string {
	rest, ok := strings.CutPrefix(line, "export")
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return line
	}

	return strings.TrimLeft(rest, " \t")
}

func parseValue(raw string) (string, error) {
	switch {
	case strings.HasPrefix(raw, "'"):
		return parseQuoted(raw, '\'', false)
	case strings.HasPrefix(raw, `"`):
		return parseQuoted(raw, '"', true)
	}

	for index := 1; index < len(raw); index++ {
		if raw[index] == '#' && (raw[index-1] == ' ' || raw[index-1] == '\t') {
			raw = raw[:index]
			break
		}
	}

	return strings.TrimSpace(raw), nil
}

func parseQuoted(raw string, quote byte, escapes bool) (string, error) {
	var value strings.Builder
	for index := 1; index < len(raw); index++ {
		char := raw[index]
		if escapes && char == '\\' && index+1 < len(raw) {
			switch raw[index+1] {
			case 'n':
				value.WriteByte('\n')
				index++
				continue
			case '"':
				value.WriteByte('"')
				index++
				continue
			case '\\':
				value.WriteByte('\\')
				index++
				continue
			}
		}
		if char == quote {
			if err := checkTrailing(raw[index+1:]); err != nil {
				return "", err
			}

			return value.String(), nil
		}
		value.WriteByte(char)
	}

	return "", fmt.Errorf("unterminated quoted value")
}

func checkTrailing(rest string) error {
	rest = strings.TrimSpace(rest)
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("unexpected %q after quoted value", rest)
	}

	return nil
}
