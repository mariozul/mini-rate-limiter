package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	appvendors "github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/redis/go-redis/v9"
)

// RedisLock implements internal/application/vendors.DistributedLock on top of
// the Redis SET NX EX primitive. The lock value is a fixed "1" placeholder —
// callers identify locks by key, not value. Pair with NewNoOp when Redis is
// disabled.
type RedisLock struct {
	client *redis.Client
}

// Compile-time check that *RedisLock satisfies the application port.
var _ appvendors.DistributedLock = (*RedisLock)(nil)

// NewLock wraps the given *redis.Client into a DistributedLock adapter.
func NewLock(c *redis.Client) *RedisLock {
	return &RedisLock{client: c}
}

// Acquire issues a `SET key "1" NX EX <ttl-seconds>` and returns (true, nil)
// on success or (false, nil) when the key already exists. Network / protocol
// errors surface as a non-nil error so callers can distinguish "lock busy"
// from "lock backend unavailable".
func (l *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	_, err := l.client.SetArgs(ctx, key, "1", redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis SET NX failed for key %s: %w", key, err)
	}
	return true, nil
}

// Release deletes the lock key. A missing key (TTL already expired) is not an
// error — by the time we release a self-healed lock the critical section is
// over and we just want to surface real backend failures.
func (l *RedisLock) Release(ctx context.Context, key string) error {
	if err := l.client.Del(ctx, key).Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("redis DEL failed for key %s: %w", key, err)
	}
	return nil
}
