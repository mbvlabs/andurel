package storage

import (
	"testing"
	"time"

	"github.com/riverqueue/river"
)

func validQueueConfig() QueueConfig {
	return QueueConfig{
		CancelledJobRetentionPeriod: 24 * time.Hour,
		CompletedJobRetentionPeriod: 24 * time.Hour,
		DiscardedJobRetentionPeriod: 168 * time.Hour,
		FetchCooldown:               100 * time.Millisecond,
		PollInterval:                time.Second,
		JobCleanerTimeout:           30 * time.Second,
		JobStuckThreshold:           10 * time.Second,
		JobTimeout:                  time.Minute,
		MaxAttempts:                 25,
		MaxWorkers:                  100,
		ReindexerTimeout:            time.Minute,
		RescueStuckJobsAfter:        time.Hour,
	}
}

func TestQueueConfigValidate(t *testing.T) {
	config := validQueueConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	config.MaxWorkers = 0
	if err := config.Validate(); err == nil {
		t.Fatal("Validate accepted zero max workers")
	}

	config = validQueueConfig()
	config.JobTimeout = -time.Second
	if err := config.Validate(); err == nil {
		t.Fatal("Validate accepted a negative job timeout")
	}
}

func TestQueueConfigRiverConfig(t *testing.T) {
	config := validQueueConfig()
	config.ID = "web"
	riverConfig := config.RiverConfig()
	if riverConfig.ID != "web" {
		t.Fatalf("RiverConfig ID = %q, want web", riverConfig.ID)
	}
	if riverConfig.FetchPollInterval != config.PollInterval {
		t.Fatalf(
			"FetchPollInterval = %v, want %v",
			riverConfig.FetchPollInterval,
			config.PollInterval,
		)
	}
	if riverConfig.Queues != nil {
		t.Fatal("RiverConfig should not set processor queues")
	}
}

func TestQueueConfigCollectionsAreIsolated(t *testing.T) {
	config := DefaultQueueConfig()
	config.Queues = map[string]river.QueueConfig{"email": {MaxWorkers: 5}}
	cloned := config.Clone()
	config.Queues["email"] = river.QueueConfig{MaxWorkers: 10}
	config.ReindexerIndexNames[0] = "changed"
	if cloned.Queues["email"].MaxWorkers != 5 {
		t.Fatalf("Clone retained mutable queue definitions: %#v", cloned.Queues)
	}
	if cloned.ReindexerIndexNames[0] != "river_job_args_index" {
		t.Fatalf("Clone retained mutable indexes: %#v", cloned.ReindexerIndexNames)
	}

	first := cloned.RiverConfig()
	first.Queues["email"] = river.QueueConfig{MaxWorkers: 20}
	first.ReindexerIndexNames[0] = "changed again"
	second := cloned.RiverConfig()
	if second.Queues["email"].MaxWorkers != 5 {
		t.Fatalf("RiverConfig retained mutable queue definitions: %#v", second.Queues)
	}
	if second.ReindexerIndexNames[0] != "river_job_args_index" {
		t.Fatalf("RiverConfig retained mutable indexes: %#v", second.ReindexerIndexNames)
	}
	if DefaultQueueConfig().ReindexerIndexNames[0] != "river_job_args_index" {
		t.Fatal("mutating configuration changed defaults")
	}
}

func TestQueueConfigRejectsInvalidQueueDefinitions(t *testing.T) {
	for name, queues := range map[string]map[string]river.QueueConfig{
		"invalid name": {"EMAIL": {MaxWorkers: 5}},
		"no workers":   {"email": {MaxWorkers: 0}},
	} {
		t.Run(name, func(t *testing.T) {
			config := DefaultQueueConfig()
			config.Queues = queues
			if err := config.Validate(); err == nil {
				t.Fatalf("Validate accepted invalid queue definitions: %#v", queues)
			}
		})
	}
}
