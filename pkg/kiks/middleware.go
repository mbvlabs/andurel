package kiks

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v5"
)

// ClientRedirectHeader marks a response that navigates in the browser without
// an HTTP 3xx status (Datastar/SSE redirects). hypermedia.Redirect sets the
// same header name.
const ClientRedirectHeader = "X-Andurel-Client-Redirect"

type middlewareConfig struct {
	skip []string
}

// MiddlewareOption configures EchoMiddleware.
type MiddlewareOption func(*middlewareConfig)

// SkipPrefixes skips jar load/persist for matching URL path prefixes.
func SkipPrefixes(prefixes ...string) MiddlewareOption {
	return func(config *middlewareConfig) {
		config.skip = append(config.skip, prefixes...)
	}
}

type persistWriter struct {
	http.ResponseWriter
	jar        *Jar
	bag        *bag
	request    *http.Request
	status     int
	persisted  bool
	persistErr error
}

var (
	_ http.Flusher                              = (*persistWriter)(nil)
	_ interface{ Unwrap() http.ResponseWriter } = (*persistWriter)(nil)
	_ interface{ FlushError() error }           = (*persistWriter)(nil)
)

func (writer *persistWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *persistWriter) WriteHeader(status int) {
	if writer.status == 0 {
		writer.status = status
	}
	writer.save()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *persistWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.save()
	if writer.persistErr != nil {
		return 0, writer.persistErr
	}
	return writer.ResponseWriter.Write(body)
}

func (writer *persistWriter) Flush() {
	_ = writer.FlushError()
}

func (writer *persistWriter) FlushError() error {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.save()
	if writer.persistErr != nil {
		return writer.persistErr
	}
	if flusher, ok := writer.ResponseWriter.(interface{ FlushError() error }); ok {
		return flusher.FlushError()
	}
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (writer *persistWriter) save() {
	if writer.persisted {
		return
	}
	writer.persisted = true
	writer.persistErr = writer.jar.persist(writer.ResponseWriter, writer.request, writer.bag, writer.status)
}

func skipPath(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func shouldPersistFlashes(status int, header http.Header) bool {
	if header != nil && header.Get(ClientRedirectHeader) != "" {
		return true
	}
	if status >= 300 && status < 400 {
		return true
	}
	return status == http.StatusConflict
}

// EchoMiddleware eager-loads the session and flash cookie onto context.Context
// and persists bagged cookies once before the response is committed. Bagged
// named cookies load lazily on Get/Exists; native (unmarked) defs are skipped.
func (jar *Jar) EchoMiddleware(opts ...MiddlewareOption) echo.MiddlewareFunc {
	config := middlewareConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if skipPath(c.Request().URL.Path, config.skip) {
				return next(c)
			}

			requestBag, err := jar.load(c.Request())
			if err != nil {
				return err
			}

			writer := &persistWriter{
				ResponseWriter: c.Response(),
				jar:            jar,
				bag:            requestBag,
				request:        c.Request(),
			}
			c.SetResponse(writer)
			c.SetRequest(c.Request().WithContext(withBag(c.Request().Context(), requestBag)))

			err = next(c)
			if !writer.persisted {
				if writer.status == 0 {
					if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil {
						writer.status = resp.Status
					}
				}
				writer.save()
			}
			if writer.persistErr != nil {
				if err == nil {
					return writer.persistErr
				}
				return errors.Join(err, writer.persistErr)
			}
			return err
		}
	}
}

func (jar *Jar) load(request *http.Request) (*bag, error) {
	requestBag := &bag{
		jar:     jar,
		request: request,
		values:  make(map[reflect.Type]any),
		deleted: make(map[reflect.Type]bool),
		dirty:   make(map[reflect.Type]bool),
		loaded:  make(map[reflect.Type]bool),
	}

	if jar.sessionDef != nil {
		if err := jar.loadSession(requestBag, request); err != nil {
			return nil, err
		}
		if err := jar.loadFlashes(requestBag, request); err != nil {
			return nil, err
		}
	}

	return requestBag, nil
}

func (jar *Jar) loadSession(requestBag *bag, request *http.Request) error {
	definition := jar.sessionDef
	key := definition.typeKey()
	requestBag.loaded[key] = true

	payload, err := jar.store.Load(request, definition.cookieName())
	if err != nil {
		if isCorruptCookie(err) {
			requestBag.sessionDirty = true
			return nil
		}
		return err
	}
	if payload == nil {
		return nil
	}
	value, err := definition.decodePayload(payload)
	if err != nil {
		if isCorruptCookie(err) {
			requestBag.sessionDirty = true
			return nil
		}
		return err
	}
	requestBag.values[key] = value
	return nil
}

func (jar *Jar) loadFlashes(requestBag *bag, request *http.Request) error {
	cookie, err := request.Cookie(jar.flashName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil
		}
		return err
	}
	flashes, decodeErr := jar.decodeFlashCookie(cookie.Value)
	if decodeErr != nil {
		if isCorruptCookie(decodeErr) {
			requestBag.flashesDirty = true
			return nil
		}
		return decodeErr
	}
	requestBag.flashes = flashes
	return nil
}

