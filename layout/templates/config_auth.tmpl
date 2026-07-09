package config

import "github.com/caarlos0/env/v10"

type auth struct {
	Pepper          string   `env:"PEPPER"`
	PreviousPeppers []string `env:"PREVIOUS_PEPPERS" envSeparator:"," envDefault:""`
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
