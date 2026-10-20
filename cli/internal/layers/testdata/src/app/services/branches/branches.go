package branches

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Service struct {
	transactor storage.Transactor
	users      domains.Users
}

func (s Service) IfUnsafe(ctx context.Context, cond bool) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		if cond {
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) IfBound(ctx context.Context, cond bool) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		if cond {
			users = s.users.InTx(tx)
		} else {
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

func (s Service) LoopUnsafe(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		for i := 0; i < 2; i++ {
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) LoopBound(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		users := s.users.InTx(tx)
		for i := 0; i < 2; i++ {
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

func (s Service) RangeAssign(ctx context.Context, items []domains.Users) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		users := s.users.InTx(tx)
		for _, users = range items {
		}
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) RangeDefine(ctx context.Context, items []domains.Users) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		for _, users := range items {
			_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
			return err
		}
		return nil
	})
}

func (s Service) RangeDefineBound(ctx context.Context, items []domains.Users) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		for _, users := range items {
			bound := users.InTx(tx)
			_, err := bound.Find(ctx, [16]byte{})
			return err
		}
		return nil
	})
}

func (s Service) RangeAssignBound(ctx context.Context, items []domains.Users) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		users := s.users.InTx(tx)
		for _, users = range items {
			users = users.InTx(tx)
			_, err := users.Find(ctx, [16]byte{})
			return err
		}
		return nil
	})
}

func (s Service) SwitchUnsafe(ctx context.Context, value int) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		switch value {
		case 1:
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) SwitchBound(ctx context.Context, value int) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		switch value {
		case 1:
			users = s.users.InTx(tx)
		default:
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

func (s Service) SelectUnsafe(ctx context.Context, ch chan int) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		select {
		case <-ch:
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{}) // want "tx-escape"
		return err
	})
}

func (s Service) SelectBound(ctx context.Context, ch chan int) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		var users domains.Users
		select {
		case <-ch:
			users = s.users.InTx(tx)
		default:
			users = s.users.InTx(tx)
		}
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}
