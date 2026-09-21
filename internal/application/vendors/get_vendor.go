package vendors

import (
	"context"
	"time"

	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
)

// GetVendorQuery is the input DTO for the GetVendor use case.
type GetVendorQuery struct {
	ID int64
}

// GetVendorResponse is the output DTO returned by the GetVendor use case.
type GetVendorResponse struct {
	ID           int64
	VendorCode   string
	CompanyName  string
	NetsuiteID   string
	Status       string
	ContactEmail string
	ContactPhone string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetVendorParams groups the dependencies for GetVendorHandler.
type GetVendorParams struct {
	Repo Repository
}

// GetVendorHandler reads a vendor by id.
type GetVendorHandler struct {
	repo Repository
}

// NewGetVendorHandler constructs a GetVendorHandler from the given params.
func NewGetVendorHandler(p GetVendorParams) *GetVendorHandler {
	return &GetVendorHandler{repo: p.Repo}
}

// Execute runs the GetVendor query. ErrNotFound from the repository is
// propagated to the caller.
func (h *GetVendorHandler) Execute(ctx context.Context, query GetVendorQuery) (GetVendorResponse, error) {
	if query.ID <= 0 {
		return GetVendorResponse{}, pkgerr.NewInvalidArgument("id required")
	}

	vendor, err := h.repo.FindByID(ctx, query.ID)
	if err != nil {
		return GetVendorResponse{}, err
	}

	return GetVendorResponse{
		ID:           vendor.ID(),
		VendorCode:   vendor.VendorCode(),
		CompanyName:  vendor.CompanyName(),
		NetsuiteID:   vendor.NetsuiteID(),
		Status:       vendor.Status().String(),
		ContactEmail: vendor.ContactEmail(),
		ContactPhone: vendor.ContactPhone(),
		CreatedAt:    vendor.CreatedAt(),
		UpdatedAt:    vendor.UpdatedAt(),
	}, nil
}
