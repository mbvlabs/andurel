package candidates

import (
	"context"
	"errors"

	"app/domains"
	"app/services/global"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Repo struct {
	GlobalUsers domains.Users
}

type Holder struct {
	users domains.Users
}

type Service struct {
	transactor storage.Transactor
	repo       Repo
}

var globalUsers domains.Users

func (s Service) usersOf() domains.Users { return domains.Users{} }

func (s Service) Calls(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err1 := s.usersOf().Find(ctx, [16]byte{})        // want "tx-escape"
		_, err2 := s.repo.GlobalUsers.Find(ctx, [16]byte{}) // want "tx-escape"
		repos := map[string]domains.Users{}
		_, err3 := repos["x"].Find(ctx, [16]byte{}) // want "tx-escape"
		var anyUsers any = domains.Users{}
		_, err4 := anyUsers.(domains.Users).Find(ctx, [16]byte{}) // want "tx-escape"
		_, err5 := global.Users.Find(ctx, [16]byte{})             // want "tx-escape"
		_ = globalUsers                                           // want "tx-escape"
		_ = domains.Users{}
		_ = Holder{users: domains.Users{}}
		return errors.Join(err1, err2, err3, err4, err5)
	})
}

func (s Service) Bound(ctx context.Context) error {
	return storage.AtomicOps(ctx, s.transactor, func(ctx context.Context, tx storage.Tx) error {
		users := s.usersOf().InTx(tx)
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}
