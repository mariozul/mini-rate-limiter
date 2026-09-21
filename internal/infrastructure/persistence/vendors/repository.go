package vendors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	appvendors "github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
	"github.com/astronautsid/astro-boilerplate/internal/infrastructure/persistence"
	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
	"github.com/astronautsid/astro-golibs/safesql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository is the Postgres-backed implementation of
// internal/application/vendors.Repository. It owns the SQL for the `vendors`
// table and routes writes to writeDB / reads to readDB per the read/write
// split convention.
//
// writeDB and readDB are typed as safesql.MasterDB / safesql.ReplicaDB so
// the type system enforces "no writes through the replica" at compile time.
type Repository struct {
	writeDB safesql.MasterDB
	readDB  safesql.ReplicaDB
}

// Compile-time check that *Repository satisfies the application port.
var _ appvendors.Repository = (*Repository)(nil)

// NewRepository constructs a Repository wired to the master (write) and slave
// (read) connections. Callers in main.go should pass the values returned by
// persistence.NewMasterDB(...) and persistence.NewReplicaDB(...) respectively.
func NewRepository(write safesql.MasterDB, read safesql.ReplicaDB) *Repository {
	return &Repository{writeDB: write, readDB: read}
}

// columns lists the projection used by every SELECT in this repository. The
// order MUST match the field order of Model so sqlx StructScan works.
const columns = `id, vendor_code, company_name, netsuite_id, contact_email,
	contact_phone, status, created_at, updated_at`

// Save inserts a new vendor row and returns the newly assigned primary key.
// When tx is non-nil the insert runs inside that transaction (used by command
// use cases that need atomicity with the audit-log write); when tx is nil the
// repository talks directly to writeDB. A unique-constraint violation on
// vendor_code is mapped to pkgerr.NewInvalidArgument so application code can
// translate to a clean transport error.
//
// Implementation note: safesql.MasterDB does not expose NamedQuery family
// methods, so the no-tx branch converts the named query to positional via
// sqlx.Named + Rebind. The tx branch uses *sqlx.Tx directly (which retains
// the named-query family) since we still own that handle.
func (r *Repository) Save(ctx context.Context, tx appvendors.Tx, v *entity.Vendor) (int64, error) {
	m := fromEntity(v)
	const q = `INSERT INTO vendors
		(vendor_code, company_name, netsuite_id, contact_email, contact_phone,
		 status, created_at, updated_at)
		VALUES (:vendor_code, :company_name, :netsuite_id, :contact_email, :contact_phone,
		        :status, :created_at, :updated_at)
		RETURNING id`

	if sqlxTx := persistence.SqlxTx(tx); sqlxTx != nil {
		stmt, err := sqlxTx.PrepareNamedContext(ctx, q)
		if err != nil {
			return 0, mapWriteError(err)
		}
		defer func() { _ = stmt.Close() }()

		var id int64
		if err := stmt.QueryRowxContext(ctx, m).Scan(&id); err != nil {
			return 0, mapWriteError(err)
		}
		return id, nil
	}

	positional, args, err := sqlx.Named(q, m)
	if err != nil {
		return 0, mapWriteError(err)
	}
	positional = r.writeDB.Rebind(positional)
	var id int64
	if err := r.writeDB.QueryRowxContext(ctx, positional, args...).Scan(&id); err != nil {
		return 0, mapWriteError(err)
	}
	return id, nil
}

// Update writes back the mutable fields of an existing vendor. ID is required;
// callers MUST go through the aggregate's behavior methods first so updatedAt
// has been refreshed. Reuses tx vs writeDB selection from Save.
func (r *Repository) Update(ctx context.Context, tx appvendors.Tx, v *entity.Vendor) error {
	m := fromEntity(v)
	const q = `UPDATE vendors
		SET company_name = :company_name,
		    netsuite_id = :netsuite_id,
		    contact_email = :contact_email,
		    contact_phone = :contact_phone,
		    status = :status,
		    updated_at = :updated_at
		WHERE id = :id`

	if sqlxTx := persistence.SqlxTx(tx); sqlxTx != nil {
		if _, err := sqlxTx.NamedExecContext(ctx, q, m); err != nil {
			return mapWriteError(err)
		}
		return nil
	}

	positional, args, err := sqlx.Named(q, m)
	if err != nil {
		return mapWriteError(err)
	}
	positional = r.writeDB.Rebind(positional)
	if _, err := r.writeDB.ExecContext(ctx, positional, args...); err != nil {
		return mapWriteError(err)
	}
	return nil
}

// FindByID loads a vendor by primary key from the read replica. Returns a
// pkgerr.NotFoundError wrapping ErrNotFound when no row matches so application
// code can branch on errors.Is(err, pkgerr.ErrNotFound).
func (r *Repository) FindByID(ctx context.Context, id int64) (*entity.Vendor, error) {
	q := `SELECT ` + columns + ` FROM vendors WHERE id = $1`
	var m Model
	if err := r.readDB.GetContext(ctx, &m, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerr.NewNotFound("vendor", fmt.Sprintf("%d", id))
		}
		return nil, err
	}
	return toEntity(m)
}

// FindByCode loads a vendor by its human-readable code. Same not-found
// semantics as FindByID.
func (r *Repository) FindByCode(ctx context.Context, code string) (*entity.Vendor, error) {
	q := `SELECT ` + columns + ` FROM vendors WHERE vendor_code = $1`
	var m Model
	if err := r.readDB.GetContext(ctx, &m, q, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerr.NewNotFound("vendor", code)
		}
		return nil, err
	}
	return toEntity(m)
}

// mapWriteError translates known database errors to typed pkgerr wrappers.
// Unique-violation (Postgres SQLSTATE 23505) maps to NewInvalidArgument so a
// duplicate vendor_code surfaces as a 4xx-equivalent at the transport layer
// rather than a 5xx server error.
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if pqErr.Code == "23505" {
			return pkgerr.NewInvalidArgument("vendor exists")
		}
	}
	// Some drivers / mocks surface uniqueness as a plain message; fall back to
	// a textual sniff so go-sqlmock-driven tests can exercise this branch.
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return pkgerr.NewInvalidArgument("vendor exists")
	}
	return err
}
