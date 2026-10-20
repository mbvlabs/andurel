package clean

import (
	"context"

	"app/domains"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mbvlabs/andurel/pkg/storage"
)

var _ pgtype.Text

type Service struct {
	transactor storage.Transactor
	users      domains.Users
	queue      storage.InsertQueue
}

// Bound locals are clean.
func (s Service) Run(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		users, queue := s.users.InTx(tx), s.queue.InTx(tx)
		if _, err := users.Find(ctx, [16]byte{}); err != nil {
			return err
		}
		_, err := queue.Insert(ctx, nil, nil)
		return err
	})
}

// A method-value callback whose body binds what it uses is clean.
func (s Service) Method(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, s.work)
}

func (s Service) work(ctx context.Context, tx storage.Tx) error {
	users := s.users.InTx(tx)
	_, err := users.Find(ctx, [16]byte{})
	return err
}

// Passing the non-bindable receiver itself does not make its fields captured.
func (s Service) RunAggregate(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return s.helper(s)
	})
}

func (s Service) helper(Service) error { return nil }
