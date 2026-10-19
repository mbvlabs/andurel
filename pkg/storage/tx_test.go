package storage

import (
	"context"
	"errors"
	"testing"
)

type txRecorder struct {
	begun      int
	committed  int
	rolledBack int
	rollbackOK bool
}

func (r *txRecorder) connection() stubConnection {
	return stubConnection{
		begin: func(context.Context) (Transaction, error) {
			r.begun++
			// A pointer, so Tx values wrapping it stay comparable.
			return &stubTransaction{
				commit: func(context.Context) error {
					r.committed++
					return nil
				},
				rollback: func(ctx context.Context) error {
					r.rolledBack++
					r.rollbackOK = ctx.Err() == nil
					return nil
				},
			}, nil
		},
	}
}

func TestAtomicOpsCommitsWhenFnReturnsNil(t *testing.T) {
	var rec txRecorder

	err := AtomicOps(t.Context(), NewTransactor(rec.connection()), func(context.Context, Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if rec.committed != 1 || rec.rolledBack != 0 {
		t.Fatalf("committed=%d rolledBack=%d, want 1/0", rec.committed, rec.rolledBack)
	}
}

func TestAtomicOpsRollsBackAndReturnsFnError(t *testing.T) {
	var rec txRecorder
	errBoom := errors.New("boom")

	err := AtomicOps(t.Context(), NewTransactor(rec.connection()), func(context.Context, Tx) error {
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want %v", err, errBoom)
	}
	if rec.committed != 0 || rec.rolledBack != 1 {
		t.Fatalf("committed=%d rolledBack=%d, want 0/1", rec.committed, rec.rolledBack)
	}
}

func TestAtomicOpsRollsBackAndRepanics(t *testing.T) {
	var rec txRecorder

	defer func() {
		if recover() == nil {
			t.Fatal("expected the panic to propagate")
		}
		if rec.committed != 0 || rec.rolledBack != 1 {
			t.Fatalf("committed=%d rolledBack=%d, want 0/1", rec.committed, rec.rolledBack)
		}
	}()

	_ = AtomicOps(t.Context(), NewTransactor(rec.connection()), func(context.Context, Tx) error {
		panic("boom")
	})
}

func TestAtomicOpsRollsBackWithLiveContextAfterCancel(t *testing.T) {
	var rec txRecorder
	ctx, cancel := context.WithCancel(t.Context())

	err := AtomicOps(ctx, NewTransactor(rec.connection()), func(ctx context.Context, _ Tx) error {
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if rec.rolledBack != 1 || !rec.rollbackOK {
		t.Fatalf("rolledBack=%d rollbackOK=%v, want 1/true", rec.rolledBack, rec.rollbackOK)
	}
}

func TestAtomicOpsOnBoundTransactorJoinsWithoutCommitting(t *testing.T) {
	var rec txRecorder
	transactor := NewTransactor(rec.connection())

	err := AtomicOps(t.Context(), transactor, func(ctx context.Context, tx Tx) error {
		inner := 0
		if err := AtomicOps(ctx, transactor.InTx(tx), func(_ context.Context, joined Tx) error {
			inner++
			if joined != tx {
				t.Fatal("joined AtomicOps must receive the outer Tx")
			}
			return nil
		}); err != nil {
			return err
		}
		if inner != 1 {
			t.Fatalf("inner ran %d times, want 1", inner)
		}
		if rec.committed != 0 {
			t.Fatal("joined AtomicOps must not commit")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("AtomicOps: %v", err)
	}
	if rec.begun != 1 || rec.committed != 1 {
		t.Fatalf("begun=%d committed=%d, want 1/1", rec.begun, rec.committed)
	}
}

func TestAtomicOpsJoinedErrorRollsBackOuter(t *testing.T) {
	var rec txRecorder
	transactor := NewTransactor(rec.connection())
	errInner := errors.New("inner")

	err := AtomicOps(t.Context(), transactor, func(ctx context.Context, tx Tx) error {
		return AtomicOps(ctx, transactor.InTx(tx), func(context.Context, Tx) error {
			return errInner
		})
	})
	if !errors.Is(err, errInner) {
		t.Fatalf("err = %v, want %v", err, errInner)
	}
	if rec.committed != 0 || rec.rolledBack != 1 {
		t.Fatalf("committed=%d rolledBack=%d, want 0/1", rec.committed, rec.rolledBack)
	}
}

func TestUnwrapPanicsOnZeroTx(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected Unwrap to panic on a zero Tx")
		}
	}()

	Unwrap(Tx{})
}
