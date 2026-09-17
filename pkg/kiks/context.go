package kiks

import (
	"context"
	"reflect"
)

type contextKey struct{}

type bag struct {
	jar          *Jar
	values       map[reflect.Type]any
	deleted      map[reflect.Type]bool
	dirty        map[reflect.Type]bool
	flashes      []FlashMessage
	sessionDirty bool
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
func Get[T any](ctx context.Context) T {
	value, _ := Lookup[T](ctx)
	return value
}

// Lookup returns the cookie value of type T and whether it was present.
func Lookup[T any](ctx context.Context) (T, bool) {
	var zero T
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return zero, false
	}
	key := reflect.TypeFor[T]()
	if requestBag.deleted[key] {
		return zero, false
	}
	value, ok := requestBag.values[key]
	if !ok || value == nil {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
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
	delete(requestBag.deleted, key)
	requestBag.dirty[key] = true
	if definition.kind() == kindSession {
		requestBag.sessionDirty = true
	}
}

// Delete removes a cookie or session value from the request bag.
func Delete[T any](ctx context.Context) {
	requestBag := bagFrom(ctx)
	definition, ok := defFor[T](requestBag)
	if !ok {
		return
	}
	key := definition.typeKey()
	delete(requestBag.values, key)
	requestBag.deleted[key] = true
	requestBag.dirty[key] = true
	if definition.kind() == kindSession {
		requestBag.sessionDirty = true
	}
}
