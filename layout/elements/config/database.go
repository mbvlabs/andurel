package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
)

var DB Database = newDatabase()

type Database struct {
	Port         string `env:"DB_PORT"`
	Host         string `env:"DB_HOST"`
	Name         string `env:"DB_NAME"`
	User         string `env:"DB_USER"`
	Password     string `env:"DB_PASSWORD"`
	DatabaseKind string `env:"DB_KIND"`
	SslMode      string `env:"DB_SSL_MODE"`
}

func (d Database) GetDatabaseURL() string {
	return fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		d.DatabaseKind, d.User, d.Password, d.Host, d.Port,
		d.Name, d.SslMode,
	)
}

func newDatabase() Database {
	dataCfg := Database{}

	if err := env.ParseWithOptions(&dataCfg, env.Options{
		RequiredIfNoDef: true,
	}); err != nil {
		panic(err)
	}

	return dataCfg
}
