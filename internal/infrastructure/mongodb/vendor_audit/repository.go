// Package vendor_audit implements the AuditLogWriter port using a MongoDB
// collection. A NoOp variant ships in the same package so main.go can pick
// one at startup based on whether MONGO_URI is set.
package vendor_audit

import (
	"context"
	"fmt"

	appvendors "github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// collectionName is the canonical Mongo collection that stores vendor audit
// events. Pulled out as a constant so test fixtures and migrations can
// reference the same string.
const collectionName = "vendor_audit"

// Repository writes vendor audit entries to MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// Compile-time check that *Repository satisfies the application port.
var _ appvendors.AuditLogWriter = (*Repository)(nil)

// NewRepository binds the writer to the `vendor_audit` collection on the
// supplied database. The collection is created lazily by Mongo on first
// InsertOne; no schema migration is required.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection(collectionName)}
}

// Write persists an audit entry. The document layout is flat (no embedded
// envelope) so analytics queries can filter on top-level fields without a
// projection step.
func (r *Repository) Write(ctx context.Context, entry appvendors.AuditLogEntry) error {
	doc := bson.M{
		"vendor_id":   entry.VendorID,
		"action":      entry.Action,
		"occurred_at": entry.OccurredAt,
		"payload":     entry.Payload,
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("vendor_audit: insert failed: %w", err)
	}
	return nil
}

// NoOpRepository is the AuditLogWriter used when Mongo is disabled. Calls to
// Write are dropped on the floor — acceptable for local dev where audit logs
// are not part of the inner-loop story.
type NoOpRepository struct{}

// Compile-time check that NoOpRepository satisfies the application port.
var _ appvendors.AuditLogWriter = (*NoOpRepository)(nil)

// NewNoOp returns an AuditLogWriter that ignores every write.
func NewNoOp() *NoOpRepository {
	return &NoOpRepository{}
}

// Write is a no-op.
func (NoOpRepository) Write(ctx context.Context, entry appvendors.AuditLogEntry) error {
	return nil
}
