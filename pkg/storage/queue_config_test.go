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

func TestWithQueueConfig(t *testing.T) {
	config := validQueueConfig()
	config.MaxAttempts = 3
	config.ID = "insert"
	var riverConfig river.Config
	WithQueueConfig(config)(&riverConfig)
	if riverConfig.MaxAttempts != 3 || riverConfig.ID != "insert" {
		t.Fatalf("WithQueueConfig did not apply settings: %#v", riverConfig)
	}
}
