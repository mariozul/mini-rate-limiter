// Package v1 implements the vendor gRPC surface against the astro-proto
// erp.VendorService contract. The wire protocol lives in
// github.com/astronautsid/astro-proto; this package only adapts those proto
// types to the application command/query handlers.
//
// Only CreateVendor and GetVendor are implemented in the boilerplate — the
// other RPCs declared by erp.VendorService are inherited from
// erppb.UnimplementedVendorServiceServer and return codes.Unimplemented.
package v1

import (
	"context"
	"strconv"

	"github.com/mariozul/mini-rate-limiter/internal/application/vendors"
	"github.com/mariozul/mini-rate-limiter/internal/interface/grpc/grpcerr"
	logger "github.com/astronautsid/astro-golibs/logger"
	erppb "github.com/astronautsid/astro-proto/golang/pb/erp"
)

// vendorCreator is the local port the gRPC handler depends on for vendor
// creation. *vendors.CreateVendorHandler satisfies it; the interface lets us
// inject a fake in tests without touching the application package.
type vendorCreator interface {
	Execute(ctx context.Context, cmd vendors.CreateVendorCommand) (vendors.CreateVendorResponse, error)
}

// vendorGetter is the local port for the read side. *vendors.GetVendorHandler
// satisfies it.
type vendorGetter interface {
	Execute(ctx context.Context, q vendors.GetVendorQuery) (vendors.GetVendorResponse, error)
}

// Handler binds erp.VendorService's CreateVendor + GetVendor RPCs to the
// application-layer handlers.
type Handler struct {
	erppb.UnimplementedVendorServiceServer
	create vendorCreator
	get    vendorGetter
	logger logger.Logger
}

// NewHandler constructs a Handler. The concrete application handlers
// (*vendors.CreateVendorHandler, *vendors.GetVendorHandler) satisfy the local
// vendorCreator/vendorGetter ports.
func NewHandler(
	create *vendors.CreateVendorHandler,
	get *vendors.GetVendorHandler,
	log logger.Logger,
) *Handler {
	return &Handler{
		create: create,
		get:    get,
		logger: log,
	}
}

// CreateVendor dispatches the create command and maps the result onto the
// erp.CreateVendorResponse envelope.
//
// Field mapping notes (proto → application):
//   - vendor_code        → VendorCode
//   - company_name       → CompanyName
//   - netsuite_id (int64) → NetsuiteID (string)   [strconv.FormatInt]
//   - email              → ContactEmail
//   - vendor_phone_number → ContactPhone
//
// Field mapping notes (application → proto Vendor):
//   - resp.ID            → data.id
//   - req.CompanyName    → data.company_name (echoed; the application response
//     does not carry it back, but we just created it)
//   - resp.Status == "ACTIVE" → data.status (bool — proto's status is a
//     coarse active/inactive flag while the domain has a richer state machine)
func (h *Handler) CreateVendor(ctx context.Context, req *erppb.CreateVendorRequest) (*erppb.CreateVendorResponse, error) {
	cmd := vendors.CreateVendorCommand{
		VendorCode:   req.GetVendorCode(),
		CompanyName:  req.GetCompanyName(),
		NetsuiteID:   strconv.FormatInt(req.GetNetsuiteId(), 10),
		ContactEmail: req.GetEmail(),
		ContactPhone: req.GetVendorPhoneNumber(),
	}

	resp, err := h.create.Execute(ctx, cmd)
	if err != nil {
		return nil, grpcerr.ToStatusError(err)
	}

	return &erppb.CreateVendorResponse{
		Data: &erppb.Vendor{
			Id:          resp.ID,
			CompanyName: req.GetCompanyName(),
			Status:      resp.Status == "ACTIVE",
		},
	}, nil
}

// GetVendor runs the query and maps the result onto the erp.GetVendorResponse
// envelope.
//
// Field mapping notes (application → proto Data):
//   - resp.ID            → data.id
//   - resp.NetsuiteID    → data.netsuite_id (string → int64; 0 on parse error)
//   - resp.VendorCode    → data.vendor_code
//   - resp.Status == "ACTIVE" → data.active
//   - resp.CompanyName   → data.company_name
//   - resp.ContactEmail  → data.email
//   - resp.ContactPhone  → data.vendor_phone_number
func (h *Handler) GetVendor(ctx context.Context, req *erppb.GetVendorRequest) (*erppb.GetVendorResponse, error) {
	resp, err := h.get.Execute(ctx, vendors.GetVendorQuery{ID: req.GetId()})
	if err != nil {
		return nil, grpcerr.ToStatusError(err)
	}

	netsuiteID, _ := strconv.ParseInt(resp.NetsuiteID, 10, 64)
	return &erppb.GetVendorResponse{
		Data: &erppb.GetVendorResponse_Data{
			Id:                resp.ID,
			NetsuiteId:        netsuiteID,
			VendorCode:        resp.VendorCode,
			Active:            resp.Status == "ACTIVE",
			CompanyName:       resp.CompanyName,
			Email:             resp.ContactEmail,
			VendorPhoneNumber: resp.ContactPhone,
		},
	}, nil
}
