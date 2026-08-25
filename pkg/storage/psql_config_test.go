package storage

import "testing"

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.DatabaseKind != DefaultDatabaseKind ||
		config.Host != DefaultDatabaseHost ||
		config.Port != DefaultDatabasePort ||
		config.Name != DefaultDatabaseName ||
		config.User != DefaultDatabaseUser ||
		config.Password != DefaultDatabasePassword ||
		config.SSLMode != DefaultDatabaseSSLMode {
		t.Fatalf("unexpected default config: %#v", config)
	}
}

func TestConfigDatabaseURL(t *testing.T) {
	config := Config{
		DatabaseKind: "postgres",
		Host:         "::1",
		Port:         "5432",
		Name:         "application db",
		User:         "user@example.com",
		Password:     "secret:/?",
		SSLMode:      "require",
	}

	got, err := config.DatabaseURL()
	if err != nil {
		t.Fatalf("DatabaseURL returned an error: %v", err)
	}
	want := "postgres://user%40example.com:secret%3A%2F%3F@[::1]:5432/application%20db?sslmode=require"
	if got != want {
		t.Fatalf("DatabaseURL = %q, want %q", got, want)
	}
}

func TestConfigDatabaseURLRejectsMissingValues(t *testing.T) {
	config := DefaultConfig()
	config.Host = ""
	if _, err := config.DatabaseURL(); err == nil {
		t.Fatal("DatabaseURL accepted an empty host")
	}
}
