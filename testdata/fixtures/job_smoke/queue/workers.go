package queue

import (
	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/riverqueue/river"
)

func NewWorkers(
	transactional email.TransactionalSender,
	marketing email.MarketingSender,
) *river.Workers {
	workers := river.NewWorkers()
	storage.AddEmailWorkers(workers, transactional, marketing)

	return workers
}
