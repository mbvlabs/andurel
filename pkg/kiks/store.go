package kiks

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CookieAttrs are the HTTP cookie attributes used when a Store writes or
// expires the AppCookie.
type CookieAttrs struct {
	Path     string
	Domain   string
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

func attrsFromOptions(opts cookieOptions) CookieAttrs {
	return CookieAttrs{
		Path:     opts.path,
		Domain:   opts.domain,
		MaxAge:   opts.maxAge,
		Secure:   opts.secure,
		HTTPOnly: opts.httpOnly,
		SameSite: opts.sameSite,
	}
}

// Store persists the session App payload. Flashes never go through Store;
// the jar keeps them in a dedicated flash cookie.
type Store interface {
	Load(r *http.Request, name string) (payload []byte, err error)
	Save(w http.ResponseWriter, r *http.Request, name string, payload []byte, attrs CookieAttrs) error
	Destroy(w http.ResponseWriter, r *http.Request, name string, attrs CookieAttrs) error
}

type sessionDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type maxAgeSetter interface {
	setMaxAge(seconds int)
}

// CookieStore keeps the App payload in an encrypted AppCookie.
type CookieStore struct {
	codec *securecookie.SecureCookie
}

// NewCookieStore constructs a cookie-backed session store.
func NewCookieStore(keys Keys) (*CookieStore, error) {
	if len(keys.Authentication) < 32 {
		return nil, errors.New("kiks: authentication key must be at least 32 bytes")
	}
	if len(keys.Encryption) == 0 {
		return nil, errors.New("kiks: encryption key is required")
	}
	return &CookieStore{
		codec: securecookie.New(keys.Authentication, keys.Encryption),
	}, nil
}

func (store *CookieStore) setMaxAge(seconds int) {
	if seconds > 0 {
		store.codec.MaxAge(seconds)
	}
}

func (store *CookieStore) Load(r *http.Request, name string) ([]byte, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, err
	}
	var payload []byte
	if err := store.codec.Decode(name, cookie.Value, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (store *CookieStore) Save(
	w http.ResponseWriter,
	_ *http.Request,
	name string,
	payload []byte,
	attrs CookieAttrs,
) error {
	raw, err := store.codec.Encode(name, payload)
	if err != nil {
		return err
	}
	http.SetCookie(w, newCookieFromAttrs(name, raw, attrs))
	return nil
}

func (store *CookieStore) Destroy(
	w http.ResponseWriter,
	_ *http.Request,
	name string,
	attrs CookieAttrs,
) error {
	http.SetCookie(w, expireCookieFromAttrs(name, attrs))
	return nil
}

// databaseStore keeps the App payload in a sessions row and a signed id in
// AppCookie. Flashes stay in the jar's flash cookie. Unexported until the
// database driver is productized again.
type databaseStore struct {
	codec *securecookie.SecureCookie
	db    sessionDB
}

func newDatabaseStore(keys Keys, db sessionDB) (*databaseStore, error) {
	if len(keys.Authentication) < 32 {
		return nil, errors.New("kiks: authentication key must be at least 32 bytes")
	}
	if db == nil {
		return nil, errors.New("kiks: database store requires a sessionDB")
	}
	return &databaseStore{
		codec: securecookie.New(keys.Authentication, nil),
		db:    db,
	}, nil
}

func (store *databaseStore) setMaxAge(seconds int) {
	if seconds > 0 {
		store.codec.MaxAge(seconds)
	}
}

func (store *databaseStore) Load(r *http.Request, name string) ([]byte, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, err
	}
	var id string
	if err := store.codec.Decode(name, cookie.Value, &id); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	var payload []byte
	err = store.db.QueryRow(
		r.Context(),
		`SELECT payload FROM sessions WHERE id = $1 AND expires_at > $2`,
		id,
		time.Now().UTC(),
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return payload, nil
}

func (store *databaseStore) Save(
	w http.ResponseWriter,
	r *http.Request,
	name string,
	payload []byte,
	attrs CookieAttrs,
) error {
	if attrs.MaxAge <= 0 {
		return errors.New("kiks: session MaxAge must be greater than zero")
	}
	id, err := store.sessionID(r, name)
	if err != nil {
		return err
	}
	if id == "" {
		id, err = newSessionID()
		if err != nil {
			return err
		}
	}
	expiresAt := time.Now().UTC().Add(time.Duration(attrs.MaxAge) * time.Second)
	if _, err := store.db.Exec(
		r.Context(),
		`INSERT INTO sessions (id, payload, expires_at) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET payload = EXCLUDED.payload, expires_at = EXCLUDED.expires_at`,
		id,
		payload,
		expiresAt,
	); err != nil {
		return err
	}
	raw, err := store.codec.Encode(name, id)
	if err != nil {
		return err
	}
	http.SetCookie(w, newCookieFromAttrs(name, raw, attrs))
	return nil
}

func (store *databaseStore) Destroy(
	w http.ResponseWriter,
	r *http.Request,
	name string,
	attrs CookieAttrs,
) error {
	id, err := store.sessionID(r, name)
	if err != nil && !isCorruptCookie(err) {
		return err
	}
	if id != "" {
		_, _ = store.db.Exec(r.Context(), `DELETE FROM sessions WHERE id = $1`, id)
	}
	http.SetCookie(w, expireCookieFromAttrs(name, attrs))
	return nil
}

func (store *databaseStore) sessionID(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", nil
		}
		return "", err
	}
	var id string
	if err := store.codec.Decode(name, cookie.Value, &id); err != nil {
		return "", err
	}
	return id, nil
}

func newSessionID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func newCookieFromAttrs(name, value string, attrs CookieAttrs) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     attrs.Path,
		Domain:   attrs.Domain,
		MaxAge:   attrs.MaxAge,
		Secure:   attrs.Secure,
		HttpOnly: attrs.HTTPOnly,
		SameSite: attrs.SameSite,
	}
}

func expireCookieFromAttrs(name string, attrs CookieAttrs) *http.Cookie {
	cookie := newCookieFromAttrs(name, "", attrs)
	cookie.MaxAge = -1
	return cookie
}
