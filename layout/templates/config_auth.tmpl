package config

import "github.com/caarlos0/env/v10"

type auth struct {
	PasswordSalt         string `env:"PASSWORD_SALT"`
	SessionKey           string `env:"SESSION_KEY"`
	SessionEncryptionKey string `env:"SESSION_ENCRYPTION_KEY"`
	TokenSigningKey      string `env:"TOKEN_SIGNING_KEY"`
}

func newAuthConfig() auth {
	authenticationCfg := auth{}

	if err := env.ParseWithOptions(&authenticationCfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return authenticationCfg
}
