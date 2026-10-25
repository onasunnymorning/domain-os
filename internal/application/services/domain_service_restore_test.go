package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// The restore's pricing, events and writes, with the repositories mocked.
// The rules themselves are tested with entities.Domain (restore_test.go),
// and the EPP command end to end in internal/interface/epp.

// restoreTestPrices are the GA phase's prices, distinct so each quote shows
// which transaction it priced.
const (
	restoreTestRenewal = 2000
	restoreTestRestore = 4000
)

// restoreDomains is a domain repository holding domains by name. Reads
// return copies; writes store a copy and keep the poll messages they queue,
// unless changed says to refuse the next writes of a domain as
// entities.ErrDomainChanged.
type restoreDomains struct {
	repositories.DomainRepository
	byName   map[string]*entities.Domain
	changed  map[string]int
	readErr  error
	writes   int
	messages []*entities.PollMessage // queued by the writes made
}

func (r *restoreDomains) GetDomainByName(_ context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	d, ok := r.byName[name]
	if !ok {
		return nil, entities.ErrDomainNotFound
	}
	c := d.DeepCopy()
	if !preloadHosts {
		c.Hosts = nil // as the repository reads it
	}
	return c, nil
}

func (r *restoreDomains) UpdateDomainColumnsIfUnchanged(_ context.Context, d *entities.Domain, msgs ...*entities.PollMessage) (*entities.Domain, error) {
	r.writes++
	if r.changed[d.Name.String()] > 0 {
		r.changed[d.Name.String()]--
		return nil, entities.ErrDomainChanged
	}
	r.byName[d.Name.String()] = d.DeepCopy()
	r.messages = append(r.messages, msgs...)
	return d.DeepCopy(), nil
}

// add stores domains.
func (r *restoreDomains) add(doms ...*entities.Domain) {
	for _, d := range doms {
		r.byName[d.Name.String()] = d.DeepCopy()
	}
}

// restoreTestService is a DomainService over restoreDomains, with a GA
// phase priced at the restoreTest prices; setPolicy, if given, changes the
// phase's policy. It returns the events published.
func restoreTestService(t *testing.T, setPolicy func(*entities.PhasePolicy)) (*DomainService, *restoreDomains, *[]entities.DomainEvent) {
	t.Helper()
	domains := &restoreDomains{byName: map[string]*entities.Domain{}, changed: map[string]int{}}
	tldRepo := new(mockTLDRepository)
	pub := new(batchMockEventPublisher)
	tld := newTestTLDWithGAPhase("com")
	price, err := entities.NewPrice("USD", 1000, restoreTestRenewal, 3000, restoreTestRestore)
	require.NoError(t, err)
	_, err = tld.Phases[0].AddPrice(*price)
	require.NoError(t, err)
	if setPolicy != nil {
		setPolicy(&tld.Phases[0].Policy)
	}
	tldRepo.On("GetByName", mock.Anything, "com", true).Return(tld, nil)
	var events []entities.DomainEvent
	pub.On("Publish", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		events = append(events, args.Get(1).([]entities.DomainEvent)...)
	}).Return(nil)
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	require.NoError(t, err)
	svc := NewDomainService(domains, repositories.NewMockHostRepository(), *NewRoidService(idgen, "TEST"), new(mockNNDNRepository),
		tldRepo, new(mockPhaseRepository), new(mockPremiumLabelRepository), new(mockFXRepository), new(repositories.MockRegistrarRepository), pub, passThroughTx{})
	return svc, domains, &events
}

// deletedForRestore is a domain deleted a day ago, in its redemption
// period, that expired expiredDaysAgo days ago (negative: expires later).
func deletedForRestore(name string, expiredDaysAgo int) *entities.Domain {
	now := time.Now().UTC()
	d, _ := entities.NewDomain("4100_DOM-APEX", name, "rar1", "")
	d.ExpiryDate = now.AddDate(0, 0, -expiredDaysAgo)
	d.Status = entities.DomainStatus{PendingDelete: true, Inactive: true}
	d.RGPStatus.RedemptionPeriodEnd = now.AddDate(0, 0, 29)
	d.RGPStatus.PurgeDate = now.AddDate(0, 0, 34)
	d.UpdatedAt = now.Add(-time.Hour)
	return d
}

func lifecycle(t *testing.T, e entities.DomainEvent) *entities.DomainLifeCycleEvent {
	t.Helper()
	le, ok := e.Data.(*entities.DomainLifeCycleEvent)
	require.True(t, ok, "event data is %T", e.Data)
	return le
}

