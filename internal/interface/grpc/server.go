// Package grpc owns the gRPC transport surface: the Server constructor that
// registers handlers and interceptors, and the runtime lifecycle (Serve,
// GracefulStop). The composition root in cmd/grpc/main.go is the only caller.
package grpc

import (
	grpctrace "github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2"
	golibsmd "github.com/astronautsid/astro-golibs/grpc/metadata"
	healthv1 "github.com/astronautsid/astro-boilerplate/internal/interface/grpc/handler/health/v1"
	vendorv1 "github.com/astronautsid/astro-boilerplate/internal/interface/grpc/handler/vendors/v1"
	logger "github.com/astronautsid/astro-golibs/logger"
	erppb "github.com/astronautsid/astro-proto/golang/pb/erp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// NewServer constructs a *grpc.Server with the vendor and health services
// registered and (optionally) reflection enabled. Caller owns lifecycle:
// listen, Serve, and GracefulStop are not done here so the composition root
// can wire its own SIGINT/SIGTERM handling.
//
// The vendor service contract comes from astro-proto's erp.VendorService —
// the boilerplate only implements CreateVendor and GetVendor; the remaining
// RPCs declared by erp.VendorService are inherited from the proto's
// UnimplementedVendorServiceServer and return codes.Unimplemented.
//
// Interceptor chain (outer → inner):
//   - Datadog APM: dd-trace-go grpc contrib creates the server-side span
//     and tags it with traceServiceName so spans land under the same
//     service in the Datadog UI as the tracer started in main.go.
//   - Astronautsid metadata: astro-golibs/grpc/metadata extracts the
//     standard astronautsid metadata keys (X-Request-ID, X-User-ID,
//     X-Device-ID, X-App-Version, x-forwarded-for, etc.) from the
//     incoming context and stores them as typed context values.
//     Handlers / application code can read them via metadata.Value(ctx, K).
//
// Datadog is outermost so the span covers metadata extraction and any
// handler error mapping.
func NewServer(
	log logger.Logger,
	traceServiceName string,
	vendorHandler *vendorv1.Handler,
	healthHandler *healthv1.Handler,
	reflectionEnabled bool,
) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpctrace.UnaryServerInterceptor(grpctrace.WithService(traceServiceName)),
			golibsmd.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpctrace.StreamServerInterceptor(grpctrace.WithService(traceServiceName)),
			golibsmd.UnaryStreamInterceptor(),
		),
	)

	erppb.RegisterVendorServiceServer(srv, vendorHandler)
	grpc_health_v1.RegisterHealthServer(srv, healthHandler)

	if reflectionEnabled {
		reflection.Register(srv)
		log.Info("grpc server reflection enabled")
	}

	return srv
}
