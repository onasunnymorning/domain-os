package services

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/mock"
)

// ── mockTLDRepository ──────────────────────────────────────────────────────────

type mockTLDRepository struct {
	mock.Mock
}

func (m *mockTLDRepository) Create(ctx context.Context, tld *entities.TLD) error {
	return m.Called(ctx, tld).Error(0)
}

func (m *mockTLDRepository) GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	args := m.Called(ctx, name, preloadAll)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.TLD), args.Error(1)
}

func (m *mockTLDRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]*entities.TLD), args.String(1), args.Error(2)
}

func (m *mockTLDRepository) Update(ctx context.Context, tld *entities.TLD) error {
	return m.Called(ctx, tld).Error(0)
}

func (m *mockTLDRepository) DeleteByName(ctx context.Context, name string) error {
	return m.Called(ctx, name).Error(0)
}

func (m *mockTLDRepository) Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(int64), args.Error(1)
}

// ── mockNNDNRepository ─────────────────────────────────────────────────────────

type mockNNDNRepository struct {
	mock.Mock
}

func (m *mockNNDNRepository) CreateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error) {
	args := m.Called(ctx, nndn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.NNDN), args.Error(1)
}

func (m *mockNNDNRepository) GetNNDN(ctx context.Context, name string) (*entities.NNDN, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.NNDN), args.Error(1)
}

func (m *mockNNDNRepository) UpdateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error) {
	args := m.Called(ctx, nndn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.NNDN), args.Error(1)
}

func (m *mockNNDNRepository) DeleteNNDN(ctx context.Context, name string) error {
	return m.Called(ctx, name).Error(0)
}

func (m *mockNNDNRepository) ListNNDNs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.NNDN, string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]*entities.NNDN), args.String(1), args.Error(2)
}

func (m *mockNNDNRepository) Count(ctx context.Context, filter queries.ListNndnsFilter) (int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(int64), args.Error(1)
}

// ── batchMockEventPublisher ────────────────────────────────────────────────────

type batchMockEventPublisher struct {
	mock.Mock
}

func (m *batchMockEventPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

// ── mockPhaseRepository ────────────────────────────────────────────────────────

type mockPhaseRepository struct {
	mock.Mock
}

func (m *mockPhaseRepository) CreatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	args := m.Called(ctx, phase)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Phase), args.Error(1)
}

func (m *mockPhaseRepository) GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error) {
	args := m.Called(ctx, tld, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Phase), args.Error(1)
}

func (m *mockPhaseRepository) DeletePhaseByTLDAndName(ctx context.Context, tld, name string) error {
	return m.Called(ctx, tld, name).Error(0)
}

func (m *mockPhaseRepository) UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	args := m.Called(ctx, phase)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Phase), args.Error(1)
}

func (m *mockPhaseRepository) ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	args := m.Called(ctx, tld, pageSize, pageCursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Phase), args.Error(1)
}

func (m *mockPhaseRepository) ListActiveGAPhases(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	args := m.Called(ctx, pageSize, pageCursor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Phase), args.Error(1)
}

// ── mockPremiumLabelRepository ─────────────────────────────────────────────────

type mockPremiumLabelRepository struct {
	mock.Mock
}

func (m *mockPremiumLabelRepository) Create(ctx context.Context, pl *entities.PremiumLabel) (*entities.PremiumLabel, error) {
	args := m.Called(ctx, pl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.PremiumLabel), args.Error(1)
}

func (m *mockPremiumLabelRepository) GetByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error) {
	args := m.Called(ctx, label, list, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.PremiumLabel), args.Error(1)
}

func (m *mockPremiumLabelRepository) DeleteByLabelListAndCurrency(ctx context.Context, label, list, currency string) error {
	return m.Called(ctx, label, list, currency).Error(0)
}

func (m *mockPremiumLabelRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumLabel, string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]*entities.PremiumLabel), args.String(1), args.Error(2)
}

// ── mockFXRepository ───────────────────────────────────────────────────────────

type mockFXRepository struct {
	mock.Mock
}

func (m *mockFXRepository) UpdateAll(ctx context.Context, fxs []*postgres.FX) error {
	return m.Called(ctx, fxs).Error(0)
}

func (m *mockFXRepository) ListByBaseCurrency(ctx context.Context, baseCurrency string) ([]*entities.FX, error) {
	args := m.Called(ctx, baseCurrency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.FX), args.Error(1)
}

func (m *mockFXRepository) GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error) {
	args := m.Called(ctx, baseCurrency, targetCurrency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.FX), args.Error(1)
}

// ── helper: construct a test DomainService ──────────────────────────────────────

func newTestBatchDomainService(
	domRepo *repositories.MockDomainRepository,
	hostRepo *repositories.MockHostRepository,
	rarRepo *repositories.MockRegistrarRepository,
	tldRepo *mockTLDRepository,
	nndnRepo *mockNNDNRepository,
	eventPub *batchMockEventPublisher,
	phaseRepo *mockPhaseRepository,
	premiumRepo *mockPremiumLabelRepository,
	fxRepo *mockFXRepository,
) *DomainService {
	idgen, _ := snowflakeidgenerator.NewIDGenerator()
	roidSvc := NewRoidService(idgen)
	return NewDomainService(domRepo, hostRepo, *roidSvc, nndnRepo, tldRepo, phaseRepo, premiumRepo, fxRepo, rarRepo, eventPub)
}
