package kiks

import (
	"bytes"
	"encoding/gob"
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
	jar       *Jar
	bag       *bag
	request   *http.Request
	status    int
	persisted bool
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
	writer.jar.persist(writer.ResponseWriter, writer.request, writer.bag, writer.status)
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

// EchoMiddleware loads defined cookies onto context.Context and persists the
// bag once before the response is committed.
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
			return err
		}
	}
}

func (jar *Jar) load(request *http.Request) (*bag, error) {
	requestBag := &bag{
		jar:     jar,
		values:  make(map[reflect.Type]any),
		deleted: make(map[reflect.Type]bool),
		dirty:   make(map[reflect.Type]bool),
	}

	for _, definition := range jar.defs {
		cookie, err := request.Cookie(definition.cookieName())
		if err != nil {
			continue
		}
		if definition.kind() == kindSession {
			if loadErr := jar.loadSession(requestBag, definition, cookie.Value); loadErr != nil {
				return nil, loadErr
			}
			continue
		}
		payload, decodeErr := jar.decodeCookie(definition, cookie.Value)
		if decodeErr != nil {
			if isDecodeError(decodeErr) {
				requestBag.dirty[definition.typeKey()] = true
				continue
			}
			return nil, decodeErr
		}
		value, err := definition.decodePayload(payload)
		if err != nil {
			if isDecodeError(err) {
				requestBag.dirty[definition.typeKey()] = true
				continue
			}
			return nil, err
		}
		requestBag.values[definition.typeKey()] = value
	}

	return requestBag, nil
}

func (jar *Jar) loadSession(
	requestBag *bag,
	definition Definition,
	raw string,
) error {
	blob, err := jar.decodeSessionBlob(definition.cookieName(), raw)
	if err != nil {
		if isDecodeError(err) {
			requestBag.sessionDirty = true
			return nil
		}
		return err
	}
	return jar.applySessionBlob(requestBag, definition, blob)
}

func (jar *Jar) applySessionBlob(
	requestBag *bag,
	definition Definition,
	blob sessionBlob,
) error {
	value, err := definition.decodePayload(blob.Payload)
	if err != nil {
		if isDecodeError(err) {
			requestBag.sessionDirty = true
			return nil
		}
		return err
	}
	requestBag.values[definition.typeKey()] = value
	requestBag.flashes = blob.Flashes
	return nil
}

func (jar *Jar) persist(
	writer http.ResponseWriter,
	request *http.Request,
	requestBag *bag,
	status int,
) {
	persistFlashes := shouldPersistFlashes(status, writer.Header())
	outgoingFlashes := requestBag.flashes
	if !persistFlashes {
		outgoingFlashes = nil
	}
	clearFlashes := !persistFlashes && len(requestBag.flashes) > 0

	for _, definition := range jar.defs {
		if definition.kind() == kindSession {
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
			continue
		}
		raw, err := jar.encodeCookie(definition, payload)
		if err != nil {
			continue
		}
		http.SetCookie(writer, newCookie(definition.cookieName(), raw, opts))
	}

	if jar.sessionDef == nil {
		return
	}
	definition := jar.sessionDef
	opts := definition.options()
	value := requestBag.values[definition.typeKey()]
	if value == nil {
		value = definition.zeroValue()
	}
	emptyValue := isZeroValue(value)
	if emptyValue && len(outgoingFlashes) == 0 {
		if requestBag.sessionDirty || requestBag.deleted[definition.typeKey()] {
			http.SetCookie(writer, expireCookie(definition.cookieName(), opts))
			if !jar.cookieDrv && requestBag.sessionID != "" {
				_ = jar.store.Destroy(request.Context(), requestBag.sessionID)
			}
		}
		return
	}
	if !requestBag.sessionDirty && !clearFlashes {
		return
	}

	payload, err := definition.encodePayload(value)
	if err != nil {
		return
	}
	blob := sessionBlob{Payload: payload, Flashes: outgoingFlashes}
	encoded, err := encodeSessionBlobBytes(blob)
	if err != nil {
		return
	}

	if jar.cookieDrv {
		raw, err := jar.encrypted.Encode(definition.cookieName(), encoded)
		if err != nil {
			return
		}
		http.SetCookie(writer, newCookie(definition.cookieName(), raw, opts))
		return
	}

	sessionID := requestBag.sessionID
	if sessionID == "" {
		sessionID = newSessionID()
		requestBag.sessionID = sessionID
	}
	maxAge := durationSeconds(opts.maxAge)
	if err := jar.store.Save(request.Context(), sessionID, encoded, maxAge); err != nil {
		return
	}
	raw, err := jar.signed.Encode(definition.cookieName(), sessionID)
	if err != nil {
		return
	}
	http.SetCookie(writer, newCookie(definition.cookieName(), raw, opts))
}

func (jar *Jar) encodeCookie(definition Definition, payload []byte) (string, error) {
	switch definition.kind() {
	case kindPlain:
		return encodePlain(payload), nil
	case kindSigned:
		return jar.signed.Encode(definition.cookieName(), payload)
	case kindEncrypted:
		return jar.encrypted.Encode(definition.cookieName(), payload)
	default:
		return "", errors.New("kiks: unsupported cookie kind")
	}
}

func (jar *Jar) decodeCookie(definition Definition, raw string) ([]byte, error) {
	switch definition.kind() {
	case kindPlain:
		return decodePlain(raw)
	case kindSigned:
		var payload []byte
		if err := jar.signed.Decode(definition.cookieName(), raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case kindEncrypted:
		var payload []byte
		if err := jar.encrypted.Decode(definition.cookieName(), raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	default:
		return nil, errors.New("kiks: unsupported cookie kind")
	}
}

func (jar *Jar) decodeSessionBlob(name, raw string) (sessionBlob, error) {
	var encoded []byte
	if err := jar.encrypted.Decode(name, raw, &encoded); err != nil {
		return sessionBlob{}, err
	}
	return decodeSessionBlobBytes(encoded)
}

func encodeSessionBlobBytes(blob sessionBlob) ([]byte, error) {
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(blob); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func decodeSessionBlobBytes(payload []byte) (sessionBlob, error) {
	var blob sessionBlob
	if len(payload) == 0 {
		return blob, nil
	}
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&blob); err != nil {
		return blob, err
	}
	return blob, nil
}

func isDecodeError(err error) bool {
	if err == nil {
		return false
	}
	if secureErr, ok := errors.AsType[securecookie.Error](err); ok {
		return secureErr.IsDecode() && !secureErr.IsUsage() && !secureErr.IsInternal()
	}
	return true
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
