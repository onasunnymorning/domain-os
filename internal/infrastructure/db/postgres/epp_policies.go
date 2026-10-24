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
	SessionsPerRegistrar int `gorm:"not null"`
	SessionsPerLogin     int `gorm:"not null"`
	SessionsPerFamily    int `gorm:"not null"`
	CheckNames           int `gorm:"not null"`
	// Rates and FamilyRates are columns rate_check_per_second,
	// rate_check_burst, ... and family_rate_check_per_second, ....
	Rates         EPPRateColumns `gorm:"embedded;embeddedPrefix:rate_"`
	FamilyRates   EPPRateColumns `gorm:"embedded;embeddedPrefix:family_rate_"`
	CreatesPerDay int            `gorm:"not null;default:0"`
	// FailedCreates and FamilyFailedCreates are columns
	// failed_creates_failures, failed_creates_ratio_percent and
	// family_failed_creates_....
	FailedCreates       EPPFailedCreateColumns `gorm:"embedded;embeddedPrefix:failed_creates_"`
	FamilyFailedCreates EPPFailedCreateColumns `gorm:"embedded;embeddedPrefix:family_failed_creates_"`
	UpdatedAt           time.Time              `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy           string
}

// EPPFailedCreateColumns are a failed-create budget. Like the rates, rows
// from before it existed are given their starting values by the migration;
// the column default of 0 is only so the columns can be added.
type EPPFailedCreateColumns struct {
	Failures     int `gorm:"not null;default:0"`
	RatioPercent int `gorm:"not null;default:0"`
}

// EPPRateColumns are a policy's command rates, a rate and a burst per
// class. Rows from before the rates existed are given their starting
// values by the migration (seedEPPPolicies); the column default of 0 is
// only so the columns can be added to them.
type EPPRateColumns struct {
	CheckPerSecond     int `gorm:"not null;default:0"`
	CheckBurst         int `gorm:"not null;default:0"`
	InfoPerSecond      int `gorm:"not null;default:0"`
	InfoBurst          int `gorm:"not null;default:0"`
	TransformPerSecond int `gorm:"not null;default:0"`
	TransformBurst     int `gorm:"not null;default:0"`
	PollPerSecond      int `gorm:"not null;default:0"`
	PollBurst          int `gorm:"not null;default:0"`
}

func toRateColumns(r entities.EPPCommandRates) EPPRateColumns {
	return EPPRateColumns{
		CheckPerSecond: r.Check.PerSecond, CheckBurst: r.Check.Burst,
		InfoPerSecond: r.Info.PerSecond, InfoBurst: r.Info.Burst,
		TransformPerSecond: r.Transform.PerSecond, TransformBurst: r.Transform.Burst,
		PollPerSecond: r.Poll.PerSecond, PollBurst: r.Poll.Burst,
	}
}

func (c EPPRateColumns) toEntity() entities.EPPCommandRates {
	return entities.EPPCommandRates{
		Check:     entities.EPPRate{PerSecond: c.CheckPerSecond, Burst: c.CheckBurst},
		Info:      entities.EPPRate{PerSecond: c.InfoPerSecond, Burst: c.InfoBurst},
		Transform: entities.EPPRate{PerSecond: c.TransformPerSecond, Burst: c.TransformBurst},
		Poll:      entities.EPPRate{PerSecond: c.PollPerSecond, Burst: c.PollBurst},
	}
}

// TableName pins the table name.
func (EPPPolicyRecord) TableName() string { return "epp_policies" }

// EPPPolicyAssignmentRecord is a registrar's assigned policy
// (epp_policy_assignments); a registrar with none has "default". Deleting
// the registrar deletes it; a policy assigned cannot be deleted.
type EPPPolicyAssignmentRecord struct {
	ClID   string `gorm:"primaryKey"`
	Policy string `gorm:"not null;size:32;index"`
}

// TableName pins the table name.
func (EPPPolicyAssignmentRecord) TableName() string { return "epp_policy_assignments" }

// RegistrarFamilyRecord is the GORM model for registrar_families.
type RegistrarFamilyRecord struct {
	Name        string `gorm:"primaryKey;size:32"`
	Description string
	// Policy is NULL for "default"; a policy assigned cannot be deleted.
	Policy    *string   `gorm:"size:32;index"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy string
}

