package postgres

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// Registrar is the GORM representation of a Registrar
type Registrar struct {
	ClID        string `gorm:"primary_key"`
	Name        string `gorm:"unique;not null"`
	NickName    string `gorm:"unique;not null"`
	GurID       int
	Email       string
	Status      string `gorm:"not null"`
	Street1Int  string
	Street2Int  string
	Street3Int  string
	CityInt     string
	SPInt       string `gorm:"column:sp_int"`
	PCInt       string `gorm:"column:pc_int"`
	CCInt       string `gorm:"column:cc_int"`
	Street1Loc  string
	Street2Loc  string
	Street3Loc  string
	CityLoc     string
	SPLoc       string `gorm:"column:sp_loc"`
	PCLoc       string `gorm:"column:pc_loc"`
	CCLoc       string `gorm:"column:cc_loc"`
	Voice       string
	Fax         string
	URL         string
	Whois43     string
	Whois80     string
	RdapBaseUrl string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// FK relationships with contacts
	Contacts        []*Contact `gorm:"foreignKey:ClID"`
	ContactsCreated []*Contact `gorm:"foreignKey:CrRr"`
	ContactsUpdated []*Contact `gorm:"foreignKey:UpRr"`

	// FK relationships with hosts
	Hosts        []*Host `gorm:"foreignKey:ClID"`
	HostsCreated []*Host `gorm:"foreignKey:CrRr"`
	HostsUpdated []*Host `gorm:"foreignKey:UpRr"`
}

func (Registrar) TableName() string {
	return "registrars"
}

// GormRegistrarRepository implements the RegistrarRepository interface
type GormRegistrarRepository struct {
	db *gorm.DB
}

// NewGormRegistrarRepository returns a new GormRegistrarRepository
func NewGormRegistrarRepository(db *gorm.DB) *GormRegistrarRepository {
	return &GormRegistrarRepository{
		db: db,
	}
}

// GetByClID looks up a Regsitrar by ite ClID and returns it
func (r *GormRegistrarRepository) GetByClID(ctx context.Context, clid string) (*entities.Registrar, error) {
	dbRar := &Registrar{}

	err := r.db.WithContext(ctx).Where("cl_id = ?", clid).First(dbRar).Error
	if err != nil {
		return nil, err
	}

	rar := FromDBRegistrar(dbRar)

	return rar, nil
}

// Create Creates a new Registrar in the repository
func (r *GormRegistrarRepository) Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// Map
	dbRar := ToDBRegistrar(rar)

	err := r.db.WithContext(ctx).Create(dbRar).Error
	if err != nil {
		return nil, err
	}
	// Read the data from the repo to ensure we return the same data that was written
	soredDbRar, err := r.GetByClID(ctx, rar.ClID.String())
	if err != nil {
		return nil, err
	}

	return soredDbRar, nil
}

// Update Updates a registrar in the repository
func (r *GormRegistrarRepository) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// map
	dbRar := ToDBRegistrar(rar)

	err := r.db.WithContext(ctx).Save(dbRar).Error
	if err != nil {
		return nil, err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDbRar, err := r.GetByClID(ctx, rar.ClID.String())
	if err != nil {
		return nil, err
	}

	return storedDbRar, nil
}

// Delete Deletes a registrar from the repository
func (r *GormRegistrarRepository) Delete(ctx context.Context, clid string) error {
	return r.db.WithContext(ctx).Where("cl_id = ?", clid).Delete(&Registrar{}).Error
}

// List returns a list of registrars
func (r *GormRegistrarRepository) List(ctx context.Context, pagesize int, cursor string) ([]*entities.Registrar, error) {
	dbRars := []*Registrar{}

	err := r.db.WithContext(ctx).Order("cl_id ASC").Limit(pagesize).Find(&dbRars, "cl_id > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	rars := make([]*entities.Registrar, len(dbRars))
	for i, dbRar := range dbRars {
		rars[i] = FromDBRegistrar(dbRar)
	}

	return rars, nil
}
