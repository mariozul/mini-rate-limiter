package vendors

import (
	"context"
	"time"

	"github.com/astronautsid/astro-boilerplate/pkg/pagination"
)

// ListFilter narrows a vendor list query. Empty fields are ignored.
type ListFilter struct {
	Status string
	Search string
}

// VendorListItem is the projection returned by Reader.List. It is a read-model
// DTO (NOT a domain entity) and is shaped for downstream interface handlers.
type VendorListItem struct {
	ID          int64
	VendorCode  string
	CompanyName string
	Status      string
	CreatedAt   time.Time
}

// Reader serves read-only vendor projections sourced from the read replica.
// Implementations return (items, totalCount, error) where totalCount reflects
// the filtered result set regardless of pagination.
type Reader interface {
	List(ctx context.Context, filter ListFilter, page pagination.Pagination) ([]VendorListItem, int, error)
}
