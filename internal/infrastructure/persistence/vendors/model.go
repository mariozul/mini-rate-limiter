// Package vendor implements the persistence adapter for the Vendor aggregate.
// It owns the SQL schema mapping (Model) and the Repository that satisfies
// internal/application/vendor.Repository.
package vendors

import (
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/domain/entity"
	"github.com/mariozul/mini-rate-limiter/internal/domain/valueobject"
)

// Model is the row-level projection of the `vendors` Postgres table. It is the
// only place in the codebase that knows about the table's column layout —
// repository SQL uses these `db:` tags via sqlx Named*Context helpers.
//
// Status is stored as VARCHAR(32) in the DB and parsed into a domain value
// object by toEntity; ID is the SERIAL primary key (int64 in Go).
type Model struct {
	ID           int64     `db:"id"`
	VendorCode   string    `db:"vendor_code"`
	CompanyName  string    `db:"company_name"`
	NetsuiteID   string    `db:"netsuite_id"`
	ContactEmail string    `db:"contact_email"`
	ContactPhone string    `db:"contact_phone"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// fromEntity flattens a domain aggregate into a Model ready for insert/update.
// It is the inverse of toEntity. Status is unwrapped to its underlying string
// for storage.
func fromEntity(v *entity.Vendor) Model {
	return Model{
		ID:           v.ID(),
		VendorCode:   v.VendorCode(),
		CompanyName:  v.CompanyName(),
		NetsuiteID:   v.NetsuiteID(),
		ContactEmail: v.ContactEmail(),
		ContactPhone: v.ContactPhone(),
		Status:       v.Status().String(),
		CreatedAt:    v.CreatedAt(),
		UpdatedAt:    v.UpdatedAt(),
	}
}

// toEntity hydrates a domain aggregate from a row. It parses the status string
// into a value object and uses entity.Reconstitute (bypasses invariant
// validation — the row was validated when first inserted). An unknown status
// value in the DB surfaces as a non-nil error.
func toEntity(m Model) (*entity.Vendor, error) {
	status, err := valueobject.ParseVendorStatus(m.Status)
	if err != nil {
		return nil, err
	}
	return entity.Reconstitute(
		m.ID,
		m.VendorCode,
		m.CompanyName,
		m.NetsuiteID,
		m.ContactEmail,
		m.ContactPhone,
		status,
		m.CreatedAt,
		m.UpdatedAt,
	), nil
}
