package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/domain/entity"
	"github.com/mariozul/mini-rate-limiter/internal/domain/valueobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testNow   = time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	testLater = testNow.Add(1 * time.Hour)
)

func newPendingVendor(t *testing.T) *entity.Vendor {
	t.Helper()
	v, err := entity.NewVendor("V-001", "Acme Corp", "NS-1", "ops@acme.test", "+62-21-0000", testNow)
	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, valueobject.VendorStatusPending, v.Status())
	return v
}

func TestNewVendor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		vendorCode   string
		companyName  string
		netsuiteID   string
		contactEmail string
		contactPhone string
		wantField    string
		wantErr      bool
	}{
		{
			name:         "happy path",
			vendorCode:   "V-001",
			companyName:  "Acme Corp",
			netsuiteID:   "NS-1",
			contactEmail: "ops@acme.test",
			contactPhone: "+62-21-0000",
		},
		{
			name:         "missing vendorCode",
			vendorCode:   "",
			companyName:  "Acme Corp",
			netsuiteID:   "NS-1",
			contactEmail: "ops@acme.test",
			contactPhone: "+62-21-0000",
			wantField:    "vendorCode",
			wantErr:      true,
		},
		{
			name:         "missing companyName",
			vendorCode:   "V-001",
			companyName:  "",
			netsuiteID:   "NS-1",
			contactEmail: "ops@acme.test",
			contactPhone: "+62-21-0000",
			wantField:    "companyName",
			wantErr:      true,
		},
		{
			name:         "missing netsuiteID",
			vendorCode:   "V-001",
			companyName:  "Acme Corp",
			netsuiteID:   "",
			contactEmail: "ops@acme.test",
			contactPhone: "+62-21-0000",
			wantField:    "netsuiteID",
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, err := entity.NewVendor(tc.vendorCode, tc.companyName, tc.netsuiteID, tc.contactEmail, tc.contactPhone, testNow)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, v, "constructor must not return a partially-constructed entity")

				var verr *entity.ValidationError
				require.True(t, errors.As(err, &verr), "expected *entity.ValidationError")
				assert.Equal(t, tc.wantField, verr.Field)
				assert.True(t, errors.Is(err, entity.ErrValidation))
				return
			}

			require.NoError(t, err)
			require.NotNil(t, v)
			assert.Equal(t, tc.vendorCode, v.VendorCode())
			assert.Equal(t, tc.companyName, v.CompanyName())
			assert.Equal(t, tc.netsuiteID, v.NetsuiteID())
			assert.Equal(t, tc.contactEmail, v.ContactEmail())
			assert.Equal(t, tc.contactPhone, v.ContactPhone())
			assert.Equal(t, valueobject.VendorStatusPending, v.Status())
			assert.Equal(t, testNow, v.CreatedAt())
			assert.Equal(t, testNow, v.UpdatedAt())
			assert.Equal(t, int64(0), v.ID())
		})
	}
}

func TestVendor_Activate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		startStatus  valueobject.VendorStatus
		setup        func(t *testing.T, v *entity.Vendor)
		wantErr      bool
		wantStatusOn valueobject.VendorStatus
	}{
		{
			name:        "from Pending",
			startStatus: valueobject.VendorStatusPending,
			setup:       func(t *testing.T, v *entity.Vendor) {},
		},
		{
			name:        "from Inactive",
			startStatus: valueobject.VendorStatusInactive,
			setup: func(t *testing.T, v *entity.Vendor) {
				require.NoError(t, v.Activate(testNow))
				require.NoError(t, v.Deactivate(testNow))
			},
		},
		{
			name:        "from Suspended",
			startStatus: valueobject.VendorStatusSuspended,
			setup: func(t *testing.T, v *entity.Vendor) {
				require.NoError(t, v.Activate(testNow))
				require.NoError(t, v.Suspend(testNow))
			},
		},
		{
			name:        "from Active is invalid",
			startStatus: valueobject.VendorStatusActive,
			setup: func(t *testing.T, v *entity.Vendor) {
				require.NoError(t, v.Activate(testNow))
			},
			wantErr:      true,
			wantStatusOn: valueobject.VendorStatusActive,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := newPendingVendor(t)
			tc.setup(t, v)
			require.Equal(t, tc.startStatus, v.Status())

			err := v.Activate(testLater)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, entity.ErrInvalidTransition))
				assert.Equal(t, tc.wantStatusOn, v.Status(), "status must not change on failed transition")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, valueobject.VendorStatusActive, v.Status())
			assert.Equal(t, testLater, v.UpdatedAt())
		})
	}
}

