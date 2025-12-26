// Package storage provides abstractions for queue interactions and default implementations.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type Queue interface {
	Shutdown(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type InsertQueue interface {
	Insert(
		ctx context.Context,
		args river.JobArgs,
		opts *river.InsertOpts,
	) (*rivertype.JobInsertResult, error)
	InsertTx(
		ctx context.Context,
		tx pgx.Tx,
		args river.JobArgs,
		opts *river.InsertOpts,
	) (*rivertype.JobInsertResult, error)
	InsertMany(
		ctx context.Context,
		params []river.InsertManyParams,
	) ([]*rivertype.JobInsertResult, error)
	InsertManyTx(
		ctx context.Context,
		tx pgx.Tx,
		params []river.InsertManyParams,
	) ([]*rivertype.JobInsertResult, error)
	InsertManyFast(ctx context.Context, params []river.InsertManyParams) (int, error)
	InsertManyFastTx(ctx context.Context, tx pgx.Tx, params []river.InsertManyParams) (int, error)
}
