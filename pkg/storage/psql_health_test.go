package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type healthConnector struct {
	pingErr error
}

func (c healthConnector) Connect(context.Context) (driver.Conn, error) {
	return healthConnection{pingErr: c.pingErr}, nil
}

func (healthConnector) Driver() driver.Driver {
	return healthDriver{}
}

type healthDriver struct{}

func (healthDriver) Open(string) (driver.Conn, error) {
	return healthConnection{}, nil
}

type healthConnection struct {
	pingErr error
}

func (healthConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (healthConnection) Close() error {
	return nil
}

func (healthConnection) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c healthConnection) Ping(context.Context) error {
	return c.pingErr
}

func TestPostgresHealth(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
	}{
		{name: "healthy"},
		{name: "unhealthy", pingErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB := sql.OpenDB(healthConnector{pingErr: tt.pingErr})
			db := &Postgres{bun: bun.NewDB(sqlDB, pgdialect.New())}
			t.Cleanup(func() { _ = db.Close() })

			err := db.Health(context.Background())
			if tt.pingErr == nil {
				if err != nil {
					t.Fatalf("Health: %v", err)
				}
				return
			}

			if !errors.Is(err, tt.pingErr) {
				t.Fatalf("Health error = %v, want wrapped %v", err, tt.pingErr)
			}
			if !strings.Contains(err.Error(), "storage: ping database") {
				t.Fatalf("Health error = %q, want storage context", err)
			}
		})
	}
}
