// Package external hosts the boilerplate's outbound gRPC client adapters.
//
// Composition root SHALL call NewGrpcClientConn exactly once per upstream
// service and multiplex any number of logical adapters off the returned
// connection. Every adapter under this tree SHALL translate proto-generated
// types to plain Go types at its boundary so the application layer never
// imports github.com/astronautsid/astro-proto/...
//
// See internal/infrastructure/external/boilerplate/client.go for the
// reference adapter shape (one ClientConn → one typed wrapper → application
// DTOs).
package external

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	golibsgrpc "github.com/astronautsid/astro-golibs/grpc/util"
	"github.com/astronautsid/astro-boilerplate/internal/config"
	"google.golang.org/grpc"
)

// NewGrpcClientConn opens a single *grpc.ClientConn to the upstream service
// described by cfg, using astro-golibs/grpc/util as the underlying dialer.
// Callers own the connection lifecycle and SHALL call Close() at shutdown;
// multiple logical adapters MUST share one connection rather than redialing.
//
// astro-golibs handles transport credentials (TLS 1.3 when cfg.TLS is true),
// Datadog tracing (service span tag = appName), and a bounded dial timeout
// (cfg.TimeoutSec). cfg.Addr is parsed as a standard "host:port" string;
// astro-golibs takes them as separate arguments so we split here.
//
// To enable retry/backoff on transient failures (codes.Unavailable,
// codes.Unknown), pass golibsgrpc.WithRetry(...) as an extra option. The
// boilerplate omits retry by default so failures surface immediately.
func NewGrpcClientConn(ctx context.Context, cfg config.GrpcServiceConfig, appName string) (*grpc.ClientConn, error) {
	host, portStr, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("parse addr %q: %w", cfg.Addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("parse port %q: %w", portStr, err)
	}

	conn, err := golibsgrpc.NewConnection(ctx,
		golibsgrpc.WithHost(host),
		golibsgrpc.WithPort(port),
		golibsgrpc.WithTLS(cfg.TLS),
		golibsgrpc.WithTimeout(time.Duration(cfg.TimeoutSec)*time.Second),
		golibsgrpc.WithServiceName(appName),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.Addr, err)
	}
	return conn, nil
}
