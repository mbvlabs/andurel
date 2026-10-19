// Package storage provides abstractions for queue interactions and default implementations.
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// InsertQueue is the subset of River used to insert jobs. Whether an insert
// is transactional depends only on whether the queue was bound with InTx.
type InsertQueue interface {
	Insert(context.Context, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
	InsertMany(context.Context, []river.InsertManyParams) ([]*rivertype.JobInsertResult, error)
	InsertManyFast(context.Context, []river.InsertManyParams) (int, error)
	// InTx returns a queue whose inserts join tx: the jobs become visible to
	// workers only if tx commits.
	InTx(tx Tx) InsertQueue
}

// QueueOption configures the underlying River client.
type QueueOption func(*river.Config)

// WithRiverConfig replaces the complete River configuration. Options are
// applied in order, so field-specific options after this one override it.
func WithRiverConfig(config river.Config) QueueOption {
	config = cloneRiverConfig(config)
	return func(target *river.Config) { *target = cloneRiverConfig(config) }
}

func cloneRiverConfig(config river.Config) river.Config {
	config.Queues = maps.Clone(config.Queues)
	config.ReindexerIndexNames = slices.Clone(config.ReindexerIndexNames)
	config.Hooks = slices.Clone(config.Hooks)
	config.Middleware = slices.Clone(config.Middleware)
	config.PeriodicJobs = slices.Clone(config.PeriodicJobs)
	return config
}

func WithRiverAdvisoryLockPrefix(prefix int32) QueueOption {
	return func(config *river.Config) { config.AdvisoryLockPrefix = prefix }
}

func WithRiverCancelledJobRetentionPeriod(period time.Duration) QueueOption {
	return func(config *river.Config) { config.CancelledJobRetentionPeriod = period }
}

func WithRiverCompletedJobRetentionPeriod(period time.Duration) QueueOption {
	return func(config *river.Config) { config.CompletedJobRetentionPeriod = period }
}

func WithRiverDiscardedJobRetentionPeriod(period time.Duration) QueueOption {
	return func(config *river.Config) { config.DiscardedJobRetentionPeriod = period }
}

func WithRiverErrorHandler(handler river.ErrorHandler) QueueOption {
	return func(config *river.Config) { config.ErrorHandler = handler }
}

func WithRiverFetchCooldown(cooldown time.Duration) QueueOption {
	return func(config *river.Config) { config.FetchCooldown = cooldown }
}

func WithRiverFetchPollInterval(interval time.Duration) QueueOption {
	return func(config *river.Config) { config.FetchPollInterval = interval }
}

func WithRiverID(id string) QueueOption {
	return func(config *river.Config) { config.ID = id }
}

func WithRiverJobCleanerTimeout(timeout time.Duration) QueueOption {
	return func(config *river.Config) { config.JobCleanerTimeout = timeout }
}

func WithRiverJobStuckHandler(handler river.JobStuckHandler) QueueOption {
	return func(config *river.Config) { config.JobStuckHandler = handler }
}

func WithRiverJobStuckThreshold(threshold time.Duration) QueueOption {
	return func(config *river.Config) { config.JobStuckThreshold = threshold }
}

func WithRiverJobTimeout(timeout time.Duration) QueueOption {
	return func(config *river.Config) { config.JobTimeout = timeout }
}

func WithRiverHooks(hooks ...rivertype.Hook) QueueOption {
	hooks = slices.Clone(hooks)
	return func(config *river.Config) { config.Hooks = slices.Clone(hooks) }
}

func WithRiverLogger(logger *slog.Logger) QueueOption {
	return func(config *river.Config) { config.Logger = logger }
}

func WithRiverMaxAttempts(maxAttempts int) QueueOption {
	return func(config *river.Config) { config.MaxAttempts = maxAttempts }
}

func WithRiverMiddleware(middleware ...rivertype.Middleware) QueueOption {
	middleware = slices.Clone(middleware)
	return func(config *river.Config) { config.Middleware = slices.Clone(middleware) }
}

func WithRiverPeriodicJobs(jobs ...*river.PeriodicJob) QueueOption {
	jobs = slices.Clone(jobs)
	return func(config *river.Config) { config.PeriodicJobs = slices.Clone(jobs) }
}

func WithRiverPollOnly(pollOnly bool) QueueOption {
	return func(config *river.Config) { config.PollOnly = pollOnly }
}

func WithRiverQueues(queues map[string]river.QueueConfig) QueueOption {
	queues = maps.Clone(queues)
	return func(config *river.Config) { config.Queues = maps.Clone(queues) }
}

func WithRiverReindexerSchedule(schedule river.PeriodicSchedule) QueueOption {
	return func(config *river.Config) { config.ReindexerSchedule = schedule }
}

func WithRiverReindexerIndexNames(names ...string) QueueOption {
	names = slices.Clone(names)
	return func(config *river.Config) { config.ReindexerIndexNames = slices.Clone(names) }
}

func WithRiverReindexerTimeout(timeout time.Duration) QueueOption {
	return func(config *river.Config) { config.ReindexerTimeout = timeout }
}

func WithRiverRescueStuckJobsAfter(period time.Duration) QueueOption {
	return func(config *river.Config) { config.RescueStuckJobsAfter = period }
}

func WithRiverRetryPolicy(policy river.ClientRetryPolicy) QueueOption {
	return func(config *river.Config) { config.RetryPolicy = policy }
}

func WithRiverSchema(schema string) QueueOption {
	return func(config *river.Config) { config.Schema = schema }
}

func WithRiverSoftStopTimeout(timeout time.Duration) QueueOption {
	return func(config *river.Config) { config.SoftStopTimeout = timeout }
}

func WithRiverSkipUnknownJobCheck(skip bool) QueueOption {
	return func(config *river.Config) { config.SkipUnknownJobCheck = skip }
}

func WithRiverTestConfig(test river.TestConfig) QueueOption {
	return func(config *river.Config) { config.Test = test }
}

func WithRiverTestOnly(testOnly bool) QueueOption {
	return func(config *river.Config) { config.TestOnly = testOnly }
}

func WithRiverWorkers(workers *river.Workers) QueueOption {
	return func(config *river.Config) { config.Workers = workers }
}

// QueueInsert is an independently constructible River job inserter. A
// QueueInsert returned by InTx inserts on that transaction.
type QueueInsert struct {
	client *river.Client[pgx.Tx]
	bound  Tx
}

var _ InsertQueue = (*QueueInsert)(nil)

// NewQueueInsert creates an insert-only queue client using connection.
func NewQueueInsert(connection Connection, config QueueConfig, options ...QueueOption) (*QueueInsert, error) {
	client, err := newQueueClient(connection, config, options...)
	if err != nil {
		return nil, err
	}
	return &QueueInsert{client: client}, nil
}

// InTx returns a QueueInsert whose inserts join tx.
func (q *QueueInsert) InTx(tx Tx) InsertQueue {
	return &QueueInsert{client: q.client, bound: tx}
}

func (q *QueueInsert) Insert(
	ctx context.Context,
	args river.JobArgs,
	opts *river.InsertOpts,
) (*rivertype.JobInsertResult, error) {
	if q.bound.tx == nil {
		return q.client.Insert(ctx, args, opts)
	}
	pgxTx, err := pgxTxFrom(q.bound.tx)
	if err != nil {
		return nil, err
	}
	return q.client.InsertTx(ctx, pgxTx, args, opts)
}

func (q *QueueInsert) InsertMany(
	ctx context.Context,
	params []river.InsertManyParams,
) ([]*rivertype.JobInsertResult, error) {
	if q.bound.tx == nil {
		return q.client.InsertMany(ctx, params)
	}
	pgxTx, err := pgxTxFrom(q.bound.tx)
	if err != nil {
		return nil, err
	}
	return q.client.InsertManyTx(ctx, pgxTx, params)
}

func (q *QueueInsert) InsertManyFast(
	ctx context.Context,
	params []river.InsertManyParams,
) (int, error) {
	if q.bound.tx == nil {
		return q.client.InsertManyFast(ctx, params)
	}
	pgxTx, err := pgxTxFrom(q.bound.tx)
	if err != nil {
		return 0, err
	}
	return q.client.InsertManyFastTx(ctx, pgxTx, params)
}

// QueueProcessor owns the lifecycle of a River worker client.
type QueueProcessor struct {
	client *river.Client[pgx.Tx]
}

// NewQueueProcessor creates a worker client using connection.
func NewQueueProcessor(connection Connection, config QueueConfig, options ...QueueOption) (*QueueProcessor, error) {
	client, err := newQueueClient(connection, config, options...)
	if err != nil {
		return nil, err
	}
	return &QueueProcessor{client: client}, nil
}

// Start starts processing jobs and returns once the processor is ready.
func (q *QueueProcessor) Start(ctx context.Context) error {
	return q.client.Start(ctx)
}

// Stop gracefully stops processing jobs.
func (q *QueueProcessor) Stop(ctx context.Context) error {
	return q.client.Stop(ctx)
}

func newQueueClient(connection Connection, config QueueConfig, options ...QueueOption) (*river.Client[pgx.Tx], error) {
	if connection == nil {
		return nil, fmt.Errorf("storage: queue connection is required")
	}
	pool, err := poolFrom(connection)
	if err != nil {
		return nil, err
	}

	config = config.Clone()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	riverConfig := config.RiverConfig()
	for _, option := range options {
		if option != nil {
			option(&riverConfig)
		}
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &riverConfig)
	if err != nil {
		return nil, fmt.Errorf("storage: create queue client: %w", err)
	}
	return client, nil
}
