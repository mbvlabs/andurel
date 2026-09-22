package kiks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/securecookie"
)

// Read decodes a native (unmarked) Plain/Signed/Encrypted cookie from r.
// T must be registered on jar and must not be bagged. Missing cookie is
// (zero, nil). ctx is reserved for API consistency with Get/Set.
func Read[T any](ctx context.Context, jar *Jar, r *http.Request) (T, error) {
	_ = ctx
	var zero T
	definition, err := nativeDef[T](jar)
	if err != nil {
		return zero, err
	}
	if r == nil {
		return zero, nil
	}
	cookie, err := r.Cookie(definition.cookieName())
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return zero, nil
		}
		return zero, err
	}
	payload, err := jar.decodeCookie(definition, cookie.Value)
	if err != nil {
		return zero, err
	}
	value, err := definition.decodePayload(payload)
	if err != nil {
		return zero, err
	}
	if value == nil {
		return zero, nil
	}
	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("kiks: decoded type %T does not match %s", value, reflect.TypeFor[T]())
	}
	return typed, nil
}

// Write encodes a native cookie onto w immediately (Set-Cookie).
func Write[T any](ctx context.Context, jar *Jar, w http.ResponseWriter, value T) error {
	_ = ctx
	definition, err := nativeDef[T](jar)
	if err != nil {
		return err
	}
	if w == nil {
		return errors.New("kiks: ResponseWriter is required")
	}
	payload, err := definition.encodePayload(value)
	if err != nil {
		return err
	}
	raw, err := jar.encodeCookie(definition, payload)
	if err != nil {
		return err
	}
	http.SetCookie(w, newCookie(definition.cookieName(), raw, definition.options()))
	return nil
}

// Clear expires a native cookie on w immediately.
func Clear[T any](ctx context.Context, jar *Jar, w http.ResponseWriter) error {
	_ = ctx
	definition, err := nativeDef[T](jar)
	if err != nil {
		return err
	}
	if w == nil {
		return errors.New("kiks: ResponseWriter is required")
	}
	http.SetCookie(w, expireCookie(definition.cookieName(), definition.options()))
	return nil
}

func nativeDef[T any](jar *Jar) (Definition, error) {
	if jar == nil {
		return nil, errors.New("kiks: jar is required")
	}
	definition, ok := jar.byType[reflect.TypeFor[T]()]
	if !ok {
		return nil, ErrUnknownType
	}
	if definition.isBagged() {
		return nil, ErrBagCookie
	}
	return definition, nil
}

func encodeCookieRaw(definition Definition, signed, encrypted *securecookie.SecureCookie, payload []byte) (string, error) {
	switch definition.kind() {
	case kindPlain:
		return encodePlain(payload), nil
	case kindSigned:
		return signed.Encode(definition.cookieName(), payload)
	case kindEncrypted:
		return encrypted.Encode(definition.cookieName(), payload)
	default:
		return "", errors.New("kiks: unsupported cookie kind")
	}
}

func decodeCookieRaw(definition Definition, signed, encrypted *securecookie.SecureCookie, raw string) ([]byte, error) {
	switch definition.kind() {
	case kindPlain:
		return decodePlain(raw)
	case kindSigned:
		var payload []byte
		if err := signed.Decode(definition.cookieName(), raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case kindEncrypted:
		var payload []byte
		if err := encrypted.Decode(definition.cookieName(), raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	default:
		return nil, errors.New("kiks: unsupported cookie kind")
	}
}
