// Package mongodb wires the optional MongoDB client. The boilerplate's only
// Mongo-backed adapter is the vendor audit-log writer; an empty Mongo URI in
// config signals "disabled" and the composition root substitutes the NoOp
// audit writer.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrDisabled signals that Mongo was deliberately left unconfigured. Callers
// in main.go branch on errors.Is(err, mongodb.ErrDisabled) to choose the
// NoOp audit writer instead of failing startup.
var ErrDisabled = errors.New("mongo disabled (URI empty)")

// NewMongoDB opens a MongoDB connection from the provided MongoConfig, pings
// to verify reachability, and returns the named database handle. The supplied
// context bounds the Connect+Ping handshake; pass a context with a deadline
// from main.go.
//
// The connection identity comes from cfg.URI verbatim — hosts, credentials,
// replica-set name, authSource, and TLS options are all part of the URI
// injected by whoever provisions the cluster. The service does not assemble
// URIs from parts.
//
// cfg.Database is the application's choice of database within that cluster
// (passed to `client.Database()`), not part of the connection string.
//
// Pool behaviour is service-owned: MaxPoolSize / MinPoolSize / MaxConnIdleTime,
// plus the two latency budgets (ConnectTimeout for the initial handshake,
// ServerSelectionTimeout for routing every subsequent op). Zero values are
// preserved as zero so the mongo driver falls back to its documented defaults
// (MaxPoolSize 100, MinPoolSize 0, no idle limit, 30s connect, 30s selection)
// when the operator hasn't expressed an opinion.
//
// Returns ErrDisabled when cfg.URI is empty so the caller can fall back to
// the NoOp variant without inspecting strings.
func NewMongoDB(ctx context.Context, cfg config.MongoConfig) (*mongo.Database, error) {
	if cfg.URI == "" {
		return nil, ErrDisabled
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mongo: empty Database (set mongo.database)")
	}

	clientOpts := options.Client().ApplyURI(cfg.URI)

	if cfg.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize > 0 {
		clientOpts.SetMinPoolSize(cfg.MinPoolSize)
	}
	if cfg.MaxConnIdleTimeSec > 0 {
		clientOpts.SetMaxConnIdleTime(time.Duration(cfg.MaxConnIdleTimeSec) * time.Second)
	}
	if cfg.ConnectTimeoutSec > 0 {
		clientOpts.SetConnectTimeout(time.Duration(cfg.ConnectTimeoutSec) * time.Second)
	}
	if cfg.ServerSelectionTimeoutSec > 0 {
		clientOpts.SetServerSelectionTimeout(time.Duration(cfg.ServerSelectionTimeoutSec) * time.Second)
	}

	// Bound the Connect+Ping handshake. Prefer the operator-configured
	// connect timeout; fall back to 10s as a sensible boilerplate default.
	handshakeBudget := 10 * time.Second
	if cfg.ConnectTimeoutSec > 0 {
		handshakeBudget = time.Duration(cfg.ConnectTimeoutSec) * time.Second
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, handshakeBudget)
	defer cancel()

	client, err := mongo.Connect(handshakeCtx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo: connect failed: %w", err)
	}
	if err := client.Ping(handshakeCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping failed: %w", err)
	}

	return client.Database(cfg.Database), nil
}
