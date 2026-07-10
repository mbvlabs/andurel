package config

import "github.com/caarlos0/env/v11"

type app struct {
	Host                 string   `env:"HOST" envDefault:"localhost"`
	Port                 string   `env:"PORT" envDefault:"8080"`
	SessionKey           string   `env:"SESSION_KEY"`
	SessionEncryptionKey string   `env:"SESSION_ENCRYPTION_KEY"`
	TokenSigningKey      string   `env:"TOKEN_SIGNING_KEY"`
	CSRFStrategy         string   `env:"CSRF_STRATEGY" envDefault:"header_only"`
	CSRFTrustedOrigins   []string `env:"CSRF_TRUSTED_ORIGINS" envSeparator:","`
}

func newAppConfig() app {
	appCfg := app{}

	if err := env.ParseWithOptions(&appCfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return appCfg
}