func (jar *Jar) persist(
	writer http.ResponseWriter,
	request *http.Request,
	requestBag *bag,
	status int,
) error {
	persistFlashes := shouldPersistFlashes(status, writer.Header())
	outgoingFlashes := requestBag.flashes
	if !persistFlashes {
		outgoingFlashes = nil
	}
	clearFlashes := !persistFlashes && (len(requestBag.flashes) > 0 || requestBag.flashesDirty)

	for _, definition := range jar.defs {
		if definition.kind() == kindSession || !definition.isBagged() {
			continue
		}
		key := definition.typeKey()
		if !requestBag.dirty[key] {
			continue
		}
		opts := definition.options()
		if requestBag.deleted[key] {
			http.SetCookie(writer, expireCookie(definition.cookieName(), opts))
			continue
		}
		value := requestBag.values[key]
		payload, err := definition.encodePayload(value)
		if err != nil {
			return err
		}
		raw, err := jar.encodeCookie(definition, payload)
		if err != nil {
			return err
		}
		http.SetCookie(writer, newCookie(definition.cookieName(), raw, opts))
	}

	if jar.sessionDef != nil {
		definition := jar.sessionDef
		opts := definition.options()
		attrs := attrsFromOptions(opts)
		key := definition.typeKey()

		if requestBag.deleted[key] {
			if err := jar.store.Destroy(writer, request, definition.cookieName(), attrs); err != nil {
				return err
			}
		} else if requestBag.sessionDirty {
			value := requestBag.values[key]
			if value == nil {
				value = definition.zeroValue()
			}
			if isZeroValue(value) {
				if err := jar.store.Destroy(writer, request, definition.cookieName(), attrs); err != nil {
					return err
				}
			} else {
				payload, err := definition.encodePayload(value)
				if err != nil {
					return err
				}
				if err := jar.store.Save(writer, request, definition.cookieName(), payload, attrs); err != nil {
					return err
				}
			}
		}

		flashOpts := opts
		if clearFlashes || requestBag.deleted[key] {
			http.SetCookie(writer, expireCookie(jar.flashName, flashOpts))
		} else if persistFlashes && (requestBag.flashesDirty || len(outgoingFlashes) > 0) {
			if len(outgoingFlashes) == 0 {
				http.SetCookie(writer, expireCookie(jar.flashName, flashOpts))
			} else {
				raw, err := jar.encodeFlashCookie(outgoingFlashes)
				if err != nil {
					return err
				}
				http.SetCookie(writer, newCookie(jar.flashName, raw, flashOpts))
			}
		}
	}
	return nil
}

func (jar *Jar) encodeCookie(definition Definition, payload []byte) (string, error) {
	return encodeCookieRaw(definition, jar.signed, jar.encrypted, payload)
}

func (jar *Jar) decodeCookie(definition Definition, raw string) ([]byte, error) {
	return decodeCookieRaw(definition, jar.signed, jar.encrypted, raw)
}

func (jar *Jar) encodeFlashCookie(flashes []FlashMessage) (string, error) {
	encoded, err := json.Marshal(flashes)
	if err != nil {
		return "", err
	}
	return jar.encrypted.Encode(jar.flashName, encoded)
}

func (jar *Jar) decodeFlashCookie(raw string) ([]FlashMessage, error) {
	var encoded []byte
	if err := jar.encrypted.Decode(jar.flashName, raw, &encoded); err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		return nil, nil
	}
	var flashes []FlashMessage
	if err := json.Unmarshal(encoded, &flashes); err != nil {
		return nil, corruptPayloadError{err: err}
	}
	return flashes, nil
}

// corruptPayloadError marks intentional payload / flash JSON decode failures
// so middleware can recover them as corrupt cookies without treating infra
// errors the same way.
type corruptPayloadError struct {
	err error
}

func (e corruptPayloadError) Error() string {
	if e.err == nil {
		return "kiks: corrupt cookie payload"
	}
	return e.err.Error()
}

func (e corruptPayloadError) Unwrap() error { return e.err }

func isCorruptCookie(err error) bool {
	if err == nil {
		return false
	}
	if secureErr, ok := errors.AsType[securecookie.Error](err); ok {
		return secureErr.IsDecode() && !secureErr.IsUsage() && !secureErr.IsInternal()
	}
	var payloadErr corruptPayloadError
	return errors.As(err, &payloadErr)
}

func newCookie(name, value string, opts cookieOptions) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     opts.path,
		Domain:   opts.domain,
		MaxAge:   opts.maxAge,
		Secure:   opts.secure,
		HttpOnly: opts.httpOnly,
		SameSite: opts.sameSite,
	}
}

func expireCookie(name string, opts cookieOptions) *http.Cookie {
	cookie := newCookie(name, "", opts)
	cookie.MaxAge = -1
	return cookie
}

func isZeroValue(value any) bool {
	if value == nil {
		return true
	}
	return reflect.ValueOf(value).IsZero()
}
