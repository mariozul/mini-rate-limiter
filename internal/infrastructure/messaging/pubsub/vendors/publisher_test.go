package vendors

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	golibspubsub "github.com/astronautsid/astro-golibs/pubsub/v2"
	erppb "github.com/astronautsid/astro-proto/golang/pb/erp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/astronautsid/astro-boilerplate/internal/domain/event"
)

// fakeSvc captures the last Publish call so the test can verify the topic
// the publisher chose and round-trip the marshaled proto bytes.
type fakeSvc struct {
	topic   string
	msg     golibspubsub.Message
	called  int
	pubErr  error
	retMsgID string
}

func (f *fakeSvc) Publish(_ context.Context, topic string, msg golibspubsub.Message) (string, error) {
	f.called++
	f.topic = topic
	f.msg = msg
	return f.retMsgID, f.pubErr
}

func TestPublisher_SendVendorCreated_RoundTripsProto(t *testing.T) {
	svc := &fakeSvc{retMsgID: "msg-1"}
	p := NewPublisher(svc, "vendor-events")

	err := p.SendVendorCreated(context.Background(), event.VendorCreated{
		VendorID:       42,
		VendorCode:     "V-042",
		CompanyName:    "Acme",
		OccurredAtTime: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, 1, svc.called)
	require.Equal(t, "vendor-events", svc.topic, "publisher must use the topic bound at construction")

	// The wire payload is proto-encoded erppb.Vendor — round-trip it back
	// to verify the field mapping is what the schema expects.
	var got erppb.Vendor
	require.NoError(t, proto.Unmarshal(svc.msg.Data, &got))
	require.Equal(t, int64(42), got.GetId())
	require.Equal(t, "Acme", got.GetCompanyName())
	require.False(t, got.GetStatus(), "newly-created vendor is PENDING → status=false")
}

func TestPublisher_SendVendorCreated_PropagatesPublishError(t *testing.T) {
	svc := &fakeSvc{pubErr: stderrors.New("broker down")}
	p := NewPublisher(svc, "vendor-events")

	err := p.SendVendorCreated(context.Background(), event.VendorCreated{VendorID: 1})
	require.Error(t, err)
	require.ErrorContains(t, err, "broker down")
	require.ErrorContains(t, err, "vendor-events", "error must name the topic for ops")
}

func TestNoopPublisher_SendVendorCreated_ReturnsNil(t *testing.T) {
	require.NoError(t, NewNoopPublisher().SendVendorCreated(context.Background(), event.VendorCreated{}))
}
