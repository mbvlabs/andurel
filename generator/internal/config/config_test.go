package config

import (
	"reflect"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	got := NewDefaultConfig()
	want := &Config{
		MigrationDirs: []string{"database/migrations"},
		DatabaseType:  "postgresql",
		PackageName:   "models",
		GenerateJSON:  true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewDefaultConfig() = %#v, want %#v", got, want)
	}
}
