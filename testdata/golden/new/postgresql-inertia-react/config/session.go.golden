package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gosimple/slug"
)

const DefaultSessionMaxAge = 604800

type Session struct {
	Name              string
	AuthenticationKey []byte
	EncryptionKey     []byte
	MaxAge            int
}

func NewSession(app App) (Session, error) {
	env := newEnvironment()
	authenticationKey := env.RequiredString("SESSION_KEY")
	encryptionKey := env.RequiredString("SESSION_ENCRYPTION_KEY")
	cfg := Session{
		Name:   "app_sess_" + slug.Make(strings.ToLower(app.ProjectName)) + "-" + app.Environment,
		MaxAge: env.Int("SESSION_MAX_AGE", DefaultSessionMaxAge),
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
	errs = append(errs, env.Err())
	if err := errors.Join(errs...); err != nil {
		return Session{}, fmt.Errorf("config: session: %w", err)
	}

	return cfg, nil
}
