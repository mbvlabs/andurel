// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

type resolvedPage struct {
	props          map[string]any
	merge          []string
	prepend        []string
	deepMerge      []string
	matchOn        []string
	scroll         map[string]ScrollMetadata
	deferred       map[string][]string
	rescued        []string
	once           map[string]OnceMetadata
	resolvedShared []string
}

type propResolver struct {
	etx       *echo.Context
	request   Request
	component string
	partial   bool
	result    resolvedPage
}

func resolvePageProps(
	etx *echo.Context,
	request Request,
	component string,
	shared, page Props,
) (resolvedPage, error) {
	if protectedPropExists(shared, "errors") {
		return resolvedPage{}, &Error{
			Kind:      ErrorProps,
			Operation: "merge shared props",
			Component: component,
			Method:    etx.Request().Method,
			URL:       requestURL(etx),
			PropPath:  "errors",
			Err:       fmt.Errorf("framework prop is protected"),
		}
	}
	if protectedPropExists(page, "errors") {
		return resolvedPage{}, &Error{
			Kind:      ErrorProps,
			Operation: "merge page props",
			Component: component,
			Method:    etx.Request().Method,
			URL:       requestURL(etx),
			PropPath:  "errors",
			Err:       fmt.Errorf("framework prop is protected"),
		}
	}

	combined := make(Props, len(shared)+len(page))
	maps.Copy(combined, shared)
	maps.Copy(combined, page)
	var err error
	combined, err = unpackDotProps(combined)
	if err != nil {
		return resolvedPage{}, &Error{
			Kind:      ErrorProps,
			Operation: "unpack dotted props",
			Component: component,
			Method:    etx.Request().Method,
			URL:       requestURL(etx),
			Err:       err,
		}
	}

	resolver := &propResolver{
		etx:       etx,
		request:   request,
		component: component,
		partial:   request.IsPartialFor(component),
		result: resolvedPage{
			props:          make(map[string]any, len(combined)+1),
			scroll:         make(map[string]ScrollMetadata),
			deferred:       make(map[string][]string),
			once:           make(map[string]OnceMetadata),
			resolvedShared: sharedTopLevelKeys(shared),
		},
	}
	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		value := combined[key]
		resolved, include, err := resolver.resolveValue(key, value, false)
		if err != nil {
			return resolvedPage{}, err
		}
		if include {
			resolver.result.props[key] = resolved
		}
	}
	sortResolvedMetadata(&resolver.result)
	return resolver.result, nil
}

func (r *propResolver) resolveValue(
	path string,
	value any,
	inheritedAlways bool,
) (any, bool, error) {
	prop, isPolicy := value.(Prop)
	dynamicallyResolved := false
	if isPolicy {
		inheritedAlways = inheritedAlways || prop.always
		if r.onceRetained(path, prop) {
			r.recordRetainedOncePolicy(path, prop)
			return nil, false, nil
		}
		if !r.shouldInclude(path, prop, inheritedAlways) {
			r.recordUnresolvedPolicy(path, prop)
			return nil, false, nil
		}
		resolved, err := evaluateProp(r.etx, prop.value)
		if err != nil {
			if prop.deferred && prop.rescue {
				r.etx.Logger().
					Error("inertia rescued deferred prop", "component", r.component, "url", requestURL(r.etx), "prop", path, "error", err)
				r.result.rescued = append(r.result.rescued, path)
				return nil, false, nil
			}
			return nil, false, &Error{
				Kind:      ErrorProps,
				Operation: "resolve",
				Component: r.component,
				Method:    r.etx.Request().Method,
				URL:       requestURL(r.etx),
				PropPath:  path,
				Err:       err,
			}
		}
		if returnedProp, ok := resolved.(Prop); ok {
			return r.resolveValue(path, returnedProp, inheritedAlways)
		}
		if err := r.recordPolicy(path, prop, resolved); err != nil {
			return nil, false, err
		}
		value = resolved
		dynamicallyResolved = true
	}

	if !isPolicy && !r.pathIncluded(path, inheritedAlways) {
		return nil, false, nil
	}
	if !isPolicy {
		resolved, lazy, err := evaluateLazyProp(r.etx, value)
		if err != nil {
			return nil, false, &Error{
				Kind:      ErrorProps,
				Operation: "resolve",
				Component: r.component,
				Method:    r.etx.Request().Method,
				URL:       requestURL(r.etx),
				PropPath:  path,
				Err:       err,
			}
		}
		if lazy {
			if returnedProp, ok := resolved.(Prop); ok {
				return r.resolveValue(path, returnedProp, inheritedAlways)
			}
			value = resolved
			dynamicallyResolved = true
		}
	}
	return r.resolveNested(path, value, inheritedAlways || dynamicallyResolved)
}