func TestVendor_Deactivate(t *testing.T) {
	t.Parallel()

	t.Run("from Active", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		require.NoError(t, v.Activate(testNow))

		require.NoError(t, v.Deactivate(testLater))
		assert.Equal(t, valueobject.VendorStatusInactive, v.Status())
		assert.Equal(t, testLater, v.UpdatedAt())
	})

	t.Run("from Pending is invalid", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		err := v.Deactivate(testLater)
		require.Error(t, err)
		assert.True(t, errors.Is(err, entity.ErrInvalidTransition))
		assert.Equal(t, valueobject.VendorStatusPending, v.Status())
		assert.Equal(t, testNow, v.UpdatedAt())
	})

	t.Run("from Suspended is invalid", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		require.NoError(t, v.Activate(testNow))
		require.NoError(t, v.Suspend(testNow))

		err := v.Deactivate(testLater)
		require.Error(t, err)
		assert.True(t, errors.Is(err, entity.ErrInvalidTransition))
		assert.Equal(t, valueobject.VendorStatusSuspended, v.Status())
	})
}

func TestVendor_Suspend(t *testing.T) {
	t.Parallel()

	t.Run("from Active", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		require.NoError(t, v.Activate(testNow))

		require.NoError(t, v.Suspend(testLater))
		assert.Equal(t, valueobject.VendorStatusSuspended, v.Status())
		assert.Equal(t, testLater, v.UpdatedAt())
	})

	t.Run("from Pending is invalid", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		err := v.Suspend(testLater)
		require.Error(t, err)
		assert.True(t, errors.Is(err, entity.ErrInvalidTransition))
		assert.Equal(t, valueobject.VendorStatusPending, v.Status())
	})

	t.Run("from Inactive is invalid", func(t *testing.T) {
		t.Parallel()

		v := newPendingVendor(t)
		require.NoError(t, v.Activate(testNow))
		require.NoError(t, v.Deactivate(testNow))

		err := v.Suspend(testLater)
		require.Error(t, err)
		assert.True(t, errors.Is(err, entity.ErrInvalidTransition))
		assert.Equal(t, valueobject.VendorStatusInactive, v.Status())
	})
}

func TestVendor_UpdateContact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		email     string
		phone     string
		wantField string
		wantErr   bool
	}{
		{name: "happy path", email: "new@acme.test", phone: "+62-21-1111"},
		{name: "missing email", email: "", phone: "+62-21-1111", wantField: "contactEmail", wantErr: true},
		{name: "missing phone", email: "new@acme.test", phone: "", wantField: "contactPhone", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := newPendingVendor(t)
			originalEmail := v.ContactEmail()
			originalPhone := v.ContactPhone()

			err := v.UpdateContact(tc.email, tc.phone, testLater)
			if tc.wantErr {
				require.Error(t, err)
				var verr *entity.ValidationError
				require.True(t, errors.As(err, &verr))
				assert.Equal(t, tc.wantField, verr.Field)
				assert.True(t, errors.Is(err, entity.ErrValidation))
				assert.Equal(t, originalEmail, v.ContactEmail(), "fields must not change on failed update")
				assert.Equal(t, originalPhone, v.ContactPhone())
				assert.Equal(t, testNow, v.UpdatedAt())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.email, v.ContactEmail())
			assert.Equal(t, tc.phone, v.ContactPhone())
			assert.Equal(t, testLater, v.UpdatedAt())
		})
	}
}
