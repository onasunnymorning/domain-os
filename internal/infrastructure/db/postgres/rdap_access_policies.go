package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ---------------------------------------------------------------------------
// RDAP access policies — ADR 0022, docs/rdap/POLICY_REFERENCE.md § Access
// policies. Shaped like the publication policies, under the same lock.
// ---------------------------------------------------------------------------

// RDAPAccessPolicyRecord is the GORM model for rdap_access_policies.
type RDAPAccessPolicyRecord struct {
	Name                  string `gorm:"primaryKey;size:32"`
	Description           string
	PerAddressPerSecond   int       `gorm:"not null"`
	PerAddressBurst       int       `gorm:"not null"`
	PerAggregatePerSecond int       `gorm:"not null;default:0"`
	PerAggregateBurst     int       `gorm:"not null;default:0"`
	NotFoundCost          int       `gorm:"not null"`
	UpdatedAt             time.Time `gorm:"not null;autoUpdateTime:false"`
	UpdatedBy             string
}

// TableName pins the table name.
func (RDAPAccessPolicyRecord) TableName() string { return "rdap_access_policies" }

// RDAPAccessNetworkRecord assigns a network (a canonical CIDR) a policy. A
// policy assigned cannot be deleted.
type RDAPAccessNetworkRecord struct {
	Network string `gorm:"primaryKey;size:64"`
	Policy  string `gorm:"not null;size:32;index"`
}

// TableName pins the table name.
func (RDAPAccessNetworkRecord) TableName() string { return "rdap_access_networks" }

func toRDAPAccessRecord(p *entities.RDAPAccessPolicy) *RDAPAccessPolicyRecord {
	l := p.Limits
	return &RDAPAccessPolicyRecord{
		Name: p.Name, Description: p.Description,
		PerAddressPerSecond: l.PerAddress.PerSecond, PerAddressBurst: l.PerAddress.Burst,
		PerAggregatePerSecond: l.PerAggregate.PerSecond, PerAggregateBurst: l.PerAggregate.Burst,
		NotFoundCost: l.NotFoundCost, UpdatedAt: p.UpdatedAt, UpdatedBy: p.UpdatedBy,
	}
}

func (r *RDAPAccessPolicyRecord) toEntity() *entities.RDAPAccessPolicy {
	return &entities.RDAPAccessPolicy{
		Name: r.Name, Description: r.Description,
		Limits: entities.RDAPAccessLimits{
			PerAddress:   entities.EPPRate{PerSecond: r.PerAddressPerSecond, Burst: r.PerAddressBurst},
			PerAggregate: entities.EPPRate{PerSecond: r.PerAggregatePerSecond, Burst: r.PerAggregateBurst},
			NotFoundCost: r.NotFoundCost,
		},
		UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy,
	}
}

// RDAPAccessPolicyRepository is the Postgres
// repositories.RDAPAccessPolicyRepository.
type RDAPAccessPolicyRepository struct{ db *gorm.DB }

// NewRDAPAccessPolicyRepository returns the repository.
func NewRDAPAccessPolicyRepository(db *gorm.DB) *RDAPAccessPolicyRepository {
	return &RDAPAccessPolicyRepository{db: db}
}

func (r *RDAPAccessPolicyRepository) change(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return (&RDAPPublicationPolicyRepository{db: r.db}).change(ctx, fn)
}

func rdapAccessIn(tx *gorm.DB, name string) (*entities.RDAPAccessPolicy, error) {
	var rec RDAPAccessPolicyRecord
	err := tx.Where("name = ?", name).Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec.toEntity(), nil
}

// ListPolicies implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) ListPolicies(ctx context.Context) ([]*entities.RDAPAccessPolicy, error) {
	var recs []RDAPAccessPolicyRecord
	if err := r.db.WithContext(ctx).Order("name").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*entities.RDAPAccessPolicy, len(recs))
	for i := range recs {
		out[i] = recs[i].toEntity()
	}
	return out, nil
}

// GetPolicy implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) GetPolicy(ctx context.Context, name string) (*entities.RDAPAccessPolicy, error) {
	p, err := rdapAccessIn(r.db.WithContext(ctx), name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyNotFound, name)
	}
	return p, nil
}

