// Package grpcerr translates application/domain errors into gRPC status errors.
// It is shared across every handler package under internal/interface/grpc/handler
// so per-domain handler packages can map errors uniformly without duplicating
// translation logic.
package grpcerr

import (
	stderrors "errors"

	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatusError translates application/domain errors into gRPC status errors.
//
// The mapping is intentionally narrow about codes but defensive about messages:
// unknown errors collapse to codes.Internal with a generic message so the
// transport boundary never leaks internal failure detail.
func ToStatusError(err error) error {
	if err == nil {
		return nil
	}

	// Preserve any already-typed status error (e.g., errors returned by the
	// upstream gRPC clients) so callers see the original code/message.
	if _, ok := status.FromError(err); ok {
		return err
	}

	switch {
	case isValidation(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case stderrors.Is(err, pkgerr.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case stderrors.Is(err, pkgerr.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case stderrors.Is(err, entity.ErrInvalidTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case stderrors.Is(err, pkgerr.ErrUnsupportedAppVersion):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// isValidation reports true for either a typed *entity.ValidationError or any
// error chain whose sentinel is entity.ErrValidation.
func isValidation(err error) bool {
	var ve *entity.ValidationError
	if stderrors.As(err, &ve) {
		return true
	}
	return stderrors.Is(err, entity.ErrValidation)
}
