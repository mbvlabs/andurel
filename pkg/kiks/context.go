package kiks

import (
	"context"
	"net/http"
	"reflect"
)

type contextKey struct{}

type bag struct {
	jar          *Jar
	request      *http.Request
	values       map[reflect.Type]any
	deleted      map[reflect.Type]bool
	dirty        map[reflect.Type]bool
	loaded       map[reflect.Type]bool
	flashes      []FlashMessage
	sessionDirty bool
	flashesDirty bool
}

func withBag(ctx context.Context, requestBag *bag) context.Context {
	return context.WithValue(ctx, contextKey{}, requestBag)
}

func bagFrom(ctx context.Context) *bag {
	if ctx == nil {
		return nil
	}
	requestBag, _ := ctx.Value(contextKey{}).(*bag)
	return requestBag
}

func defFor[T any](requestBag *bag) (Definition, bool) {
	if requestBag == nil || requestBag.jar == nil {
		return nil, false
	}
	definition, ok := requestBag.jar.byType[reflect.TypeFor[T]()]
	return definition, ok
}

// Get returns the cookie value of type T, or the zero value if missing.
// Named cookies are loaded lazily on first Get; the session is eager-loaded
// by middleware.
func Get[T any](ctx context.Context) T {
	var zero T
	requestBag := bagFrom(ctx)
	definition, ok := defFor[T](requestBag)
	if !ok {
		return zero
	}
	key := definition.typeKey()
	if requestBag.deleted[key] {
		return zero
	}
	if !requestBag.loaded[key] && definition.kind() != kindSession {
		requestBag.loadNamed(definition)
	}
	value, ok := requestBag.values[key]
	if !ok || value == nil {
		return zero
	}
	typed, ok := value.(T)
	if !ok {
		return zero
	}
	return typed
}

// Exists reports whether a registered cookie of type T is present on the bag
// (and not destroyed). Named cookies are loaded lazily like Get.
func Exists[T any](ctx context.Context) bool {
	requestBag := bagFrom(ctx)
	definition, ok := defFor[T](requestBag)
	if !ok {
		return false
	}
	key := definition.typeKey()
	if requestBag.deleted[key] {
		return false
	}
	if !requestBag.loaded[key] && definition.kind() != kindSession {
		requestBag.loadNamed(definition)
	}
	_, ok = requestBag.values[key]
	return ok
}

// Set writes a cookie or session value on the request bag.
func Set[T any](ctx context.Context, value T) {
	requestBag := bagFrom(ctx)
	definition, ok := defFor[T](requestBag)
	if !ok {
		return
	}
	key := definition.typeKey()
	requestBag.values[key] = value
	requestBag.loaded[key] = true
	delete(requestBag.deleted, key)
	requestBag.dirty[key] = true
	if definition.kind() == kindSession {
		requestBag.sessionDirty = true
	}
}

// Destroy clears a cookie or session value from the request bag. Persistence
// expires the cookie and, for sessions, calls Store.Destroy.
func Destroy[T any](ctx context.Context) {
	requestBag := bagFrom(ctx)
	definition, ok := defFor[T](requestBag)
	if !ok {
		return
	}
	key := definition.typeKey()
	delete(requestBag.values, key)
	requestBag.deleted[key] = true
	requestBag.loaded[key] = true
	requestBag.dirty[key] = true
	if definition.kind() == kindSession {
		requestBag.sessionDirty = true
		requestBag.flashes = nil
		requestBag.flashesDirty = true
	}
}

func (requestBag *bag) loadNamed(definition Definition) {
	key := definition.typeKey()
	requestBag.loaded[key] = true
	if requestBag.request == nil || requestBag.jar == nil {
		return
	}
	cookie, err := requestBag.request.Cookie(definition.cookieName())
	if err != nil {
		return
	}
	payload, decodeErr := requestBag.jar.decodeCookie(definition, cookie.Value)
	if decodeErr != nil {
		if isDecodeError(decodeErr) {
			requestBag.dirty[key] = true
			return
		}
		return
	}
	value, err := definition.decodePayload(payload)
	if err != nil {
		if isDecodeError(err) {
			requestBag.dirty[key] = true
			return
		}
		return
	}
	requestBag.values[key] = value
}
