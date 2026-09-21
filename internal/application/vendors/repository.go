package vendors

import (
	"context"

	"github.com/astronautsid/astro-boilerplate/internal/domain/entity"
)

// Repository persists and loads Vendor aggregates. Implementations live in
// internal/infrastructure/persistence/vendor and hydrate entities via
// entity.Reconstitute.
//
// FindBy* methods MUST return (nil, pkg/errors.NewNotFound(...)) when the
// requested row does not exist. Save MUST return the newly assigned database
// ID for the inserted row.
type Repository interface {
	Save(ctx context.Context, tx Tx, vendor *entity.Vendor) (int64, error)
	Update(ctx context.Context, tx Tx, vendor *entity.Vendor) error
	FindByID(ctx context.Context, id int64) (*entity.Vendor, error)
	FindByCode(ctx context.Context, code string) (*entity.Vendor, error)
}
