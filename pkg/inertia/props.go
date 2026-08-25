// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

// Props is the data exposed to an Inertia page. Policy constructors can be
// freely composed around individual values.
type Props map[string]any

// Set adds or replaces a prop and returns p for fluent response construction.
func (p Props) Set(key string, value any) Props {
	if p != nil {
		p[key] = value
	}
	return p
}

// FromStruct converts exported JSON fields on a struct into Props. It is
// intended for generated backend-owned page payloads. Invalid inputs produce
// empty props; use FromStructChecked when the input is dynamic.
func FromStruct(value any) Props {
	props, _ := FromStructChecked(value)
	return props
}

// FromStructChecked is the error-returning form of FromStruct.
func FromStructChecked(value any) (Props, error) {
	result := make(Props)
	rv := reflect.ValueOf(value)
	for rv.IsValid() && rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return result, nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("inertia: FromStruct requires a struct, got %T", value)
	}
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, include := jsonFieldName(field)
		if !include {
			continue
		}
		if field.Anonymous && name == "" {
			embedded, err := FromStructChecked(rv.Field(i).Interface())
			if err != nil {
				return nil, err
			}
			maps.Copy(result, embedded)
			continue
		}
		if name == "" {
			name = field.Name
		}
		result[name] = rv.Field(i).Interface()
	}
	return result, nil
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	tag, exists := field.Tag.Lookup("json")
	if !exists {
		return field.Name, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	return name, true
}

// Resolver is a request-scoped lazy prop callback.
type Resolver func(*echo.Context) (any, error)

// PropResolver may be implemented by application-defined lazy values.
type PropResolver interface {
	ResolveInertiaProp(*echo.Context) (any, error)
}

// Prop is a composable response-boundary policy around a value or resolver.
type Prop struct {
	value any

	always   bool
	optional bool
	deferred bool
	group    string
	rescue   bool

	merge     bool
	prepend   bool
	deepMerge bool
	matchOn   []string

	once       bool
	onceKey    string
	expiresAt  *time.Time
	expiresFor *time.Duration
	forceFresh bool

	scroll any
	// scrollWrapper is the nested collection merged by infinite scrolling.
	scrollWrapper string
}

func asProp(value any) Prop {
	if prop, ok := value.(Prop); ok {
		return prop
	}
	return Prop{value: value}
}

// Always makes a prop survive partial only/except filtering.
func Always(value any) Prop {
	prop := asProp(value)
	prop.always = true
	prop.optional = false
	return prop
}

// Optional omits a prop on full visits and resolves it when explicitly selected.
func Optional(value any) Prop {
	prop := asProp(value)
	prop.optional = true
	return prop
}

// DeferredOption configures a deferred prop.
type DeferredOption func(*Prop)

// InGroup assigns a deferred prop to a client reload group.
func InGroup(group string) DeferredOption {
	return func(prop *Prop) {
		if group != "" {
			prop.group = group
		}
	}
}

// Rescue marks a deferred resolver failure as recoverable.
func Rescue() DeferredOption { return func(prop *Prop) { prop.rescue = true } }

// Deferred announces a prop on full visits and resolves it on a selected reload.
func Deferred(value any, opts ...DeferredOption) Prop {
	prop := asProp(value)
	prop.deferred = true
	prop.optional = true
	if prop.group == "" {
		prop.group = "default"
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&prop)
		}
	}
	return prop
}

// Merge appends the incoming value to the retained client prop.
func Merge(value any) Prop {
	prop := asProp(value)
	prop.merge = true
	prop.prepend = false
	prop.deepMerge = false
	return prop
}

// Prepend prepends the incoming value to the retained client prop.
func Prepend(value any) Prop {
	prop := asProp(value)
	prop.merge = true
	prop.prepend = true
	prop.deepMerge = false
	return prop
}

// DeepMerge recursively merges the incoming value into the retained client prop.
func DeepMerge(value any) Prop {
	prop := asProp(value)
	prop.merge = true
	prop.deepMerge = true
	return prop
}

// MatchOn adds matching paths for a merge prop.
func MatchOn(value any, paths ...string) Prop {
	prop := asProp(value)
	prop.merge = true
	prop.matchOn = append(prop.matchOn, cleanPaths(paths)...)
	return prop
}

// OnceOption configures a once prop.
type OnceOption func(*Prop)

// OnceKey sets a stable retention key. The prop path is used by default.
func OnceKey(key string) OnceOption { return func(prop *Prop) { prop.onceKey = key } }

// OnceExpiresAt sets an absolute client retention expiry.
func OnceExpiresAt(expiry time.Time) OnceOption {
	return func(prop *Prop) { prop.expiresAt = &expiry }
}

// OnceFor sets a retention expiry relative to prop evaluation time.
func OnceFor(duration time.Duration) OnceOption {
	return func(prop *Prop) {
		prop.expiresFor = &duration
	}
}

// ForceFresh makes the server resolve a once prop even if the client retained it.
func ForceFresh() OnceOption { return func(prop *Prop) { prop.forceFresh = true } }

// Once retains a resolved prop on the client across subsequent pages.
func Once(value any, opts ...OnceOption) Prop {
	prop := asProp(value)
	prop.once = true
	for _, opt := range opts {
		if opt != nil {
			opt(&prop)
		}
	}
	return prop
}

// ProvidesScrollMetadata supplies infinite-scroll cursor information.
type ProvidesScrollMetadata interface {
	GetPageName() string
	GetPreviousPage() any
	GetNextPage() any
	GetCurrentPage() any
}

// ScrollMetadataResolver derives metadata from the resolved prop value.
type ScrollMetadataResolver func(*echo.Context, any) (ScrollMetadata, error)

// Scroll adds infinite-scroll metadata and merge behavior to a prop.
func Scroll(value, metadata any) Prop {
	return ScrollAt(value, "data", metadata)
}

// ScrollAt adds infinite-scroll metadata and merges a nested collection path.
func ScrollAt(value any, wrapper string, metadata any) Prop {
	prop := asProp(value)
	prop.merge = true
	prop.scroll = metadata
	prop.scrollWrapper = strings.Trim(strings.TrimSpace(wrapper), ".")
	return prop
}

func cleanPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			result = append(result, path)
		}
	}
	return result
}
