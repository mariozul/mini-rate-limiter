// Package vendors hosts the vendor-domain Pub/Sub publisher. Each business
// domain gets its own subpackage under internal/infrastructure/messaging/
// pubsub/ so the wire-format mapping for that domain stays small and
// readable: this file is the entire surface for vendor events.
//
// The application port (internal/application/vendors.NotificationService)
// takes domain events. This adapter converts them to the proto messages the
// broker topic expects (astro-proto's erppb.Vendor, validated by GCP
// Pub/Sub Schema), marshals via proto.Marshal, and publishes through the
// astro-golibs/pubsub service that the composition root constructs once.
package vendors

import (
	"context"
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	golibspubsub "github.com/astronautsid/astro-golibs/pubsub/v2"
	erppb "github.com/astronautsid/astro-proto/golang/pb/erp"
	"google.golang.org/protobuf/proto"

	appvendors "github.com/mariozul/mini-rate-limiter/internal/application/vendors"
	"github.com/mariozul/mini-rate-limiter/internal/domain/event"
)

// pubsubService is the structural surface of astro-golibs/pubsub's
// unexported *_service that this publisher depends on. Declared locally so
// Go's structural typing flows the concrete value through without naming
// the upstream unexported type. The interface is intentionally narrow —
// only what this domain's publisher actually calls.
type pubsubService interface {
	Publish(ctx context.Context, topic string, msg golibspubsub.Message) (string, error)
}

// Compile-time port satisfaction. If either struct stops satisfying the
// application port, the build fails here rather than at the composition
// root.
var (
	_ appvendors.NotificationService = (*Publisher)(nil)
	_ appvendors.NotificationService = (*NoopPublisher)(nil)
)

// Publisher is the real GCP-backed vendor publisher. It binds to a single
// topic at construction; services with multiple vendor topics should
// construct one Publisher per topic.
type Publisher struct {
	svc   pubsubService
	topic string
}

// NewPublisher wires an astro-golibs Pub/Sub service to a topic. The
// composition root owns the service lifecycle (client + topic caches); this
// adapter only holds a reference.
func NewPublisher(svc pubsubService, topic string) *Publisher {
	return &Publisher{svc: svc, topic: topic}
}

// SendVendorCreated translates the domain event to erppb.Vendor, marshals
// it as proto, and publishes. The status field is hardcoded to false
// because newly-created vendors land in the domain's PENDING state, not
// ACTIVE; the erppb proto encodes that as a boolean active flag.
func (p *Publisher) SendVendorCreated(ctx context.Context, e event.VendorCreated) error {
	msg := &erppb.Vendor{
		Id:          e.VendorID,
		CompanyName: e.CompanyName,
		Status:      false,
	}
	return p.publish(ctx, msg)
}

// publish is the chokepoint every Send* method funnels through. It opens a
// Datadog APM span around the network call so latency lands in the UI, then
// proto-marshals and publishes. No tags are attached to the span by default
// (concrete services can layer tracer.Tag options or span.SetTag calls on
// top per their observability needs).
//
// astro-golibs's Publish blocks on result.Get internally, so any
// server-side ack error surfaces synchronously and is returned to the
// caller.
func (p *Publisher) publish(ctx context.Context, msg proto.Message) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "pubsub.publish")
	defer span.Finish()

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal vendor event: %w", err)
	}
	if _, err := p.svc.Publish(ctx, p.topic, golibspubsub.Message{Data: data}); err != nil {
		return fmt.Errorf("publish to %q: %w", p.topic, err)
	}
	return nil
}

// NoopPublisher is the swap-in for local dev and tests where GCP is
// unavailable. Every Send* method returns nil without touching the wire.
type NoopPublisher struct{}

// NewNoopPublisher returns a NoopPublisher.
func NewNoopPublisher() *NoopPublisher { return &NoopPublisher{} }

// SendVendorCreated is a no-op that returns nil.
func (NoopPublisher) SendVendorCreated(_ context.Context, _ event.VendorCreated) error { return nil }
