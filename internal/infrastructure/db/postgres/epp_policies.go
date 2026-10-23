package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ---------------------------------------------------------------------------
// EPP policies and registrar families — ADR 0019, docs/epp/ARCHITECTURE.md
// §5. Like the credentials, every write commits with its outbox rows.
// ---------------------------------------------------------------------------

// EPPPolicyRecord is the GORM model for epp_policies.
type EPPPolicyRecord struct {
	Name                 string `gorm:"primaryKey;size:32"`
	Description          string
	SessionsPerRegistrar int       `gorm:"not null"`
	SessionsPerLogin     int       `gorm:"not null"`
	SessionsPerFamily    int       `gorm:"not null"`
	CheckNames           int       `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy            string
}

// TableName pins the table name.
func (EPPPolicyRecord) TableName() string { return "epp_policies" }

// EPPPolicyAssignmentRecord is a registrar's assigned policy
// (epp_policy_assignments); a registrar with none has "default".
type EPPPolicyAssignmentRecord struct {
	ClID   string `gorm:"primaryKey;size:16"`
	Policy string `gorm:"not null;size:32;index"`
}

// TableName pins the table name.
func (EPPPolicyAssignmentRecord) TableName() string { return "epp_policy_assignments" }

// RegistrarFamilyRecord is the GORM model for registrar_families.
type RegistrarFamilyRecord struct {
	Name        string `gorm:"primaryKey;size:32"`
	Description string
	// Policy is empty for "default".
	Policy    string    `gorm:"size:32;index"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy string
}

// TableName pins the table name.
func (RegistrarFamilyRecord) TableName() string { return "registrar_families" }

// RegistrarFamilyMemberRecord makes a registrar a member of a family. The
// registrar is the key, so it is in at most one.
type RegistrarFamilyMemberRecord struct {
	ClID   string `gorm:"primaryKey;size:16"`
	Family string `gorm:"not null;size:32;index"`
}

// TableName pins the table name.
func (RegistrarFamilyMemberRecord) TableName() string { return "registrar_family_members" }

// EPPPolicyRepository is the Postgres repositories.EPPPolicyRepository.
type EPPPolicyRepository struct{ db *gorm.DB }

// NewEPPPolicyRepository returns the repository.
func NewEPPPolicyRepository(db *gorm.DB) *EPPPolicyRepository { return &EPPPolicyRepository{db: db} }

func toPolicyRecord(p *entities.EPPPolicy) *EPPPolicyRecord {
	return &EPPPolicyRecord{
		Name: p.Name, Description: p.Description,
		SessionsPerRegistrar: p.Limits.SessionsPerRegistrar, SessionsPerLogin: p.Limits.SessionsPerLogin,
		SessionsPerFamily: p.Limits.SessionsPerFamily, CheckNames: p.Limits.CheckNames,
		UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy,
	}
}

func (r *EPPPolicyRecord) toEntity() *entities.EPPPolicy {
	return &entities.EPPPolicy{
		Name: r.Name, Description: r.Description,
		Limits: entities.EPPPolicyLimits{
			SessionsPerRegistrar: r.SessionsPerRegistrar, SessionsPerLogin: r.SessionsPerLogin,
			SessionsPerFamily: r.SessionsPerFamily, CheckNames: r.CheckNames,
		},
		UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy,
	}
}

// writeEvents writes outbox rows in tx.
func writeEvents(tx *gorm.DB, events []entities.DomainEvent) error {
	for _, e := range events {
		rec, err := ToDBDomainEvent(e)
		if err != nil {
			return fmt.Errorf("encode event %s: %w", e.Type, err)
		}
		if err := tx.Create(&rec).Error; err != nil {
			return fmt.Errorf("write event %s: %w", e.Type, err)
		}
	}
	return nil
}

// ListPolicies implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) ListPolicies(ctx context.Context) ([]*entities.EPPPolicy, error) {
	var recs []EPPPolicyRecord
	if err := r.db.WithContext(ctx).Order("name").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.EPPPolicy, len(recs))
	for i := range recs {
		out[i] = recs[i].toEntity()
	}
	return out, nil
}

// GetPolicy implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) GetPolicy(ctx context.Context, name string) (*entities.EPPPolicy, error) {
	var rec EPPPolicyRecord
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return rec.toEntity(), nil
}

// SavePolicy implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) SavePolicy(ctx context.Context, p *entities.EPPPolicy, events []entities.DomainEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(toPolicyRecord(p)).Error; err != nil {
			return err
		}
		return writeEvents(tx, events)
	})
}

// DeletePolicy implements repositories.EPPPolicyRepository. The policy's
// row is locked first, so an assignment made meanwhile is seen.
func (r *EPPPolicyRepository) DeletePolicy(ctx context.Context, name string, events []entities.DomainEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", name).Take(&EPPPolicyRecord{}).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
		}
		if err != nil {
			return err
		}
		var users []string
		if err := tx.Model(&EPPPolicyAssignmentRecord{}).Where("policy = ?", name).Order("cl_id").Limit(5).Pluck("cl_id", &users).Error; err != nil {
			return err
		}
		var families []string
		if err := tx.Model(&RegistrarFamilyRecord{}).Where("policy = ?", name).Order("name").Limit(5).Pluck("name", &families).Error; err != nil {
			return err
		}
		if len(users)+len(families) > 0 {
			return fmt.Errorf("%w: assigned to %s", entities.ErrEPPPolicyInUse, strings.Join(append(users, families...), ", "))
		}
		if err := tx.Where("name = ?", name).Delete(&EPPPolicyRecord{}).Error; err != nil {
			return err
		}
		return writeEvents(tx, events)
	})
}

