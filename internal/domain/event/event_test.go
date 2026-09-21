package event_test

import (
	"testing"
	"time"

	"github.com/astronautsid/astro-boilerplate/internal/domain/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertions that the example events implement DomainEvent.
var (
	_ event.DomainEvent = event.VendorCreated{}
	_ event.DomainEvent = event.VendorUpdated{}
)

func TestVendorCreated(t *testing.T) {
	t.Parallel()

	occurred := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	e := event.VendorCreated{
		VendorID:       42,
		VendorCode:     "V-042",
		OccurredAtTime: occurred,
	}

	var de event.DomainEvent = e
	require.Equal(t, "42", de.AggregateID())
	require.Equal(t, occurred, de.OccurredAt())
	assert.Equal(t, int64(42), e.VendorID)
	assert.Equal(t, "V-042", e.VendorCode)
}

func TestVendorCreated_NegativeID(t *testing.T) {
	t.Parallel()

	e := event.VendorCreated{VendorID: -7}
	require.Equal(t, "-7", e.AggregateID())
}

func TestVendorUpdated(t *testing.T) {
	t.Parallel()

	occurred := time.Date(2026, 5, 18, 11, 30, 0, 0, time.UTC)
	changes := map[string]string{
		"contactEmail": "new@acme.test",
		"contactPhone": "+62-21-1111",
	}
	e := event.VendorUpdated{
		VendorID:       1001,
		OccurredAtTime: occurred,
		Changes:        changes,
	}

	var de event.DomainEvent = e
	require.Equal(t, "1001", de.AggregateID())
	require.Equal(t, occurred, de.OccurredAt())
	assert.Equal(t, changes, e.Changes)
}
