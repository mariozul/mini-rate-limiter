// Package vendor declares the application-layer ports and use-case handlers
// for the Vendor aggregate. Ports are co-located with the use cases that
// consume them; infrastructure adapters import this package and implement
// the interfaces declared here.
package vendors

// Tx is the opaque transaction handle threaded through repository writes by
// TransactionManager. The persistence adapter type-asserts to its concrete
// implementation (e.g., *sqlx.Tx); application code never inspects the interior.
type Tx interface{}
