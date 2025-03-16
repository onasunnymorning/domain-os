package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// NNDN is the GORM representation of an NNDN object for database interaction.
type NNDN struct {
	Name      string `gorm:"primaryKey"` // ASCII Name as primary key
	UName     string // Unicode Name, should only be populated if the blocked string is an IDN
	TLDName   string `gorm:"not null;foreignKey"` // TLD Name as a foreign key
	TLD       TLD
	NameState string `gorm:"not null"` // State of the NNDN, not null
	Reason    string // Reason for the NNDN being blocked
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GormNNDNRepository implements the Repo interface
type GormNNDNRepository struct {
	db *gorm.DB
}

// NewGormNNDNRepository returns a new GormNNDNRepository
func NewGormNNDNRepository(db *gorm.DB) *GormNNDNRepository {
	return &GormNNDNRepository{
		db: db,
	}
}

// TableName specifies the table Name for NNDN.
func (NNDN) TableName() string {
	return "nndns"
}

// toNNDN converts a NNDN to a domain model *entities.NNDN.
func (n *NNDN) toNNDN() *entities.NNDN {
	return &entities.NNDN{
		Name:      entities.DomainName(n.Name),
		UName:     entities.DomainName(n.UName),
		TLDName:   entities.DomainName(n.TLDName),
		NameState: entities.NNDNState(n.NameState),
		Reason:    entities.ClIDType(n.Reason),
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

// fromNNDN converts a domain model NNDN to a NNDN.
func fromNNDN(n *entities.NNDN) *NNDN {
	return &NNDN{
		Name:      n.Name.String(),
		UName:     n.UName.String(),
		TLDName:   n.TLDName.String(),
		NameState: string(n.NameState),
		Reason:    string(n.Reason),
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

func (r *GormNNDNRepository) CreateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error) {
	gormNNDN := fromNNDN(nndn)
	result := r.db.WithContext(ctx).Create(gormNNDN)
	if err := result.Error; err != nil {
		var perr *pgconn.PgError
		if errors.As(err, &perr) && perr.Code == "23505" {
			return nil, entities.ErrDuplicateNNDN
		}
		return nil, result.Error
	}
	return gormNNDN.toNNDN(), nil
}

func (r *GormNNDNRepository) GetNNDN(ctx context.Context, name string) (*entities.NNDN, error) {
	var gormNNDN NNDN
	result := r.db.WithContext(ctx).Where("Name = ?", name).First(&gormNNDN)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, entities.ErrNNDNNotFound
		}
		return nil, result.Error
	}
	return gormNNDN.toNNDN(), nil
}

func (r *GormNNDNRepository) UpdateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error) {
	gormNNDN := fromNNDN(nndn)
	err := r.db.WithContext(ctx).Save(gormNNDN).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrTLDNotFound
		}
		return nil, err
	}
	return gormNNDN.toNNDN(), nil
}

func (r *GormNNDNRepository) DeleteNNDN(ctx context.Context, name string) error {
	result := r.db.WithContext(ctx).Where("Name = ?", name).Delete(&NNDN{})
	return result.Error
}

func (r *GormNNDNRepository) ListNNDNs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.NNDN, string, error) {
	// Get a query object ordering by name (PK used for cursor pagination)
	dbQuery := r.db.WithContext(ctx).Order("Name ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		dbQuery = dbQuery.Where("Name > ?", params.PageCursor)
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the limit to determine if there are more results

	// Execute the query
	var gormNNDNs []*NNDN
	err := dbQuery.Find(&gormNNDNs).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there are more results
	hasMore := len(gormNNDNs) == params.PageSize+1
	if hasMore {
		// Return only up to the limit
		gormNNDNs = gormNNDNs[:params.PageSize]
	}

	// Map the GormNNDNs to NNDNs}
	nndns := make([]*entities.NNDN, len(gormNNDNs))
	for i, gNNDN := range gormNNDNs {
		nndns[i] = gNNDN.toNNDN()
	}

	// Set the cursor to the last name in the list
	var newCursor string
	if hasMore {
		newCursor = nndns[len(nndns)-1].Name.String()
	}

	return nndns, newCursor, nil
}
