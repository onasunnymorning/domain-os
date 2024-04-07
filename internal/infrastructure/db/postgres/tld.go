package postgres

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// TLD is a GORM struct representing a TLD in the database
type TLD struct {
	Name      string `gorm:"primary_key"`
	Type      string
	UName     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Phases    []Phase `gorm:"foreignKey:TLDName;references:Name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// ToDBTLD converts a TLD struct to a DBTLD struct
func ToDBTLD(tld *entities.TLD) *TLD {
	dbTLD := &TLD{
		Name:      tld.Name.String(),
		Type:      tld.Type.String(),
		UName:     tld.UName.String(),
		CreatedAt: tld.CreatedAt,
		UpdatedAt: tld.UpdatedAt,
	}

	for _, phase := range tld.Phases {
		dbPhase := &Phase{}
		dbPhase.FromEntity(&phase)
		dbTLD.Phases = append(dbTLD.Phases, *dbPhase)
	}
	return dbTLD
}

// FromDBTLD converts a DBTLD struct to a TLD struct
func FromDBTLD(dbtld *TLD) *entities.TLD {
	tld := &entities.TLD{
		Name:      entities.DomainName(dbtld.Name),
		Type:      entities.TLDType(dbtld.Type),
		UName:     entities.DomainName(dbtld.UName),
		CreatedAt: dbtld.CreatedAt.UTC(),
		UpdatedAt: dbtld.UpdatedAt.UTC(),
	}
	for _, dbphase := range dbtld.Phases {
		phase := dbphase.ToEntity()
		tld.Phases = append(tld.Phases, *phase)

	}
	return tld
}

// GormTLDRepository implements the TLDRepo interface
type GormTLDRepository struct {
	db *gorm.DB
}

// NewGormTLDRepo returns a new GormTLDRepo
func NewGormTLDRepo(db *gorm.DB) *GormTLDRepository {
	return &GormTLDRepository{
		db: db,
	}
}

// GetByName returns a TLD by name
func (repo *GormTLDRepository) GetByName(name string) (*entities.TLD, error) {
	dbtld := &TLD{}

	err := repo.db.Preload("Phases").Where("name = ?", name).First(dbtld).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrTLDNotFound
		}
		return nil, err
	}

	tld := FromDBTLD(dbtld)

	return tld, nil
}

// Create creates a new TLD in the database
func (repo *GormTLDRepository) Create(tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.Create(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(tld.Name.String())
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}

// List returns a list of all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (repo *GormTLDRepository) List(pageSize int, pageCursor string) ([]*entities.TLD, error) {
	dbtlds := []*TLD{}

	err := repo.db.Order("name ASC").Limit(pageSize).Find(&dbtlds, "name > ?", pageCursor).Error
	if err != nil {
		return nil, err
	}

	tlds := make([]*entities.TLD, len(dbtlds))
	for i, dbtld := range dbtlds {
		tlds[i] = FromDBTLD(dbtld)
	}

	return tlds, nil
}

// Delete deletes a TLD from the database
func (repo *GormTLDRepository) DeleteByName(name string) error {
	return repo.db.Where("name = ?", name).Delete(&TLD{}).Error
}

// Update updates a TLD in the database
func (repo *GormTLDRepository) Update(tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.Save(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(tld.Name.String())
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}
