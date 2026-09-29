package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/generator/internal/types"
	"github.com/mbvlabs/andurel/v2/internal/naming"
)

// CustomField describes one field on a non-table-backed model.
type CustomField struct {
	Name       string // PascalCase Go field name
	Type       string // Go type
	ColumnName string // snake_case andurel tag
}

// CustomModelData is the template payload for model_custom.tmpl.
type CustomModelData struct {
	EntityName      string
	NamespaceVar    string
	NamespaceType   string
	ReceiverName    string
	ModulePath      string
	Fields          []CustomField
	StandardImports []string
	ExternalImports []string
}

var customTypeAliases = map[string]struct {
	goType string
	imp    string
}{
	"uuid":        {"uuid.UUID", "uuid"},
	"string":      {"string", ""},
	"text":        {"string", ""},
	"int":         {"int32", ""},
	"int32":       {"int32", ""},
	"int64":       {"int64", ""},
	"bool":        {"bool", ""},
	"bytes":       {"[]byte", ""},
	"float":       {"float64", ""},
	"float64":     {"float64", ""},
	"time":        {"pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"},
	"timestamptz": {"pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"},
	"date":        {"pgtype.Date", "github.com/jackc/pgx/v5/pgtype"},
	"json":        {"[]byte", ""},
	"jsonb":       {"[]byte", ""},
}

// ParseCustomFieldSpecs parses name:type field specs for custom model generation.
func ParseCustomFieldSpecs(specs []string) ([]CustomField, []string, []string, error) {
	if len(specs) == 0 {
		return nil, nil, nil, fmt.Errorf(
			"--custom requires at least one field:type specification",
		)
	}

	fields := make([]CustomField, 0, len(specs))
	importSet := make(map[string]bool)
	seenColumns := make(map[string]bool)
	seenNames := make(map[string]bool)

	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return nil, nil, nil, fmt.Errorf("empty field specification")
		}

		namePart, typePart, ok := strings.Cut(spec, ":")
		if !ok {
			return nil, nil, nil, fmt.Errorf(
				"invalid field specification %q: expected name:type",
				spec,
			)
		}
		namePart = strings.TrimSpace(namePart)
		typePart = strings.TrimSpace(typePart)
		if namePart == "" || typePart == "" {
			return nil, nil, nil, fmt.Errorf(
				"invalid field specification %q: name and type are required",
				spec,
			)
		}

		columnName := naming.ToSnakeCase(namePart)
		if columnName == "" {
			return nil, nil, nil, fmt.Errorf(
				"invalid field name %q: could not derive snake_case tag",
				namePart,
			)
		}
		goName := types.FormatFieldName(columnName)
		if seenColumns[columnName] || seenNames[goName] {
			return nil, nil, nil, fmt.Errorf(
				"duplicate field %q (column %q)",
				namePart,
				columnName,
			)
		}
		seenColumns[columnName] = true
		seenNames[goName] = true

		goType, imp, err := resolveCustomFieldType(typePart)
		if err != nil {
			return nil, nil, nil, err
		}
		if imp != "" {
			importSet[imp] = true
		}

		fields = append(fields, CustomField{
			Name:       goName,
			Type:       goType,
			ColumnName: columnName,
		})
	}

	stdImports, extImports := groupAndSortCustomImports(importSet)
	return fields, stdImports, extImports, nil
}

func resolveCustomFieldType(typePart string) (goType, imp string, err error) {
	if alias, ok := customTypeAliases[strings.ToLower(typePart)]; ok {
		return alias.goType, alias.imp, nil
	}

	// Escape hatch: raw Go types that look qualified or are slices.
	if strings.Contains(typePart, ".") || strings.Contains(typePart, "[]") {
		return typePart, importForRawGoType(typePart), nil
	}

	known := make([]string, 0, len(customTypeAliases))
	for alias := range customTypeAliases {
		known = append(known, alias)
	}
	sort.Strings(known)
	return "", "", fmt.Errorf(
		"unknown type alias %q (allowed: %s; or a raw Go type containing '.' or '[]')",
		typePart,
		strings.Join(known, ", "),
	)
}

func importForRawGoType(goType string) string {
	switch {
	case strings.Contains(goType, "uuid.UUID"):
		return "uuid"
	case strings.Contains(goType, "pgtype."):
		return "github.com/jackc/pgx/v5/pgtype"
	case strings.Contains(goType, "time.Time"):
		return "time"
	case strings.Contains(goType, "json.RawMessage"):
		return "encoding/json"
	case strings.HasPrefix(goType, "sql.Null"):
		return "database/sql"
	default:
		return ""
	}
}

func groupAndSortCustomImports(importSet map[string]bool) (stdImports, extImports []string) {
	for imp := range importSet {
		if strings.Contains(imp, ".") {
			extImports = append(extImports, imp)
		} else {
			stdImports = append(stdImports, imp)
		}
	}
	sort.Strings(stdImports)
	sort.Strings(extImports)
	return stdImports, extImports
}
