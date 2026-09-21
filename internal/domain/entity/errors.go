// Package entity contains aggregate roots and their behavior for the domain layer.
//
// Domain errors are exposed as sentinel values (for fixed conditions) or typed
// wrappers (for variable-field errors). Both satisfy the standard library's
// errors.Is/errors.As contract so application handlers can branch on them.
package entity

import (
	"errors"
	"fmt"
)

// ErrInvalidTransition is returned when a lifecycle method is invoked on an
// entity that is in a state from which the transition is not allowed.
var ErrInvalidTransition = errors.New("invalid state transition")

// ErrValidation is the sentinel returned by ValidationError.Unwrap so callers
// can use errors.Is(err, entity.ErrValidation) to detect any validation failure.
var ErrValidation = errors.New("validation error")

// ValidationError represents a domain validation failure for a specific field.
// It wraps ErrValidation, so errors.Is(err, ErrValidation) returns true and
// errors.As(err, &validationErr) yields the typed value with field context.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}

// Unwrap returns ErrValidation so the error chain supports errors.Is.
func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// NewValidationError constructs a *ValidationError with the given field and message.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}
