// Package boilerplate is the reference outbound-gRPC adapter for this
// scaffold. It wraps astro-proto's example BoilerplateService — a single-
// RPC service that exists for exactly this purpose — and demonstrates the
// adapter pattern every real external client should follow:
//
//  1. One *grpc.ClientConn per upstream service (dialed by main.go via
//     external.NewGrpcClientConn).
//  2. A typed wrapper struct that holds the proto-generated client and a
//     per-RPC timeout.
//  3. Public methods on the wrapper that take plain Go types in, call the
//     proto client, and return plain Go types out — so the application
//     layer never imports github.com/astronautsid/astro-proto/...
//
// Concrete services replace this package with one per upstream
// (account/, logistic/, ims/, etc.), each adapting that upstream's proto
// client to its own application port.
package boilerplate

import (
	"context"
	"fmt"
	"time"

	boilerplatepb "github.com/astronautsid/astro-proto/golang/pb/boilerplate"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client is the typed adapter around boilerplatepb.BoilerplateServiceClient.
// Hold one per upstream connection; share across goroutines (the underlying
// proto client is goroutine-safe).
type Client struct {
	pb         boilerplatepb.BoilerplateServiceClient
	timeoutSec int
}

// NewClient constructs the adapter from a *grpc.ClientConn opened by
// external.NewGrpcClientConn. timeoutSec bounds each RPC; pass the same
// value the dialer used so per-RPC and dial timeouts stay aligned.
func NewClient(conn *grpc.ClientConn, timeoutSec int) *Client {
	return &Client{
		pb:         boilerplatepb.NewBoilerplateServiceClient(conn),
		timeoutSec: timeoutSec,
	}
}

// GetResult calls the upstream GetBoilerplateResult RPC and returns the
// result string. The proto response envelope is unwrapped here so callers
// see a plain string + error, not the nested Data/Error layout.
func (c *Client) GetResult(ctx context.Context) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(c.timeoutSec)*time.Second)
	defer cancel()

	resp, err := c.pb.GetBoilerplateResult(callCtx, &emptypb.Empty{})
	if err != nil {
		return "", fmt.Errorf("get boilerplate result: %w", err)
	}
	return resp.GetData().GetResult(), nil
}
