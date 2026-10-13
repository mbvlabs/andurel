package queue

import (
	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/riverqueue/river"
)

// NewWorkers builds the River worker registry and registers every queue worker
// on it. This is the explicit, ordered worker list: adding a worker is one line
// here. There is no dependency-injection container and no reflection.
func NewWorkers(
	transactional email.TransactionalSender,
	marketing email.MarketingSender,
) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendTransactionalEmailWorker(transactional))
	river.AddWorker(workers, NewSendMarketingEmailWorker(marketing))

	return workers
}
