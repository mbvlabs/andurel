package kiks

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/securecookie"
)

// Keys holds the gorilla/securecookie hash and optional encryption keys.
type Keys struct {
	Authentication []byte
	Encryption     []byte
}

type defKind int

const (
	kindPlain defKind = iota
	kindSigned
	kindEncrypted
	kindSession
)

type cookieOptions struct {
	path     string
	domain   string
	maxAge   int
	secure   bool
	httpOnly bool
	sameSite http.SameSite
}

// Option configures a named cookie definition.
type Option func(*cookieOptions)

// HTTPOnly marks the cookie inaccessible to JavaScript.
func HTTPOnly() Option {
	return func(options *cookieOptions) {
		options.httpOnly = true
	}
}

// Secure sets the Secure attribute.
func Secure(secure bool) Option {
	return func(options *cookieOptions) {
		options.secure = secure
	}
}

// MaxAge sets cookie lifetime in seconds.
func MaxAge(seconds int) Option {
	return func(options *cookieOptions) {
		options.maxAge = seconds
	}
}

// Path sets the cookie path. Defaults to "/".
func Path(path string) Option {
	return func(options *cookieOptions) {
		options.path = path
	}
}

// Domain sets the cookie domain.
func Domain(domain string) Option {
	return func(options *cookieOptions) {
		options.domain = domain
	}
}

// SameSite sets the SameSite attribute.
func SameSite(mode http.SameSite) Option {
	return func(options *cookieOptions) {
		options.sameSite = mode
	}
}

func defaultCookieOptions() cookieOptions {
	return cookieOptions{
		path:     "/",
		httpOnly: true,
		sameSite: http.SameSiteLaxMode,
	}
}

func applyOptions(opts []Option) cookieOptions {
	options := defaultCookieOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.path == "" {
		options.path = "/"
	}
	return options
}

// Definition is a named cookie registered on a Jar.
type Definition interface {
	cookieName() string
	kind() defKind
	typeKey() reflect.Type
	options() cookieOptions
	encodePayload(value any) ([]byte, error)
	decodePayload(payload []byte) (any, error)
	zeroValue() any
}

type typedDef[T any] struct {
	name string
	k    defKind
	opts cookieOptions
}

func (definition *typedDef[T]) cookieName() string     { return definition.name }
func (definition *typedDef[T]) kind() defKind          { return definition.k }
func (definition *typedDef[T]) typeKey() reflect.Type  { return reflect.TypeFor[T]() }
func (definition *typedDef[T]) options() cookieOptions { return definition.opts }
func (definition *typedDef[T]) zeroValue() any         { var zero T; return zero }

func (definition *typedDef[T]) encodePayload(value any) ([]byte, error) {
	typed, ok := value.(T)
	if !ok && value != nil {
		return nil, fmt.Errorf("kiks: value type %T does not match cookie %s", value, definition.name)
	}
	if value == nil {
		var zero T
		typed = zero
	}
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(typed); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (definition *typedDef[T]) decodePayload(payload []byte) (any, error) {
	var value T
	if len(payload) == 0 {
		return value, nil
	}
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&value); err != nil {
		return value, err
	}
	return value, nil
}

// Plain defines an unsigned HTTP cookie.
func Plain[T any](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindPlain, opts: applyOptions(opts)}
}

// Signed defines a tamper-evident HTTP cookie.
func Signed[T any](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindSigned, opts: applyOptions(opts)}
}

// Encrypted defines a signed and encrypted HTTP cookie.
func Encrypted[T any](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindEncrypted, opts: applyOptions(opts)}
}

// Session defines the application session bag (value + flashes).
func Session[T any](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindSession, opts: applyOptions(opts)}
}

type sessionBlob struct {
	Payload []byte
	Flashes []FlashMessage
}

// Jar holds cookie definitions and codecs.
type Jar struct {
	signed     *securecookie.SecureCookie
	encrypted  *securecookie.SecureCookie
	defs       []Definition
	byType     map[reflect.Type]Definition
	sessionDef Definition
}

// New constructs a cookie jar. Session values live in an encrypted cookie.
func New(keys Keys, defs ...Definition) (*Jar, error) {
	if len(keys.Authentication) < 32 {
		return nil, errors.New("kiks: authentication key must be at least 32 bytes")
	}
	if len(keys.Encryption) == 0 {
		return nil, errors.New("kiks: encryption key is required")
	}
	if len(defs) == 0 {
		return nil, errors.New("kiks: at least one cookie definition is required")
	}

	jar := &Jar{
		signed:    securecookie.New(keys.Authentication, nil),
		encrypted: securecookie.New(keys.Authentication, keys.Encryption),
		defs:      defs,
		byType:    make(map[reflect.Type]Definition, len(defs)),
	}

	var sessionCount int
	names := make(map[string]struct{}, len(defs))
	for _, definition := range defs {
		if definition == nil {
			return nil, errors.New("kiks: nil cookie definition")
		}
		name := definition.cookieName()
		if name == "" {
			return nil, errors.New("kiks: cookie name is required")
		}
		if _, exists := names[name]; exists {
			return nil, fmt.Errorf("kiks: duplicate cookie name %q", name)
		}
		names[name] = struct{}{}
		key := definition.typeKey()
		if _, exists := jar.byType[key]; exists {
			return nil, fmt.Errorf("kiks: duplicate cookie type %s", key)
		}
		jar.byType[key] = definition
		if definition.kind() == kindSession {
			sessionCount++
			jar.sessionDef = definition
			maxAge := definition.options().maxAge
			if maxAge > 0 {
				jar.signed.MaxAge(maxAge)
				jar.encrypted.MaxAge(maxAge)
			}
		}
	}
	if sessionCount != 1 {
		return nil, errors.New("kiks: exactly one Session definition is required")
	}

	return jar, nil
}

func (jar *Jar) codecFor(kind defKind) *securecookie.SecureCookie {
	switch kind {
	case kindEncrypted, kindSession:
		return jar.encrypted
	case kindSigned:
		return jar.signed
	default:
		return nil
	}
}

func encodePlain(payload []byte) string {
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodePlain(raw string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(raw)
}
