package kiks

import (
	"context"
	"errors"
	"net/http"
	"reflect"
)

var (
	// ErrNoBag is returned by Get/Exists/Set/Destroy when middleware has not
	// attached a bag.
	ErrNoBag = errors.New("kiks: no cookie bag on context")
	// ErrUnknownType is returned when T is not registered on the jar
	// (for example Get[Cart] when the jar has *Cart).
	ErrUnknownType = errors.New("kiks: cookie type not registered")
	// ErrNotBag is returned by Get/Exists/Set/Destroy when T is registered
	// as a native-only cookie (not Bagged / NewSession).
	ErrNotBag = errors.New("kiks: cookie is native-only; use Read/Write/Clear")
	// ErrBagCookie is returned by Read/Write/Clear when T is a bagged
	// cookie (Bagged / NewSession); use Get/Set/Destroy instead.
	ErrBagCookie = errors.New("kiks: cookie is bagged; use Get/Set/Destroy")
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
// by middleware. ErrNoBag / ErrUnknownType are returned for a missing bag or
// unregistered type; absence of a cookie value is (zero, nil).
func Get[T any](ctx context.Context) (T, error) {
	var zero T
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return zero, ErrNoBag
	}
	definition, ok := defFor[T](requestBag)
	if !ok {
		return zero, ErrUnknownType
	}
	if !definition.isBagged() {
		return zero, ErrNotBag
	}
	key := definition.typeKey()
	if requestBag.deleted[key] {
		return zero, nil
	}
	if !requestBag.loaded[key] && definition.kind() != kindSession {
		requestBag.loadNamed(definition)
	}
	value, ok := requestBag.values[key]
	if !ok || value == nil {
		return zero, nil
	}
	typed, ok := value.(T)
	if !ok {
		return zero, ErrUnknownType
	}
	return typed, nil
}

// Exists reports whether a registered cookie of type T is present on the bag
// (and not destroyed). Named cookies are loaded lazily like Get.
func Exists[T any](ctx context.Context) (bool, error) {
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return false, ErrNoBag
	}
	definition, ok := defFor[T](requestBag)
	if !ok {
		return false, ErrUnknownType
	}
	if !definition.isBagged() {
		return false, ErrNotBag
	}
	key := definition.typeKey()
	if requestBag.deleted[key] {
		return false, nil
	}
	if !requestBag.loaded[key] && definition.kind() != kindSession {
		requestBag.loadNamed(definition)
	}
	_, ok = requestBag.values[key]
	return ok, nil
}

// Set writes a cookie or session value on the request bag.
func Set[T any](ctx context.Context, value T) error {
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return ErrNoBag
	}
	definition, ok := defFor[T](requestBag)
	if !ok {
		return ErrUnknownType
	}
	if !definition.isBagged() {
		return ErrNotBag
	}
	key := definition.typeKey()
	requestBag.values[key] = value
	requestBag.loaded[key] = true
	delete(requestBag.deleted, key)
	requestBag.dirty[key] = true
	if definition.kind() == kindSession {
		requestBag.sessionDirty = true
	}
	return nil
}

// Destroy clears a cookie or session value from the request bag. Persistence
// expires the cookie and, for sessions, calls Store.Destroy.
func Destroy[T any](ctx context.Context) error {
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return ErrNoBag
	}
	definition, ok := defFor[T](requestBag)
	if !ok {
		return ErrUnknownType
	}
	if !definition.isBagged() {
		return ErrNotBag
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
	return nil
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
		if isCorruptCookie(decodeErr) {
			requestBag.dirty[key] = true
		}
		return
	}
	value, err := definition.decodePayload(payload)
	if err != nil {
		if isCorruptCookie(err) {
			requestBag.dirty[key] = true
		}
		return
	}
	requestBag.values[key] = value
}
