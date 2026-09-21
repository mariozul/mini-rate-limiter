package cache

import (
	"context"
	"time"

	appvendors "github.com/astronautsid/astro-boilerplate/internal/application/vendors"
)

// NoOpLock implements DistributedLock without contacting any external service.
// Acquire always reports success and Release does nothing — use it from
// main.go when REDIS_ADDRESS is empty so the binary still boots in local
// development without a Redis dependency.
type NoOpLock struct{}

// Compile-time check that NoOpLock satisfies the application port.
var _ appvendors.DistributedLock = (*NoOpLock)(nil)

// NewNoOp returns a DistributedLock that never blocks and never errors.
func NewNoOp() *NoOpLock {
	return &NoOpLock{}
}

// Acquire always returns (true, nil) — the caller behaves as if it owns the
// lock. This is safe for single-instance local dev; do NOT use in production
// with multiple replicas.
func (NoOpLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return true, nil
}

// Release always returns nil.
func (NoOpLock) Release(ctx context.Context, key string) error {
	return nil
}
