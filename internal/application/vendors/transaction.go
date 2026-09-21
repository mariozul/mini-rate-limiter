package vendors

import "context"

// TransactionManager coordinates multi-repository writes inside a single
// database transaction. If the supplied closure returns a non-nil error, the
// transaction is rolled back; on nil it is committed.
type TransactionManager interface {
	Execute(ctx context.Context, fn func(tx Tx) error) error
}
