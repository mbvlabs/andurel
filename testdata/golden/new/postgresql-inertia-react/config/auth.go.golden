package config

import (
	"fmt"
)

type Auth struct {
	TokenSigningKey string
	Pepper          string
	PreviousPeppers []string
}

func NewAuth() (Auth, error) {
	env := newEnvironment()
	cfg := Auth{
		TokenSigningKey: env.RequiredString("TOKEN_SIGNING_KEY"),
		Pepper:          env.RequiredString("PEPPER"),
		PreviousPeppers: env.Strings("PREVIOUS_PEPPERS", nil),
	}
	if err := env.Err(); err != nil {
		return Auth{}, fmt.Errorf("config: auth: %w", err)
	}

	return cfg, nil
}
