package public

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Service struct {
	transactor storage.Transactor
}

func (s Service) Run(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return s.Other(ctx) // want "tx-public-call"
	})
}

func (s Service) AliasBefore(ctx context.Context) error {
	self := s
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return self.Other(ctx) // want "tx-public-call"
	})
}

func (s Service) AliasInside(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		self := s
		return self.Other(ctx) // want "tx-public-call"
	})
}

func (s Service) PointerAlias(ctx context.Context) error {
	self := &s
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return self.Other(ctx) // want "tx-public-call"
	})
}

func (s Service) MethodValueBefore(ctx context.Context) error {
	fn := s.Other
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return fn(ctx) // want "tx-public-call"
	})
}

func (s Service) MethodValueInside(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		fn := s.Other // want "tx-public-call"
		return fn(ctx)
	})
}

func (s Service) Unicode(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		return s.Ünicode(ctx) // want "tx-public-call"
	})
}

func (s Service) Shadow(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		s := Service{}
		return s.Other(ctx)
	})
}

func (s Service) Other(ctx context.Context) error { return nil }

func (s Service) Ünicode(ctx context.Context) error { return nil }