// TableName pins the table name.
func (RegistrarFamilyRecord) TableName() string { return "registrar_families" }

// RegistrarFamilyMemberRecord makes a registrar a member of a family. The
// registrar is the key, so it is in at most one. Deleting the registrar or
// the family deletes it.
type RegistrarFamilyMemberRecord struct {
	ClID   string `gorm:"primaryKey"`
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
		Rates: toRateColumns(p.Limits.Rates), FamilyRates: toRateColumns(p.Limits.FamilyRates),
		CreatesPerDay: p.Limits.CreatesPerDay,
		FailedCreates: EPPFailedCreateColumns(p.Limits.FailedCreates), FamilyFailedCreates: EPPFailedCreateColumns(p.Limits.FamilyFailedCreates),
		UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy,
	}
}

func (r *EPPPolicyRecord) toEntity() *entities.EPPPolicy {
	return &entities.EPPPolicy{
		Name: r.Name, Description: r.Description,
		Limits: entities.EPPPolicyLimits{
			SessionsPerRegistrar: r.SessionsPerRegistrar, SessionsPerLogin: r.SessionsPerLogin,
			SessionsPerFamily: r.SessionsPerFamily, CheckNames: r.CheckNames,
			Rates: r.Rates.toEntity(), FamilyRates: r.FamilyRates.toEntity(),
			CreatesPerDay: r.CreatesPerDay,
			FailedCreates: entities.EPPFailedCreateBudget(r.FailedCreates), FamilyFailedCreates: entities.EPPFailedCreateBudget(r.FamilyFailedCreates),
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

// eppPolicyLock is the transaction-scoped advisory lock every policy
// change takes, so changes are serialised: each reads what it replaces,
// and builds its events from that, with no other change in between. Policy
// changes are rare administrative writes; one lock for all of them keeps
// assignments, families and policies consistent with each other too.
const eppPolicyLock int64 = 0x45505050 // "EPPP"

// change runs fn in a transaction holding eppPolicyLock.
func (r *EPPPolicyRepository) change(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", eppPolicyLock).Error; err != nil {
				return fmt.Errorf("lock EPP policies: %w", err)
			}
		}
		return fn(tx)
	})
}

// policyIn reads a policy in tx, or nil if there is none.
func policyIn(tx *gorm.DB, name string) (*entities.EPPPolicy, error) {
	var rec EPPPolicyRecord
	err := tx.Where("name = ?", name).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec.toEntity(), nil
}

// SavePolicy implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) SavePolicy(ctx context.Context, p *entities.EPPPolicy, events func(*entities.EPPPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := policyIn(tx, p.Name)
		if err != nil {
			return err
		}
		if err := tx.Save(toPolicyRecord(p)).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// DeletePolicy implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) DeletePolicy(ctx context.Context, name string, events func(*entities.EPPPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := policyIn(tx, name)
		if err != nil {
			return err
		}
		if before == nil {
			return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
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
		return writeEvents(tx, events(before))
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

// AssignPolicy implements repositories.EPPPolicyRepository. The foreign
// keys refuse a registrar or policy that does not exist, including one
// deleted since the caller checked.
func (r *EPPPolicyRepository) AssignPolicy(ctx context.Context, clid entities.ClIDType, policy string, events func(string) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		var before []string
		if err := tx.Model(&EPPPolicyAssignmentRecord{}).Where("cl_id = ?", clid.String()).Pluck("policy", &before).Error; err != nil {
			return err
		}
		if policy == "" {
			if err := tx.Where("cl_id = ?", clid.String()).Delete(&EPPPolicyAssignmentRecord{}).Error; err != nil {
				return err
			}
		} else {
			if p, err := policyIn(tx, policy); err != nil {
				return err
			} else if p == nil {
				return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, policy)
			}
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "cl_id"}}, DoUpdates: clause.AssignmentColumns([]string{"policy"}),
			}).Create(&EPPPolicyAssignmentRecord{ClID: clid.String(), Policy: policy}).Error
			if isForeignKeyViolation(err) {
				return fmt.Errorf("%w: %s", entities.ErrRegistrarNotFound, clid)
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

// isForeignKeyViolation reports whether err is Postgres's
// foreign_key_violation.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
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
			Name: f.Name, Description: f.Description, Members: byFamily[f.Name],
			UpdatedAt: f.UpdatedAt.UTC(), UpdatedBy: f.UpdatedBy,
		}
		if f.Policy != nil {
			out[i].Policy = *f.Policy
		}
		if out[i].Members == nil {
			out[i].Members = []entities.ClIDType{}
		}
	}
	return out, nil
}

