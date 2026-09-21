package entity

import (
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/domain/valueobject"
)

// Vendor is the example aggregate root used by the boilerplate. All fields are
// unexported; callers interact with the aggregate through accessor methods and
// behavior methods (constructor, state transitions, mutators).
type Vendor struct {
	id           int64
	vendorCode   string
	companyName  string
	netsuiteID   string
	contactEmail string
	contactPhone string
	status       valueobject.VendorStatus
	createdAt    time.Time
	updatedAt    time.Time
}

// NewVendor constructs a Vendor in the Pending status. It validates that the
// required identifying fields (vendorCode, companyName, netsuiteID) are
// non-empty and returns a *ValidationError naming the offending field on miss.
// A partially-constructed entity is never returned: callers always receive
// either a fully-valid *Vendor or a nil pointer plus an error.
func NewVendor(vendorCode, companyName, netsuiteID, contactEmail, contactPhone string, now time.Time) (*Vendor, error) {
	if vendorCode == "" {
		return nil, NewValidationError("vendorCode", "required")
	}
	if companyName == "" {
		return nil, NewValidationError("companyName", "required")
	}
	if netsuiteID == "" {
		return nil, NewValidationError("netsuiteID", "required")
	}

	return &Vendor{
		vendorCode:   vendorCode,
		companyName:  companyName,
		netsuiteID:   netsuiteID,
		contactEmail: contactEmail,
		contactPhone: contactPhone,
		status:       valueobject.VendorStatusPending,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// Reconstitute rebuilds a Vendor from persisted data without re-running
// invariants. Use this only from the persistence adapter when loading a row
// from storage — application code MUST go through NewVendor instead.
func Reconstitute(
	id int64,
	vendorCode, companyName, netsuiteID, contactEmail, contactPhone string,
	status valueobject.VendorStatus,
	createdAt, updatedAt time.Time,
) *Vendor {
	return &Vendor{
		id:           id,
		vendorCode:   vendorCode,
		companyName:  companyName,
		netsuiteID:   netsuiteID,
		contactEmail: contactEmail,
		contactPhone: contactPhone,
		status:       status,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// AssignID sets the aggregate identifier after a successful first persist.
// It refuses to overwrite an already-assigned identifier so the persistence
// adapter cannot accidentally renumber an existing aggregate.
func (v *Vendor) AssignID(id int64) error {
	if v.id != 0 {
		return ErrInvalidTransition
	}
	if id <= 0 {
		return NewValidationError("id", "must be positive")
	}
	v.id = id
	return nil
}

// ID returns the aggregate identifier. Zero until the vendor has been persisted.
func (v *Vendor) ID() int64 { return v.id }

// VendorCode returns the human-readable vendor code.
func (v *Vendor) VendorCode() string { return v.vendorCode }

// CompanyName returns the vendor's registered company name.
func (v *Vendor) CompanyName() string { return v.companyName }

// NetsuiteID returns the NetSuite system identifier for this vendor.
func (v *Vendor) NetsuiteID() string { return v.netsuiteID }

// ContactEmail returns the vendor contact email.
func (v *Vendor) ContactEmail() string { return v.contactEmail }

// ContactPhone returns the vendor contact phone number.
func (v *Vendor) ContactPhone() string { return v.contactPhone }

// Status returns the current lifecycle status as a value object.
func (v *Vendor) Status() valueobject.VendorStatus { return v.status }

// CreatedAt returns the creation timestamp set by the constructor.
func (v *Vendor) CreatedAt() time.Time { return v.createdAt }

// UpdatedAt returns the timestamp of the most recent mutation.
func (v *Vendor) UpdatedAt() time.Time { return v.updatedAt }

// Activate moves the vendor to the Active status. Allowed from Pending,
// Inactive, and Suspended. Returns ErrInvalidTransition otherwise without
// mutating the entity.
func (v *Vendor) Activate(now time.Time) error {
	switch v.status {
	case valueobject.VendorStatusPending,
		valueobject.VendorStatusInactive,
		valueobject.VendorStatusSuspended:
		v.status = valueobject.VendorStatusActive
		v.updatedAt = now
		return nil
	default:
		return ErrInvalidTransition
	}
}

// Deactivate moves the vendor to the Inactive status. Only allowed from
// Active; returns ErrInvalidTransition otherwise without mutating the entity.
func (v *Vendor) Deactivate(now time.Time) error {
	if v.status != valueobject.VendorStatusActive {
		return ErrInvalidTransition
	}
	v.status = valueobject.VendorStatusInactive
	v.updatedAt = now
	return nil
}

// Suspend moves the vendor to the Suspended status. Only allowed from Active;
// returns ErrInvalidTransition otherwise without mutating the entity.
func (v *Vendor) Suspend(now time.Time) error {
	if v.status != valueobject.VendorStatusActive {
		return ErrInvalidTransition
	}
	v.status = valueobject.VendorStatusSuspended
	v.updatedAt = now
	return nil
}

// UpdateContact replaces both contact fields. Both inputs must be non-empty;
// either is reported via a *ValidationError. On success the updatedAt
// timestamp is refreshed.
func (v *Vendor) UpdateContact(email, phone string, now time.Time) error {
	if email == "" {
		return NewValidationError("contactEmail", "required")
	}
	if phone == "" {
		return NewValidationError("contactPhone", "required")
	}
	v.contactEmail = email
	v.contactPhone = phone
	v.updatedAt = now
	return nil
}
