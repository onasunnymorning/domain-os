package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

// PhaseRepository is the GORM postgres implementation of the PhaseRepository interface
type PhaseRepository struct {
	db *gorm.DB
}

// NewGormPhaseRepository creates a new instance of PhaseRepository
func NewGormPhaseRepository(db *gorm.DB) *PhaseRepository {
	return &PhaseRepository{db: db}
}

// CreatePhase creates a new phase
func (r *PhaseRepository) CreatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	gormPhase := &Phase{}
	gormPhase.FromEntity(phase)
	err := r.db.WithContext(ctx).Create(gormPhase).Error
	if err != nil {
		return nil, err
	}
	return gormPhase.ToEntity(), nil
}

// GetPhaseByTLDAndName gets a phase by its name
func (r *PhaseRepository) GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error) {
	var phase Phase
	err := r.db.WithContext(ctx).Preload("Fees").Preload("Prices").Where("name = ? AND tld_name = ?", name, tld).First(&phase).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrPhaseNotFound
		}
		return nil, err
	}
	return phase.ToEntity(), nil
}

// DeletePhaseByTLDAndName deletes a phase, within scope. A phase that does not
// exist, or whose TLD is outside the scope, is ErrPhaseNotFound.
func (r *PhaseRepository) DeletePhaseByTLDAndName(ctx context.Context, scope entities.RegistryScope, tld, name string) error {
	q, err := scopeToTLDs(r.db.WithContext(ctx).Where("name = ? AND tld_name = ?", name, tld), scope)
	if err != nil {
		return err
	}
	return deletedOrNotFound(q.Delete(&Phase{}), entities.ErrPhaseNotFound)
}

// UpdatePhase updates a phase. It will Omit Price and Fee updates. Use specific prices and fees repository for that
//
// An UPDATE rather than a Save, for the reasons in DomainRepository.UpdateDomain,
// and the tld_name in the WHERE keeps a phase from being moved to another TLD
// by an entity whose TLDName was changed (#415). A phase carries a TLD's launch
// policy, prices and fees, so moving one moves all three.
func (r *PhaseRepository) UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	gormPhase := &Phase{}
	gormPhase.FromEntity(phase)
	gormPhase.UpdatedAt = time.Now().UTC() // Save() used to stamp this; see UpdateDomain
	res := r.db.WithContext(ctx).
		Model(&Phase{}).
		Where("id = ? AND tld_name = ?", gormPhase.ID, gormPhase.TLDName).
		Select("*").
		Omit("Prices", "Fees").
		Updates(gormPhase)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, entities.ErrPhaseNotFound
	}
	return gormPhase.ToEntity(), nil
}

// ListPhasesByTLD lists all phases for a TLD
func (r *PhaseRepository) ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	var gormPhases []*Phase
	var pageCursorInt64 int64
	var err error
	// pageCursor for phases is of type int64, so convert it to int64
	// TODO: improve error handling
	if pageCursor != "" {
		pageCursorInt64, err = strconv.ParseInt(pageCursor, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	err = r.db.WithContext(ctx).Where("tld_name = ?", tld).Order("id ASC").Limit(pageSize).Find(&gormPhases, "id > ?", pageCursorInt64).Error
	if err != nil {
		return nil, err // this is hard to test
	}
	phases := make([]*entities.Phase, len(gormPhases))
	for i, phase := range gormPhases {
		phases[i] = phase.ToEntity()
	}
	return phases, nil
}

// ListActiveGAPhases lists all active phases for all TLDs
func (r *PhaseRepository) ListActiveGAPhases(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	var gormPhases []*Phase
	var pageCursorInt64 int64
	var err error
	// pageCursor for phases is of type int64, so convert it to int64
	if pageCursor != "" {
		pageCursorInt64, err = strconv.ParseInt(pageCursor, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	err = r.db.WithContext(ctx).Where("type = ?", "GA").Order("id ASC").Limit(pageSize).Find(&gormPhases, "id > ?", pageCursorInt64).Error
	if err != nil {
		return nil, err
	}
	phases := make([]*entities.Phase, len(gormPhases))
	for i, phase := range gormPhases {
		phases[i] = phase.ToEntity()
	}
	return phases, nil
}

// ListDistinctBaseCurrencies returns the distinct base currencies configured
// across all phases. Used by the FX update workflow to determine which base
// currencies need exchange rates for quoting.
func (r *PhaseRepository) ListDistinctBaseCurrencies(ctx context.Context) ([]string, error) {
	var currencies []string
	err := r.db.WithContext(ctx).
		Model(&Phase{}).
		Distinct("base_currency").
		Where("base_currency <> ''").
		Order("base_currency ASC").
		Pluck("base_currency", &currencies).Error
	if err != nil {
		return nil, err
	}
	return currencies, nil
}