// familyIn reads a family in tx, or nil if there is none.
func familyIn(tx *gorm.DB, name string) (*entities.RegistrarFamily, error) {
	fs, err := listFamilies(tx, name)
	if err != nil || len(fs) == 0 {
		return nil, err
	}
	return fs[0], nil
}

// GetFamily implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) GetFamily(ctx context.Context, name string) (*entities.RegistrarFamily, error) {
	f, err := familyIn(r.db.WithContext(ctx), name)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
	}
	return f, nil
}

// SaveFamily implements repositories.EPPPolicyRepository. Its members are
// replaced.
func (r *EPPPolicyRepository) SaveFamily(ctx context.Context, f *entities.RegistrarFamily, events func(*entities.RegistrarFamily) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := familyIn(tx, f.Name)
		if err != nil {
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
		rec := &RegistrarFamilyRecord{Name: f.Name, Description: f.Description, UpdatedAt: f.UpdatedAt, UpdatedBy: f.UpdatedBy}
		if f.Policy != "" {
			if p, err := policyIn(tx, f.Policy); err != nil {
				return err
			} else if p == nil {
				return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, f.Policy)
			}
			policy := f.Policy
			rec.Policy = &policy
		}
		if err := tx.Save(rec).Error; err != nil {
			return err
		}
		if err := tx.Where("family = ?", f.Name).Delete(&RegistrarFamilyMemberRecord{}).Error; err != nil {
			return err
		}
		for _, m := range members {
			err := tx.Create(&RegistrarFamilyMemberRecord{ClID: m, Family: f.Name}).Error
			if isForeignKeyViolation(err) {
				return fmt.Errorf("family member %s: %w", m, entities.ErrRegistrarNotFound)
			}
			if err != nil {
				return err
			}
		}
		return writeEvents(tx, events(before))
	})
}

