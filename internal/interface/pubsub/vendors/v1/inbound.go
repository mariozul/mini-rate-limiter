// Package v1 hosts the v1 inbound transport handlers for vendor-related
// Pub/Sub subscriptions. This package is the analogue of
// internal/interface/grpc/handler/vendors/v1 on the gRPC side: it owns the
// transport boundary (Pub/Sub message decode + Ack/Nack semantics) and
// dispatches to application use cases.
//
// The boilerplate ships a minimal logging-only Handle method so operators
// can verify subscription wiring end-to-end without a downstream consumer.
// Real services SHALL replace the logging body with an application call
// (e.g., myCommand.Execute(ctx, ...)).
package v1

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	cloudpubsub "cloud.google.com/go/pubsub/v2"

	logger "github.com/astronautsid/astro-golibs/logger"
)

// EventHandler is the inbound transport handler for vendor events. It owns
// the Pub/Sub message lifecycle (decode + Ack/Nack) so the subscription
// adapter (internal/infrastructure/messaging/pubsub) stays purely about
// registration plumbing.
type EventHandler struct {
	logger logger.Logger
}

// NewEventHandler constructs an EventHandler with the given application
// logger. The logger is the only collaborator because the scaffold's Handle
// method only logs; downstream services replace this constructor signature
// (and the struct) when they add real dependencies (command handlers,
// metrics, etc.).
func NewEventHandler(log logger.Logger) *EventHandler {
	return &EventHandler{logger: log}
}

// inboundEnvelope is the minimal JSON shape every inbound vendor event
// shares. Real handlers decode into a richer DTO once the event name
// dispatches them to a specific use case.
type inboundEnvelope struct {
	Event string `json:"event"`
}

// Handle decodes the message envelope, logs a one-liner identifying the
// event, and acks. On decode failure it nacks: a malformed payload is
// usually a poison message that a real service would route to a DLQ via
// the subscription's dead-letter policy. Nacking (rather than acking on
// decode failure) means the broker will redeliver per its retry policy
// until either the message succeeds or the DLQ takes it; for boilerplate
// purposes that surfaces wiring bugs early instead of silently dropping
// them.
//
// Datadog APM: opens a root consumer span for the message receive so
// latency lands in the UI; no extra tags are attached. Concrete services
// that need richer metadata (resource name, event type, error tagging) can
// layer it on at the span via tracer.Tag options or span.SetTag. The
// publisher does not inject trace context into msg.Attributes, so consumer
// spans live in their own traces (not linked to the producer's).
//
// Concrete services replace the log line with an application call
// (e.g., a command.Execute(ctx, ...)) and choose Ack/Nack based on the
// returned error: nil → Ack, transient error → Nack and let the broker
// retry, permanent error → Ack and emit a metric.
func (h *EventHandler) Handle(ctx context.Context, msg *cloudpubsub.Message) {
	span, ctx := tracer.StartSpanFromContext(ctx, "pubsub.consume")
	defer span.Finish()

	var env inboundEnvelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		h.logger.Error(fmt.Sprintf(
			"vendor inbound: decode failed: id=%s err=%v",
			msg.ID, err,
		))
		msg.Nack()
		return
	}

	h.logger.Info(fmt.Sprintf(
		"vendor inbound: event=%q id=%s",
		env.Event, msg.ID,
	))
	msg.Ack()
	_ = ctx // reserved for downstream command.Execute(ctx, ...) calls
}
