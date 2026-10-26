package config

import (
	"fmt"

	"github.com/mbvlabs/andurel/pkg/env"
)

type Auth struct {
	TokenSigningKey string
	Pepper          string
	PreviousPeppers []string
}

func NewAuth() (Auth, error) {
	e := env.New()
	cfg := Auth{
		TokenSigningKey: e.RequiredString("TOKEN_SIGNING_KEY"),
		Pepper:          e.RequiredString("PEPPER"),
		PreviousPeppers: e.Strings("PREVIOUS_PEPPERS", nil),
	}
	if err := e.Err(); err != nil {
		return Auth{}, fmt.Errorf("config: auth: %w", err)
	}

	return cfg, nil
}
