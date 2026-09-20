package config

import (
	"errors"
	"fmt"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/riverqueue/river"
)

type QueueInsert struct {
	Config storage.QueueConfig
}

type QueueWorker struct {
	Config storage.QueueConfig
}

func NewQueueInsert() (QueueInsert, error) {
	env := newEnvironment()
	cfg := storage.DefaultQueueConfig()
	cfg.MaxAttempts = env.Int("QUEUE_MAX_ATTEMPTS", cfg.MaxAttempts)
	cfg.Schema = env.String("QUEUE_SCHEMA", cfg.Schema)
	cfg.Queues = nil
	if err := errors.Join(env.Err(), cfg.Validate()); err != nil {
		return QueueInsert{}, fmt.Errorf("config: queue insert: %w", err)
	}

	return QueueInsert{Config: cfg}, nil
}

func NewQueueWorker(insert QueueInsert) (QueueWorker, error) {
	env := newEnvironment()
	cfg := insert.Config.Clone()
	maxWorkers := env.Int("QUEUE_MAX_WORKERS", cfg.MaxWorkers)
	cfg.AdvisoryLockPrefix = env.Int32("QUEUE_ADVISORY_LOCK_PREFIX", cfg.AdvisoryLockPrefix)
	cfg.CancelledJobRetentionPeriod = env.Duration(
		"QUEUE_CANCELLED_JOB_RETENTION_PERIOD",
		cfg.CancelledJobRetentionPeriod,
	)
	cfg.CompletedJobRetentionPeriod = env.Duration(
		"QUEUE_COMPLETED_JOB_RETENTION_PERIOD",
		cfg.CompletedJobRetentionPeriod,
	)
	cfg.DiscardedJobRetentionPeriod = env.Duration(
		"QUEUE_DISCARDED_JOB_RETENTION_PERIOD",
		cfg.DiscardedJobRetentionPeriod,
	)
	cfg.FetchCooldown = env.Duration("QUEUE_FETCH_COOLDOWN", cfg.FetchCooldown)
	cfg.PollInterval = env.Duration("QUEUE_POLL_INTERVAL", cfg.PollInterval)
	cfg.ID = env.String("QUEUE_ID", cfg.ID)
	cfg.JobCleanerTimeout = env.Duration("QUEUE_JOB_CLEANER_TIMEOUT", cfg.JobCleanerTimeout)
	cfg.JobStuckThreshold = env.Duration("QUEUE_JOB_STUCK_THRESHOLD", cfg.JobStuckThreshold)
	cfg.JobTimeout = env.Duration("QUEUE_JOB_TIMEOUT", cfg.JobTimeout)
	cfg.PollOnly = env.Bool("QUEUE_POLL_ONLY", cfg.PollOnly)
	cfg.ReindexerTimeout = env.Duration("QUEUE_REINDEXER_TIMEOUT", cfg.ReindexerTimeout)
	cfg.RescueStuckJobsAfter = env.Duration(
		"QUEUE_RESCUE_STUCK_JOBS_AFTER",
		cfg.RescueStuckJobsAfter,
	)
	cfg.SoftStopTimeout = env.Duration("QUEUE_SOFT_STOP_TIMEOUT", cfg.SoftStopTimeout)
	cfg.SkipJobKindValidation = env.Bool(
		"QUEUE_SKIP_JOB_KIND_VALIDATION",
		cfg.SkipJobKindValidation,
	)
	cfg.SkipUnknownJobCheck = env.Bool(
		"QUEUE_SKIP_UNKNOWN_JOB_CHECK",
		cfg.SkipUnknownJobCheck,
	)
	cfg.MaxWorkers = maxWorkers
	cfg.Queues = map[string]river.QueueConfig{
		river.QueueDefault: {MaxWorkers: maxWorkers},
	}
	if err := errors.Join(env.Err(), cfg.Validate()); err != nil {
		return QueueWorker{}, fmt.Errorf("config: queue worker: %w", err)
	}

	return QueueWorker{Config: cfg}, nil
}
