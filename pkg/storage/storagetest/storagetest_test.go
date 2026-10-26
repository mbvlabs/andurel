package storagetest

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func TestRunMainSkipsWhenClusterFails(t *testing.T) {
	var cluster *Cluster
	sentinel := errors.New("docker unavailable")

	code, err := runMain(
		func() int {
			t.Fatal("tests must not run when the cluster fails")

			return 0
		},
		&cluster,
		func(context.Context) (*Cluster, error) {
			return nil, sentinel
		},
	)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("runMain error = %v, want %v", err, sentinel)
	}
	if cluster != nil {
		t.Fatalf("cluster = %#v, want nil", cluster)
	}
}

func TestRunMainRunsTestsAndClosesCluster(t *testing.T) {
	var cluster *Cluster
	started := &Cluster{}

	code, err := runMain(
		func() int { return 7 },
		&cluster,
		func(context.Context) (*Cluster, error) {
			return started, nil
		},
	)
	if err != nil {
		t.Fatalf("runMain error = %v, want nil", err)
	}
	if code != 7 {
		t.Fatalf("code = %d, want 7", code)
	}
	if cluster != started {
		t.Fatalf("cluster = %#v, want %#v", cluster, started)
	}
}

func TestClusterDatabaseURL(t *testing.T) {
	cluster := &Cluster{
		host:     "127.0.0.1",
		port:     "55432",
		user:     "testuser",
		password: "testpass",
		adminDB:  "postgres",
	}

	got := cluster.databaseURL("test_123")
	want := "postgres://testuser:testpass@127.0.0.1:55432/test_123?sslmode=disable"
	if got != want {
		t.Fatalf("databaseURL = %q, want %q", got, want)
	}
}

func TestClusterDatabaseConfig(t *testing.T) {
	cluster := &Cluster{
		host:     "127.0.0.1",
		port:     "55432",
		user:     "testuser",
		password: "testpass",
		adminDB:  "postgres",
	}

	config := cluster.databaseConfig("test_123")

	want := storage.DefaultConfig()
	want.Host = cluster.host
	want.Port = cluster.port
	want.Name = "test_123"
	want.User = cluster.user
	want.Password = cluster.password
	want.SSLMode = "disable"
	if !reflect.DeepEqual(config, want) {
		t.Fatalf("databaseConfig = %#v, want %#v", config, want)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("databaseConfig.Validate = %v, want nil", err)
	}

	databaseURL, err := config.DatabaseURL()
	if err != nil {
		t.Fatalf("databaseConfig.DatabaseURL = %v", err)
	}
	if wantURL := cluster.databaseURL("test_123"); databaseURL != wantURL {
		t.Fatalf("databaseConfig.DatabaseURL = %q, want %q", databaseURL, wantURL)
	}
}

func TestClusterCloseNil(t *testing.T) {
	var cluster *Cluster
	if err := cluster.Close(context.Background()); err != nil {
		t.Fatalf("Close on nil cluster = %v, want nil", err)
	}

	cluster = &Cluster{}
	if err := cluster.Close(context.Background()); err != nil {
		t.Fatalf("Close on cluster without container = %v, want nil", err)
	}
}

func TestNewClusterPreflightErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("provider discovery panic", func(t *testing.T) {
		cluster, err := newCluster(ctx, func(context.Context) error {
			panic("docker host not found")
		})

		if cluster != nil {
			t.Fatalf("newCluster cluster = %#v, want nil", cluster)
		}
		if err == nil {
			t.Fatal("newCluster error = nil, want panic converted to error")
		}
		for _, want := range []string{"start postgres container", "check docker provider", "docker host not found"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("newCluster error = %q, want %q", err, want)
			}
		}
	})

	t.Run("provider discovery error", func(t *testing.T) {
		sentinel := errors.New("docker socket unavailable")
		cluster, err := newCluster(ctx, func(context.Context) error {
			return sentinel
		})

		if cluster != nil {
			t.Fatalf("newCluster cluster = %#v, want nil", cluster)
		}
		if !errors.Is(err, sentinel) {
			t.Fatalf("newCluster error = %v, want wrapped %v", err, sentinel)
		}
		if !strings.Contains(err.Error(), "check docker provider") {
			t.Fatalf("newCluster error = %q, want docker provider context", err)
		}
	})
}

func TestValidateMigrations(t *testing.T) {
	migrations := fstest.MapFS{
		"migrations/00001_init.sql": &fstest.MapFile{Data: []byte("-- +goose Up\n")},
	}

	tests := []struct {
		name         string
		migrations   fs.FS
		migrationDir string
		want         string
	}{
		{
			name:         "nil filesystem",
			migrations:   nil,
			migrationDir: "migrations",
			want:         "migrations filesystem is required",
		},
		{
			name:         "blank directory",
			migrations:   migrations,
			migrationDir: "  ",
			want:         "migration directory is required",
		},
		{
			name:         "valid",
			migrations:   migrations,
			migrationDir: "migrations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMigrations(tt.migrations, tt.migrationDir)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateMigrations = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateMigrations = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Fatalf("validateMigrations = %q, want %q", err, tt.want)
			}
		})
	}
}
