package vendors

import (
	"context"
	"time"
)

// AuditLogEntry is the structured record written to the audit log on every
// state-changing vendor operation. Payload carries free-form, action-specific
// fields (e.g., "previous_status", "new_email").
type AuditLogEntry struct {
	VendorID   int64
	Action     string
	OccurredAt time.Time
	Payload    map[string]string
}

// AuditLogWriter persists audit log entries to a write-optimized store
// (MongoDB in the reference implementation). Failures inside a transactional
// use case MUST roll back the surrounding transaction.
type AuditLogWriter interface {
	Write(ctx context.Context, entry AuditLogEntry) error
}
