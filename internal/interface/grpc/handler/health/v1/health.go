// Package v1 implements the standard grpc_health_v1.HealthServer.
//
// The handler ties three readiness signals together:
//
//  1. A readiness flag flipped to false at the start of graceful
//     shutdown (see pkg/shutdown). While the flag is false, Check
//     returns NOT_SERVING immediately regardless of DB state — this
//     lets the load balancer delist the pod BEFORE the listener
//     closes (the "pre-stop sleep" pattern).
//
//  2. A SELECT 1 round-trip against the Postgres master to verify
//     write-side connectivity.
//
//  3. A SELECT 1 round-trip against the Postgres replica to verify
//     read-side connectivity.
//
// Any of those checks failing maps to NOT_SERVING. The gRPC health
// protocol's convention is that the RPC itself succeeds (returns nil
// error) and signals "unhealthy" via the response Status field; we
// preserve that, log the failing dependency at Warn level for ops
// visibility, and let the caller decide what to do.
package v1

import (
	"context"
	"database/sql"
	"fmt"

	logger "github.com/astronautsid/astro-golibs/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/atomic"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// masterChecker is the slice of safesql.MasterDB the health handler
// depends on. Declared locally so tests can pass a fake without pulling
// the whole safesql interface tree into the test setup.
type masterChecker interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// replicaChecker is the slice of safesql.ReplicaDB the health handler
// depends on.
type replicaChecker interface {
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}

// Handler implements grpc_health_v1.HealthServer with readiness +
// dependency checks.
type Handler struct {
	grpc_health_v1.UnimplementedHealthServer
	master  masterChecker
	replica replicaChecker
	log     logger.Logger
	ready   *atomic.Bool
}

// NewHandler constructs a Handler. Readiness starts true; call
// MarkUnready at the beginning of graceful shutdown to flip it.
func NewHandler(master masterChecker, replica replicaChecker, log logger.Logger) *Handler {
	r := atomic.NewBool(true)
	return &Handler{
		master:  master,
		replica: replica,
		log:     log,
		ready:   r,
	}
}

// MarkUnready flips the readiness flag to false. Subsequent Check calls
// will short-circuit to NOT_SERVING without touching the database.
// Idempotent.
func (h *Handler) MarkUnready() {
	h.ready.Store(false)
}

// IsReady reports the current readiness flag. Mainly intended for
// integration tests; production callers should rely on Check.
func (h *Handler) IsReady() bool {
	return h.ready.Load()
}

// Check answers SERVING when (a) readiness is still true AND (b) both
// the master and replica connections respond to SELECT 1. The service
// name on the request is ignored — the boilerplate publishes a single
// overall health status rather than per-service health.
//
// The DB ping uses ExecContext on master (so we exercise the write
// connection's session even though no rows come back) and
// QueryRowxContext on replica (which goes through the read connection).
// Both inherit the request's context so the probe respects gRPC
// deadlines.
func (h *Handler) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if !h.ready.Load() {
		return notServing(), nil
	}

	if _, err := h.master.ExecContext(ctx, "SELECT 1"); err != nil {
		h.log.Warn(fmt.Sprintf("health: postgres master ping failed: %v", err))
		return notServing(), nil
	}

	var probe int
	if err := h.replica.QueryRowxContext(ctx, "SELECT 1").Scan(&probe); err != nil {
		h.log.Warn(fmt.Sprintf("health: postgres replica ping failed: %v", err))
		return notServing(), nil
	}

	return serving(), nil
}

func serving() *grpc_health_v1.HealthCheckResponse {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}
}

func notServing() *grpc_health_v1.HealthCheckResponse {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}
}
