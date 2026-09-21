package valueobject_test

import (
	"testing"

	"github.com/mariozul/mini-rate-limiter/internal/domain/valueobject"
	"github.com/stretchr/testify/require"
)

func TestParseVendorStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    valueobject.VendorStatus
		wantErr bool
	}{
		{name: "pending", input: "PENDING", want: valueobject.VendorStatusPending},
		{name: "active", input: "ACTIVE", want: valueobject.VendorStatusActive},
		{name: "inactive", input: "INACTIVE", want: valueobject.VendorStatusInactive},
		{name: "suspended", input: "SUSPENDED", want: valueobject.VendorStatusSuspended},
		{name: "unknown literal", input: "BOGUS", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "wrong case", input: "pending", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := valueobject.ParseVendorStatus(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, valueobject.VendorStatus(""), got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVendorStatus_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input valueobject.VendorStatus
		want  bool
	}{
		{name: "pending is valid", input: valueobject.VendorStatusPending, want: true},
		{name: "active is valid", input: valueobject.VendorStatusActive, want: true},
		{name: "inactive is valid", input: valueobject.VendorStatusInactive, want: true},
		{name: "suspended is valid", input: valueobject.VendorStatusSuspended, want: true},
		{name: "zero value is invalid", input: valueobject.VendorStatus(""), want: false},
		{name: "unknown literal is invalid", input: valueobject.VendorStatus("BOGUS"), want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.input.IsValid())
		})
	}
}

func TestVendorStatus_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "PENDING", valueobject.VendorStatusPending.String())
	require.Equal(t, "ACTIVE", valueobject.VendorStatusActive.String())
	require.Equal(t, "INACTIVE", valueobject.VendorStatusInactive.String())
	require.Equal(t, "SUSPENDED", valueobject.VendorStatusSuspended.String())
}
