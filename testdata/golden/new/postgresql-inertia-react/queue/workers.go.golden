package queue

import (
	"github.com/mbvlabs/andurel/pkg/email"
	"github.com/riverqueue/river"
)

func NewWorkers(
	transactional email.TransactionalSender,
	marketing email.MarketingSender,
) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendTransactionalEmailWorker(transactional))
	river.AddWorker(workers, NewSendMarketingEmailWorker(marketing))

	return workers
}
