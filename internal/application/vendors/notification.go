package vendors

import (
	"context"

	"github.com/astronautsid/astro-boilerplate/internal/domain/event"
)

// NotificationService dispatches vendor-related notifications (e.g., FCM,
// email, Pub/Sub fan-out).
//
// The port takes a domain event rather than the entity so the application
// layer stays free of wire-format concerns: callers construct
// event.VendorCreated and pass it through; the infrastructure adapter owns
// the translation from the domain event to whatever the broker expects
// (proto, JSON, Avro). Adding a new lifecycle event = one new method here
// + one new method in the per-domain adapter (internal/infrastructure/
// messaging/pubsub/vendors/publisher.go).
type NotificationService interface {
	// SendVendorCreated is invoked after a successful CreateVendor commit.
	// Publish failures are treated as soft by the caller — the audit log
	// inside the transaction is the durable record; this event is a
	// notification, not a contract.
	SendVendorCreated(ctx context.Context, e event.VendorCreated) error
}