// DeleteFamily implements repositories.EPPPolicyRepository.
func (r *EPPPolicyRepository) DeleteFamily(ctx context.Context, name string, events func(*entities.RegistrarFamily) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := familyIn(tx, name)
		if err != nil {
			return err
		}
		if before == nil {
			return fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
		}
		if err := tx.Where("family = ?", name).Delete(&RegistrarFamilyMemberRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("name = ?", name).Delete(&RegistrarFamilyRecord{}).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// eppPolicyForeignKeys are the policy tables' foreign keys
// (ensureEPPPolicyForeignKeys): a registrar's deletion deletes its
// assignment and its family membership; a policy assigned to a registrar or
// a family cannot be deleted; a family's deletion deletes its members.
var eppPolicyForeignKeys = []struct{ table, name, def string }{
	{"epp_policy_assignments", "fk_epp_policy_assignments_registrar", "FOREIGN KEY (cl_id) REFERENCES registrars(cl_id) ON DELETE CASCADE"},
	{"epp_policy_assignments", "fk_epp_policy_assignments_policy", "FOREIGN KEY (policy) REFERENCES epp_policies(name) ON DELETE RESTRICT"},
	{"registrar_families", "fk_registrar_families_policy", "FOREIGN KEY (policy) REFERENCES epp_policies(name) ON DELETE RESTRICT"},
	{"registrar_family_members", "fk_registrar_family_members_registrar", "FOREIGN KEY (cl_id) REFERENCES registrars(cl_id) ON DELETE CASCADE"},
	{"registrar_family_members", "fk_registrar_family_members_family", "FOREIGN KEY (family) REFERENCES registrar_families(name) ON DELETE CASCADE"},
}

// ensureEPPPolicyForeignKeys adds eppPolicyForeignKeys that are missing.
// They are declared here rather than as GORM relations, which would put
// the registrar's key on the wrong table. Postgres only: the SQLite
// databases some tests migrate do without.
func ensureEPPPolicyForeignKeys(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, fk := range eppPolicyForeignKeys {
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
	if err := backfillEPPPolicyRates(db); err != nil {
		return err
	}
	return backfillEPPPolicyFailedCreates(db)
}

// startingLimits is the limits a stored policy starts from: a built-in
// policy's own, any other's the default's.
func startingLimits(name string) entities.EPPPolicyLimits {
	for _, p := range entities.BuiltInEPPPolicies() {
		if p.Name == name {
			return p.Limits
		}
	}
	return entities.DefaultEPPPolicyLimits()
}

// backfillEPPPolicyFailedCreates gives policies stored before the
// failed-create budget existed (Phase 6a) their starting budgets. A stored
// budget has at least 1 failure, so 0 marks the rows to fill.
func backfillEPPPolicyFailedCreates(db *gorm.DB) error {
	var recs []EPPPolicyRecord
	if err := db.Where("failed_creates_failures = 0").Find(&recs).Error; err != nil {
		return fmt.Errorf("find EPP policies without a failed-create budget: %w", err)
	}
	for _, rec := range recs {
		l := startingLimits(rec.Name)
		err := db.Model(&EPPPolicyRecord{}).Where("name = ? AND failed_creates_failures = 0", rec.Name).Updates(map[string]any{
			"failed_creates_failures": l.FailedCreates.Failures, "failed_creates_ratio_percent": l.FailedCreates.RatioPercent,
			"family_failed_creates_failures": l.FamilyFailedCreates.Failures, "family_failed_creates_ratio_percent": l.FamilyFailedCreates.RatioPercent,
		}).Error
		if err != nil {
			return fmt.Errorf("fill EPP policy %s's failed-create budget: %w", rec.Name, err)
		}
	}
	return nil
}

// backfillEPPPolicyRates gives policies stored before command rates and
// the create quota existed (Phase 5b) their starting values: a built-in
// policy its own, any other the default's, with check bursts raised to the
// policy's check_names where that is larger. A stored policy always has a
// check rate of at least 1, so 0 marks the rows to fill.
func backfillEPPPolicyRates(db *gorm.DB) error {
	var recs []EPPPolicyRecord
	if err := db.Where("rate_check_per_second = 0").Find(&recs).Error; err != nil {
		return fmt.Errorf("find EPP policies without rates: %w", err)
	}
	for _, rec := range recs {
		l := startingLimits(rec.Name)
		// A check burst holds at least one full check of the policy's own
		// size, or its largest check could never pass.
		l.Rates.Check.Burst = max(l.Rates.Check.Burst, rec.CheckNames)
		l.FamilyRates.Check.Burst = max(l.FamilyRates.Check.Burst, rec.CheckNames)
		cols := map[string]any{"creates_per_day": l.CreatesPerDay}
		for prefix, r := range map[string]entities.EPPCommandRates{"rate_": l.Rates, "family_rate_": l.FamilyRates} {
			for class, v := range map[string]entities.EPPRate{"check": r.Check, "info": r.Info, "transform": r.Transform, "poll": r.Poll} {
				cols[prefix+class+"_per_second"], cols[prefix+class+"_burst"] = v.PerSecond, v.Burst
			}
		}
		err := db.Model(&EPPPolicyRecord{}).Where("name = ? AND rate_check_per_second = 0", rec.Name).Updates(cols).Error
		if err != nil {
			return fmt.Errorf("fill EPP policy %s's rates: %w", rec.Name, err)
		}
	}
	return nil
}
