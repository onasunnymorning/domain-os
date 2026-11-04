package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LaunchApplicationRecord is one launch application
// (entities.LaunchApplication, ADR 0024 §3). ID is the number of its
// applicationID; the installation's repository ID is appended on read, as
// for a ROID (ADR 0017). A registrar holds at most one open application per
// name and phase: uq_launch_applications_open, a partial unique index over
// the non-final statuses (connection.go), enforces it for every writer.
type LaunchApplicationRecord struct {
	ID        int64  `gorm:"primaryKey"`
	Name      string `gorm:"not null;index"`
	TLDName   string `gorm:"not null;index:idx_launch_applications_phase,priority:1"`
	PhaseName string `gorm:"size:16;not null;index:idx_launch_applications_phase,priority:2"`
	PhaseMode string `gorm:"size:16;not null"`
	ClID      string `gorm:"not null"`
	Status    string `gorm:"size:24;not null;index"`

	RegistrantID *string
	AdminID      *string
	TechID       *string
	BillingID    *string
	Years        int `gorm:"not null"`
	// AuthInfo is the code's salted hash (RFC 9154), NULL when none is set.
	AuthInfo *string
	// The quote at filing, billed at allocation (entities.ApplicationQuote).
	QuoteCurrency string `gorm:"size:3;not null"`
	QuoteAmount   int64  `gorm:"not null"`
	QuoteClass    string
	Reason        string
	// DomainRoID is the number of the domain's ROID once allocated.
	DomainRoID *int64
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`

	// Hosts and DSRecords go with the application: written with it, deleted
	// with it.
	Hosts     []LaunchApplicationHost     `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE"`
	DSRecords []LaunchApplicationDSRecord `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table name.
func (LaunchApplicationRecord) TableName() string { return "launch_applications" }

// LaunchApplicationHost is one of an application's nameservers, by name:
// a host object of the applicant, looked up again at allocation.
type LaunchApplicationHost struct {
	ID            int64  `gorm:"primaryKey"`
	ApplicationID int64  `gorm:"not null;uniqueIndex:idx_launch_application_hosts_unique,priority:1"`
	Name          string `gorm:"not null;uniqueIndex:idx_launch_application_hosts_unique,priority:2"`
}

// TableName pins the table name.
func (LaunchApplicationHost) TableName() string { return "launch_application_hosts" }

// LaunchApplicationDSRecord is one of an application's DS records.
type LaunchApplicationDSRecord struct {
	ID            int64  `gorm:"primaryKey"`
	ApplicationID int64  `gorm:"not null;uniqueIndex:idx_launch_application_ds_unique,priority:1"`
	KeyTag        int    `gorm:"not null;uniqueIndex:idx_launch_application_ds_unique,priority:2"`
	Algorithm     int    `gorm:"not null;uniqueIndex:idx_launch_application_ds_unique,priority:3"`
	DigestType    int    `gorm:"not null;uniqueIndex:idx_launch_application_ds_unique,priority:4"`
	Digest        string `gorm:"not null;uniqueIndex:idx_launch_application_ds_unique,priority:5"`
}

// TableName pins the table name.
func (LaunchApplicationDSRecord) TableName() string { return "launch_application_ds_records" }

// launchApplicationOpenIndex is the partial unique index behind
// entities.ErrApplicationExists: one open application per registrar, name
// and phase. The statuses listed are the non-final ones
// (entities.ApplicationStatus.IsFinal).
const launchApplicationOpenIndex = "CREATE UNIQUE INDEX IF NOT EXISTS uq_launch_applications_open ON launch_applications (name, phase_name, cl_id) WHERE status IN ('pendingValidation', 'pendingAllocation')"

// LaunchApplicationRepository is the Postgres
// repositories.LaunchApplicationRepository.
type LaunchApplicationRepository struct {
	db         *gorm.DB
	repository entities.RepositoryID
}

// NewLaunchApplicationRepository returns the repository. repository is the
// installation's ROID repository ID (ADR 0017), which application IDs
// carry too.
func NewLaunchApplicationRepository(db *gorm.DB, repository entities.RepositoryID) *LaunchApplicationRepository {
	return &LaunchApplicationRepository{db: db, repository: repository}
}

// CreateApplication implements repositories.LaunchApplicationRepository.
func (r *LaunchApplicationRepository) CreateApplication(ctx context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error) {
	rec, err := toApplicationRecord(a)
	if err != nil {
		return nil, err
	}
	stored := &LaunchApplicationRecord{}
	err = dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rec).Error; err != nil {
			if isUniqueViolation(err) {
				return entities.ErrApplicationExists
			}
			return err
		}
		return tx.Preload("Hosts").Preload("DSRecords", orderApplicationDS).First(stored, rec.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return r.fromApplicationRecord(stored), nil
}

// GetApplicationByID implements repositories.LaunchApplicationRepository.
func (r *LaunchApplicationRepository) GetApplicationByID(ctx context.Context, id entities.RoidType) (*entities.LaunchApplication, error) {
	n, err := id.Int64()
	if err != nil || id.ObjectIdentifier() != entities.APPLICATION_ROID_ID || id.SystemIdentifier() != r.repository.String() {
		return nil, entities.ErrApplicationNotFound
	}
	var rec LaunchApplicationRecord
	err = dbFrom(ctx, r.db).Preload("Hosts").Preload("DSRecords", orderApplicationDS).First(&rec, n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entities.ErrApplicationNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.fromApplicationRecord(&rec), nil
}

// ListApplicationsByName implements repositories.LaunchApplicationRepository.
func (r *LaunchApplicationRepository) ListApplicationsByName(ctx context.Context, name string) ([]*entities.LaunchApplication, error) {
	var recs []LaunchApplicationRecord
	if err := dbFrom(ctx, r.db).Preload("Hosts").Preload("DSRecords", orderApplicationDS).
		Where("name = ?", name).Order("created_at, id").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.LaunchApplication, len(recs))
	for i := range recs {
		out[i] = r.fromApplicationRecord(&recs[i])
	}
	return out, nil
}

// CountApplicationsByPhase implements repositories.LaunchApplicationRepository.
func (r *LaunchApplicationRepository) CountApplicationsByPhase(ctx context.Context, tld, phase string) (int64, error) {
	var n int64
	err := dbFrom(ctx, r.db).Model(&LaunchApplicationRecord{}).Where("tld_name = ? AND phase_name = ?", tld, phase).Count(&n).Error
	return n, err
}

// UpdateApplicationIfUnchanged implements
// repositories.LaunchApplicationRepository. The row is written only if it
// is still at the version a was read with; its nameservers and DS records
// are replaced with it.
func (r *LaunchApplicationRepository) UpdateApplicationIfUnchanged(ctx context.Context, a *entities.LaunchApplication) (*entities.LaunchApplication, error) {
	rec, err := toApplicationRecord(a)
	if err != nil {
		return nil, err
	}
	read := a.UpdatedAt
	rec.UpdatedAt = time.Now().UTC()
	stored := &LaunchApplicationRecord{}
	err = dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := written(updateIfAt(tx.Model(&LaunchApplicationRecord{}).Where("id = ?", rec.ID), read).
			Select("status", "registrant_id", "admin_id", "tech_id", "billing_id", "years", "auth_info",
				"quote_currency", "quote_amount", "quote_class", "reason", "domain_ro_id", "updated_at").
			Updates(rec)); err != nil {
			return err
		}
		if err := tx.Where("application_id = ?", rec.ID).Delete(&LaunchApplicationHost{}).Error; err != nil {
			return err
		}
		if err := tx.Where("application_id = ?", rec.ID).Delete(&LaunchApplicationDSRecord{}).Error; err != nil {
			return err
		}
		if len(rec.Hosts) > 0 {
			if err := tx.Create(&rec.Hosts).Error; err != nil {
				return err
			}
		}
		if len(rec.DSRecords) > 0 {
			if err := tx.Create(&rec.DSRecords).Error; err != nil {
				return err
			}
		}
		return tx.Preload("Hosts").Preload("DSRecords", orderApplicationDS).First(stored, rec.ID).Error
	})
	if err != nil {
		return nil, r.notWritten(ctx, rec.ID, err)
	}
	return r.fromApplicationRecord(stored), nil
}

// DeleteApplicationIfUnchanged implements
// repositories.LaunchApplicationRepository. Its nameservers and DS records
// go with it (ON DELETE CASCADE).
func (r *LaunchApplicationRepository) DeleteApplicationIfUnchanged(ctx context.Context, a *entities.LaunchApplication) error {
	id, err := a.ID.Int64()
	if err != nil {
		return entities.ErrApplicationNotFound
	}
	err = written(updateIfAt(dbFrom(ctx, r.db).Where("id = ?", id), a.UpdatedAt).Delete(&LaunchApplicationRecord{}))
	if err != nil {
		return r.notWritten(ctx, id, err)
	}
	return nil
}

// notWritten turns a conditional write's errNotWritten into
// ErrApplicationChanged, or ErrApplicationNotFound if the application is
// gone.
func (r *LaunchApplicationRepository) notWritten(ctx context.Context, id int64, err error) error {
	if !errors.Is(err, errNotWritten) {
		return err
	}
	var n int64
	if cerr := dbFrom(ctx, r.db).Model(&LaunchApplicationRecord{}).Where("id = ?", id).Count(&n).Error; cerr != nil {
		return cerr
	}
	if n == 0 {
		return entities.ErrApplicationNotFound
	}
	return entities.ErrApplicationChanged
}

// orderApplicationDS orders an application's DS records as they were
// given, so a read returns them in a stable order.
func orderApplicationDS(db *gorm.DB) *gorm.DB { return db.Order("id") }

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func toApplicationRecord(a *entities.LaunchApplication) (*LaunchApplicationRecord, error) {
	id, err := a.ID.Int64()
	if err != nil {
		return nil, err
	}
	rec := &LaunchApplicationRecord{
		ID: id, Name: a.Name.String(), TLDName: a.TLDName.String(), PhaseName: a.PhaseName.String(), PhaseMode: string(a.PhaseMode),
		ClID: a.ClID.String(), Status: string(a.Status),
		RegistrantID: optionalString(a.RegistrantID.String()), AdminID: optionalString(a.AdminID.String()),
		TechID: optionalString(a.TechID.String()), BillingID: optionalString(a.BillingID.String()),
		Years: a.Years, AuthInfo: optionalString(string(a.AuthInfo)),
		QuoteCurrency: a.Quote.Currency, QuoteAmount: a.Quote.Amount, QuoteClass: a.Quote.Class,
		Reason: a.Reason, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	if a.DomainRoID != "" {
		n, err := a.DomainRoID.Int64()
		if err != nil {
			return nil, err
		}
		rec.DomainRoID = &n
	}
	for _, h := range a.HostNames {
		rec.Hosts = append(rec.Hosts, LaunchApplicationHost{ApplicationID: id, Name: h})
	}
	for _, ds := range a.DSRecords {
		rec.DSRecords = append(rec.DSRecords, LaunchApplicationDSRecord{
			ApplicationID: id, KeyTag: int(ds.KeyTag), Algorithm: int(ds.Algorithm), DigestType: int(ds.DigestType), Digest: ds.Digest,
		})
	}
	return rec, nil
}

func (r *LaunchApplicationRepository) fromApplicationRecord(rec *LaunchApplicationRecord) *entities.LaunchApplication {
	id, _ := entities.NewRoidType(rec.ID, entities.RoidTypeApplication, r.repository)
	a := &entities.LaunchApplication{
		ID: id, Name: entities.DomainName(rec.Name), TLDName: entities.DomainName(rec.TLDName), PhaseName: entities.ClIDType(rec.PhaseName),
		PhaseMode: entities.LaunchMode(rec.PhaseMode), ClID: entities.ClIDType(rec.ClID), Status: entities.ApplicationStatus(rec.Status),
		RegistrantID: entities.ClIDType(stringOf(rec.RegistrantID)), AdminID: entities.ClIDType(stringOf(rec.AdminID)),
		TechID: entities.ClIDType(stringOf(rec.TechID)), BillingID: entities.ClIDType(stringOf(rec.BillingID)),
		Years: rec.Years, AuthInfo: entities.AuthInfoHash(stringOf(rec.AuthInfo)),
		Quote:  entities.ApplicationQuote{Currency: rec.QuoteCurrency, Amount: rec.QuoteAmount, Class: rec.QuoteClass},
		Reason: rec.Reason, CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
	}
	if rec.DomainRoID != nil {
		a.DomainRoID, _ = entities.NewRoidType(*rec.DomainRoID, entities.RoidTypeDomain, r.repository)
	}
	for _, h := range rec.Hosts {
		a.HostNames = append(a.HostNames, h.Name)
	}
	for _, ds := range rec.DSRecords {
		a.DSRecords = append(a.DSRecords, entities.DSRecord{
			KeyTag: uint16(ds.KeyTag), Algorithm: uint8(ds.Algorithm), DigestType: uint8(ds.DigestType), Digest: ds.Digest, // #nosec G115 -- written from those types
		})
	}
	return a
}
