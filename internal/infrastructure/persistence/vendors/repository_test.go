package vendors

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
	"github.com/astronautsid/astro-boilerplate/internal/domain/valueobject"
	pkgerr "github.com/astronautsid/astro-boilerplate/pkg/errors"
	"github.com/astronautsid/astro-golibs/safesql"
	"github.com/stretchr/testify/require"
)

// newMockDB returns a *sql.DB backed by go-sqlmock and adapts it into both
// safesql interface flavors so tests can exercise the real SQL strings
// without a live database. The cleanup hook closes the underlying connection.
func newMockDB(t *testing.T) (safesql.MasterDB, safesql.ReplicaDB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return safesql.NewMasterDB(db, "postgres"), safesql.NewReplicaDB(db, "postgres"), mock
}

func makeVendor(t *testing.T) *entity.Vendor {
	t.Helper()
	v, err := entity.NewVendor("V-001", "Acme Co", "NS-1", "ops@acme.test", "+62-800-0000", time.Unix(1710000000, 0).UTC())
	require.NoError(t, err)
	return v
}

func TestRepository_Save_Success(t *testing.T) {
	t.Parallel()
	master, _, mock := newMockDB(t)
	repo := &Repository{writeDB: master}

	mock.ExpectQuery("INSERT INTO vendors").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	id, err := repo.Save(context.Background(), nil, makeVendor(t))
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Save_UniqueViolation(t *testing.T) {
	t.Parallel()
	master, _, mock := newMockDB(t)
	repo := &Repository{writeDB: master}

	// go-sqlmock can't construct a real pq.Error, but mapWriteError falls back
	// to a textual sniff for the word "unique" so this exercises the same path
	// observable to callers (typed pkgerr.InvalidArgumentError out).
	mock.ExpectQuery("INSERT INTO vendors").
		WillReturnError(errors.New("pq: duplicate key value violates unique constraint \"vendors_vendor_code_key\""))

	_, err := repo.Save(context.Background(), nil, makeVendor(t))
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrInvalidArgument)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindByID_Found(t *testing.T) {
	t.Parallel()
	_, replica, mock := newMockDB(t)
	repo := &Repository{readDB: replica}

	now := time.Unix(1710000000, 0).UTC()
	cols := []string{
		"id", "vendor_code", "company_name", "netsuite_id",
		"contact_email", "contact_phone", "status", "created_at", "updated_at",
	}
	mock.ExpectQuery("SELECT .+ FROM vendors WHERE id").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			7, "V-001", "Acme Co", "NS-1", "ops@acme.test", "+62-800-0000",
			string(valueobject.VendorStatusPending), now, now,
		))

	v, err := repo.FindByID(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, int64(7), v.ID())
	require.Equal(t, "V-001", v.VendorCode())
	require.Equal(t, valueobject.VendorStatusPending, v.Status())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	_, replica, mock := newMockDB(t)
	repo := &Repository{readDB: replica}

	mock.ExpectQuery("SELECT .+ FROM vendors WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	v, err := repo.FindByID(context.Background(), 999)
	require.Nil(t, v)
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindByCode_Found(t *testing.T) {
	t.Parallel()
	_, replica, mock := newMockDB(t)
	repo := &Repository{readDB: replica}

	now := time.Unix(1710000000, 0).UTC()
	cols := []string{
		"id", "vendor_code", "company_name", "netsuite_id",
		"contact_email", "contact_phone", "status", "created_at", "updated_at",
	}
	mock.ExpectQuery("SELECT .+ FROM vendors WHERE vendor_code").
		WithArgs("V-001").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			1, "V-001", "Acme Co", "NS-1", "ops@acme.test", "+62-800-0000",
			string(valueobject.VendorStatusActive), now, now,
		))

	v, err := repo.FindByCode(context.Background(), "V-001")
	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, "V-001", v.VendorCode())
	require.Equal(t, valueobject.VendorStatusActive, v.Status())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindByCode_NotFound(t *testing.T) {
	t.Parallel()
	_, replica, mock := newMockDB(t)
	repo := &Repository{readDB: replica}

	mock.ExpectQuery("SELECT .+ FROM vendors WHERE vendor_code").
		WithArgs("V-missing").
		WillReturnError(sql.ErrNoRows)

	v, err := repo.FindByCode(context.Background(), "V-missing")
	require.Nil(t, v)
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerr.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
