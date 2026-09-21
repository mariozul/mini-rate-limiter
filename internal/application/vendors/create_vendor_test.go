package vendors_test

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/astronautsid/astro-boilerplate/internal/application/vendors"
	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
	"github.com/astronautsid/astro-boilerplate/internal/domain/event"
	"github.com/astronautsid/astro-boilerplate/internal/domain/valueobject"
	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
	"github.com/stretchr/testify/require"
)

// --- Test doubles -----------------------------------------------------------

type fakeRepo struct {
	savedVendor   *entity.Vendor
	saveID        int64
	saveErr       error
	saveCalled    int
	updateCalled  int
	updateErr     error
	findByCodeRes *entity.Vendor
	findByCodeErr error
	findByIDRes   *entity.Vendor
	findByIDErr   error
}

func (f *fakeRepo) Save(_ context.Context, _ vendors.Tx, v *entity.Vendor) (int64, error) {
	f.saveCalled++
	if f.saveErr != nil {
		return 0, f.saveErr
	}
	f.savedVendor = v
	return f.saveID, nil
}
func (f *fakeRepo) Update(_ context.Context, _ vendors.Tx, _ *entity.Vendor) error {
	f.updateCalled++
	return f.updateErr
}
func (f *fakeRepo) FindByID(_ context.Context, _ int64) (*entity.Vendor, error) {
	return f.findByIDRes, f.findByIDErr
}
func (f *fakeRepo) FindByCode(_ context.Context, _ string) (*entity.Vendor, error) {
	return f.findByCodeRes, f.findByCodeErr
}

type fakeTxManager struct {
	executed int
}

func (f *fakeTxManager) Execute(ctx context.Context, fn func(tx vendors.Tx) error) error {
	f.executed++
	return fn(nil)
}

type fakeLock struct {
	acquired      bool
	acquireErr    error
	acquireCalled int
	releaseCalled int
}

func (f *fakeLock) Acquire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	f.acquireCalled++
	return f.acquired, f.acquireErr
}
func (f *fakeLock) Release(_ context.Context, _ string) error {
	f.releaseCalled++
	return nil
}

type fakeAuditLog struct {
	entries []vendors.AuditLogEntry
	err     error
}

func (f *fakeAuditLog) Write(_ context.Context, entry vendors.AuditLogEntry) error {
	f.entries = append(f.entries, entry)
	return f.err
}

type fakeNotifier struct {
	sendCreatedCount int
	sendCreatedErr   error
	lastCreatedEvent event.VendorCreated
}

func (f *fakeNotifier) SendVendorCreated(_ context.Context, e event.VendorCreated) error {
	f.sendCreatedCount++
	f.lastCreatedEvent = e
	return f.sendCreatedErr
}

// --- Tests ------------------------------------------------------------------

func newHandler(repo *fakeRepo, tx *fakeTxManager, lock *fakeLock, audit *fakeAuditLog, notify *fakeNotifier, _ time.Time) *vendors.CreateVendorHandler {
	return vendors.NewCreateVendorHandler(vendors.CreateVendorParams{
		Repo:      repo,
		TxManager: tx,
		Lock:      lock,
		AuditLog:  audit,
		Notify:    notify,
	})
}

func validCommand() vendors.CreateVendorCommand {
	return vendors.CreateVendorCommand{
		VendorCode:   "V-001",
		CompanyName:  "Acme",
		NetsuiteID:   "NS-1",
		ContactEmail: "ops@acme.test",
		ContactPhone: "+62811",
	}
}

func TestCreateVendor_HappyPath(t *testing.T) {
	repo := &fakeRepo{saveID: 123, findByCodeErr: pkgerr.NewNotFound("Vendor", "V-001")}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, time.Time{})

	start := time.Now()
	resp, err := h.Execute(context.Background(), validCommand())
	end := time.Now()
	require.NoError(t, err)
	require.Equal(t, int64(123), resp.ID)
	require.Equal(t, "V-001", resp.VendorCode)
	require.Equal(t, string(valueobject.VendorStatusPending), resp.Status)

	require.Equal(t, 1, lock.acquireCalled)
	require.Equal(t, 1, lock.releaseCalled)
	require.Equal(t, 1, repo.saveCalled)
	require.Equal(t, 1, tx.executed)
	require.Len(t, audit.entries, 1)
	require.Equal(t, 1, notify.sendCreatedCount, "publish-on-create must fire exactly once on commit")
	require.Equal(t, int64(123), notify.lastCreatedEvent.VendorID, "event must carry the persisted ID")
	require.Equal(t, "V-001", notify.lastCreatedEvent.VendorCode)
	require.Equal(t, "Acme", notify.lastCreatedEvent.CompanyName)
	// Handler stamps event time via time.Now(); we just verify it lies
	// within the window of the Execute call rather than pinning a fixed
	// value (the clock is no longer injectable).
	require.WithinRange(t, notify.lastCreatedEvent.OccurredAtTime, start, end)

	entry := audit.entries[0]
	require.Equal(t, int64(123), entry.VendorID)
	require.Equal(t, "CREATE", entry.Action)
	require.WithinRange(t, entry.OccurredAt, start, end)
	require.Equal(t, "V-001", entry.Payload["vendor_code"])

	require.NotNil(t, repo.savedVendor)
	require.Equal(t, int64(123), repo.savedVendor.ID())
}

