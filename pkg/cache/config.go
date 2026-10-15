package cache

import "time"

// Config selects the driver and its options. Use Driver "memory" for a single
// process and "redis" when replicas must share cached values and rate-limit
// counters.
type Config struct {
	// Driver is "memory" or "redis". Anything else is an error.
	Driver string
	// Namespace prefixes every key. Defaults to "andurel" when empty so
	// Flush never touches keys outside the application.
	Namespace string
	// DefaultTTL applies when callers pass a ttl <= 0 to Remember-style
	// helpers that resolve defaults. Zero means no expiry.
	DefaultTTL time.Duration
	// MemoryMaxSize bounds the memory driver's entries. Defaults to 1000.
	MemoryMaxSize int
	// Redis configures the redis driver. Ignored by the memory driver.
	Redis RedisConfig
}

// RedisConfig is the redis driver connection surface.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// New builds the Store described by cfg.
func New(cfg Config) (Store, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "andurel"
	}
	if cfg.MemoryMaxSize <= 0 {
		cfg.MemoryMaxSize = 1000
	}

	switch cfg.Driver {
	case "", "memory":
		return newMemoryStore(cfg.Namespace, cfg.MemoryMaxSize), nil
	case "redis":
		return newRedisStore(cfg.Namespace, cfg.Redis)
	default:
		return nil, &DriverError{Driver: cfg.Driver}
	}
}

// DriverError reports an unknown cache driver name.
type DriverError struct {
	Driver string
}

func (e *DriverError) Error() string {
	return "cache: unknown driver " + e.Driver
}
