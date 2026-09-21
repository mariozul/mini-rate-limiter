package grpcerr_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/mariozul/mini-rate-limiter/internal/domain/entity"
	"github.com/mariozul/mini-rate-limiter/internal/interface/grpc/grpcerr"
	pkgerr "github.com/mariozul/mini-rate-limiter/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusError(t *testing.T) {
	t.Parallel()

	existingStatusErr := status.Error(codes.PermissionDenied, "already a status error")

	testCases := []struct {
		name         string
		err          error
		expectedCode codes.Code
		expectedMsg  string
		passthrough  bool
	}{
		{
			name: "nil returns nil",
		},
		{
			name:         "existing status error passes through unchanged",
			err:          existingStatusErr,
			expectedCode: codes.PermissionDenied,
			expectedMsg:  "already a status error",
			passthrough:  true,
		},
		{
			name:         "typed ValidationError maps to InvalidArgument",
			err:          entity.NewValidationError("vendor_code", "is required"),
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "validation: vendor_code: is required",
		},
		{
			name:         "ErrValidation sentinel chain maps to InvalidArgument",
			err:          fmt.Errorf("wrapping: %w", entity.ErrValidation),
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "InvalidArgument sentinel maps to InvalidArgument",
			err:          pkgerr.NewInvalidArgument("missing field"),
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid argument: missing field",
		},
		{
			name:         "NotFound sentinel maps to NotFound",
			err:          pkgerr.NewNotFound("Vendor", "42"),
			expectedCode: codes.NotFound,
			expectedMsg:  "Vendor not found (id: 42)",
		},
		{
			name:         "ErrInvalidTransition maps to FailedPrecondition",
			err:          fmt.Errorf("wrap: %w", entity.ErrInvalidTransition),
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:         "ErrUnsupportedAppVersion maps to InvalidArgument",
			err:          fmt.Errorf("wrap: %w", pkgerr.ErrUnsupportedAppVersion),
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "unknown error maps to Internal with generic message",
			err:          stderrors.New("boom: secret leak"),
			expectedCode: codes.Internal,
			expectedMsg:  "internal error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := grpcerr.ToStatusError(tc.err)
			if tc.err == nil {
				require.NoError(t, got)
				return
			}
			require.Error(t, got)
			if tc.passthrough {
				require.Same(t, tc.err, got)
			}
			require.Equal(t, tc.expectedCode, status.Code(got))
			if tc.expectedMsg != "" {
				require.Equal(t, tc.expectedMsg, status.Convert(got).Message())
			}
		})
	}
}
