package kiks

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/securecookie"
)

// Cookie is the payload codec for a named cookie or session bag value.
// MarshalCookie returns raw payload bytes; kiks signs, encrypts, or stores
// those bytes afterwards — never return a Set-Cookie string.
type Cookie interface {
	MarshalCookie() ([]byte, error)
	UnmarshalCookie([]byte) error
}

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

// Definition describes a named cookie registered on a Jar.
// NewSession and Bagged(...) defs ride the middleware bag; unmarked
// Plain/Signed/Encrypted defs are native-only (Read/Write/Clear).
type Definition interface {
	cookieName() string
	kind() defKind
	typeKey() reflect.Type
	options() cookieOptions
	encodePayload(value any) ([]byte, error)
	decodePayload(payload []byte) (any, error)
	zeroValue() any
	isBagged() bool
}

type typedDef[T Cookie] struct {
	name string
	k    defKind
	opts cookieOptions
}

func (definition *typedDef[T]) cookieName() string     { return definition.name }
func (definition *typedDef[T]) kind() defKind          { return definition.k }
func (definition *typedDef[T]) typeKey() reflect.Type  { return reflect.TypeFor[T]() }
func (definition *typedDef[T]) options() cookieOptions { return definition.opts }
func (definition *typedDef[T]) zeroValue() any         { var zero T; return zero }
func (definition *typedDef[T]) isBagged() bool {
	return definition.k == kindSession
}

func (definition *typedDef[T]) encodePayload(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	typed, ok := value.(T)
	if !ok {
		return nil, fmt.Errorf("kiks: value type %T does not match cookie %s", value, definition.name)
	}
	rv := reflect.ValueOf(typed)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, nil
	}
	return typed.MarshalCookie()
}

func (definition *typedDef[T]) decodePayload(payload []byte) (any, error) {
	var value T
	if len(payload) == 0 {
		return value, nil
	}
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rv := reflect.ValueOf(value)
		if !rv.IsValid() || rv.IsNil() {
			value = reflect.New(rt.Elem()).Interface().(T)
		}
	}
	if err := value.UnmarshalCookie(payload); err != nil {
		return value, corruptPayloadError{err: err}
	}
	return value, nil
}

type baggedDef struct {
	inner Definition
}

func (definition *baggedDef) cookieName() string     { return definition.inner.cookieName() }
func (definition *baggedDef) kind() defKind          { return definition.inner.kind() }
func (definition *baggedDef) typeKey() reflect.Type  { return definition.inner.typeKey() }
func (definition *baggedDef) options() cookieOptions { return definition.inner.options() }
func (definition *baggedDef) encodePayload(value any) ([]byte, error) {
	return definition.inner.encodePayload(value)
}
func (definition *baggedDef) decodePayload(payload []byte) (any, error) {
	return definition.inner.decodePayload(payload)
}
func (definition *baggedDef) zeroValue() any { return definition.inner.zeroValue() }
func (definition *baggedDef) isBagged() bool { return true }

// Bagged marks a Plain/Signed/Encrypted definition for the middleware bag
// (lazy Get/Set + deferred persist). NewSession is always bagged. Unmarked
// named defs stay registered for type-keyed Read/Write/Clear only.
func Bagged(def Definition) Definition {
	if def == nil {
		return nil
	}
	if def.isBagged() || def.kind() == kindSession {
		return def
	}
	return &baggedDef{inner: def}
}

// Plain defines an unsigned HTTP cookie.
func Plain[T Cookie](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindPlain, opts: applyOptions(opts)}
}

// Signed defines a tamper-evident HTTP cookie.
func Signed[T Cookie](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindSigned, opts: applyOptions(opts)}
}

// Encrypted defines a signed and encrypted HTTP cookie.
func Encrypted[T Cookie](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindEncrypted, opts: applyOptions(opts)}
}

// NewSession defines the application session bag (exactly one per jar).
func NewSession[T Cookie](name string, opts ...Option) Definition {
	return &typedDef[T]{name: name, k: kindSession, opts: applyOptions(opts)}
}

// Jar holds cookie definitions, codecs, and the session Store.
type Jar struct {
	signed     *securecookie.SecureCookie
	encrypted  *securecookie.SecureCookie
	store      Store
	defs       []Definition
	byType     map[reflect.Type]Definition
	sessionDef Definition
	flashName  string
}

// NewJar constructs a cookie jar. Session payload persistence goes through
// store; flashes always use a dedicated flash cookie derived from the session name.
func NewJar(keys Keys, store Store, defs ...Definition) (*Jar, error) {
	if store == nil {
		return nil, errors.New("kiks: store is required")
	}
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
		store:     store,
		defs:      defs,
		byType:    make(map[reflect.Type]Definition, len(defs)),
	}

	var sessionCount int
	names := make(map[string]struct{}, len(defs)+1)
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
			jar.flashName = name + "_flash"
			if _, exists := names[jar.flashName]; exists {
				return nil, fmt.Errorf("kiks: flash cookie name %q conflicts with a definition", jar.flashName)
			}
			names[jar.flashName] = struct{}{}
			maxAge := definition.options().maxAge
			if maxAge > 0 {
				jar.signed.MaxAge(maxAge)
				jar.encrypted.MaxAge(maxAge)
				if setter, ok := store.(maxAgeSetter); ok {
					setter.setMaxAge(maxAge)
				}
			}
		}
	}
	if sessionCount != 1 {
		return nil, errors.New("kiks: exactly one NewSession definition is required")
	}

	return jar, nil
}

func encodePlain(payload []byte) string {
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodePlain(raw string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(raw)
}
