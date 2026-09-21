package vendors_test

import (
	"context"
	"testing"
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/application/vendors"
	"github.com/mariozul/mini-rate-limiter/internal/domain/entity"
	"github.com/mariozul/mini-rate-limiter/internal/domain/valueobject"
	pkgerr "github.com/mariozul/mini-rate-limiter/pkg/errors"
	"github.com/stretchr/testify/require"
)

func reconstituted(id int64) *entity.Vendor {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return entity.Reconstitute(
		id,
		"V-001",
		"Acme",
		"NS-1",
		"ops@acme.test",
		"+62811",
		valueobject.VendorStatusActive,
		now,
		now,
	)
}

func TestGetVendor_HappyPath(t *testing.T) {
	repo := &fakeRepo{findByIDRes: reconstituted(7)}

	h := vendors.NewGetVendorHandler(vendors.GetVendorParams{Repo: repo})

	resp, err := h.Execute(context.Background(), vendors.GetVendorQuery{ID: 7})
	require.NoError(t, err)
	require.Equal(t, int64(7), resp.ID)
	require.Equal(t, "V-001", resp.VendorCode)
	require.Equal(t, "Acme", resp.CompanyName)
	require.Equal(t, "NS-1", resp.NetsuiteID)
	require.Equal(t, string(valueobject.VendorStatusActive), resp.Status)
	require.Equal(t, "ops@acme.test", resp.ContactEmail)
	require.Equal(t, "+62811", resp.ContactPhone)
}

func TestGetVendor_InvalidQuery(t *testing.T) {
	h := vendors.NewGetVendorHandler(vendors.GetVendorParams{Repo: &fakeRepo{}})

	_, err := h.Execute(context.Background(), vendors.GetVendorQuery{ID: 0})
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrInvalidArgument)
}

func TestGetVendor_RepoNotFound(t *testing.T) {
	repo := &fakeRepo{findByIDErr: pkgerr.NewNotFound("Vendor", "7")}

	h := vendors.NewGetVendorHandler(vendors.GetVendorParams{Repo: repo})

	_, err := h.Execute(context.Background(), vendors.GetVendorQuery{ID: 7})
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrNotFound)
}
