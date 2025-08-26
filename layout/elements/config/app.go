package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
)

var App Application = newApp()

const (
	DevEnvironment  = "development"
	ProdEnvironment = "production"
	TestEnvironment = "testing"
)

type Application struct {
	ServerHost  string `env:"SERVER_HOST"`
	ServerPort  string `env:"SERVER_PORT"`
	Domain      string `env:"APP_DOMAIN"`
	ProjectName string `env:"PROJECT_NAME"`
	Env         string `env:"ENVIRONMENT"`
}

func (a Application) GetFullDomain() string {
	if a.Env == ProdEnvironment {
		return fmt.Sprintf("%v://%v", "https", a.Domain)
	}

	return fmt.Sprintf(
		"%v://%v:%v",
		"http",
		a.Domain,
		a.ServerPort,
	)
}

func newApp() Application {
	appCfg := Application{}

	if err := env.ParseWithOptions(&appCfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return appCfg
}
