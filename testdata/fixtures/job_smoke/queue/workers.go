package queue

import (
	"github.com/mbvlabs/andurel/pkg/postmand"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/riverqueue/river"
)

func NewWorkers(
	transactional postmand.TransactionalSender,
	marketing postmand.MarketingSender,
) *river.Workers {
	workers := river.NewWorkers()
	storage.AddEmailWorkers(workers, transactional, marketing)

	return workers
}
