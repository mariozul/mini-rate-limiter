package pubsub

import (
	"context"
	"time"

	cloudpubsub "cloud.google.com/go/pubsub/v2"
	golibspubsub "github.com/astronautsid/astro-golibs/pubsub/v2"
)

// subscriberService is the structural surface of astro-golibs/pubsub
// *_service that the Subscriber adapter depends on. The upstream type is
// unexported and therefore unnameable across package boundaries; declaring
// the interface here lets Go's structural typing wire the concrete value
// through as long as the method set matches. Per-domain publisher
// subpackages declare their own equivalent (see e.g. vendors/publisher.go's
// pubsubService) so each adapter advertises only the surface it actually
// uses.
type subscriberService interface {
	RegisterSubscription(
		ctx context.Context,
		cfg golibspubsub.SubscriptionConfig,
		fn func(context.Context, *cloudpubsub.Message),
	) error
}

// Subscriber wraps an astro-golibs pubsub service and exposes a thin
// Register method with the boilerplate's default ReceiveSettings preset.
// Composition root constructs one Subscriber per process and registers each
// subscription via Register.
type Subscriber struct {
	svc subscriberService
}

// NewSubscriber constructs a Subscriber backed by the given astro-golibs
// service. The svc parameter is typed as the local subscriberService
// interface — see base.go's package doc for why.
func NewSubscriber(svc subscriberService) *Subscriber {
	return &Subscriber{svc: svc}
}

// Register starts a long-lived consumer for the given subscription ID with
// the boilerplate's standard ReceiveSettings (NumGoroutines: 4,
// MaxOutstandingMessage: 10, MaxOutstandingBytes: 1 MiB,
// MaxExtension: 10 minutes). astro-golibs spawns a goroutine internally and
// retries with exponential backoff up to 5 times on transient errors;
// passing a cancellable context to ctx is how callers stop the consumer.
//
// Concrete services that need different tuning should bypass Register and
// call svc.RegisterSubscription directly with a custom SubscriptionConfig.
func (s *Subscriber) Register(
	ctx context.Context,
	subID string,
	handler func(ctx context.Context, msg *cloudpubsub.Message),
) error {
	cfg := golibspubsub.SubscriptionConfig{
		SubID:                 subID,
		NumGoroutines:         4,
		MaxOutstandingMessage: 10,
		MaxOutstandingBytes:   1 << 20, // 1 MiB
		MaxExtension:          10 * time.Minute,
	}
	return s.svc.RegisterSubscription(ctx, cfg, handler)
}
