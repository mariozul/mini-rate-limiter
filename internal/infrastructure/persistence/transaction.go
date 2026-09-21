package persistence

import (
	"context"

	"github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/astronautsid/astro-golibs/safesql"
	"github.com/jmoiron/sqlx"
)

// Manager is the persistence-adapter implementation of vendors.TransactionManager.
// It wraps a safesql.MasterDB and threads *sqlx.Tx through fn behind the
// opaque vendors.Tx interface so application code stays free of database/sql
// imports.
type Manager struct {
	db safesql.MasterDB
}

// Compile-time assertion that *Manager satisfies the port.
var _ vendors.TransactionManager = (*Manager)(nil)

// NewTransactionManager constructs a TransactionManager backed by the given
// master connection. Reads do not use this manager; they go directly to the
// read connection inside each repository.
func NewTransactionManager(db safesql.MasterDB) vendors.TransactionManager {
	return &Manager{db: db}
}

// Execute delegates to safesql.MasterDB.ExecuteWithTx, which handles begin /
// commit / rollback semantics (including rollback on panic) internally.
// fn receives the opaque vendors.Tx so application code stays free of
// database/sql + sqlx imports.
func (m *Manager) Execute(ctx context.Context, fn func(tx vendors.Tx) error) error {
	return m.db.ExecuteWithTx(ctx, func(tx *sqlx.Tx) error {
		return fn(tx)
	})
}

// SqlxTx casts the opaque vendors.Tx handle to *sqlx.Tx. It returns nil when
// tx is nil so unit-test code paths that pass a nil Tx (e.g., calling Save
// outside a transaction in a test) do not blow up. Production callers always
// pass the value supplied by Manager.Execute, which is a real *sqlx.Tx.
func SqlxTx(tx vendors.Tx) *sqlx.Tx {
	if tx == nil {
		return nil
	}
	return tx.(*sqlx.Tx)
}
