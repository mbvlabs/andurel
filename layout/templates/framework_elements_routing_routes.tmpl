// Package routing provides implementations of different kinds of routes.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package routing

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func configureName(name, prefix string) string {
	if prefix == "" {
		return name
	}

	return prefix + "." + name
}

func configurePath(path, prefix, name string) string {
	return sanitizePath(buildPath(path, prefix, name), name)
}

func buildPath(path, prefix, name string) string {
	if prefix == "" {
		return path
	}

	// Empty path means index route - just use prefix as-is
	if path == "" {
		return prefix
	}

	prefixEndsWithSlash := strings.HasSuffix(prefix, "/")
	pathStartsWithSlash := strings.HasPrefix(path, "/")

	if prefixEndsWithSlash != pathStartsWithSlash {
		return prefix + path
	}

	if prefixEndsWithSlash && pathStartsWithSlash {
		slog.Warn("unexpected prefix/path configuration. double slash between prefix and path", "route", name, "prefix", prefix, "path", path)
		return prefix + path[1:]
	}

	slog.Warn("unexpected prefix/path configuration. missing slash between prefix and path", "route", name, "prefix", prefix, "path", path)
	return prefix + "/" + path
}

func sanitizePath(path, name string) string {
	// Trim leading/trailing whitespace
	if trimmed := strings.TrimSpace(path); trimmed != path {
		slog.Warn("path contains leading/trailing whitespace. trimming", "route", name, "path", path)
		path = trimmed
	}

	// Replace backslashes with forward slashes (Windows-style paths)
	if strings.Contains(path, "\\") {
		slog.Warn("path contains backslashes. converting to forward slashes", "route", name, "path", path)
		path = strings.ReplaceAll(path, "\\", "/")
	}

	// Remove query string if present
	if idx := strings.Index(path, "?"); idx != -1 {
		slog.Warn("path contains query string. removing", "route", name, "path", path)
		path = path[:idx]
	}

	// Remove fragment if present
	if idx := strings.Index(path, "#"); idx != -1 {
		slog.Warn("path contains fragment. removing", "route", name, "path", path)
		path = path[:idx]
	}

	// Ensure path starts with '/'
	if !strings.HasPrefix(path, "/") {
		slog.Warn("path does not start with '/'. prepending '/'", "route", name, "path", path)
		path = "/" + path
	}

	// Check for double slashes and fix
	if strings.Contains(path, "//") {
		slog.Warn("path contains double slashes. removing", "route", name, "path", path)

		// Single pass: build result without consecutive slashes
		var b strings.Builder
		b.Grow(len(path))
		prevSlash := false
		for i := 0; i < len(path); i++ {
			if path[i] == '/' {
				if prevSlash {
					continue
				}
				prevSlash = true
			} else {
				prevSlash = false
			}
			b.WriteByte(path[i])
		}
		path = b.String()
	}

	// Remove trailing slash unless path is just "/"
	// Paths like "/" or "" (root) are valid, but "/articles/" should become "/articles"
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		slog.Warn("path has trailing slash. removing", "route", name, "path", path)
		path = strings.TrimSuffix(path, "/")
	}

	return path
}

type Route struct {
	name   string
	path   string
	prefix string
}

var _ SimpleRoute = (*Route)(nil)

// NewSimpleRoute creates a base route that takes no parameters
func NewSimpleRoute(path, name, prefix string) Route {
	return Route{name, path, prefix}
}

func (r Route) URL() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r Route) Name() string {
	return configureName(r.name, r.prefix)
}

func (r Route) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

type RouteWithUUIDID Route

var _ UUIDIDRoute = (*RouteWithUUIDID)(nil)

// NewRouteWithUUIDID creates an id route that takes a uuid as a parameter
func NewRouteWithUUIDID(path, name, prefix string) RouteWithUUIDID {
	return RouteWithUUIDID{name, path, prefix}
}

func (r RouteWithUUIDID) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithUUIDID) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithUUIDID) URL(id uuid.UUID) string {
	return strings.Replace(r.Path(), ":id", id.String(), 1)
}

type RouteWithSerialID Route

var _ SerialIDRoute = (*RouteWithSerialID)(nil)

// NewRouteWithSerialID creates an id route that takes an int32 as a parameter
func NewRouteWithSerialID(path, name, prefix string) RouteWithSerialID {
	return RouteWithSerialID{name, path, prefix}
}

func (r RouteWithSerialID) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithSerialID) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithSerialID) URL(id int32) string {
	return strings.Replace(r.Path(), ":id", strconv.Itoa(int(id)), 1)
}

type RouteWithBigSerialID Route

var _ BigSerialIDRoute = (*RouteWithBigSerialID)(nil)

// NewRouteWithBigSerialID creates an id route that takes an int64 as a parameter
func NewRouteWithBigSerialID(path, name, prefix string) RouteWithBigSerialID {
	return RouteWithBigSerialID{name, path, prefix}
}

func (r RouteWithBigSerialID) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithBigSerialID) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithBigSerialID) URL(id int64) string {
	return strings.Replace(r.Path(), ":id", strconv.FormatInt(id, 10), 1)
}

type RouteWithStringID Route

var _ StringIDRoute = (*RouteWithStringID)(nil)

// NewRouteWithStringID creates an id route that takes a string as a parameter
func NewRouteWithStringID(path, name, prefix string) RouteWithStringID {
	return RouteWithStringID{name, path, prefix}
}

func (r RouteWithStringID) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithStringID) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithStringID) URL(id string) string {
	return strings.Replace(r.Path(), ":id", id, 1)
}

type RouteWithIDs Route

var _ IDsRoute = (*RouteWithIDs)(nil)

// NewRouteWithMultipleIDs creates a route that takes a map[string]uuid as parameters
func NewRouteWithMultipleIDs(path, name, prefix string) RouteWithIDs {
	return RouteWithIDs{name, path, prefix}
}

func (r RouteWithIDs) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithIDs) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithIDs) URL(ids map[string]uuid.UUID) string {
	route := r.Path()
	for param, id := range ids {
		route = strings.Replace(route, ":"+param, id.String(), 1)
	}

	return route
}

type RouteWithSlug Route

var _ ParamRoute = (*RouteWithSlug)(nil)

func NewRouteWithSlug(path, name, prefix string) RouteWithSlug {
	return RouteWithSlug{name, path, prefix}
}

func (r RouteWithSlug) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithSlug) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithSlug) URL(slug string) string {
	return strings.Replace(r.Path(), ":slug", slug, 1)
}

type RouteWithToken Route

var _ ParamRoute = (*RouteWithToken)(nil)

func NewRouteWithToken(path, name, prefix string) RouteWithToken {
	return RouteWithToken{name, path, prefix}
}

func (r RouteWithToken) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithToken) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithToken) URL(token string) string {
	return strings.Replace(r.Path(), ":token", token, 1)
}

type RouteWithFile Route

var _ ParamRoute = (*RouteWithFile)(nil)

func NewRouteWithFile(path, name, prefix string) RouteWithFile {
	return RouteWithFile{name, path, prefix}
}

func (r RouteWithFile) Name() string {
	return configureName(r.name, r.prefix)
}

func (r RouteWithFile) Path() string {
	return configurePath(r.path, r.prefix, r.name)
}

func (r RouteWithFile) URL(file string) string {
	return strings.Replace(r.Path(), ":file", file, 1)
}
