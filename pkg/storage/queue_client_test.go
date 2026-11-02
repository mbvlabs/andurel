package storage

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

func TestQueueInsertClientReturnsUnderlyingClient(t *testing.T) {
	client := &river.Client[pgx.Tx]{}
	queue := &QueueInsert{client: client}

	if got := queue.Client(); got != client {
		t.Fatalf("Client() = %p, want %p", got, client)
	}
}

func TestQueueInsertInTxKeepsClient(t *testing.T) {
	client := &river.Client[pgx.Tx]{}
	queue := &QueueInsert{client: client}

	bound, ok := queue.InTx(Tx{}).(*QueueInsert)
	if !ok {
		t.Fatalf("InTx returned %T, want *QueueInsert", bound)
	}
	if got := bound.Client(); got != client {
		t.Fatalf("bound Client() = %p, want %p", got, client)
	}
}
