package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── helpers ────────────────────────────────────────────────────────────────────

// newTestTLDWithGAPhase builds a TLD with a current GA phase using default policy.
func newTestTLDWithGAPhase(tldName string) *entities.TLD {
	tld := &entities.TLD{
		Name: entities.DomainName(tldName),
		Type: entities.TLDTypeGTLD,
	}
	starts := time.Now().UTC().AddDate(-1, 0, 0) // started 1 year ago
	phase, _ := entities.NewPhase("GA-phase", string(entities.PhaseTypeGA), starts)
	phase.TLDName = entities.DomainName(tldName)
	tld.Phases = []entities.Phase{*phase}
	return tld
}

// newTestDomain builds a minimal domain in "ok" state with the given expiry.
func newTestDomain(name, clid, roid string, expiry time.Time) *entities.Domain {
	d, _ := entities.NewDomain(roid, name, clid, "Auth1234!@")
	d.ExpiryDate = expiry
	d.Status.OK = true
	return d
}

// ptrBool returns a pointer to a bool value.
func ptrBool(v bool) *bool { return &v }

// ────────────────────────────────────────────────────────────────────────────────
// BatchExpireDomains
// ────────────────────────────────────────────────────────────────────────────────

func TestBatchExpireDomains_Success(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	// Two expired domains under the same TLD
	dom1 := newTestDomain("alpha.com", "rar1", "1001_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	dom2 := newTestDomain("beta.com", "rar2", "1002_DOM-APEX", time.Now().UTC().AddDate(0, 0, -2))

	names := []string{"alpha.com", "beta.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom1, dom2}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	// UpdateDomain should succeed for each — the domain is mutated in-place before this call
	domRepo.On("UpdateDomain", mock.Anything, mock.MatchedBy(func(d *entities.Domain) bool {
		return d.Name.String() == "alpha.com"
	})).Return(dom1, nil)
	domRepo.On("UpdateDomain", mock.Anything, mock.MatchedBy(func(d *entities.Domain) bool {
		return d.Name.String() == "beta.com"
	})).Return(dom2, nil)

	// Event publishing
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchExpireDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 2)
	assert.Empty(t, result.Failed)
	domRepo.AssertNumberOfCalls(t, "UpdateDomain", 2)
}

func TestBatchExpireDomains_EmptyList(t *testing.T) {
	svc := newTestBatchDomainService(
		new(repositories.MockDomainRepository),
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	result := svc.BatchExpireDomains(context.Background(), []string{})

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
}

func TestBatchExpireDomains_DomainNotFound(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	// Return only one domain for two requested names
	dom1 := newTestDomain("alpha.com", "rar1", "1001_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	names := []string{"alpha.com", "missing.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom1}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)
	domRepo.On("UpdateDomain", mock.Anything, mock.AnythingOfType("*entities.Domain")).
		Return(dom1, nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchExpireDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 1)
	assert.Contains(t, result.Succeeded, "alpha.com")
	// A missing domain is not an error: it was deleted between listing and
	// processing (or purged by an earlier retry) — the batch converges.
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "missing.com")
}