func TestCreateVendor_PublishFailureDoesNotRollBack(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{saveID: 123, findByCodeErr: pkgerr.NewNotFound("Vendor", "V-001")}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{sendCreatedErr: stderrors.New("pubsub down")}

	h := newHandler(repo, tx, lock, audit, notify, now)

	resp, err := h.Execute(context.Background(), validCommand())
	require.NoError(t, err, "publish failure must NOT surface to the caller — the commit already succeeded")
	require.Equal(t, int64(123), resp.ID)
	require.Equal(t, 1, repo.saveCalled, "vendor must remain saved despite publish failure")
	require.Equal(t, 1, tx.executed)
	require.Len(t, audit.entries, 1, "audit must remain written despite publish failure")
	require.Equal(t, 1, notify.sendCreatedCount, "publish was still attempted exactly once")
}

func TestCreateVendor_LockContention(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: false}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, now)

	_, err := h.Execute(context.Background(), validCommand())
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrInvalidArgument)
	require.Equal(t, 0, repo.saveCalled)
	require.Equal(t, 0, tx.executed)
	require.Empty(t, audit.entries)
	require.Equal(t, 0, notify.sendCreatedCount, "must never notify when the tx never ran")
	require.Equal(t, 1, lock.acquireCalled)
}

func TestCreateVendor_DuplicateCode(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	existing, err := entity.NewVendor("V-001", "Acme", "NS-1", "", "", now)
	require.NoError(t, err)

	repo := &fakeRepo{findByCodeRes: existing}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, now)

	_, execErr := h.Execute(context.Background(), validCommand())
	require.Error(t, execErr)
	require.ErrorIs(t, execErr, pkgerr.ErrInvalidArgument)
	require.Equal(t, 0, repo.saveCalled)
	require.Equal(t, 0, tx.executed)
	require.Empty(t, audit.entries)
	require.Equal(t, 0, notify.sendCreatedCount, "must never notify on duplicate code")
}

func TestCreateVendor_DomainValidationFailure(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{findByCodeErr: pkgerr.NewNotFound("Vendor", "")}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, now)

	cmd := validCommand()
	cmd.VendorCode = ""

	_, err := h.Execute(context.Background(), cmd)
	require.Error(t, err)
	var ve *entity.ValidationError
	require.True(t, stderrors.As(err, &ve), "expected *entity.ValidationError, got %T", err)
	require.Equal(t, "vendorCode", ve.Field)
	require.Equal(t, 0, repo.saveCalled)
	require.Equal(t, 0, tx.executed)
	require.Empty(t, audit.entries)
	require.Equal(t, 0, notify.sendCreatedCount, "must never notify on validation failure")
}

func TestCreateVendor_SaveFailureRollsBack(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		findByCodeErr: pkgerr.NewNotFound("Vendor", "V-001"),
		saveErr:       stderrors.New("db down"),
	}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, now)

	_, err := h.Execute(context.Background(), validCommand())
	require.Error(t, err)
	require.Equal(t, 1, tx.executed)
	require.Equal(t, 1, repo.saveCalled)
	require.Empty(t, audit.entries, "audit must not be called when save fails")
	require.Equal(t, 0, notify.sendCreatedCount, "must never notify when save rolls back")
}

func TestCreateVendor_AuditFailureRollsBack(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		findByCodeErr: pkgerr.NewNotFound("Vendor", "V-001"),
		saveID:        99,
	}
	tx := &fakeTxManager{}
	lock := &fakeLock{acquired: true}
	audit := &fakeAuditLog{err: stderrors.New("mongo down")}
	notify := &fakeNotifier{}

	h := newHandler(repo, tx, lock, audit, notify, now)

	_, err := h.Execute(context.Background(), validCommand())
	require.Error(t, err)
	require.Equal(t, 1, tx.executed)
	require.Equal(t, 1, repo.saveCalled)
	require.Len(t, audit.entries, 1, "audit was attempted")
	require.Equal(t, 0, notify.sendCreatedCount, "must never notify when audit rolls back")
}
