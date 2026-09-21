// Package valueobject contains validated value objects used by domain entities.
//
// Value objects parse and validate primitive input at the domain boundary so
// untyped strings and ints do not leak into entity fields.
package valueobject

import "errors"

// VendorStatus represents the lifecycle status of a vendor aggregate.
type VendorStatus string

// Vendor status constants.
const (
	VendorStatusPending   VendorStatus = "PENDING"
	VendorStatusActive    VendorStatus = "ACTIVE"
	VendorStatusInactive  VendorStatus = "INACTIVE"
	VendorStatusSuspended VendorStatus = "SUSPENDED"
)

// String returns the underlying string representation of the status.
func (s VendorStatus) String() string {
	return string(s)
}

// IsValid reports whether s is one of the recognised VendorStatus constants.
func (s VendorStatus) IsValid() bool {
	switch s {
	case VendorStatusPending, VendorStatusActive, VendorStatusInactive, VendorStatusSuspended:
		return true
	default:
		return false
	}
}

// ParseVendorStatus converts a primitive string into a VendorStatus.
// It returns the zero value and a non-nil error for unknown input.
func ParseVendorStatus(s string) (VendorStatus, error) {
	vs := VendorStatus(s)
	if !vs.IsValid() {
		return VendorStatus(""), errors.New("invalid vendor status: " + s)
	}
	return vs, nil
}
