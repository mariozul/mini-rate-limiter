// Package event declares the DomainEvent interface and scaffolded event types
// for the boilerplate's example aggregate. Application code declares the
// DomainEventPublisher port; infrastructure adapters are not required to be
// wired in this scaffold.
package event

import (
	"strconv"
	"time"
)

// DomainEvent is implemented by every domain event. Events are immutable
// records of something that happened inside an aggregate.
type DomainEvent interface {
	// AggregateID returns the stringified identifier of the aggregate that
	// produced the event. Encoded as a string so heterogeneous publishers can
	// handle multiple aggregate ID types uniformly.
	AggregateID() string
	// OccurredAt returns the time the event was produced inside the domain.
	OccurredAt() time.Time
}

// VendorCreated is emitted when a new vendor aggregate is persisted for the
// first time. Fields carry everything a downstream consumer (or wire-format
// adapter) needs to act on the event without reaching back into the entity.
type VendorCreated struct {
	VendorID       int64
	VendorCode     string
	CompanyName    string
	OccurredAtTime time.Time
}

// AggregateID returns the vendor identifier as a base-10 string.
func (e VendorCreated) AggregateID() string {
	return strconv.FormatInt(e.VendorID, 10)
}

// OccurredAt returns the time the event was produced.
func (e VendorCreated) OccurredAt() time.Time {
	return e.OccurredAtTime
}

// VendorUpdated is emitted when an existing vendor's mutable fields change.
// Changes is a free-form map describing which fields changed; it is meant for
// audit / debugging use, not for downstream business logic.
type VendorUpdated struct {
	VendorID       int64
	OccurredAtTime time.Time
	Changes        map[string]string
}

// AggregateID returns the vendor identifier as a base-10 string.
func (e VendorUpdated) AggregateID() string {
	return strconv.FormatInt(e.VendorID, 10)
}

// OccurredAt returns the time the event was produced.
func (e VendorUpdated) OccurredAt() time.Time {
	return e.OccurredAtTime
}