func (r *propResolver) shouldInclude(path string, prop Prop, always bool) bool {
	if (prop.deferred || prop.optional) && !r.partial {
		return false
	}
	return r.pathIncluded(path, always)
}

func (r *propResolver) pathIncluded(path string, always bool) bool {
	if always || !r.partial {
		return true
	}
	if len(r.request.Only) > 0 && !pathMatchesAny(path, r.request.Only) {
		return false
	}
	return !pathExcluded(path, r.request.Except)
}

func (r *propResolver) resolveNested(path string, value any, always bool) (any, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, true, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return value, true, nil
		}
		result := make(map[string]any, rv.Len())
		keys := make([]string, 0, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			keys = append(keys, iter.Key().String())
		}
		slices.Sort(keys)
		for _, key := range keys {
			child, include, err := r.resolveValue(
				joinPath(path, key),
				rv.MapIndex(reflect.ValueOf(key).Convert(rv.Type().Key())).Interface(),
				always,
			)
			if err != nil {
				return nil, false, err
			}
			if include {
				result[key] = child
			}
		}
		return result, true, nil
	case reflect.Struct:
		// Values with custom JSON encoders (notably time.Time) stay opaque.
		if rv.Type().Implements(reflect.TypeFor[interface{ MarshalJSON() ([]byte, error) }]()) ||
			reflect.PointerTo(rv.Type()).
				Implements(reflect.TypeFor[interface{ MarshalJSON() ([]byte, error) }]()) {
			return value, true, nil
		}
		result := make(map[string]any, rv.NumField())
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
			if name == "" {
				name = field.Name
			}
			child, childIncluded, err := r.resolveValue(
				joinPath(path, name),
				rv.Field(i).Interface(),
				always,
			)
			if err != nil {
				return nil, false, err
			}
			if childIncluded {
				result[name] = child
			}
		}
		return result, true, nil
	default:
		return value, true, nil
	}
}

func (r *propResolver) announceDeferred(path string, prop Prop) {
	if !prop.deferred || r.partial {
		return
	}
	group := prop.group
	if group == "" {
		group = "default"
	}
	r.result.deferred[group] = append(r.result.deferred[group], path)
}

func (r *propResolver) announceOnce(path string, prop Prop) {
	if !prop.once || !r.metadataIncluded(path) {
		return
	}
	key := prop.onceKey
	if key == "" {
		key = path
	}
	metadata := OnceMetadata{Prop: path}
	expiresAt := prop.expiresAt
	if prop.expiresFor != nil {
		expiry := time.Now().Add(*prop.expiresFor)
		expiresAt = &expiry
	}
	if expiresAt != nil {
		milliseconds := expiresAt.UnixMilli()
		metadata.ExpiresAt = &milliseconds
	}
	r.result.once[key] = metadata
}

func (r *propResolver) recordUnresolvedPolicy(path string, prop Prop) {
	r.announceDeferred(path, prop)
	r.announceOnce(path, prop)
	r.recordUnresolvedMergePolicy(path, prop)
}

func (r *propResolver) recordRetainedOncePolicy(path string, prop Prop) {
	r.announceOnce(path, prop)
	r.recordUnresolvedMergePolicy(path, prop)
}

