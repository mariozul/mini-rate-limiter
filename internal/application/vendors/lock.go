package vendors

import (
	"context"
	"time"
)

// DistributedLock guards against concurrent execution of a critical section
// across processes. Acquire returns (true, nil) on success and (false, nil)
// when the lock is already held; non-nil error indicates a backend failure.
type DistributedLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}
