package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gosimple/slug"
	"github.com/mbvlabs/andurel/pkg/env"
)

const DefaultSessionMaxAge = 604800

type Session struct {
	Name              string
	AuthenticationKey []byte
	EncryptionKey     []byte
	MaxAge            int
	// CookieDomain is the Domain attribute for the session and CSRF cookies.
	// Empty keeps them host-only, one set per domain. Set it (for example
	// ".example.com") to share a sign-in across subdomains.
	CookieDomain string
}

func NewSession(app App) (Session, error) {
	e := env.New()
	authenticationKey := e.RequiredString("SESSION_KEY")
	encryptionKey := e.RequiredString("SESSION_ENCRYPTION_KEY")
	cfg := Session{
		Name:         "app_sess_" + slug.Make(strings.ToLower(app.ProjectName)) + "-" + app.Environment,
		MaxAge:       e.Int("SESSION_MAX_AGE", DefaultSessionMaxAge),
		CookieDomain: strings.TrimSpace(e.String("SESSION_COOKIE_DOMAIN", "")),
	}
	var errs []error
	if value, err := hex.DecodeString(authenticationKey); err != nil {
		errs = append(errs, fmt.Errorf("SESSION_KEY must be hexadecimal: %w", err))
	} else {
		cfg.AuthenticationKey = value
	}
	if value, err := hex.DecodeString(encryptionKey); err != nil {
		errs = append(errs, fmt.Errorf("SESSION_ENCRYPTION_KEY must be hexadecimal: %w", err))
	} else {
		cfg.EncryptionKey = value
	}
	if cfg.MaxAge < 1 {
		errs = append(errs, fmt.Errorf("SESSION_MAX_AGE must be greater than zero"))
	}
	if strings.ContainsAny(cfg.CookieDomain, "/:") {
		errs = append(errs, fmt.Errorf(
			"SESSION_COOKIE_DOMAIN must be a bare domain such as example.com or .example.com, without a scheme, path or port",
		))
	}
	errs = append(errs, e.Err())
	if err := errors.Join(errs...); err != nil {
		return Session{}, fmt.Errorf("config: session: %w", err)
	}

	return cfg, nil
}
