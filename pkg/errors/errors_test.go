package errors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConstructors(t *testing.T) {
	t.Parallel()

	notFound := NewNotFound("Ticket", "10")
	require.ErrorIs(t, notFound, ErrNotFound)
	require.Equal(t, "Ticket not found (id: 10)", notFound.Error())

	invalid := NewInvalidArgument("missing field")
	require.ErrorIs(t, invalid, ErrInvalidArgument)
	require.Equal(t, "invalid argument: missing field", invalid.Error())
}