func (r *propResolver) recordUnresolvedMergePolicy(path string, prop Prop) {
	// Composed category metadata is independent in v3. Deferred props do not
	// emit scroll metadata before they resolve, but their merge policy remains
	// announced so the follow-up result is applied consistently.
	if (prop.deferred || prop.optional) && prop.merge && !resetMatches(path, r.request.Reset) {
		if prop.scroll != nil {
			r.recordScrollMerge(path, prop)
		} else {
			r.recordMerge(path, prop)
		}
	}
}

func (r *propResolver) onceRetained(path string, prop Prop) bool {
	if !prop.once || !r.request.Inertia || r.partial || prop.forceFresh {
		return false
	}
	key := prop.onceKey
	if key == "" {
		key = path
	}
	return slices.Contains(r.request.ExceptOnceProps, key)
}

func (r *propResolver) recordPolicy(path string, prop Prop, resolved any) error {
	r.announceOnce(path, prop)
	if prop.deferred && !r.partial {
		r.announceDeferred(path, prop)
	}
	reset := resetMatches(path, r.request.Reset)
	if prop.scroll != nil {
		metadata, err := resolveScrollMetadata(r.etx, prop.scroll, resolved)
		if err != nil {
			return &Error{
				Kind:      ErrorProps,
				Operation: "resolve scroll metadata",
				Component: r.component,
				Method:    r.etx.Request().Method,
				URL:       requestURL(r.etx),
				PropPath:  path,
				Err:       err,
			}
		}
		metadata.Reset = reset
		r.result.scroll[path] = metadata
	}
	if reset {
		return nil
	}
	if prop.scroll != nil {
		r.recordScrollMerge(path, prop)
		return nil
	}
	if !prop.merge {
		return nil
	}
	r.recordMerge(path, prop)
	return nil
}

func (r *propResolver) recordMerge(path string, prop Prop) {
	if !r.metadataIncluded(path) {
		return
	}
	if prop.deepMerge {
		r.result.deepMerge = append(r.result.deepMerge, path)
	} else if prop.prepend || (prop.scroll != nil && r.request.MergeIntent == MergeIntentPrepend) {
		r.result.prepend = append(r.result.prepend, path)
	} else {
		r.result.merge = append(r.result.merge, path)
	}
	for _, match := range prop.matchOn {
		r.result.matchOn = append(r.result.matchOn, joinPath(path, match))
	}
}

func (r *propResolver) recordScrollMerge(path string, prop Prop) {
	if !r.metadataIncluded(path) {
		return
	}
	mergePath := joinPath(path, prop.scrollWrapper)
	if r.request.MergeIntent == MergeIntentPrepend {
		r.result.prepend = append(r.result.prepend, mergePath)
	} else {
		r.result.merge = append(r.result.merge, mergePath)
	}
	for _, match := range prop.matchOn {
		r.result.matchOn = append(r.result.matchOn, joinPath(mergePath, match))
	}
}

func (r *propResolver) metadataIncluded(path string) bool {
	if !r.partial {
		return true
	}
	if len(r.request.Only) > 0 && !matchesOnly(path, r.request.Only) {
		return false
	}
	return !pathExcluded(path, r.request.Except)
}

func resolveScrollMetadata(etx *echo.Context, provider, value any) (ScrollMetadata, error) {
	switch metadata := provider.(type) {
	case ScrollMetadata:
		return metadata, nil
	case *ScrollMetadata:
		if metadata == nil {
			return ScrollMetadata{}, fmt.Errorf("nil scroll metadata")
		}
		return *metadata, nil
	case ProvidesScrollMetadata:
		return ScrollMetadata{
			PageName:     metadata.GetPageName(),
			PreviousPage: metadata.GetPreviousPage(),
			NextPage:     metadata.GetNextPage(),
			CurrentPage:  metadata.GetCurrentPage(),
		}, nil
	case ScrollMetadataResolver:
		return metadata(etx, value)
	case func(*echo.Context, any) (ScrollMetadata, error):
		return metadata(etx, value)
	case func(any) (ScrollMetadata, error):
		return metadata(value)
	case func(any) ScrollMetadata:
		return metadata(value), nil
	default:
		return ScrollMetadata{}, fmt.Errorf("unsupported scroll metadata provider %T", provider)
	}
}

