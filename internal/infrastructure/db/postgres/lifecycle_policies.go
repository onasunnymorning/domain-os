package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ---------------------------------------------------------------------------
// Lifecycle policies — ADR 0023. A phase names its policy
// (phases.lifecycle_policy_name, a foreign key); the policy table holds the
// settings as stored, NULL for "the default". Like the EPP policies, every
// write commits with its outbox rows.
// ---------------------------------------------------------------------------

// LifecyclePolicyRecord is the GORM model for lifecycle_policies.
type LifecyclePolicyRecord struct {
	Name                  string `gorm:"primaryKey;size:32"`
	Description           string
	RegistrationGP        *int
	RenewalGP             *int
	AutoRenewalGP         *int
	TransferGP            *int
	TransferLockPeriod    *int
	PendingTransferPeriod *int
	RedemptionGP          *int
	PendingDeleteGP       *int
	PendingRestorePeriod  *int
	RestoreReportRequired *bool
	AllowAutoRenew        *bool
	MaxHorizon            *int
	UpdatedAt             time.Time `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy             string
}

// TableName pins the table name.
func (LifecyclePolicyRecord) TableName() string { return "lifecycle_policies" }

func toLifecycleRecord(p *entities.LifecyclePolicy) *LifecyclePolicyRecord {
	s := p.Settings
	return &LifecyclePolicyRecord{
		Name: p.Name, Description: p.Description,
		RegistrationGP: s.RegistrationGP, RenewalGP: s.RenewalGP, AutoRenewalGP: s.AutoRenewalGP, TransferGP: s.TransferGP,
		TransferLockPeriod: s.TransferLockPeriod, PendingTransferPeriod: s.PendingTransferPeriod,
		RedemptionGP: s.RedemptionGP, PendingDeleteGP: s.PendingDeleteGP, PendingRestorePeriod: s.PendingRestorePeriod,
		RestoreReportRequired: s.RestoreReportRequired, AllowAutoRenew: s.AllowAutoRenew, MaxHorizon: s.MaxHorizon,
		UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy,
	}
}

func (r *LifecyclePolicyRecord) toEntity() *entities.LifecyclePolicy {
	return &entities.LifecyclePolicy{
		Name: r.Name, Description: r.Description,
		Settings: entities.LifecycleSettings{
			RegistrationGP: r.RegistrationGP, RenewalGP: r.RenewalGP, AutoRenewalGP: r.AutoRenewalGP, TransferGP: r.TransferGP,
			TransferLockPeriod: r.TransferLockPeriod, PendingTransferPeriod: r.PendingTransferPeriod,
			RedemptionGP: r.RedemptionGP, PendingDeleteGP: r.PendingDeleteGP, PendingRestorePeriod: r.PendingRestorePeriod,
			RestoreReportRequired: r.RestoreReportRequired, AllowAutoRenew: r.AllowAutoRenew, MaxHorizon: r.MaxHorizon,
		},
		UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy,
	}
}

// LifecyclePolicyRepository is the Postgres
// repositories.LifecyclePolicyRepository.
type LifecyclePolicyRepository struct{ db *gorm.DB }

// NewLifecyclePolicyRepository returns the repository.
func NewLifecyclePolicyRepository(db *gorm.DB) *LifecyclePolicyRepository {
	return &LifecyclePolicyRepository{db: db}
}

// ListPolicies implements repositories.LifecyclePolicyRepository.
func (r *LifecyclePolicyRepository) ListPolicies(ctx context.Context) ([]*entities.LifecyclePolicy, error) {
	var recs []LifecyclePolicyRecord
	if err := r.db.WithContext(ctx).Order("name").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.LifecyclePolicy, len(recs))
	for i := range recs {
		out[i] = recs[i].toEntity()
	}
	return out, nil
}

// GetPolicy implements repositories.LifecyclePolicyRepository.
func (r *LifecyclePolicyRepository) GetPolicy(ctx context.Context, name string) (*entities.LifecyclePolicy, error) {
	p, err := lifecyclePolicyIn(r.db.WithContext(ctx), name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyNotFound, name)
	}
	return p, nil
}

// lifecyclePolicyIn reads a policy in tx, or nil if there is none.
func lifecyclePolicyIn(tx *gorm.DB, name string) (*entities.LifecyclePolicy, error) {
	var rec LifecyclePolicyRecord
	err := tx.Where("name = ?", name).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec.toEntity(), nil
}

// lifecyclePolicyLock is the transaction-scoped advisory lock every policy
// change takes, so changes are serialised (as eppPolicyLock).
const lifecyclePolicyLock int64 = 0x4c494645 // "LIFE"

func (r *LifecyclePolicyRepository) change(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", lifecyclePolicyLock).Error; err != nil {
				return fmt.Errorf("lock lifecycle policies: %w", err)
			}
		}
		return fn(tx)
	})
}

// SavePolicy implements repositories.LifecyclePolicyRepository.
func (r *LifecyclePolicyRepository) SavePolicy(ctx context.Context, p *entities.LifecyclePolicy, events func(*entities.LifecyclePolicy) ([]entities.DomainEvent, error)) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := lifecyclePolicyIn(tx, p.Name)
		if err != nil {
			return err
		}
		evs, err := events(before)
		if err != nil {
			return err
		}
		// An upsert of every column, so a setting set back to NULL (the
		// default) is written; Updates would skip the nil pointers.
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, UpdateAll: true}).Create(toLifecycleRecord(p)).Error; err != nil {
			return err
		}
		return writeEvents(tx, evs)
	})
}

// DeletePolicy implements repositories.LifecyclePolicyRepository. The
// foreign key from phases refuses a policy a phase names, including one
// named since the check below.
func (r *LifecyclePolicyRepository) DeletePolicy(ctx context.Context, name string, events func(*entities.LifecyclePolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := lifecyclePolicyIn(tx, name)
		if err != nil {
			return err
		}
		if before == nil {
			return fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyNotFound, name)
		}
		uses, err := policyUsesIn(tx, name, 5)
		if err != nil {
			return err
		}
		if len(uses) > 0 {
			names := make([]string, len(uses))
			for i, u := range uses {
				names[i] = u.TLD + "/" + u.Phase
			}
			return fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyInUse, strings.Join(names, ", "))
		}
		err = tx.Where("name = ?", name).Delete(&LifecyclePolicyRecord{}).Error
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: a phase named it", entities.ErrLifecyclePolicyInUse)
		}
		if err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// PolicyUses implements repositories.LifecyclePolicyRepository.
func (r *LifecyclePolicyRepository) PolicyUses(ctx context.Context, name string) ([]entities.LifecyclePolicyUse, error) {
	return policyUsesIn(r.db.WithContext(ctx), name, 0)
}

func policyUsesIn(tx *gorm.DB, name string, limit int) ([]entities.LifecyclePolicyUse, error) {
	var rows []struct {
		TLDName string
		Name    string
	}
	q := tx.Model(&Phase{}).Select("tld_name, name").Where("lifecycle_policy_name = ?", name).Order("tld_name, name")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.LifecyclePolicyUse, len(rows))
	for i, row := range rows {
		out[i] = entities.LifecyclePolicyUse{TLD: row.TLDName, Phase: row.Name}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Migration (ADR 0023 decision 6). Three steps around AutoMigrate:
//
//  1. prepareLifecyclePolicies, before: the table and gtld-standard exist, so
//     the lifecycle_policy_name column AutoMigrate adds to phases (NOT NULL
//     DEFAULT 'gtld-standard') and its foreign key are satisfied on every
//     existing row.
//  2. AutoMigrate adds the column and the key.
//  3. migrateLifecyclePolicies, after: while phases still carries the old
//     inline columns, each phase whose values equal gtld-standard keeps it;
//     every other distinct value set becomes a policy migrated-<tld>-<phase>
//     (named after the first phase that holds it, GA phases first, then by
//     start) and its phases name it;
//     the assignment is verified; the inline columns are dropped. One
//     transaction: a failure leaves the columns and nothing assigned.
//     A rerun finds no inline columns and does nothing.
// ---------------------------------------------------------------------------

// prepareLifecyclePolicies creates lifecycle_policies and seeds the built-in
// policy. It runs before the main AutoMigrate (see above).
func prepareLifecyclePolicies(db *gorm.DB) error {
	if err := db.AutoMigrate(&LifecyclePolicyRecord{}); err != nil {
		return fmt.Errorf("migrate lifecycle_policies: %w", err)
	}
	return seedLifecyclePolicies(db)
}

// seedLifecyclePolicies stores gtld-standard with its starting settings if
// it is not stored yet. Settings an operator has set are kept.
func seedLifecyclePolicies(db *gorm.DB) error {
	p := entities.StandardLifecyclePolicy()
	p.UpdatedAt, p.UpdatedBy = time.Now().UTC(), "migrate"
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(toLifecycleRecord(p)).Error; err != nil {
		return fmt.Errorf("seed lifecycle policy %s: %w", p.Name, err)
	}
	return nil
}

// inlineLifecycleColumns are the columns phases carried before ADR 0023,
// dropped by migrateLifecyclePolicies.
var inlineLifecycleColumns = []string{
	"registration_gp", "renewal_gp", "auto_renewal_gp", "transfer_gp", "pending_transfer_period",
	"redemption_gp", "pending_delete_gp", "pending_restore_period", "restore_report_required",
	"transfer_lock_period", "max_horizon", "allow_auto_renew",
}

// inlinePhasePolicy is a phase's lifecycle settings as the old columns held
// them: 0 meant "the default" for every duration but the transfer lock,
// which was NULL for the default and 0 for no lock; NULL booleans were the
// default.
type inlinePhasePolicy struct {
	ID                    int64
	TLDName               string
	Name                  string
	RegistrationGP        int
	RenewalGP             int
	AutoRenewalGP         int
	TransferGP            int
	PendingTransferPeriod int
	RedemptionGP          int
	PendingDeleteGP       int
	PendingRestorePeriod  int
	RestoreReportRequired *bool
	TransferLockPeriod    *int
	MaxHorizon            int
	AllowAutoRenew        *bool
}

// resolved is what the lifecycle code read from the row.
func (r inlinePhasePolicy) resolved() entities.ResolvedLifecycle {
	d := entities.DefaultLifecycle()
	orZero := func(v, def int) int {
		if v <= 0 {
			return def
		}
		return v
	}
	out := entities.ResolvedLifecycle{
		RegistrationGP:        orZero(r.RegistrationGP, d.RegistrationGP),
		RenewalGP:             orZero(r.RenewalGP, d.RenewalGP),
		AutoRenewalGP:         orZero(r.AutoRenewalGP, d.AutoRenewalGP),
		TransferGP:            orZero(r.TransferGP, d.TransferGP),
		PendingTransferPeriod: orZero(r.PendingTransferPeriod, d.PendingTransferPeriod),
		RedemptionGP:          orZero(r.RedemptionGP, d.RedemptionGP),
		PendingDeleteGP:       orZero(r.PendingDeleteGP, d.PendingDeleteGP),
		PendingRestorePeriod:  orZero(r.PendingRestorePeriod, d.PendingRestorePeriod),
		MaxHorizon:            orZero(r.MaxHorizon, d.MaxHorizon),
		TransferLockPeriod:    d.TransferLockPeriod,
		RestoreReportRequired: d.RestoreReportRequired,
		AllowAutoRenew:        d.AllowAutoRenew,
	}
	if r.TransferLockPeriod != nil {
		out.TransferLockPeriod = *r.TransferLockPeriod
	}
	if r.RestoreReportRequired != nil {
		out.RestoreReportRequired = *r.RestoreReportRequired
	}
	if r.AllowAutoRenew != nil {
		out.AllowAutoRenew = *r.AllowAutoRenew
	}
	return out
}

var notPolicyName = regexp.MustCompile(`[^a-z0-9-]+`)

// migratedPolicyName is migrated-<tld>-<phase>, made a policy name: lower
// case, other characters hyphens, cut to 32, and numbered when taken.
func migratedPolicyName(tld, phase string, taken map[string]bool) string {
	base := "migrated-" + strings.Trim(notPolicyName.ReplaceAllString(strings.ToLower(tld+"-"+phase), "-"), "-")
	base = strings.Trim(base, "-")
	if len(base) > 32 {
		base = strings.TrimRight(base[:32], "-")
	}
	name := base
	for n := 2; taken[name]; n++ {
		suffix := fmt.Sprintf("-%d", n)
		name = base
		if len(name)+len(suffix) > 32 {
			name = strings.TrimRight(name[:32-len(suffix)], "-")
		}
		name += suffix
	}
	taken[name] = true
	return name
}

// migrateLifecyclePolicies moves the phases' inline lifecycle settings into
// named policies (see the section comment) and drops the inline columns.
// Postgres only: the SQLite databases some tests migrate never had them.
func migrateLifecyclePolicies(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" || !db.Migrator().HasTable("phases") || !db.Migrator().HasColumn("phases", "registration_gp") {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", lifecyclePolicyLock).Error; err != nil {
			return fmt.Errorf("lock lifecycle policies: %w", err)
		}
		var rows []inlinePhasePolicy
		err := tx.Raw(`SELECT id, tld_name, name, COALESCE(registration_gp, 0) AS registration_gp, COALESCE(renewal_gp, 0) AS renewal_gp,
			COALESCE(auto_renewal_gp, 0) AS auto_renewal_gp, COALESCE(transfer_gp, 0) AS transfer_gp,
			COALESCE(pending_transfer_period, 0) AS pending_transfer_period, COALESCE(redemption_gp, 0) AS redemption_gp,
			COALESCE(pending_delete_gp, 0) AS pending_delete_gp, COALESCE(pending_restore_period, 0) AS pending_restore_period,
			restore_report_required, transfer_lock_period, COALESCE(max_horizon, 0) AS max_horizon, allow_auto_renew
			FROM phases ORDER BY tld_name, (type <> 'GA'), starts, id`).Scan(&rows).Error
		if err != nil {
			return fmt.Errorf("read the phases' lifecycle settings: %w", err)
		}
		standard := entities.DefaultLifecycle()
		taken := map[string]bool{entities.LifecyclePolicyStandard: true}
		var existing []string
		if err := tx.Model(&LifecyclePolicyRecord{}).Pluck("name", &existing).Error; err != nil {
			return err
		}
		for _, n := range existing {
			taken[n] = true
		}
		byValues := map[entities.ResolvedLifecycle]string{standard: entities.LifecyclePolicyStandard}
		assigned := map[int64]string{}
		for _, row := range rows {
			values := row.resolved()
			name, ok := byValues[values]
			if !ok {
				name = migratedPolicyName(row.TLDName, row.Name, taken)
				p := &entities.LifecyclePolicy{
					Name:        name,
					Description: fmt.Sprintf("Migrated from phase %s of %s (ADR 0023); rename as you see fit", row.Name, row.TLDName),
					Settings:    values.Settings(),
					UpdatedAt:   time.Now().UTC(), UpdatedBy: "migrate",
				}
				if err := p.Settings.Validate(); err != nil {
					return fmt.Errorf("phase %s of %s holds settings no policy may: %w", row.Name, row.TLDName, err)
				}
				if err := tx.Create(toLifecycleRecord(p)).Error; err != nil {
					return fmt.Errorf("create lifecycle policy %s: %w", name, err)
				}
				byValues[values] = name
			}
			if err := tx.Exec("UPDATE phases SET lifecycle_policy_name = ? WHERE id = ?", name, row.ID).Error; err != nil {
				return fmt.Errorf("assign lifecycle policy %s to phase %d: %w", name, row.ID, err)
			}
			assigned[row.ID] = name
		}
		// Verify: every phase's named policy resolves to what the row held.
		stored, err := NewLifecyclePolicyRepository(tx).ListPolicies(context.Background())
		if err != nil {
			return err
		}
		resolvedByName := map[string]entities.ResolvedLifecycle{}
		for _, p := range stored {
			resolvedByName[p.Name] = p.Resolved()
		}
		for _, row := range rows {
			if got, want := resolvedByName[assigned[row.ID]], row.resolved(); got != want {
				return fmt.Errorf("lifecycle policy migration: phase %s of %s would change: %+v is not %+v", row.Name, row.TLDName, got, want)
			}
		}
		for _, col := range inlineLifecycleColumns {
			if err := tx.Exec("ALTER TABLE phases DROP COLUMN IF EXISTS " + col).Error; err != nil { // #nosec G202 -- the column names are the compile-time literals above
				return fmt.Errorf("drop phases.%s: %w", col, err)
			}
		}
		return nil
	})
}