func TestBatchExpireDomains_AlreadyPendingDelete_Skipped(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	// Domain already expired by a previous (partially completed) attempt
	dom := newTestDomain("done.com", "rar1", "1003_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	dom.Status.OK = false
	dom.Status.PendingDelete = true
	names := []string{"done.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	result := svc.BatchExpireDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "done.com")
	// No write must have happened — this is the retry-idempotency guarantee
	domRepo.AssertNotCalled(t, "UpdateDomain", mock.Anything, mock.Anything)
}

func TestBatchExpireDomains_PartialFailure(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom1 := newTestDomain("good.com", "rar1", "1001_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	dom2 := newTestDomain("bad.com", "rar2", "1002_DOM-APEX", time.Now().UTC().AddDate(0, 0, -2))
	names := []string{"good.com", "bad.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom1, dom2}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	// First UpdateDomain call succeeds, second fails
	domRepo.On("UpdateDomain", mock.Anything, mock.MatchedBy(func(d *entities.Domain) bool {
		return d.Name.String() == "good.com"
	})).Return(dom1, nil)
	domRepo.On("UpdateDomain", mock.Anything, mock.MatchedBy(func(d *entities.Domain) bool {
		return d.Name.String() == "bad.com"
	})).Return((*entities.Domain)(nil), errors.New("db connection lost"))

	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchExpireDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 1)
	assert.Contains(t, result.Succeeded, "good.com")
	assert.Len(t, result.Failed, 1)
	assert.Equal(t, "bad.com", result.Failed[0].DomainName)
	assert.Contains(t, result.Failed[0].Error, "update failed")
}

func TestBatchExpireDomains_FetchError(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"a.com", "b.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return(nil, errors.New("database unavailable"))

	result := svc.BatchExpireDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 2)
	for _, f := range result.Failed {
		assert.Contains(t, f.Error, "batch fetch failed")
	}
}

// ────────────────────────────────────────────────────────────────────────────────
// BatchPurgeDomains
// ────────────────────────────────────────────────────────────────────────────────

func TestBatchPurgeDomains_Success(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("purge-me.com", "rar1", "2001_DOM-APEX", time.Now().UTC().AddDate(-1, 0, 0))
	dom.RGPStatus.PurgeDate = time.Now().UTC().AddDate(0, 0, -1) // purge date in the past
	names := []string{"purge-me.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, true).
		Return([]*entities.Domain{dom}, nil)

	domRepo.On("DeleteDomainByName", mock.Anything, "purge-me.com").Return(nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchPurgeDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 1)
	assert.Contains(t, result.Succeeded, "purge-me.com")
	assert.Empty(t, result.Failed)
	domRepo.AssertCalled(t, "DeleteDomainByName", mock.Anything, "purge-me.com")
}

func TestBatchPurgeDomains_CannotBePurged(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("too-soon.com", "rar1", "2002_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	dom.RGPStatus.PurgeDate = time.Now().UTC().AddDate(0, 0, 10) // purge date in the future
	names := []string{"too-soon.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, true).
		Return([]*entities.Domain{dom}, nil)

	result := svc.BatchPurgeDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, "purge date is in the future")
}

func TestBatchPurgeDomains_WithDropCatch(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	nndnRepo := new(mockNNDNRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		nndnRepo,
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("drop.com", "rar1", "2003_DOM-APEX", time.Now().UTC().AddDate(-1, 0, 0))
	dom.RGPStatus.PurgeDate = time.Now().UTC().AddDate(0, 0, -1)
	dom.DropCatch = true
	names := []string{"drop.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, true).
		Return([]*entities.Domain{dom}, nil)

	expectedNNDN, _ := entities.NewNNDN("drop.com")
	nndnRepo.On("CreateNNDN", mock.Anything, mock.AnythingOfType("*entities.NNDN")).
		Return(expectedNNDN, nil)

	domRepo.On("DeleteDomainByName", mock.Anything, "drop.com").Return(nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchPurgeDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 1)
	assert.Empty(t, result.Failed)
	nndnRepo.AssertCalled(t, "CreateNNDN", mock.Anything, mock.AnythingOfType("*entities.NNDN"))
}

func TestBatchPurgeDomains_EmptyList(t *testing.T) {
	svc := newTestBatchDomainService(
		new(repositories.MockDomainRepository),
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	result := svc.BatchPurgeDomains(context.Background(), []string{})

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
}

func TestBatchPurgeDomains_FetchError(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"a.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, true).
		Return(nil, errors.New("db error"))

	result := svc.BatchPurgeDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, "batch fetch failed")
}

func TestBatchPurgeDomains_DomainNotFound(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"ghost.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, true).
		Return([]*entities.Domain{}, nil) // no domains returned

	result := svc.BatchPurgeDomains(context.Background(), names)

	// Already purged (e.g. by an earlier retry of the same batch) — a no-op,
	// not a failure. This is what makes purge retries converge.
	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "ghost.com")
	domRepo.AssertNotCalled(t, "DeleteDomainByName", mock.Anything, mock.Anything)
}

// ────────────────────────────────────────────────────────────────────────────────
// BatchAutoRenewDomains
// ────────────────────────────────────────────────────────────────────────────────

func TestBatchAutoRenewDomains_EmptyList(t *testing.T) {
	svc := newTestBatchDomainService(
		new(repositories.MockDomainRepository),
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	result := svc.BatchAutoRenewDomains(context.Background(), []string{}, 1)

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
}

func TestBatchAutoRenewDomains_FetchError(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"renew.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return(nil, errors.New("connection refused"))

	result := svc.BatchAutoRenewDomains(context.Background(), names, 1)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, "batch fetch failed")
}

func TestBatchAutoRenewDomains_RegistrarDisabled(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	rarRepo := new(repositories.MockRegistrarRepository)
	tldRepo := new(mockTLDRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		rarRepo,
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("norenew.com", "rar-no-ar", "3001_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	names := []string{"norenew.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	// Registrar has Autorenew = false
	rar := &entities.Registrar{
		ClID:      entities.ClIDType("rar-no-ar"),
		Autorenew: false,
	}
	rarRepo.On("GetByClID", mock.Anything, "rar-no-ar", false).Return(rar, nil)

	result := svc.BatchAutoRenewDomains(context.Background(), names, 1)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, ErrAutoRenewNotEnabledRar.Error())
}

func TestBatchAutoRenewDomains_PhaseDisallowsAutoRenew(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("noar.com", "rar1", "3002_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	names := []string{"noar.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	// Build a TLD where the GA phase disallows auto-renew
	tld := newTestTLDWithGAPhase("com")
	tld.Phases[0].Policy.AllowAutoRenew = ptrBool(false)
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	result := svc.BatchAutoRenewDomains(context.Background(), names, 1)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, ErrAutoRenewNotEnabledTLD.Error())
}

func TestBatchAutoRenewDomains_DomainNotFound(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"ghost.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{}, nil)

	result := svc.BatchAutoRenewDomains(context.Background(), names, 1)

	// Deleted between listing and processing — a no-op, not a failure.
	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "ghost.com")
}

func TestBatchAutoRenewDomains_FutureExpiry_Skipped(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	// The domain's expiry is in the future: an earlier (partially completed)
	// attempt already renewed it. Retrying the batch must NOT renew it again.
	dom := newTestDomain("alreadyrenewed.com", "rar1", "3003_DOM-APEX", time.Now().UTC().AddDate(1, 0, 0))
	names := []string{"alreadyrenewed.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	result := svc.BatchAutoRenewDomains(context.Background(), names, 1)

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "alreadyrenewed.com")
	// No write and no billing event — the double-renewal guard
	domRepo.AssertNotCalled(t, "UpdateDomain", mock.Anything, mock.Anything)
}

// ────────────────────────────────────────────────────────────────────────────────
// BatchRestoreDomains
// ────────────────────────────────────────────────────────────────────────────────

func TestBatchRestoreDomains_EmptyList(t *testing.T) {
	svc := newTestBatchDomainService(
		new(repositories.MockDomainRepository),
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	result := svc.BatchRestoreDomains(context.Background(), []string{})

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
}

func TestBatchRestoreDomains_FetchError(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"restore.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return(nil, errors.New("db timeout"))

	result := svc.BatchRestoreDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, "batch fetch failed")
}

func TestBatchRestoreDomains_DomainNotFound(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"nothere.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{}, nil)

	result := svc.BatchRestoreDomains(context.Background(), names)

	// Deleted between listing and processing — a no-op, not a failure.
	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "nothere.com")
}

// newPendingRestoreDomain builds a domain in pendingRestore state with an
// expiry date in the recent past, as left behind by Domain.Restore().
func newPendingRestoreDomain(name, clid, roid string) *entities.Domain {
	d, _ := entities.NewDomain(roid, name, clid, "Auth1234!@")
	d.ExpiryDate = time.Now().UTC().AddDate(0, 0, -20)
	d.Status.OK = false
	d.Status.PendingRestore = true
	return d
}

// newTestTLDWithGAPhaseAndPrice builds a TLD whose GA phase carries a USD
// price point so quote generation succeeds.
func newTestTLDWithGAPhaseAndPrice(tldName string) *entities.TLD {
	tld := newTestTLDWithGAPhase(tldName)
	price, _ := entities.NewPrice("USD", 1000, 1000, 1000, 1000)
	_, _ = tld.Phases[0].AddPrice(*price)
	return tld
}

func TestBatchRestoreDomains_Success_SingleAtomicWrite(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	eventPub := new(batchMockEventPublisher)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		eventPub,
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newPendingRestoreDomain("restoreme.com", "rar1", "4002_DOM-APEX")
	expiryBefore := dom.ExpiryDate
	names := []string{"restoreme.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	tld := newTestTLDWithGAPhaseAndPrice("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	// GetQuote fetches the domain again internally
	domRepo.On("GetDomainByName", mock.Anything, "restoreme.com", false).Return(dom, nil)

	domRepo.On("UpdateDomain", mock.Anything, mock.AnythingOfType("*entities.Domain")).Return(dom, nil)
	eventPub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result := svc.BatchRestoreDomains(context.Background(), names)

	assert.Len(t, result.Succeeded, 1)
	assert.Contains(t, result.Succeeded, "restoreme.com")
	assert.Empty(t, result.Failed)
	assert.Empty(t, result.Skipped)

	// Both mutations landed in ONE write — no half-restored state is possible
	domRepo.AssertNumberOfCalls(t, "UpdateDomain", 1)
	assert.False(t, dom.Status.PendingRestore, "PendingRestore must be unset")
	assert.Equal(t, expiryBefore.AddDate(1, 0, 0), dom.ExpiryDate, "expiry must be extended by exactly 1 year")
}

func TestBatchRestoreDomains_NotPendingRestore_Skipped(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	// Restore already completed by an earlier (retried) attempt
	dom := newTestDomain("alreadydone.com", "rar1", "4003_DOM-APEX", time.Now().UTC().AddDate(1, 0, 0))
	names := []string{"alreadydone.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	result := svc.BatchRestoreDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)
	assert.Len(t, result.Skipped, 1)
	assert.Contains(t, result.Skipped, "alreadydone.com")
	// No write and no second force-renew — the retry-idempotency guarantee
	domRepo.AssertNotCalled(t, "UpdateDomain", mock.Anything, mock.Anything)
}

func TestBatchRestoreDomains_QuoteFails_NoStateChange(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newPendingRestoreDomain("quotefail.com", "rar1", "4004_DOM-APEX")
	names := []string{"quotefail.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	tld := newTestTLDWithGAPhaseAndPrice("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	// GetQuote fetches the domain internally — make it fail
	domRepo.On("GetDomainByName", mock.Anything, "quotefail.com", false).
		Return((*entities.Domain)(nil), errors.New("db timeout"))

	result := svc.BatchRestoreDomains(context.Background(), names)

	assert.Empty(t, result.Succeeded)
	assert.Len(t, result.Failed, 1)
	assert.Contains(t, result.Failed[0].Error, "quote failed")
	// Quote failure must not mutate the domain
	assert.True(t, dom.Status.PendingRestore, "PendingRestore must remain set")
	domRepo.AssertNotCalled(t, "UpdateDomain", mock.Anything, mock.Anything)
}

// ────────────────────────────────────────────────────────────────────────────────
// PartitionExpiredDomains
// ────────────────────────────────────────────────────────────────────────────────

func TestPartitionExpiredDomains_EmptyList(t *testing.T) {
	svc := newTestBatchDomainService(
		new(repositories.MockDomainRepository),
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	result := svc.PartitionExpiredDomains(context.Background(), []string{})

	assert.Empty(t, result.EligibleForAutoRenew)
	assert.Empty(t, result.EligibleForExpiry)
	assert.Empty(t, result.Skipped)
	assert.Empty(t, result.Failures)
}

func TestPartitionExpiredDomains_Mixed(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	rarRepo := new(repositories.MockRegistrarRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		rarRepo,
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	expired := time.Now().UTC().AddDate(0, 0, -1)

	// Eligible: expired, renewable status, registrar opted in
	domRenew := newTestDomain("renew.com", "rar-on", "6001_DOM-APEX", expired)
	// Expiry route: registrar opted out
	domExpireRar := newTestDomain("expire-rar.com", "rar-off", "6002_DOM-APEX", expired)
	// Expiry route: status prohibits renewal
	domProhibited := newTestDomain("prohibited.com", "rar-on", "6003_DOM-APEX", expired)
	domProhibited.Status.OK = false
	domProhibited.Status.ClientRenewProhibited = true
	// Skipped: already renewed by an earlier attempt
	domFuture := newTestDomain("future.com", "rar-on", "6004_DOM-APEX", time.Now().UTC().AddDate(1, 0, 0))
	// Skipped: already expired (pendingDelete)
	domPD := newTestDomain("pendingdelete.com", "rar-on", "6005_DOM-APEX", expired)
	domPD.Status.OK = false
	domPD.Status.PendingDelete = true

	names := []string{"renew.com", "expire-rar.com", "prohibited.com", "future.com", "pendingdelete.com", "ghost.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{domRenew, domExpireRar, domProhibited, domFuture, domPD}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	rarOn := &entities.Registrar{ClID: entities.ClIDType("rar-on"), Autorenew: true}
	rarOff := &entities.Registrar{ClID: entities.ClIDType("rar-off"), Autorenew: false}
	rarRepo.On("GetByClID", mock.Anything, "rar-on", false).Return(rarOn, nil)
	rarRepo.On("GetByClID", mock.Anything, "rar-off", false).Return(rarOff, nil)

	result := svc.PartitionExpiredDomains(context.Background(), names)

	assert.Equal(t, []string{"renew.com"}, result.EligibleForAutoRenew)
	assert.ElementsMatch(t, []string{"expire-rar.com", "prohibited.com"}, result.EligibleForExpiry)
	assert.ElementsMatch(t, []string{"future.com", "pendingdelete.com", "ghost.com"}, result.Skipped)
	assert.Empty(t, result.Failures)

	// Lookups must be amortized: one TLD fetch, one registrar fetch per ClID
	tldRepo.AssertNumberOfCalls(t, "GetByName", 1)
	rarRepo.AssertNumberOfCalls(t, "GetByClID", 2)
}

func TestPartitionExpiredDomains_PhaseDisallowsAutoRenew(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("noar.com", "rar1", "6006_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	names := []string{"noar.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	tld := newTestTLDWithGAPhase("com")
	tld.Phases[0].Policy.AllowAutoRenew = ptrBool(false)
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	result := svc.PartitionExpiredDomains(context.Background(), names)

	assert.Empty(t, result.EligibleForAutoRenew)
	assert.Equal(t, []string{"noar.com"}, result.EligibleForExpiry)
	assert.Empty(t, result.Failures)
}

func TestPartitionExpiredDomains_RegistrarLookupFails(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)
	tldRepo := new(mockTLDRepository)
	rarRepo := new(repositories.MockRegistrarRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		rarRepo,
		tldRepo,
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	dom := newTestDomain("lookupfail.com", "rar1", "6007_DOM-APEX", time.Now().UTC().AddDate(0, 0, -1))
	names := []string{"lookupfail.com"}

	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return([]*entities.Domain{dom}, nil)

	tld := newTestTLDWithGAPhase("com")
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)

	rarRepo.On("GetByClID", mock.Anything, "rar1", false).
		Return((*entities.Registrar)(nil), errors.New("registrar db down"))

	result := svc.PartitionExpiredDomains(context.Background(), names)

	assert.Empty(t, result.EligibleForAutoRenew)
	assert.Empty(t, result.EligibleForExpiry)
	assert.Len(t, result.Failures, 1)
	assert.Equal(t, "lookupfail.com", result.Failures[0].DomainName)
	assert.Contains(t, result.Failures[0].Error, "registrar lookup failed")
}

func TestPartitionExpiredDomains_FetchError(t *testing.T) {
	domRepo := new(repositories.MockDomainRepository)

	svc := newTestBatchDomainService(
		domRepo,
		repositories.NewMockHostRepository(),
		new(repositories.MockRegistrarRepository),
		new(mockTLDRepository),
		new(mockNNDNRepository),
		new(batchMockEventPublisher),
		new(mockPhaseRepository),
		new(mockPremiumLabelRepository),
		new(mockFXRepository),
	)

	names := []string{"a.com", "b.com"}
	domRepo.On("GetDomainsByNames", mock.Anything, names, false).
		Return(nil, errors.New("database unavailable"))

	result := svc.PartitionExpiredDomains(context.Background(), names)

	assert.Len(t, result.Failures, 2)
	for _, f := range result.Failures {
		assert.Contains(t, f.Error, "batch fetch failed")
	}
}
