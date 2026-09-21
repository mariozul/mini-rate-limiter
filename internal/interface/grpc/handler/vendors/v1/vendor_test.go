package v1

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
	erppb "github.com/astronautsid/astro-proto/golang/pb/erp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes ---------------------------------------------------------------

type fakeCreator struct {
	gotCmd vendors.CreateVendorCommand
	resp   vendors.CreateVendorResponse
	err    error
}

func (f *fakeCreator) Execute(_ context.Context, cmd vendors.CreateVendorCommand) (vendors.CreateVendorResponse, error) {
	f.gotCmd = cmd
	return f.resp, f.err
}

type fakeGetter struct {
	gotQuery vendors.GetVendorQuery
	resp     vendors.GetVendorResponse
	err      error
}

func (f *fakeGetter) Execute(_ context.Context, q vendors.GetVendorQuery) (vendors.GetVendorResponse, error) {
	f.gotQuery = q
	return f.resp, f.err
}

// newHandler wires a Handler around the given fakes. It bypasses the public
// NewHandler constructor because that takes concrete
// *vendors.CreateVendorHandler / *vendors.GetVendorHandler types; the local
// vendorCreator/vendorGetter ports let the test inject fakes directly.
func newHandler(creator vendorCreator, getter vendorGetter) *Handler {
	return &Handler{
		create: creator,
		get:    getter,
		logger: nil,
	}
}

// --- CreateVendor --------------------------------------------------------

func TestHandler_CreateVendor_SuccessMapsResponse(t *testing.T) {
	t.Parallel()
	creator := &fakeCreator{
		resp: vendors.CreateVendorResponse{
			ID:         77,
			VendorCode: "V-001",
			Status:     "ACTIVE",
		},
	}
	h := newHandler(creator, &fakeGetter{})

	resp, err := h.CreateVendor(context.Background(), &erppb.CreateVendorRequest{
		VendorCode:        "V-001",
		CompanyName:       "Acme",
		NetsuiteId:        12345,
		Email:             "ops@acme.test",
		VendorPhoneNumber: "+62-000",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.GetData())
	require.Equal(t, int64(77), resp.GetData().GetId())
	require.Equal(t, "Acme", resp.GetData().GetCompanyName())
	require.True(t, resp.GetData().GetStatus(), "ACTIVE should map to status=true")

	require.Equal(t, "V-001", creator.gotCmd.VendorCode)
	require.Equal(t, "Acme", creator.gotCmd.CompanyName)
	require.Equal(t, "12345", creator.gotCmd.NetsuiteID) // int64 → string at the boundary
	require.Equal(t, "ops@acme.test", creator.gotCmd.ContactEmail)
	require.Equal(t, "+62-000", creator.gotCmd.ContactPhone)
}

func TestHandler_CreateVendor_PendingMapsStatusFalse(t *testing.T) {
	t.Parallel()
	creator := &fakeCreator{
		resp: vendors.CreateVendorResponse{ID: 1, VendorCode: "V-1", Status: "PENDING"},
	}
	h := newHandler(creator, &fakeGetter{})

	resp, err := h.CreateVendor(context.Background(), &erppb.CreateVendorRequest{VendorCode: "V-1"})
	require.NoError(t, err)
	require.False(t, resp.GetData().GetStatus(), "non-ACTIVE status must map to false")
}

func TestHandler_CreateVendor_ValidationErrorMapsToInvalidArgument(t *testing.T) {
	t.Parallel()
	creator := &fakeCreator{
		err: entity.NewValidationError("vendor_code", "is required"),
	}
	h := newHandler(creator, &fakeGetter{})

	resp, err := h.CreateVendor(context.Background(), &erppb.CreateVendorRequest{})

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetVendor -----------------------------------------------------------

func TestHandler_GetVendor_NotFoundMapsToNotFound(t *testing.T) {
	t.Parallel()
	getter := &fakeGetter{
		err: pkgerr.NewNotFound("Vendor", "42"),
	}
	h := newHandler(&fakeCreator{}, getter)

	resp, err := h.GetVendor(context.Background(), &erppb.GetVendorRequest{Id: 42})

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, vendors.GetVendorQuery{ID: 42}, getter.gotQuery)
}

func TestHandler_GetVendor_GenericErrorMapsToInternal(t *testing.T) {
	t.Parallel()
	getter := &fakeGetter{err: stderrors.New("db down")}
	h := newHandler(&fakeCreator{}, getter)

	resp, err := h.GetVendor(context.Background(), &erppb.GetVendorRequest{Id: 7})

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	// Default branch must not leak underlying error text.
	require.NotContains(t, status.Convert(err).Message(), "db down")
}

func TestHandler_GetVendor_SuccessMapsAllFields(t *testing.T) {
	t.Parallel()
	getter := &fakeGetter{
		resp: vendors.GetVendorResponse{
			ID:           7,
			VendorCode:   "V-007",
			CompanyName:  "Acme",
			NetsuiteID:   "98765",
			Status:       "ACTIVE",
			ContactEmail: "ops@acme.test",
			ContactPhone: "+62-001",
		},
	}
	h := newHandler(&fakeCreator{}, getter)

	resp, err := h.GetVendor(context.Background(), &erppb.GetVendorRequest{Id: 7})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.GetData())
	data := resp.GetData()
	require.Equal(t, int64(7), data.GetId())
	require.Equal(t, "V-007", data.GetVendorCode())
	require.Equal(t, "Acme", data.GetCompanyName())
	require.Equal(t, int64(98765), data.GetNetsuiteId())
	require.True(t, data.GetActive())
	require.Equal(t, "ops@acme.test", data.GetEmail())
	require.Equal(t, "+62-001", data.GetVendorPhoneNumber())
}

func TestHandler_GetVendor_NetsuiteIDParseFailureYieldsZero(t *testing.T) {
	t.Parallel()
	getter := &fakeGetter{
		resp: vendors.GetVendorResponse{
			ID:         8,
			VendorCode: "V-8",
			NetsuiteID: "not-a-number",
			Status:     "PENDING",
		},
	}
	h := newHandler(&fakeCreator{}, getter)

	resp, err := h.GetVendor(context.Background(), &erppb.GetVendorRequest{Id: 8})

	require.NoError(t, err)
	require.NotNil(t, resp.GetData())
	require.Equal(t, int64(0), resp.GetData().GetNetsuiteId())
	require.False(t, resp.GetData().GetActive())
}
