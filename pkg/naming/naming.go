package naming

import (
	"strings"
	"unicode"

	"github.com/jinzhu/inflection"
)

// DeriveTableName converts a CamelCase resource name into a snake_case plural table name.
func DeriveTableName(resourceName string) string {
	snake := ToSnakeCase(resourceName)
	return inflection.Plural(snake)
}

// ToSnakeCase converts a CamelCase identifier into snake_case.
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var builder strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
					(unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
					builder.WriteByte('_')
				}
			}
			builder.WriteRune(unicode.ToLower(r))
			continue
		}

		builder.WriteRune(r)
	}

	return builder.String()
}
