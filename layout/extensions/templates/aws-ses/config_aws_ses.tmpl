package config

import (
	"github.com/caarlos0/env/v10"
)

type awsSes struct {
	Region           string `env:"AWS_REGION" envDefault:"us-east-1"`
	AccessKeyID      string `env:"AWS_SES_ACCESS_KEY_ID"`
	SecretAccessKey  string `env:"AWS_SES_SECRET_ACCESS_KEY"`
	ConfigurationSet string `env:"AWS_SES_CONFIGURATION_SET"`
}

func newAwsSesConfig() awsSes {
	cfg := awsSes{}

	if err := env.ParseWithOptions(&cfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return cfg
}