func eventTypes(events []entities.DomainEvent) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func TestRequestRestoreAsRead(t *testing.T) {
	t.Run("pendingRestore, recorded without a charge", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		dom := deletedForRestore("restoreme.com", 10)
		domains.add(dom) // GetQuote reads it

		stored, err := svc.RequestRestoreAsRead(context.Background(), dom, "rar1")
		require.NoError(t, err)
		assert.True(t, stored.Status.PendingRestore)
		assert.True(t, stored.Status.PendingDelete)
		assert.False(t, stored.RGPStatus.PendingRestoreEnd.IsZero())
		assert.Equal(t, entities.ClIDType("rar1"), stored.UpRr)
		assert.Equal(t, 1, domains.writes)
		require.Equal(t, []string{EventDomainRestoreRequested}, eventTypes(*events))
		le := lifecycle(t, (*events)[0])
		assert.Equal(t, entities.TransactionTypeRestore, le.TransactionType)
		assert.Zero(t, le.DomainYears)
		assert.Nil(t, le.Quote.Price, "a request is not charged")
	})
	t.Run("a phase without the report restores at once, with both charges", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, func(p *entities.PhasePolicy) {
			no := false
			p.RestoreReportRequired = &no
		})
		dom := deletedForRestore("noreport.com", 10)
		expiry := dom.ExpiryDate
		domains.add(dom)

		stored, err := svc.RequestRestoreAsRead(context.Background(), dom, "rar1")
		require.NoError(t, err)
		assert.False(t, stored.Status.PendingRestore)
		assert.False(t, stored.Status.PendingDelete)
		assert.Equal(t, expiry.AddDate(1, 0, 0), stored.ExpiryDate)
		assert.Equal(t, 1, domains.writes)
		assert.Equal(t, []string{EventDomainRestoreRequested, EventDomainRestored, EventDomainRestoreRenewed}, eventTypes(*events))
		assert.Nil(t, (*events)[1].Command, "no report")
	})
	t.Run("not in redemption: refused, nothing written", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		dom := deletedForRestore("late.com", 40)
		dom.RGPStatus.RedemptionPeriodEnd = time.Now().UTC().Add(-time.Hour)
		domains.add(dom)
		_, err := svc.RequestRestoreAsRead(context.Background(), dom, "rar1")
		assert.ErrorIs(t, err, entities.ErrDomainRestoreNotAllowed)
		assert.Zero(t, domains.writes)
		assert.Empty(t, *events)
	})
	t.Run("a restore that cannot be priced: refused, nothing written", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		dom := deletedForRestore("noquote.com", 10)
		domains.readErr = errors.New("db timeout") // GetQuote cannot read the domain
		_, err := svc.RequestRestoreAsRead(context.Background(), dom, "rar1")
		assert.Error(t, err)
		assert.Zero(t, domains.writes)
		assert.Empty(t, *events)
	})
}

func TestCompleteRestoreAsRead(t *testing.T) {
	now := time.Now().UTC()
	report := entities.RestoreReport{
		PreData: "before", PostData: "after", DelTime: now.Add(-48 * time.Hour), ResTime: now.Add(-time.Hour),
		ResReason: "Registrant error.", Statements: []string{"one", "two"},
	}
	requested := func(t *testing.T, name string, expiredDaysAgo int) *entities.Domain {
		d := deletedForRestore(name, expiredDaysAgo)
		require.NoError(t, d.RequestRestore(&entities.Phase{Policy: entities.NewPhasePolicy()}, now.Add(-time.Hour)))
		return d
	}

	t.Run("two charges: the restore fee and the renewal for the years added", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		dom := requested(t, "longgone.com", 400) // expired over a year ago: two years to bring it to the future
		domains.add(dom)

		stored, err := svc.CompleteRestoreAsRead(context.Background(), dom, report, "rar1")
		require.NoError(t, err)
		assert.False(t, stored.Status.PendingRestore)
		assert.False(t, stored.Status.PendingDelete)
		assert.True(t, stored.ExpiryDate.After(now))
		require.Equal(t, []string{EventDomainRestored, EventDomainRestoreRenewed}, eventTypes(*events))

		restored, renewed := (*events)[0], (*events)[1]
		assert.Equal(t, &report, restored.Command, "the report is kept with the event")
		le := lifecycle(t, restored)
		assert.Equal(t, entities.TransactionTypeRestore, le.TransactionType)
		require.NotNil(t, le.Quote.Price)
		assert.Equal(t, int64(restoreTestRestore), le.Quote.Price.Amount())

		le = lifecycle(t, renewed)
		assert.Equal(t, entities.TransactionTypeRenewal, le.TransactionType)
		assert.Equal(t, 2, le.DomainYears)
		require.NotNil(t, le.Quote.Price)
		assert.Equal(t, int64(2*restoreTestRenewal), le.Quote.Price.Amount())
	})
	t.Run("no restore requested: refused", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		dom := deletedForRestore("notrequested.com", 10)
		domains.add(dom)
		_, err := svc.CompleteRestoreAsRead(context.Background(), dom, report, "rar1")
		assert.ErrorIs(t, err, entities.ErrRestoreNotPending)
		assert.Zero(t, domains.writes)
		assert.Empty(t, *events)
	})
	t.Run("an incomplete report: refused", func(t *testing.T) {
		svc, domains, _ := restoreTestService(t, nil)
		dom := requested(t, "incomplete.com", 10)
		domains.add(dom)
		r := report
		r.Statements = []string{"one"}
		_, err := svc.CompleteRestoreAsRead(context.Background(), dom, r, "rar1")
		assert.ErrorIs(t, err, entities.ErrRestoreReportIncomplete)
		assert.Zero(t, domains.writes)
	})
	t.Run("the admin API keeps a domain with nameservers active", func(t *testing.T) {
		svc, domains, _ := restoreTestService(t, func(p *entities.PhasePolicy) {
			no := false
			p.RestoreReportRequired = &no
		})
		dom := deletedForRestore("delegated.com", 10)
		dom.Hosts = []*entities.Host{{Name: "ns1.example.net"}}
		dom.Status.Inactive = false
		domains.add(dom)
		stored, err := svc.RestoreDomain(context.Background(), "delegated.com")
		require.NoError(t, err)
		assert.False(t, stored.Status.Inactive, "read with its nameservers")
		assert.True(t, stored.Status.OK)
	})
	t.Run("the admin API: request, then report", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		domains.add(deletedForRestore("admin.com", 10))
		_, err := svc.RestoreDomain(context.Background(), "admin.com")
		require.NoError(t, err)
		stored, err := svc.ReportRestore(context.Background(), "admin.com", report)
		require.NoError(t, err)
		assert.False(t, stored.Status.PendingDelete)
		assert.Equal(t, []string{EventDomainRestoreRequested, EventDomainRestored, EventDomainRestoreRenewed}, eventTypes(*events))
	})
}