// SavePolicy implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) SavePolicy(ctx context.Context, p *entities.RDAPAccessPolicy, events func(*entities.RDAPAccessPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := rdapAccessIn(tx, p.Name)
		if err != nil {
			return err
		}
		if err := tx.Save(toRDAPAccessRecord(p)).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// DeletePolicy implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) DeletePolicy(ctx context.Context, name string, events func(*entities.RDAPAccessPolicy) []entities.DomainEvent) error {
	return r.change(ctx, func(tx *gorm.DB) error {
		before, err := rdapAccessIn(tx, name)
		if err != nil {
			return err
		}
		if before == nil {
			return fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyNotFound, name)
		}
		var nets []string
		if err := tx.Model(&RDAPAccessNetworkRecord{}).Where("policy = ?", name).Order("network").Limit(5).Pluck("network", &nets).Error; err != nil {
			return err
		}
		if len(nets) > 0 {
			return fmt.Errorf("%w: assigned to %s", entities.ErrRDAPAccessPolicyInUse, strings.Join(nets, ", "))
		}
		if err := tx.Where("name = ?", name).Delete(&RDAPAccessPolicyRecord{}).Error; err != nil {
			return err
		}
		return writeEvents(tx, events(before))
	})
}

// ListNetworks implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) ListNetworks(ctx context.Context) ([]entities.RDAPAccessNetwork, error) {
	var recs []RDAPAccessNetworkRecord
	if err := r.db.WithContext(ctx).Order("network").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]entities.RDAPAccessNetwork, 0, len(recs))
	for _, rec := range recs {
		p, err := netip.ParsePrefix(rec.Network)
		if err != nil {
			return nil, fmt.Errorf("stored RDAP access network %q: %w", rec.Network, err)
		}
		out = append(out, entities.RDAPAccessNetwork{Network: p, Policy: rec.Policy})
	}
	return out, nil
}

// Snapshot implements repositories.RDAPAccessPolicyRepository: both tables in
// one read-only repeatable-read transaction on Postgres, so a policy created
// and assigned between the two reads cannot be half seen.
func (r *RDAPAccessPolicyRepository) Snapshot(ctx context.Context) ([]*entities.RDAPAccessPolicy, []entities.RDAPAccessNetwork, error) {
	var (
		policies []*entities.RDAPAccessPolicy
		networks []entities.RDAPAccessNetwork
	)
	opts := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	if r.db.Dialector.Name() != "postgres" {
		opts = nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		in := &RDAPAccessPolicyRepository{db: tx}
		var err error
		if policies, err = in.ListPolicies(ctx); err != nil {
			return err
		}
		networks, err = in.ListNetworks(ctx)
		return err
	}, opts)
	return policies, networks, err
}

// AssignNetwork implements repositories.RDAPAccessPolicyRepository.
func (r *RDAPAccessPolicyRepository) AssignNetwork(ctx context.Context, network netip.Prefix, policy string, events func(string) []entities.DomainEvent) error {
	key := network.Masked().String()
	return r.change(ctx, func(tx *gorm.DB) error {
		var before []string
		if err := tx.Model(&RDAPAccessNetworkRecord{}).Where("network = ?", key).Pluck("policy", &before).Error; err != nil {
			return err
		}
		if policy == "" {
			if err := tx.Where("network = ?", key).Delete(&RDAPAccessNetworkRecord{}).Error; err != nil {
				return err
			}
		} else {
			if p, err := rdapAccessIn(tx, policy); err != nil {
				return err
			} else if p == nil {
				return fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyNotFound, policy)
			}
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "network"}}, DoUpdates: clause.AssignmentColumns([]string{"policy"}),
			}).Create(&RDAPAccessNetworkRecord{Network: key, Policy: policy}).Error
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

// seedRDAPAccessPolicies stores the built-in access policies with their
// starting limits if they are not stored yet.
func seedRDAPAccessPolicies(db *gorm.DB) error {
	for _, p := range entities.BuiltInRDAPAccessPolicies() {
		p.UpdatedAt, p.UpdatedBy = time.Now().UTC(), "migrate"
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(toRDAPAccessRecord(p)).Error; err != nil {
			return fmt.Errorf("seed RDAP access policy %s: %w", p.Name, err)
		}
	}
	return nil
}
