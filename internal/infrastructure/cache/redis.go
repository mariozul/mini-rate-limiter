// Package cache wires the optional Redis client and provides distributed-lock
// adapters (Redis-backed plus a NoOp fallback) that satisfy the
// internal/application/vendors.DistributedLock port.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisDB constructs a *redis.Client from the optional RedisConfig and
// verifies connectivity with a Ping. Returns an error (rather than panicking)
// so main.go can decide between fatal-exit and falling back to the NoOp lock.
//
// The connection identity comes from cfg.URL verbatim — host, credentials,
// database number, and TLS scheme (`rediss://...`) are all part of the URL
// injected by whoever provisions the connection. The service does not
// reassemble URLs from parts.
//
// Pool behaviour is service-owned: PoolSize, MinIdleConns, MaxIdleConns,
// ConnMaxIdleTime, ConnMaxLifetime, PoolTimeout, and the three I/O timeouts
// (Dial, Read, Write) are layered on top of whatever options.URL parsed out
// of the URL.
//
// Zero values for the pool / timeout knobs are preserved as zero so go-redis
// applies its own documented defaults (PoolSize = 10*NumCPU, ConnMaxIdleTime
// = 30m, etc.) — pass non-zero values explicitly when you want to override.
func NewRedisDB(cfg config.RedisConfig) (*redis.Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("redis: empty URL (set redis.url to enable)")
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse URL: %w", err)
	}

	// Layer service-owned pool + timeout settings on top of what the URL
	// already carried. We only overwrite when the cfg value is non-zero so
	// go-redis's documented defaults still apply when the operator hasn't
	// expressed an opinion.
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}
	if cfg.MaxIdleConns > 0 {
		opts.MaxIdleConns = cfg.MaxIdleConns
	}
	if cfg.ConnMaxIdleTimeSec > 0 {
		opts.ConnMaxIdleTime = time.Duration(cfg.ConnMaxIdleTimeSec) * time.Second
	}
	if cfg.ConnMaxLifetimeSec > 0 {
		opts.ConnMaxLifetime = time.Duration(cfg.ConnMaxLifetimeSec) * time.Second
	}
	if cfg.PoolTimeoutSec > 0 {
		opts.PoolTimeout = time.Duration(cfg.PoolTimeoutSec) * time.Second
	}
	if cfg.DialTimeoutSec > 0 {
		opts.DialTimeout = time.Duration(cfg.DialTimeoutSec) * time.Second
	}
	if cfg.ReadTimeoutSec > 0 {
		opts.ReadTimeout = time.Duration(cfg.ReadTimeoutSec) * time.Second
	}
	if cfg.WriteTimeoutSec > 0 {
		opts.WriteTimeout = time.Duration(cfg.WriteTimeoutSec) * time.Second
	}

	client := redis.NewClient(opts)

	// Ping under a bounded context so a wedged broker can't hold startup.
	// Reuse DialTimeout (or fall back to 5s) since that's the latency budget
	// the operator already agreed to for opening a new conn.
	pingTimeout := opts.DialTimeout
	if pingTimeout <= 0 {
		pingTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}
	return client, nil
}
