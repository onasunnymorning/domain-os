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

// ---------------------------------------------------------------------------
// RDAP publication policies — ADR 0021, docs/rdap/PUBLICATION_REFERENCE.md.
// The same shape as the EPP policies (epp_policies.go): every write commits
// with its outbox rows, under one advisory lock.
// ---------------------------------------------------------------------------

// RDAPPublicationPolicyRecord is the GORM model for
// rdap_publication_policies. The field-access matrix is one JSON column: it
// is read and written whole, and validated by the entity.
type RDAPPublicationPolicyRecord struct {
	Name           string                                                              `gorm:"primaryKey;size:32"`
	Description    string                                                              ``
	Roles          map[entities.RDAPRole]map[entities.RDAPField]entities.RDAPFieldRule `gorm:"type:jsonb;serializer:json;not null"`
	ContactFormURI string                                                              ``
	UpdatedAt      time.Time                                                           `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy      string                                                              ``
}

// TableName pins the table name.
func (RDAPPublicationPolicyRecord) TableName() string { return "rdap_publication_policies" }

// RDAPPublicationAssignmentRecord is a TLD's assigned publication policy
// (rdap_publication_assignments); a TLD with none has the built-in for its
// type. Deleting the TLD deletes it; a policy assigned cannot be deleted.
type RDAPPublicationAssignmentRecord struct {
	TLDName string `gorm:"primaryKey"`
	Policy  string `gorm:"not null;size:32;index"`
}

// TableName pins the table name.
func (RDAPPublicationAssignmentRecord) TableName() string { return "rdap_publication_assignments" }

func toRDAPPublicationRecord(p *entities.RDAPPublicationPolicy) *RDAPPublicationPolicyRecord {
	return &RDAPPublicationPolicyRecord{
		Name: p.Name, Description: p.Description, Roles: p.Roles, ContactFormURI: p.ContactFormURI,
		UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy,
	}
}

func (r *RDAPPublicationPolicyRecord) toEntity() *entities.RDAPPublicationPolicy {
	roles := r.Roles
	if roles == nil {
		roles = map[entities.RDAPRole]map[entities.RDAPField]entities.RDAPFieldRule{}
	}
	return &entities.RDAPPublicationPolicy{
		Name: r.Name, Description: r.Description, Roles: roles, ContactFormURI: r.ContactFormURI,
		UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy,
	}
}

// RDAPPublicationPolicyRepository is the Postgres
// repositories.RDAPPublicationPolicyRepository.
type RDAPPublicationPolicyRepository struct{ db *gorm.DB }

// NewRDAPPublicationPolicyRepository returns the repository.
func NewRDAPPublicationPolicyRepository(db *gorm.DB) *RDAPPublicationPolicyRepository {
	return &RDAPPublicationPolicyRepository{db: db}
}

// rdapPolicyLock serialises every RDAP policy change, publication and
// access alike, as eppPolicyLock does EPP's.
const rdapPolicyLock int64 = 0x52444150 // "RDAP"

func (r *RDAPPublicationPolicyRepository) change(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", rdapPolicyLock).Error; err != nil {
				return fmt.Errorf("lock RDAP policies: %w", err)
			}
		}
		return fn(tx)
	})
}

func rdapPublicationIn(tx *gorm.DB, name string) (*entities.RDAPPublicationPolicy, error) {
	var rec RDAPPublicationPolicyRecord
	err := tx.Where("name = ?", name).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec.toEntity(), nil
}

// ListPolicies implements repositories.RDAPPublicationPolicyRepository.
func (r *RDAPPublicationPolicyRepository) ListPolicies(ctx context.Context) ([]*entities.RDAPPublicationPolicy, error) {
	var recs []RDAPPublicationPolicyRecord
	if err := r.db.WithContext(ctx).Order("name").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.RDAPPublicationPolicy, len(recs))
	for i := range recs {
		out[i] = recs[i].toEntity()
	}
	return out, nil
}

// GetPolicy implements repositories.RDAPPublicationPolicyRepository.
func (r *RDAPPublicationPolicyRepository) GetPolicy(ctx context.Context, name string) (*entities.RDAPPublicationPolicy, error) {
	p, err := rdapPublicationIn(r.db.WithContext(ctx), name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrRDAPPublicationPolicyNotFound, name)
	}
	return p, nil
}

// SavePolicy implements repositories.RDAPPublicationPolicyRepository.
func (r *RDAPPublicationPolicyRepository) SavePolicy(ctx context.Context, p *entities.RDAPPublicationPolicy, events func(*entities.RDAPPublicationPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := rdapPublicationIn(tx, p.Name)
		if err != nil {
			return err
		}
		if err := tx.Save(toRDAPPublicationRecord(p)).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// DeletePolicy implements repositories.RDAPPublicationPolicyRepository.
func (r *RDAPPublicationPolicyRepository) DeletePolicy(ctx context.Context, name string, events func(*entities.RDAPPublicationPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := rdapPublicationIn(tx, name)
		if err != nil {
			return err
		}
		if before == nil {
			return fmt.Errorf("%w: %s", entities.ErrRDAPPublicationPolicyNotFound, name)
		}
		var tlds []string
		if err := tx.Model(&RDAPPublicationAssignmentRecord{}).Where("policy = ?", name).Order("tld_name").Limit(5).Pluck("tld_name", &tlds).Error; err != nil {
			return err
		}
		if len(tlds) > 0 {
			return fmt.Errorf("%w: assigned to %s", entities.ErrRDAPPublicationPolicyInUse, strings.Join(tlds, ", "))
		}
		if err := tx.Where("name = ?", name).Delete(&RDAPPublicationPolicyRecord{}).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// ListAssignments implements repositories.RDAPPublicationPolicyRepository.
func (r *RDAPPublicationPolicyRepository) ListAssignments(ctx context.Context) (map[entities.DomainName]string, error) {
	var recs []RDAPPublicationAssignmentRecord
	if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make(map[entities.DomainName]string, len(recs))
	for _, a := range recs {
		out[entities.DomainName(a.TLDName)] = a.Policy
	}
	return out, nil
}

// AssignPolicy implements repositories.RDAPPublicationPolicyRepository. The
// foreign keys refuse a TLD or policy that does not exist.
func (r *RDAPPublicationPolicyRepository) AssignPolicy(ctx context.Context, tld entities.DomainName, policy string, events func(string) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		var before []string
		if err := tx.Model(&RDAPPublicationAssignmentRecord{}).Where("tld_name = ?", string(tld)).Pluck("policy", &before).Error; err != nil {
			return err
		}
		if policy == "" {
			if err := tx.Where("tld_name = ?", string(tld)).Delete(&RDAPPublicationAssignmentRecord{}).Error; err != nil {
				return err
			}
		} else {
			if p, err := rdapPublicationIn(tx, policy); err != nil {
				return err
			} else if p == nil {
				return fmt.Errorf("%w: %s", entities.ErrRDAPPublicationPolicyNotFound, policy)
			}
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tld_name"}}, DoUpdates: clause.AssignmentColumns([]string{"policy"}),
			}).Create(&RDAPPublicationAssignmentRecord{TLDName: string(tld), Policy: policy}).Error
			if isForeignKeyViolation(err) {
				return fmt.Errorf("%w: %s", entities.ErrTLDNotFound, tld)
			}
			if err != nil {
				return err
			}
		}
		prev := ""
		if len(before) > 0 {
			prev = before[0]
		}
		return writeEvents(tx, events(prev))
	})
}

// rdapPublicationForeignKeys: a TLD's deletion deletes its assignment; a
// policy assigned cannot be deleted.
var rdapPublicationForeignKeys = []struct{ table, name, def string }{
	{"rdap_publication_assignments", "fk_rdap_publication_assignments_tld", "FOREIGN KEY (tld_name) REFERENCES tlds(name) ON DELETE CASCADE"},
	{"rdap_publication_assignments", "fk_rdap_publication_assignments_policy", "FOREIGN KEY (policy) REFERENCES rdap_publication_policies(name) ON DELETE RESTRICT"},
	{"rdap_access_networks", "fk_rdap_access_networks_policy", "FOREIGN KEY (policy) REFERENCES rdap_access_policies(name) ON DELETE RESTRICT"},
}

// ensureRDAPPublicationForeignKeys adds rdapPublicationForeignKeys that are
// missing. Postgres only, as for the EPP policies.
func ensureRDAPPublicationForeignKeys(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, fk := range rdapPublicationForeignKeys {
		var n int64
		if err := db.Raw("SELECT count(*) FROM pg_constraint WHERE conname = ?", fk.name).Scan(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s %s", fk.table, fk.name, fk.def)).Error; err != nil { // #nosec G201 -- table, name and definition are the compile-time literals above
			return fmt.Errorf("add foreign key %s: %w", fk.name, err)
		}
	}
	return nil
}

// seedRDAPPublicationPolicies stores the built-in policies with their
// starting rules if they are not stored yet. Rules an operator has set are
// kept.
func seedRDAPPublicationPolicies(db *gorm.DB) error {
	for _, p := range entities.BuiltInRDAPPublicationPolicies() {
		p.UpdatedAt, p.UpdatedBy = time.Now().UTC(), "migrate"
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(toRDAPPublicationRecord(p)).Error; err != nil {
			return fmt.Errorf("seed RDAP publication policy %s: %w", p.Name, err)
		}
	}
	return nil
}
