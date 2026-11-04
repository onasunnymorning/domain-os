package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TMCH data (ADR 0024 §4, roadmap 9a): four typed tables (ADR 0018). One
// row per set in tmch_sets says what is stored and how fresh it is; the
// entries of each set are in their own table, keyed by what the EPP path
// will look up (the SMD ID, the label). A set is replaced whole in one
// transaction, with its outbox events (INV-01), under an advisory lock, and
// only after re-reading the stored header under that lock: the decision
// that the fetched set supersedes the stored one is made where it cannot
// race another refresh.

// TMCHSetRecord is one data set's header: tmch_sets.
type TMCHSetRecord struct {
	Name        string `gorm:"primaryKey;size:16"`
	Version     int
	GeneratedAt time.Time `gorm:"not null"`
	FetchedAt   time.Time `gorm:"not null"`
	Source      string    `gorm:"size:512"`
	Entries     int
	Rejected    int
	ExpiresAt   *time.Time
	Fingerprint string `gorm:"size:64"`
}

func (TMCHSetRecord) TableName() string { return "tmch_sets" }

// TMCHCertificateRecord is one certificate of the CA bundle.
type TMCHCertificateRecord struct {
	SerialNumber string `gorm:"primaryKey;size:128"`
	Subject      string `gorm:"size:512"`
	NotBefore    time.Time
	NotAfter     time.Time
	PEM          string `gorm:"type:text"`
}

func (TMCHCertificateRecord) TableName() string { return "tmch_certificates" }

// TMCHSMDRevocationRecord is one revoked signed mark.
type TMCHSMDRevocationRecord struct {
	SMDID     string `gorm:"primaryKey;size:64;column:smd_id"`
	RevokedAt time.Time
}

func (TMCHSMDRevocationRecord) TableName() string { return "tmch_smd_revocations" }

// TMCHDNLLabelRecord is one label of the DNL list.
type TMCHDNLLabelRecord struct {
	Label      string `gorm:"primaryKey;size:63"`
	LookupKey  string `gorm:"type:text;not null"`
	InsertedAt time.Time
}

func (TMCHDNLLabelRecord) TableName() string { return "tmch_dnl_labels" }

// tmchBatch is how many entries one INSERT carries; the DNL list is tens of
// thousands of rows.
const tmchBatch = 1000

// TMCHRepository is the Postgres repositories.TMCHRepository (the
// repositories package imports this one, so the check is in the services
// that take the port).
type TMCHRepository struct{ db *gorm.DB }

// NewTMCHRepository returns a repository on db.
func NewTMCHRepository(db *gorm.DB) *TMCHRepository { return &TMCHRepository{db: db} }

// replace runs fn in a transaction that holds the set's advisory lock, then
// writes the set header and the events in the same transaction. Under the
// lock it re-reads the stored header: a fetched set that does not supersede
// it (entities.TMCHSetStatus.Supersedes) only has its fetch time recorded,
// and entities.ErrTMCHListStale is returned with nothing else written.
func (r *TMCHRepository) replace(ctx context.Context, set TMCHSetRecord, events []entities.DomainEvent, fn func(tx *gorm.DB) error) error {
	// stale is decided inside the transaction and reported after it: the
	// fetch time it records has to commit, which an error returned from the
	// callback would roll back.
	stale := false
	err := dbFrom(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "tmch:"+set.Name).Error; err != nil {
				return fmt.Errorf("lock tmch %s: %w", set.Name, err)
			}
		}
		var stored TMCHSetRecord
		switch err := tx.Where("name = ?", set.Name).First(&stored).Error; {
		case errors.Is(err, gorm.ErrRecordNotFound):
		case err != nil:
			return fmt.Errorf("read tmch set %s: %w", set.Name, err)
		case !set.toStatus().Supersedes(stored.toStatus()):
			stale = true
			if err := tx.Model(&TMCHSetRecord{}).Where("name = ?", set.Name).Update("fetched_at", set.FetchedAt).Error; err != nil {
				return fmt.Errorf("mark tmch %s fetched: %w", set.Name, err)
			}
			return nil
		}
		if err := fn(tx); err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, UpdateAll: true}).Create(&set).Error; err != nil {
			return fmt.Errorf("write tmch set %s: %w", set.Name, err)
		}
		if len(events) == 0 {
			return nil
		}
		records := make([]DomainEventRecord, len(events))
		for i, e := range events {
			rec, err := ToDBDomainEvent(e)
			if err != nil {
				return err
			}
			records[i] = rec
		}
		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("write tmch %s events: %w", set.Name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if stale {
		return fmt.Errorf("%w: %s generated %s", entities.ErrTMCHListStale, set.Name, set.GeneratedAt.Format(time.RFC3339))
	}
	return nil
}

// deleteAll empties a set's table.
func deleteAll(tx *gorm.DB, model interface{}) error {
	return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error
}

func tmchSetRecord(s entities.TMCHSetStatus, version int) TMCHSetRecord {
	return TMCHSetRecord{Name: s.Set, Version: version, GeneratedAt: s.GeneratedAt, FetchedAt: s.FetchedAt, Source: s.Source, Entries: s.Entries, Rejected: s.Rejected, ExpiresAt: s.ExpiresAt, Fingerprint: s.Fingerprint}
}

func (r TMCHSetRecord) toStatus() *entities.TMCHSetStatus {
	return &entities.TMCHSetStatus{Set: r.Name, GeneratedAt: r.GeneratedAt.UTC(), FetchedAt: r.FetchedAt.UTC(), Source: r.Source, Entries: r.Entries, Rejected: r.Rejected, ExpiresAt: utcPtr(r.ExpiresAt), Fingerprint: r.Fingerprint}
}