func evaluateProp(etx *echo.Context, value any) (any, error) {
	resolved, _, err := evaluateLazyProp(etx, value)
	return resolved, err
}

func evaluateLazyProp(etx *echo.Context, value any) (any, bool, error) {
	switch resolver := value.(type) {
	case PropResolver:
		resolved, err := resolver.ResolveInertiaProp(etx)
		return resolved, true, err
	case Resolver:
		resolved, err := resolver(etx)
		return resolved, true, err
	case func(*echo.Context) (any, error):
		resolved, err := resolver(etx)
		return resolved, true, err
	case func(*echo.Context) any:
		return resolver(etx), true, nil
	case func() (any, error):
		resolved, err := resolver()
		return resolved, true, err
	case func() any:
		return resolver(), true, nil
	default:
		return value, false, nil
	}
}

func pathMatchesAny(path string, selections []string) bool {
	for _, selection := range selections {
		if pathRelated(path, selection) {
			return true
		}
	}
	return false
}

func matchesOnly(path string, selections []string) bool {
	for _, selection := range selections {
		if path == selection || strings.HasPrefix(path, selection+".") {
			return true
		}
	}
	return false
}

func explicitlySelected(path string, selections []string) bool {
	for _, selection := range selections {
		if selection == path || strings.HasPrefix(selection, path+".") {
			return true
		}
	}
	return false
}

func pathExcluded(path string, exclusions []string) bool {
	for _, exclusion := range exclusions {
		if exclusion == path || strings.HasPrefix(path, exclusion+".") {
			return true
		}
	}
	return false
}

func resetMatches(path string, resets []string) bool {
	return slices.Contains(resets, path)
}

func pathRelated(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+".") || strings.HasPrefix(right, left+".")
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

func sortResolvedMetadata(result *resolvedPage) {
	slices.Sort(result.merge)
	slices.Sort(result.prepend)
	slices.Sort(result.deepMerge)
	slices.Sort(result.matchOn)
	slices.Sort(result.rescued)
	slices.Sort(result.resolvedShared)
	for group := range result.deferred {
		slices.Sort(result.deferred[group])
	}
}

func sharedTopLevelKeys(shared Props) []string {
	seen := make(map[string]struct{}, len(shared))
	for key := range shared {
		top, _, _ := strings.Cut(key, ".")
		if top != "" {
			seen[top] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func protectedPropExists(props Props, protected string) bool {
	for key := range props {
		if key == protected || strings.HasPrefix(key, protected+".") {
			return true
		}
	}
	return false
}

func unpackDotProps(props Props) (Props, error) {
	result := make(Props, len(props))
	dotted := make([]string, 0)
	for key, value := range props {
		if strings.Contains(key, ".") {
			dotted = append(dotted, key)
			continue
		}
		result[key] = value
	}
	slices.Sort(dotted)
	for _, path := range dotted {
		segments := strings.Split(path, ".")
		current := map[string]any(result)
		for index, segment := range segments[:len(segments)-1] {
			next, ok := current[segment].(map[string]any)
			if !ok {
				if typed, propsOK := current[segment].(Props); propsOK {
					next = map[string]any(typed)
				} else if current[segment] != nil {
					return nil, fmt.Errorf(
						"dotted prop %q conflicts with non-object path %q",
						path,
						strings.Join(segments[:index+1], "."),
					)
				} else {
					next = make(map[string]any)
				}
				current[segment] = next
			}
			current = next
		}
		current[segments[len(segments)-1]] = props[path]
	}
	return result, nil
}
