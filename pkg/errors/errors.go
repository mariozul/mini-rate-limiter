package errors

import (
	"errors"
	"fmt"
)

// Base sentinel errors for the domain.
var (
	ErrNotFound              = errors.New("resource not found")
	ErrInvalidArgument       = errors.New("invalid argument")
	ErrUnsupportedAppVersion = errors.New("unsupported app version")
)

// Typed wrapper errors that attach semantic meaning to arbitrary errors.

// NotFoundError wraps a resource name and identifier.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found (id: %s)", e.Resource, e.ID)
}

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// NewNotFound creates a formatted NotFoundError.
func NewNotFound(resource, id string) error {
	return &NotFoundError{Resource: resource, ID: id}
}

// InvalidArgumentError formats a bad request reason.
type InvalidArgumentError struct {
	Reason string
}

func (e *InvalidArgumentError) Error() string {
	return fmt.Sprintf("invalid argument: %s", e.Reason)
}

func (e *InvalidArgumentError) Unwrap() error { return ErrInvalidArgument }

// NewInvalidArgument wraps a validation failure reason.
func NewInvalidArgument(reason string) error {
	return &InvalidArgumentError{Reason: reason}
}
