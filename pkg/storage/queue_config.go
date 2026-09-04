package storage

import (
	"fmt"
	"time"

	"github.com/riverqueue/river"
)

// QueueConfig contains River client settings. Applications supply values;
// this type does not define defaults.
type QueueConfig struct {
	AdvisoryLockPrefix          int32
	CancelledJobRetentionPeriod time.Duration
	CompletedJobRetentionPeriod time.Duration
	DiscardedJobRetentionPeriod time.Duration
	FetchCooldown               time.Duration
	PollInterval                time.Duration
	ID                          string
	JobCleanerTimeout           time.Duration
	JobStuckThreshold           time.Duration
	JobTimeout                  time.Duration
	MaxAttempts                 int
	MaxWorkers                  int
	PollOnly                    bool
	ReindexerIndexNames         []string
	ReindexerTimeout            time.Duration
	RescueStuckJobsAfter        time.Duration
	Schema                      string
	SoftStopTimeout             time.Duration
	SkipJobKindValidation       bool
	SkipUnknownJobCheck         bool
}

// Validate verifies queue configuration.
func (config QueueConfig) Validate() error {
	for name, value := range map[string]int{
		"maximum attempts": config.MaxAttempts,
		"maximum workers":  config.MaxWorkers,
	} {
		if value < 1 {
			return fmt.Errorf("storage: %s must be at least 1", name)
		}
	}
	for name, value := range map[string]time.Duration{
		"cancelled job retention period": config.CancelledJobRetentionPeriod,
		"completed job retention period": config.CompletedJobRetentionPeriod,
		"discarded job retention period": config.DiscardedJobRetentionPeriod,
		"fetch cooldown":                 config.FetchCooldown,
		"poll interval":                  config.PollInterval,
		"job cleaner timeout":            config.JobCleanerTimeout,
		"job stuck threshold":            config.JobStuckThreshold,
		"job timeout":                    config.JobTimeout,
		"reindexer timeout":              config.ReindexerTimeout,
		"rescue stuck jobs after":        config.RescueStuckJobsAfter,
		"soft stop timeout":              config.SoftStopTimeout,
	} {
		if value < 0 {
			return fmt.Errorf("storage: %s cannot be negative", name)
		}
	}
	return nil
}

// RiverConfig returns the River client configuration represented by config.
func (config QueueConfig) RiverConfig() river.Config {
	return river.Config{
		AdvisoryLockPrefix:          config.AdvisoryLockPrefix,
		CancelledJobRetentionPeriod: config.CancelledJobRetentionPeriod,
		CompletedJobRetentionPeriod: config.CompletedJobRetentionPeriod,
		DiscardedJobRetentionPeriod: config.DiscardedJobRetentionPeriod,
		FetchCooldown:               config.FetchCooldown,
		FetchPollInterval:           config.PollInterval,
		ID:                          config.ID,
		JobCleanerTimeout:           config.JobCleanerTimeout,
		JobStuckThreshold:           config.JobStuckThreshold,
		JobTimeout:                  config.JobTimeout,
		MaxAttempts:                 config.MaxAttempts,
		PollOnly:                    config.PollOnly,
		ReindexerIndexNames:         config.ReindexerIndexNames,
		ReindexerTimeout:            config.ReindexerTimeout,
		RescueStuckJobsAfter:        config.RescueStuckJobsAfter,
		Schema:                      config.Schema,
		SoftStopTimeout:             config.SoftStopTimeout,
		SkipJobKindValidation:       config.SkipJobKindValidation,
		SkipUnknownJobCheck:         config.SkipUnknownJobCheck,
	}
}

// WithQueueConfig replaces River settings from an Andurel queue configuration.
// Options after this one override individual fields.
func WithQueueConfig(config QueueConfig) QueueOption {
	return func(target *river.Config) {
		*target = config.RiverConfig()
	}
}
