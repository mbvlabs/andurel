package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type optionRecordingConnection struct {
	options pgx.TxOptions
	calls   int
}

func (c *optionRecordingConnection) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (c *optionRecordingConnection) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (c *optionRecordingConnection) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (c *optionRecordingConnection) Health(context.Context) error {
	return nil
}

func (c *optionRecordingConnection) BeginTransaction(context.Context) (Transaction, error) {
	c.calls++

	return stubTransaction{}, nil
}

func (c *optionRecordingConnection) BeginTransactionWithOptions(
	_ context.Context,
	options pgx.TxOptions,
) (Transaction, error) {
	c.calls++
	c.options = options

	return stubTransaction{}, nil
}

func TestAtomicOpsAppliesReadOnly(t *testing.T) {
	conn := &optionRecordingConnection{}

	err := AtomicOps(t.Context(), NewTransactor(conn), func(context.Context, Tx) error {
		return nil
	}, ReadOnly())
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if conn.calls != 1 {
		t.Fatalf("begin calls = %d, want 1", conn.calls)
	}
	if conn.options.AccessMode != pgx.ReadOnly {
		t.Fatalf("access mode = %q, want %q", conn.options.AccessMode, pgx.ReadOnly)
	}
	if conn.options.IsoLevel != "" {
		t.Fatalf("isolation level = %q, want default", conn.options.IsoLevel)
	}
}

func TestAtomicOpsAppliesIsolation(t *testing.T) {
	conn := &optionRecordingConnection{}

	err := AtomicOps(t.Context(), NewTransactor(conn), func(context.Context, Tx) error {
		return nil
	}, Isolation(RepeatableRead))
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if conn.options.IsoLevel != pgx.RepeatableRead {
		t.Fatalf("isolation level = %q, want %q", conn.options.IsoLevel, pgx.RepeatableRead)
	}
	if conn.options.AccessMode != "" {
		t.Fatalf("access mode = %q, want default", conn.options.AccessMode)
	}
}

func TestAtomicOpsCombinesTransactionOptions(t *testing.T) {
	conn := &optionRecordingConnection{}

	err := AtomicOps(t.Context(), NewTransactor(conn), func(context.Context, Tx) error {
		return nil
	}, ReadOnly(), Isolation(Serializable))
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if conn.options.AccessMode != pgx.ReadOnly {
		t.Fatalf("access mode = %q, want %q", conn.options.AccessMode, pgx.ReadOnly)
	}
	if conn.options.IsoLevel != pgx.Serializable {
		t.Fatalf("isolation level = %q, want %q", conn.options.IsoLevel, pgx.Serializable)
	}
}

func TestAtomicOpsRejectsUnsupportedIsolation(t *testing.T) {
	conn := &optionRecordingConnection{}

	err := AtomicOps(t.Context(), NewTransactor(conn), func(context.Context, Tx) error {
		return nil
	}, Isolation(IsolationLevel("bogus")))
	if err == nil || !strings.Contains(err.Error(), "unsupported isolation level") {
		t.Fatalf("AtomicOps error = %v, want unsupported isolation level", err)
	}
	if conn.calls != 0 {
		t.Fatalf("begin calls = %d, want 0", conn.calls)
	}
}

func TestAtomicOpsRejectsOptionsOnBoundTransactor(t *testing.T) {
	conn := &optionRecordingConnection{}
	bound := NewTransactor(conn).InTx(Tx{tx: stubTransaction{}})

	err := AtomicOps(t.Context(), bound, func(context.Context, Tx) error {
		return nil
	}, ReadOnly())
	if err == nil || !strings.Contains(err.Error(), "unbound Transactor") {
		t.Fatalf("AtomicOps error = %v, want unbound Transactor error", err)
	}
	if conn.calls != 0 {
		t.Fatalf("begin calls = %d, want 0", conn.calls)
	}
}
