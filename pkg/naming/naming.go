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

// ToCamelCase converts a snake_case identifier into camelCase.
// Examples: "admin_users" -> "adminUsers", "product_categories" -> "productCategories"
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}

	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))

	// First part stays lowercase
	builder.WriteString(strings.ToLower(parts[0]))

	// Capitalize first letter of remaining parts
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			builder.WriteString(strings.ToUpper(parts[i][:1]))
			if len(parts[i]) > 1 {
				builder.WriteString(strings.ToLower(parts[i][1:]))
			}
		}
	}

	return builder.String()
}

// ToLowerCamelCase converts a PascalCase identifier into camelCase by lowercasing the first character.
// Examples: "NewUser" -> "newUser", "AdminUser" -> "adminUser", "User" -> "user"
func ToLowerCamelCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}

	// Convert first character to lowercase
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func Capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