// MarkFetched records a fetch that brought nothing new.
func (r *TMCHRepository) MarkFetched(ctx context.Context, set string, fetchedAt time.Time) error {
	if err := dbFrom(ctx, r.db).Model(&TMCHSetRecord{}).Where("name = ?", set).Update("fetched_at", fetchedAt.UTC()).Error; err != nil {
		return fmt.Errorf("mark tmch %s fetched: %w", set, err)
	}
	return nil
}

// ReplaceCertificates stores the bundle in place of the one stored.
func (r *TMCHRepository) ReplaceCertificates(ctx context.Context, bundle *entities.TMCHCertificates, events []entities.DomainEvent) error {
	records := make([]TMCHCertificateRecord, 0, len(bundle.Certificates))
	for _, c := range bundle.Certificates {
		records = append(records, TMCHCertificateRecord{SerialNumber: c.SerialNumber, Subject: c.Subject, NotBefore: c.NotBefore, NotAfter: c.NotAfter, PEM: c.PEM})
	}
	return r.replace(ctx, tmchSetRecord(bundle.Status(), 0), events, func(tx *gorm.DB) error {
		if err := deleteAll(tx, &TMCHCertificateRecord{}); err != nil {
			return fmt.Errorf("clear tmch certificates: %w", err)
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(&records, tmchBatch).Error
	})
}

// GetCertificates returns the stored bundle, entities.ErrTMCHDataMissing if
// none was fetched yet.
func (r *TMCHRepository) GetCertificates(ctx context.Context) (*entities.TMCHCertificates, error) {
	var set TMCHSetRecord
	if err := dbFrom(ctx, r.db).Where("name = ?", entities.TMCHSetCertificates).First(&set).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", entities.ErrTMCHDataMissing, entities.TMCHSetCertificates)
		}
		return nil, err
	}
	var records []TMCHCertificateRecord
	if err := dbFrom(ctx, r.db).Order("not_before").Find(&records).Error; err != nil {
		return nil, err
	}
	bundle := &entities.TMCHCertificates{FetchedAt: set.FetchedAt.UTC(), Source: set.Source}
	for _, rec := range records {
		bundle.Certificates = append(bundle.Certificates, entities.TMCHCertificate{SerialNumber: rec.SerialNumber, Subject: rec.Subject, NotBefore: rec.NotBefore.UTC(), NotAfter: rec.NotAfter.UTC(), PEM: rec.PEM})
	}
	return bundle, nil
}

// ReplaceSMDRL stores the revocation list in place of the one stored.
func (r *TMCHRepository) ReplaceSMDRL(ctx context.Context, list *entities.SMDRevocationList, events []entities.DomainEvent) error {
	records := make([]TMCHSMDRevocationRecord, 0, len(list.Entries))
	for _, e := range list.Entries {
		records = append(records, TMCHSMDRevocationRecord{SMDID: e.SMDID, RevokedAt: e.RevokedAt})
	}
	return r.replace(ctx, tmchSetRecord(list.Status(), list.Version), events, func(tx *gorm.DB) error {
		if err := deleteAll(tx, &TMCHSMDRevocationRecord{}); err != nil {
			return fmt.Errorf("clear tmch smdrl: %w", err)
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(&records, tmchBatch).Error
	})
}

// IsSMDRevoked reports whether the signed mark is on the stored list.
func (r *TMCHRepository) IsSMDRevoked(ctx context.Context, smdID string) (bool, error) {
	var n int64
	if err := dbFrom(ctx, r.db).Model(&TMCHSMDRevocationRecord{}).Where("smd_id = ?", strings.TrimSpace(smdID)).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ReplaceDNL stores the DNL list in place of the one stored.
func (r *TMCHRepository) ReplaceDNL(ctx context.Context, list *entities.DNLList, events []entities.DomainEvent) error {
	records := make([]TMCHDNLLabelRecord, 0, len(list.Entries))
	for _, e := range list.Entries {
		records = append(records, TMCHDNLLabelRecord{Label: e.Label, LookupKey: e.LookupKey, InsertedAt: e.InsertedAt})
	}
	return r.replace(ctx, tmchSetRecord(list.Status(), list.Version), events, func(tx *gorm.DB) error {
		if err := deleteAll(tx, &TMCHDNLLabelRecord{}); err != nil {
			return fmt.Errorf("clear tmch dnl: %w", err)
		}
		if len(records) == 0 {
			return nil
		}
		return tx.CreateInBatches(&records, tmchBatch).Error
	})
}

// LookupDNL returns the stored entry for a label, nil when not listed.
func (r *TMCHRepository) LookupDNL(ctx context.Context, label string) (*entities.DNLEntry, error) {
	var rec TMCHDNLLabelRecord
	err := dbFrom(ctx, r.db).Where("label = ?", strings.ToLower(strings.TrimSpace(label))).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entities.DNLEntry{Label: rec.Label, LookupKey: rec.LookupKey, InsertedAt: rec.InsertedAt.UTC()}, nil
}

// Status reports each set's freshness; a set never fetched is nil.
func (r *TMCHRepository) Status(ctx context.Context) (*entities.TMCHStatus, error) {
	var sets []TMCHSetRecord
	if err := dbFrom(ctx, r.db).Find(&sets).Error; err != nil {
		return nil, err
	}
	status := &entities.TMCHStatus{}
	for _, s := range sets {
		switch s.Name {
		case entities.TMCHSetCertificates:
			status.Certificates = s.toStatus()
		case entities.TMCHSetSMDRL:
			status.SMDRL = s.toStatus()
		case entities.TMCHSetDNL:
			status.DNL = s.toStatus()
		}
	}
	return status, nil
}
