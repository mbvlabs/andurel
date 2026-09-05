package storage

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"time"

	"github.com/riverqueue/river"
)

// QueueConfig contains River client settings and application queue definitions.
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
	Queues                      map[string]river.QueueConfig
	PollOnly                    bool
	ReindexerIndexNames         []string
	ReindexerTimeout            time.Duration
	RescueStuckJobsAfter        time.Duration
	Schema                      string
	SoftStopTimeout             time.Duration
	SkipJobKindValidation       bool
	SkipUnknownJobCheck         bool
}

// DefaultQueueConfig returns operational defaults and no processor queues.
// Applications choose their queue names and concurrency in Queues.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		CancelledJobRetentionPeriod: 24 * time.Hour,
		CompletedJobRetentionPeriod: 24 * time.Hour,
		DiscardedJobRetentionPeriod: 7 * 24 * time.Hour,
		FetchCooldown:               river.FetchCooldownDefault, PollInterval: river.FetchPollIntervalDefault,
		JobCleanerTimeout: 30 * time.Second, JobStuckThreshold: river.JobStuckThresholdDefault,
		JobTimeout: river.JobTimeoutDefault, MaxAttempts: river.MaxAttemptsDefault, MaxWorkers: 100,
		ReindexerTimeout: time.Minute, RescueStuckJobsAfter: time.Hour,
		ReindexerIndexNames: []string{
			"river_job_args_index", "river_job_kind", "river_job_metadata_index", "river_job_pkey",
			"river_job_prioritized_fetching_index", "river_job_state_and_finalized_at_index", "river_job_unique_idx",
		},
	}
}

// Clone returns a configuration with independently mutable collections.
func (config QueueConfig) Clone() QueueConfig {
	config.Queues = maps.Clone(config.Queues)
	config.ReindexerIndexNames = slices.Clone(config.ReindexerIndexNames)
	return config
}

// Validate verifies queue configuration.
func (config QueueConfig) Validate() error {
	if config.MaxAttempts < 0 {
		return fmt.Errorf("storage: maximum attempts cannot be negative")
	}
	if config.MaxWorkers < 1 || config.MaxWorkers > river.QueueNumWorkersMax {
		return fmt.Errorf("storage: maximum workers must be between 1 and %d", river.QueueNumWorkersMax)
	}
	for name, value := range map[string]time.Duration{
		"cancelled job retention period": config.CancelledJobRetentionPeriod,
		"completed job retention period": config.CompletedJobRetentionPeriod,
		"discarded job retention period": config.DiscardedJobRetentionPeriod,
		"job timeout":                    config.JobTimeout,
		"reindexer timeout":              config.ReindexerTimeout,
	} {
		if value < -1 {
			return fmt.Errorf("storage: %s cannot be negative except -1 (infinite)", name)
		}
	}
	for name, value := range map[string]time.Duration{
		"job cleaner timeout":     config.JobCleanerTimeout,
		"job stuck threshold":     config.JobStuckThreshold,
		"rescue stuck jobs after": config.RescueStuckJobsAfter,
		"soft stop timeout":       config.SoftStopTimeout,
	} {
		if value < 0 {
			return fmt.Errorf("storage: %s cannot be negative", name)
		}
	}
	riverConfig := config.RiverConfig()
	resolved := riverConfig.WithDefaults()
	if resolved.FetchCooldown < river.FetchCooldownMin || resolved.FetchPollInterval < river.FetchPollIntervalMin {
		return fmt.Errorf("storage: fetch cooldown and poll interval must meet River minimums")
	}
	if resolved.FetchPollInterval < resolved.FetchCooldown {
		return fmt.Errorf("storage: poll interval cannot be shorter than fetch cooldown")
	}
	if resolved.RescueStuckJobsAfter < resolved.JobTimeout {
		return fmt.Errorf("storage: rescue stuck jobs after cannot be shorter than job timeout")
	}
	if len(config.ID) > 100 {
		return fmt.Errorf("storage: queue ID cannot be longer than 100 characters")
	}
	if config.Schema != "" && (!queueSchemaPattern.MatchString(config.Schema) || len(config.Schema) > 63-1-len("river_leadership")) {
		return fmt.Errorf("storage: invalid queue schema")
	}
	for name, queue := range config.Queues {
		if len(name) > 64 || !queueNamePattern.MatchString(name) {
			return fmt.Errorf("storage: invalid queue name %q", name)
		}
		if queue.MaxWorkers < 1 || queue.MaxWorkers > river.QueueNumWorkersMax {
			return fmt.Errorf("storage: invalid maximum workers for queue %q", name)
		}
		if queue.FetchCooldown < 0 || queue.FetchPollInterval < 0 || cmp.Or(queue.FetchPollInterval, resolved.FetchPollInterval) < cmp.Or(queue.FetchCooldown, resolved.FetchCooldown) {
			return fmt.Errorf("storage: invalid fetch intervals for queue %q", name)
		}
	}
	return nil
}

var (
	queueNamePattern   = regexp.MustCompile(`^(?:[a-z0-9])+(?:[_|\-]?[a-z0-9]+)*$`)
	queueSchemaPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

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
		ReindexerIndexNames:         slices.Clone(config.ReindexerIndexNames),
		Queues:                      maps.Clone(config.Queues),
		ReindexerTimeout:            config.ReindexerTimeout,
		RescueStuckJobsAfter:        config.RescueStuckJobsAfter,
		Schema:                      config.Schema,
		SoftStopTimeout:             config.SoftStopTimeout,
		SkipJobKindValidation:       config.SkipJobKindValidation,
		SkipUnknownJobCheck:         config.SkipUnknownJobCheck,
	}
}
