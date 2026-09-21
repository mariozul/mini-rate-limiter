package vendors

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/mariozul/mini-rate-limiter/internal/domain/entity"
	"github.com/mariozul/mini-rate-limiter/internal/domain/event"
	pkgerr "github.com/mariozul/mini-rate-limiter/pkg/errors"
)

// CreateVendorCommand is the input DTO for the CreateVendor use case.
type CreateVendorCommand struct {
	VendorCode   string
	CompanyName  string
	NetsuiteID   string
	ContactEmail string
	ContactPhone string
}

// CreateVendorResponse is the output DTO for the CreateVendor use case.
type CreateVendorResponse struct {
	ID         int64
	VendorCode string
	Status     string
}

// CreateVendorParams groups the dependencies for CreateVendorHandler.
type CreateVendorParams struct {
	Repo      Repository
	TxManager TransactionManager
	Lock      DistributedLock
	AuditLog  AuditLogWriter
	Notify    NotificationService
}

// CreateVendorHandler orchestrates vendor creation: dedupe lock, code
// uniqueness check, entity construction, transactional persist + audit, then
// a best-effort notification publish.
type CreateVendorHandler struct {
	repo      Repository
	txManager TransactionManager
	lock      DistributedLock
	auditLog  AuditLogWriter
	notify    NotificationService
}

// NewCreateVendorHandler constructs a CreateVendorHandler from the given params.
func NewCreateVendorHandler(p CreateVendorParams) *CreateVendorHandler {
	return &CreateVendorHandler{
		repo:      p.Repo,
		txManager: p.TxManager,
		lock:      p.Lock,
		auditLog:  p.AuditLog,
		notify:    p.Notify,
	}
}

// Execute runs the CreateVendor use case end-to-end.
func (h *CreateVendorHandler) Execute(ctx context.Context, cmd CreateVendorCommand) (CreateVendorResponse, error) {
	lockKey := "vendor:create:" + cmd.VendorCode
	acquired, err := h.lock.Acquire(ctx, lockKey, 30*time.Second)
	if err != nil {
		return CreateVendorResponse{}, fmt.Errorf("acquire distributed lock: %w", err)
	}
	if !acquired {
		return CreateVendorResponse{}, pkgerr.NewInvalidArgument("vendor create already in progress")
	}
	defer func() {
		_ = h.lock.Release(context.Background(), lockKey)
	}()

	existing, err := h.repo.FindByCode(ctx, cmd.VendorCode)
	if err != nil && !stderrors.Is(err, pkgerr.ErrNotFound) {
		return CreateVendorResponse{}, fmt.Errorf("find vendor by code: %w", err)
	}
	if existing != nil {
		return CreateVendorResponse{}, pkgerr.NewInvalidArgument("vendor code already exists")
	}

	now := time.Now()
	vendor, err := entity.NewVendor(
		cmd.VendorCode,
		cmd.CompanyName,
		cmd.NetsuiteID,
		cmd.ContactEmail,
		cmd.ContactPhone,
		now,
	)
	if err != nil {
		return CreateVendorResponse{}, err
	}

	var newID int64
	if err := h.txManager.Execute(ctx, func(tx Tx) error {
		id, err := h.repo.Save(ctx, tx, vendor)
		if err != nil {
			return fmt.Errorf("save vendor: %w", err)
		}
		_ = vendor.AssignID(id)
		newID = id

		entry := AuditLogEntry{
			VendorID:   id,
			Action:     "CREATE",
			OccurredAt: now,
			Payload: map[string]string{
				"vendor_code":  cmd.VendorCode,
				"company_name": cmd.CompanyName,
				"netsuite_id":  cmd.NetsuiteID,
			},
		}
		if err := h.auditLog.Write(ctx, entry); err != nil {
			return fmt.Errorf("write audit log: %w", err)
		}
		return nil
	}); err != nil {
		return CreateVendorResponse{}, err
	}

	// Best-effort publish AFTER the transaction commits. Publish failures are
	// intentionally swallowed — the audit log inside the transaction is the
	// durable record; the pub/sub event is a notification, not a contract.
	// A nil notifier is tolerated so composition roots that have not yet
	// wired Notify continue to compile and run.
	//
	// The application constructs the domain event; the infrastructure
	// adapter (internal/infrastructure/messaging/pubsub/vendors) owns the
	// translation from event.VendorCreated to the wire format.
	if h.notify != nil {
		_ = h.notify.SendVendorCreated(ctx, event.VendorCreated{
			VendorID:       vendor.ID(),
			VendorCode:     vendor.VendorCode(),
			CompanyName:    vendor.CompanyName(),
			OccurredAtTime: now,
		})
	}

	return CreateVendorResponse{
		ID:         newID,
		VendorCode: vendor.VendorCode(),
		Status:     vendor.Status().String(),
	}, nil
}