// ListAssignments implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) ListAssignments(ctx context.Context) (map[entities.ClIDType]string, error) {
	var recs []EPPPolicyAssignmentRecord
	if err := r.db.WithContext(ctx).Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make(map[entities.ClIDType]string, len(recs))
	for _, a := range recs {
		out[entities.ClIDType(a.ClID)] = a.Policy
	}
	return out, nil
}

// AssignPolicy implements repositories.EPPPolicyRepository. A custom
// policy's row is locked, so a delete cannot remove it meanwhile.
func (r *EPPPolicyRepository) AssignPolicy(ctx context.Context, clid entities.ClIDType, policy string, events []entities.DomainEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cl_id = ?", clid.String()).Delete(&EPPPolicyAssignmentRecord{}).Error; err != nil {
			return err
		}
		if policy != "" {
			if err := lockPolicy(tx, policy); err != nil {
				return err
			}
			if err := tx.Create(&EPPPolicyAssignmentRecord{ClID: clid.String(), Policy: policy}).Error; err != nil {
				return err
			}
		}
		return writeEvents(tx, events)
	})
}

// lockPolicy locks a policy's row, unless it is a built-in one, which
// cannot be deleted and may not be stored yet.
func lockPolicy(tx *gorm.DB, name string) error {
	if entities.IsBuiltInEPPPolicy(name) {
		return nil
	}
	err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("name = ?", name).Take(&EPPPolicyRecord{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
	}
	return err
}

// ListFamilies implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) ListFamilies(ctx context.Context) ([]*entities.RegistrarFamily, error) {
	return listFamilies(r.db.WithContext(ctx), "")
}

func listFamilies(db *gorm.DB, name string) ([]*entities.RegistrarFamily, error) {
	q := db.Order("name")
	if name != "" {
		q = q.Where("name = ?", name)
	}
	var recs []RegistrarFamilyRecord
	if err := q.Find(&recs).Error; err != nil {
		return nil, err
	}
	mq := db.Order("cl_id")
	if name != "" {
		mq = mq.Where("family = ?", name)
	}
	var members []RegistrarFamilyMemberRecord
	if err := mq.Find(&members).Error; err != nil {
		return nil, err
	}
	byFamily := map[string][]entities.ClIDType{}
	for _, m := range members {
		byFamily[m.Family] = append(byFamily[m.Family], entities.ClIDType(m.ClID))
	}
	out := make([]*entities.RegistrarFamily, len(recs))
	for i, f := range recs {
		out[i] = &entities.RegistrarFamily{
			Name: f.Name, Description: f.Description, Policy: f.Policy,
			Members: byFamily[f.Name], UpdatedAt: f.UpdatedAt.UTC(), UpdatedBy: f.UpdatedBy,
		}
		if out[i].Members == nil {
			out[i].Members = []entities.ClIDType{}
		}
	}
	return out, nil
}

// GetFamily implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) GetFamily(ctx context.Context, name string) (*entities.RegistrarFamily, error) {
	fs, err := listFamilies(r.db.WithContext(ctx), name)
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
	}
	return fs[0], nil
}

// SaveFamily implements repositories.EPPPolicyRepository. Its members are
// replaced; a member of another family breaks the members' key and is
// entities.ErrRegistrarInFamily.
func (r *EPPPolicyRepository) SaveFamily(ctx context.Context, f *entities.RegistrarFamily, events []entities.DomainEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if f.Policy != "" {
			if err := lockPolicy(tx, f.Policy); err != nil {
				return err
			}
		}
		rec := &RegistrarFamilyRecord{Name: f.Name, Description: f.Description, Policy: f.Policy, UpdatedAt: f.UpdatedAt, UpdatedBy: f.UpdatedBy}
		if err := tx.Save(rec).Error; err != nil {
			return err
		}
		members := make([]string, len(f.Members))
		for i, m := range f.Members {
			members[i] = m.String()
		}
		if len(members) > 0 {
			var taken []RegistrarFamilyMemberRecord
			if err := tx.Where("cl_id IN ? AND family <> ?", members, f.Name).Order("cl_id").Find(&taken).Error; err != nil {
				return err
			}
			if len(taken) > 0 {
				return fmt.Errorf("%w: %s is in %s", entities.ErrRegistrarInFamily, taken[0].ClID, taken[0].Family)
			}
		}
		if err := tx.Where("family = ?", f.Name).Delete(&RegistrarFamilyMemberRecord{}).Error; err != nil {
			return err
		}
		for _, m := range members {
			// The registrar is the key: a member added to another family
			// since the check above breaks it.
			err := tx.Create(&RegistrarFamilyMemberRecord{ClID: m, Family: f.Name}).Error
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
				return fmt.Errorf("%w: %s", entities.ErrRegistrarInFamily, m)
			}
			if err != nil {
				return err
			}
		}
		return writeEvents(tx, events)
	})
}

// DeleteFamily implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) DeleteFamily(ctx context.Context, name string, events []entities.DomainEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("name = ?", name).Delete(&RegistrarFamilyRecord{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
		}
		if err := tx.Where("family = ?", name).Delete(&RegistrarFamilyMemberRecord{}).Error; err != nil {
			return err
		}
		return writeEvents(tx, events)
	})
}

// seedEPPPolicies stores the built-in policies with their starting limits
// (entities.BuiltInEPPPolicies) if they are not stored yet. Limits an
// operator has set are kept.
func seedEPPPolicies(db *gorm.DB) error {
	for _, p := range entities.BuiltInEPPPolicies() {
		p.UpdatedAt, p.UpdatedBy = time.Now().UTC(), "migrate"
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(toPolicyRecord(p)).Error; err != nil {
			return fmt.Errorf("seed EPP policy %s: %w", p.Name, err)
		}
	}
	return nil
}

