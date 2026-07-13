// Package naming transforms resource, identifier, and database names.
package naming

import (
	"fmt"
	"regexp"
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

// ToLowerCamelCaseFromAny converts snake_case or PascalCase into lower camelCase.
// It preserves existing camelCase inputs by only lowercasing the first character
// when there are no underscores.
func ToLowerCamelCaseFromAny(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, "_") {
		return ToCamelCase(s)
	}
	return ToLowerCamelCase(s)
}

// ToKebabCase converts a snake_case identifier into kebab-case.
func ToKebabCase(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// Capitalize performs capitalize.
func Capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// ToPascalCase converts a snake_case identifier into PascalCase.
// Examples: "admin_users" -> "AdminUsers", "product_categories" -> "ProductCategories"
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))

	// Capitalize first letter of each part
	for _, part := range parts {
		if len(part) > 0 {
			builder.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				builder.WriteString(strings.ToLower(part[1:]))
			}
		}
	}

	return builder.String()
}

// ParseNamespacedResource splits a resource name into an optional single-level
// namespace and resource name. Namespaces are intentionally lower-case Go
// package names; resource names remain the original PascalCase generator input.
func ParseNamespacedResource(input string) (namespace, resource string, err error) {
	parts := strings.Split(input, "/")
	switch len(parts) {
	case 1:
		if parts[0] == "" {
			return "", "", fmt.Errorf("resource name cannot be empty")
		}
		return "", parts[0], nil
	case 2:
		namespace, resource = parts[0], parts[1]
		if namespace == "" || resource == "" {
			return "", "", fmt.Errorf("invalid namespaced resource %q: namespace and resource name are required", input)
		}
		if !IsValidNamespace(namespace) {
			return "", "", fmt.Errorf("invalid namespace %q: namespace must be a valid Go package name and not a reserved path", namespace)
		}
		return namespace, resource, nil
	default:
		return "", "", fmt.Errorf("invalid namespaced resource %q: only single-level namespaces are supported", input)
	}
}

// NamespaceFromResource splits a namespaced resource name into its namespace
// and resource name parts. Invalid namespaced input is returned as an
// unnamespaced resource for backward-compatible callers that cannot surface an
// error; command paths should use ParseNamespacedResource.
func NamespaceFromResource(resourceName string) (namespace, name string) {
	namespace, name, err := ParseNamespacedResource(resourceName)
	if err != nil {
		return "", resourceName
	}
	return namespace, name
}

// IsValidNamespace validates that a namespace is a valid Go package name and not
// a reserved path.
func IsValidNamespace(namespace string) bool {
	if namespace == "" {
		return true
	}
	if namespace == "controllers" || namespace == "routes" || namespace == "router" || namespace == "views" || namespace == "models" {
		return false
	}
	valid := regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	return valid.MatchString(namespace)
}

// ControllerPackageName returns the package name for a controller file given an
// optional namespace. Empty namespace yields "controllers".
func ControllerPackageName(namespace string) string {
	if namespace == "" {
		return "controllers"
	}
	if strings.Contains(namespace, "/") {
		parts := strings.Split(namespace, "/")
		return parts[len(parts)-1]
	}
	return namespace
}

// NamespacedControllerImportPath returns the import path for a controller
// package, optionally namespaced.
func NamespacedControllerImportPath(modulePath, namespace string) string {
	if namespace == "" {
		return modulePath + "/controllers"
	}
	return modulePath + "/controllers/" + namespace
}

// NamespaceToPascal converts a slash-separated namespace path into a PascalCase
// prefix suitable for route variables and generated view symbols.
func NamespaceToPascal(namespace string) string {
	if namespace == "" {
		return ""
	}
	parts := strings.Split(namespace, "/")
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(ToPascalCase(part))
	}
	return builder.String()
}

// NamespaceRouteName converts a slash-separated namespace path into a dotted
// route-name prefix.
func NamespaceRouteName(namespace string) string {
	return strings.ReplaceAll(namespace, "/", ".")
}

// NamespaceFilePrefix converts a slash-separated namespace path into the file
// prefix used for same-package generated artifacts such as route and view files.
func NamespaceFilePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	return strings.ReplaceAll(namespace, "/", "_") + "_"
}

// DeriveResourceName converts a snake_case plural table name into a PascalCase singular resource name.
// Examples: "user_roles" -> "UserRole", "products" -> "Product", "admin_users" -> "AdminUser"
func DeriveResourceName(tableName string) string {
	singular := inflection.Singular(tableName)
	return ToPascalCase(singular)
}

// ToReceiverName generates a short receiver name from a PascalCase identifier
// by extracting and lowercasing all uppercase letters.
// Examples: "StudentFeedback" -> "sf", "Product" -> "p", "UserRole" -> "ur"
func ToReceiverName(s string) string {
	if s == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range s {
		if unicode.IsUpper(r) {
			builder.WriteRune(unicode.ToLower(r))
		}
	}

	result := builder.String()
	if result == "" {
		// Fallback: use first character lowercased
		return strings.ToLower(s[:1])
	}
	return result
}