func TestBatchRevertLapsedRestores(t *testing.T) {
	now := time.Now().UTC()
	lapsedDomain := func(name string) *entities.Domain {
		d := deletedForRestore(name, 10)
		require.NoError(t, d.RequestRestore(&entities.Phase{Policy: entities.NewPhasePolicy()}, now.AddDate(0, 0, -8)))
		return d
	}

	t.Run("reverts what lapsed; skips the rest", func(t *testing.T) {
		svc, domains, events := restoreTestService(t, nil)
		lapsed := lapsedDomain("lapsed.com")
		pending := deletedForRestore("pending.com", 10)
		require.NoError(t, pending.RequestRestore(&entities.Phase{Policy: entities.NewPhasePolicy()}, now))
		domains.add(lapsed, pending, newTestDomain("restored.com", "rar1", "4101_DOM-APEX", now.AddDate(1, 0, 0)))

		res := svc.BatchRevertLapsedRestores(context.Background(), []string{"lapsed.com", "pending.com", "restored.com", "purged.com"}, now)
		assert.Equal(t, []string{"lapsed.com"}, res.Succeeded)
		assert.ElementsMatch(t, []string{"pending.com", "restored.com", "purged.com"}, res.Skipped)
		assert.Empty(t, res.Failed)
		got := domains.byName["lapsed.com"]
		assert.False(t, got.Status.PendingRestore)
		assert.True(t, got.Status.PendingDelete)
		assert.Equal(t, lapsed.RGPStatus.RedemptionPeriodEnd, got.RGPStatus.RedemptionPeriodEnd)
		assert.Equal(t, lapsed.RGPStatus.PurgeDate, got.RGPStatus.PurgeDate)
		assert.Equal(t, 1, domains.writes)
		require.Equal(t, []string{EventDomainRestoreReverted}, eventTypes(*events))
		assert.Nil(t, lifecycle(t, (*events)[0]).Quote.Price, "a reversion charges nothing")
		require.Len(t, domains.messages, 1, "the sponsor is told, with the write")
		m := domains.messages[0]
		assert.Equal(t, entities.ClIDType("rar1"), m.ClID)
		assert.Nil(t, m.Transfer)
		assert.Contains(t, m.Message, entities.PollMsgRestoreLapsed+": no restore report for lapsed.com")
		assert.Contains(t, m.Message, "back in its redemption period")

		again := svc.BatchRevertLapsedRestores(context.Background(), []string{"lapsed.com"}, now)
		assert.Equal(t, []string{"lapsed.com"}, again.Skipped, "a retry never reverts twice")
	})
	t.Run("a domain changed meanwhile is read again", func(t *testing.T) {
		svc, domains, _ := restoreTestService(t, nil)
		domains.add(lapsedDomain("busy.com"))
		domains.changed["busy.com"] = 1
		res := svc.BatchRevertLapsedRestores(context.Background(), []string{"busy.com"}, now)
		assert.Equal(t, []string{"busy.com"}, res.Succeeded)
		assert.Equal(t, 2, domains.writes)
		assert.Len(t, domains.messages, 1, "no message for the write that was refused")
	})
	t.Run("a domain that keeps changing fails, and the others go on", func(t *testing.T) {
		svc, domains, _ := restoreTestService(t, nil)
		domains.add(lapsedDomain("busy.com"), lapsedDomain("fine.com"))
		domains.changed["busy.com"] = updateAttempts
		res := svc.BatchRevertLapsedRestores(context.Background(), []string{"busy.com", "fine.com"}, now)
		require.Len(t, res.Failed, 1)
		assert.Equal(t, "busy.com", res.Failed[0].DomainName)
		assert.Equal(t, []string{"fine.com"}, res.Succeeded)
	})
}
