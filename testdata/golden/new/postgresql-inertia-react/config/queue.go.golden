package config

import (
	"errors"
	"fmt"

	"github.com/mbvlabs/andurel/pkg/env"
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
	e := env.New()
	cfg := storage.DefaultQueueConfig()
	cfg.MaxAttempts = e.Int("QUEUE_MAX_ATTEMPTS", cfg.MaxAttempts)
	cfg.Schema = e.String("QUEUE_SCHEMA", cfg.Schema)
	cfg.Queues = nil
	if err := errors.Join(e.Err(), cfg.Validate()); err != nil {
		return QueueInsert{}, fmt.Errorf("config: queue insert: %w", err)
	}

	return QueueInsert{Config: cfg}, nil
}

func NewQueueWorker(insert QueueInsert) (QueueWorker, error) {
	e := env.New()
	cfg := insert.Config.Clone()
	maxWorkers := e.Int("QUEUE_MAX_WORKERS", cfg.MaxWorkers)
	cfg.AdvisoryLockPrefix = e.Int32("QUEUE_ADVISORY_LOCK_PREFIX", cfg.AdvisoryLockPrefix)
	cfg.CancelledJobRetentionPeriod = e.Duration(
		"QUEUE_CANCELLED_JOB_RETENTION_PERIOD",
		cfg.CancelledJobRetentionPeriod,
	)
	cfg.CompletedJobRetentionPeriod = e.Duration(
		"QUEUE_COMPLETED_JOB_RETENTION_PERIOD",
		cfg.CompletedJobRetentionPeriod,
	)
	cfg.DiscardedJobRetentionPeriod = e.Duration(
		"QUEUE_DISCARDED_JOB_RETENTION_PERIOD",
		cfg.DiscardedJobRetentionPeriod,
	)
	cfg.FetchCooldown = e.Duration("QUEUE_FETCH_COOLDOWN", cfg.FetchCooldown)
	cfg.PollInterval = e.Duration("QUEUE_POLL_INTERVAL", cfg.PollInterval)
	cfg.ID = e.String("QUEUE_ID", cfg.ID)
	cfg.JobCleanerTimeout = e.Duration("QUEUE_JOB_CLEANER_TIMEOUT", cfg.JobCleanerTimeout)
	cfg.JobStuckThreshold = e.Duration("QUEUE_JOB_STUCK_THRESHOLD", cfg.JobStuckThreshold)
	cfg.JobTimeout = e.Duration("QUEUE_JOB_TIMEOUT", cfg.JobTimeout)
	cfg.PollOnly = e.Bool("QUEUE_POLL_ONLY", cfg.PollOnly)
	cfg.ReindexerTimeout = e.Duration("QUEUE_REINDEXER_TIMEOUT", cfg.ReindexerTimeout)
	cfg.RescueStuckJobsAfter = e.Duration(
		"QUEUE_RESCUE_STUCK_JOBS_AFTER",
		cfg.RescueStuckJobsAfter,
	)
	cfg.SoftStopTimeout = e.Duration("QUEUE_SOFT_STOP_TIMEOUT", cfg.SoftStopTimeout)
	cfg.SkipJobKindValidation = e.Bool(
		"QUEUE_SKIP_JOB_KIND_VALIDATION",
		cfg.SkipJobKindValidation,
	)
	cfg.SkipUnknownJobCheck = e.Bool(
		"QUEUE_SKIP_UNKNOWN_JOB_CHECK",
		cfg.SkipUnknownJobCheck,
	)
	cfg.MaxWorkers = maxWorkers
	cfg.Queues = map[string]river.QueueConfig{
		river.QueueDefault: {MaxWorkers: maxWorkers},
	}
	if err := errors.Join(e.Err(), cfg.Validate()); err != nil {
		return QueueWorker{}, fmt.Errorf("config: queue worker: %w", err)
	}

	return QueueWorker{Config: cfg}, nil
}
