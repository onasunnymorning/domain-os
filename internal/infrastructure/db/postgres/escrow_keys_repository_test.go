package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

const (
	ekFP1 = "0123456789ABCDEF0123456789ABCDEF01234567"
	ekFP2 = "FEDCBA9876543210FEDCBA9876543210FEDCBA98"
	ekSym = "ABCDEFGHIJKLMNOP"
)

type EscrowKeysSuite struct {
	suite.Suite
	db  *gorm.DB
	now time.Time
}

func TestEscrowKeysSuite(t *testing.T) {
	suite.Run(t, new(EscrowKeysSuite))
}

func (s *EscrowKeysSuite) SetupSuite() {
	s.db = setupTestDB()
	s.Require().NoError(AutoMigrate(s.db))
	s.now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
}

func (s *EscrowKeysSuite) op(id string) entities.OperatorID {
	o, err := entities.NewOperatorID(id)
	s.Require().NoError(err)
	return o
}

func (s *EscrowKeysSuite) actx() entities.EscrowKeyAuditContext {
	return entities.EscrowKeyAuditContext{Actor: "tester", TraceID: "trace-1", CorrelationID: "corr-1", At: s.now}
}

func (s *EscrowKeysSuite) createParty(tx *gorm.DB, owner entities.EscrowKeyOwner, kind entities.EscrowPartyKind, side entities.EscrowPartySide) *entities.EscrowParty {
	p, err := entities.NewEscrowParty(owner, "p-"+uuid.NewString()[:8], kind, side, "tester", s.now)
	s.Require().NoError(err)
	ev, err := entities.NewEscrowPartyAuditEvent(s.actx(), p, entities.EscrowAuditPartyCreated)
	s.Require().NoError(err)
	s.Require().NoError(NewEscrowPartyRepository(tx).Create(context.Background(), p, ev))
	return p
}

func (s *EscrowKeysSuite) newVersion(p *entities.EscrowParty, purpose entities.EscrowKeyPurpose, n int, fp string) *entities.EscrowKeyVersion {
	spec := entities.EscrowKeyVersionSpec{Party: p, Purpose: purpose, Version: n, Fingerprint: fp, CreatedBy: "tester", At: s.now}
	switch purpose {
	case entities.EscrowKeyPurposeVerifyInbound:
		spec.ArmoredPublicKey = evKey
	case entities.EscrowKeyPurposeDecryptInbound:
		spec.ArmoredPublicKey = evKey
		spec.SecretRef = &entities.EscrowSecretRef{Name: "escrow-keys/" + p.ID.String() + "/" + fp, BackendVersionID: "v1"}
	case entities.EscrowKeyPurposePseudonymise:
		spec.SecretRef = &entities.EscrowSecretRef{Name: "escrow-keys/" + p.ID.String() + "/sym"}
	}
	v, err := entities.NewEscrowKeyVersion(spec)
	s.Require().NoError(err)
	return v
}

func (s *EscrowKeysSuite) createVersion(tx *gorm.DB, v *entities.EscrowKeyVersion) error {
	ev, err := entities.NewEscrowKeyVersionAuditEvent(s.actx(), nil, v, entities.EscrowAuditVersionImported)
	s.Require().NoError(err)
	return NewEscrowKeyVersionRepository(tx).Create(context.Background(), v, ev)
}

func (s *EscrowKeysSuite) countOutbox(tx *gorm.DB, subject string) int64 {
	var n int64
	s.Require().NoError(tx.Model(&DomainEventRecord{}).Where("subject = ? AND source = ?", subject, "domain-os/escrow-keys").Count(&n).Error)
	return n
}

func (s *EscrowKeysSuite) TestParties_ScopeIsEnforcedInSQL() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewEscrowPartyRepository(tx)
	a, b := s.op("kopA"), s.op("kopB")
	platform := entities.PlatformEscrowKeyScope(entities.NewPlatformScope())

	eve := s.createParty(tx, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	rspA := s.createParty(tx, entities.OperatorKeyOwner(a), entities.EscrowPartyRSP, entities.EscrowPartyExternal)

	got, err := repo.GetByID(ctx, entities.OperatorEscrowKeyScope(a), eve.ID)
	s.Require().NoError(err, "operators see platform parties")
	s.Equal(eve.Name, got.Name)
	s.True(got.Owner.IsPlatform())

	got, err = repo.GetByID(ctx, entities.OperatorEscrowKeyScope(a), rspA.ID)
	s.Require().NoError(err)
	s.Equal(entities.OperatorKeyOwner(a), got.Owner)

	_, err = repo.GetByID(ctx, entities.OperatorEscrowKeyScope(b), rspA.ID)
	s.ErrorIs(err, entities.ErrEscrowPartyNotFound, "never another operator's")
	_, err = repo.GetByID(ctx, platform, rspA.ID)
	s.ErrorIs(err, entities.ErrEscrowPartyNotFound, "the platform scope does not see into operators")

	_, err = repo.GetByID(ctx, entities.EscrowKeyScope{}, eve.ID)
	s.Error(err, "a zero scope is rejected, not treated as 'all'")

	list, _, err := repo.List(ctx, entities.OperatorEscrowKeyScope(b), queries.ListItemsQuery{PageSize: 200})
	s.Require().NoError(err)
	for _, p := range list {
		s.NotEqual(rspA.ID, p.ID)
	}

	s.Equal(int64(1), s.countOutbox(tx, eve.ID.String()), "party creation emits its outbox event")
}

func (s *EscrowKeysSuite) TestParties_ListPaginates() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	op := s.op("kpag")
	for i := 0; i < 3; i++ {
		p, err := entities.NewEscrowParty(entities.OperatorKeyOwner(op), "rsp", entities.EscrowPartyRSP, entities.EscrowPartyExternal, "t", s.now.Add(time.Duration(i)*time.Minute))
		s.Require().NoError(err)
		ev, err := entities.NewEscrowPartyAuditEvent(s.actx(), p, entities.EscrowAuditPartyCreated)
		s.Require().NoError(err)
		s.Require().NoError(NewEscrowPartyRepository(tx).Create(ctx, p, ev))
	}
	scope := entities.OperatorEscrowKeyScope(op)
	seen := map[uuid.UUID]bool{}
	cursor := ""
	for page := 0; page < 10; page++ {
		items, next, err := NewEscrowPartyRepository(tx).List(ctx, scope, queries.ListItemsQuery{PageSize: 1, PageCursor: cursor})
		s.Require().NoError(err)
		for _, p := range items {
			if p.Owner.Operator == op {
				s.False(seen[p.ID], "no party repeats across pages")
				seen[p.ID] = true
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	s.Len(seen, 3)
}

func (s *EscrowKeysSuite) TestVersions_CreateNumberingAndDuplicates() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewEscrowKeyVersionRepository(tx)
	eve := s.createParty(tx, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	scope := entities.PlatformEscrowKeyScope(entities.NewPlatformScope())

	n, err := repo.NextVersionNumber(ctx, scope, eve.ID, entities.EscrowKeyPurposeDecryptInbound)
	s.Require().NoError(err)
	s.Equal(1, n)

	v1 := s.newVersion(eve, entities.EscrowKeyPurposeDecryptInbound, 1, ekFP1)
	s.Require().NoError(s.createVersion(tx, v1))
	n, err = repo.NextVersionNumber(ctx, scope, eve.ID, entities.EscrowKeyPurposeDecryptInbound)
	s.Require().NoError(err)
	s.Equal(2, n)
	n, err = repo.NextVersionNumber(ctx, scope, eve.ID, entities.EscrowKeyPurposePseudonymise)
	s.Require().NoError(err)
	s.Equal(1, n, "numbering is per purpose")

	got, err := repo.GetByID(ctx, scope, v1.ID)
	s.Require().NoError(err)
	s.Equal(ekFP1, got.Fingerprint)
	s.Require().NotNil(got.SecretRef)
	s.Equal(v1.SecretRef.Name, got.SecretRef.Name)
	s.Equal("v1", got.SecretRef.BackendVersionID)
	s.Equal(entities.EscrowKeyStaged, got.State)

	dupFP := s.newVersion(eve, entities.EscrowKeyPurposeDecryptInbound, 2, ekFP1)
	s.ErrorIs(s.createVersion(tx, dupFP), entities.ErrEscrowKeyDuplicateFingerprint)

	dupNumber := s.newVersion(eve, entities.EscrowKeyPurposeDecryptInbound, 1, ekFP2)
	s.ErrorIs(s.createVersion(tx, dupNumber), entities.ErrEscrowKeyVersionConflict)

	sym := s.newVersion(eve, entities.EscrowKeyPurposePseudonymise, 1, ekSym)
	s.Require().NoError(s.createVersion(tx, sym))
	all, err := repo.ListByParty(ctx, scope, eve.ID, "")
	s.Require().NoError(err)
	s.Len(all, 2)
	got, err = repo.GetByID(ctx, scope, sym.ID)
	s.Require().NoError(err)
	s.Empty(got.ArmoredPublicKey)

	_, err = repo.GetByID(ctx, entities.OperatorEscrowKeyScope(s.op("kopZ")), v1.ID)
	s.Require().NoError(err, "a platform version is visible to operators")
	byIDs, err := repo.GetByIDs(ctx, entities.OperatorEscrowKeyScope(s.op("kopZ")), []uuid.UUID{v1.ID, sym.ID, uuid.New()})
	s.Require().NoError(err)
	s.Len(byIDs, 2)
}

func (s *EscrowKeysSuite) TestVersions_TransitionsAreConditionalAndAtomicWithAudit() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewEscrowKeyVersionRepository(tx)
	eve := s.createParty(tx, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	scope := entities.PlatformEscrowKeyScope(entities.NewPlatformScope())

	v1 := s.newVersion(eve, entities.EscrowKeyPurposePseudonymise, 1, ekSym)
	s.Require().NoError(s.createVersion(tx, v1))
	v2 := s.newVersion(eve, entities.EscrowKeyPurposePseudonymise, 2, "QRSTUVWXYZ234567")
	s.Require().NoError(s.createVersion(tx, v2))

	// Probe and activate v1.
	before := v1.Clone()
	s.Require().NoError(v1.RecordProbe(true, s.now))
	s.Require().NoError(v1.Activate(s.now))
	ev, err := entities.NewEscrowKeyVersionAuditEvent(s.actx(), before, v1, entities.EscrowAuditVersionActivated)
	s.Require().NoError(err)
	s.Require().NoError(repo.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{{Version: v1, ExpectedState: entities.EscrowKeyStaged}}, []*entities.EscrowKeyAuditEvent{ev}))
	got, err := repo.GetByID(ctx, scope, v1.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowKeyActive, got.State)
	s.True(got.LastProbeOK)
	s.NotNil(got.ActivatedAt)

	// A stale expectation is a conflict.
	stale := got.Clone()
	s.Require().NoError(stale.Deactivate(s.now))
	ev, err = entities.NewEscrowKeyVersionAuditEvent(s.actx(), got, stale, entities.EscrowAuditVersionDeactivated)
	s.Require().NoError(err)
	s.ErrorIs(repo.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{{Version: stale, ExpectedState: entities.EscrowKeyStaged}}, []*entities.EscrowKeyAuditEvent{ev}),
		entities.ErrEscrowKeyVersionConflict)

	// Atomicity: rotate v1 -> v2, with the second transition's expectation
	// wrong. Neither the first update, nor any audit row, nor any outbox event
	// may survive.
	var auditBefore, outboxBefore int64
	s.Require().NoError(tx.Model(&EscrowKeyAuditEventRecord{}).Where("party_id = ?", eve.ID).Count(&auditBefore).Error)
	outboxBefore = s.countOutbox(tx, v1.ID.String()) + s.countOutbox(tx, v2.ID.String())

	v2.LastProbeOK = true
	deact, err := entities.PlanEscrowKeyActivation(v2, []*entities.EscrowKeyVersion{got}, true, s.now.Add(time.Hour))
	s.Require().NoError(err)
	s.Require().Len(deact, 1)
	ev1, err := entities.NewEscrowKeyVersionAuditEvent(s.actx(), nil, deact[0], entities.EscrowAuditVersionDeactivated)
	s.Require().NoError(err)
	ev2, err := entities.NewEscrowKeyVersionAuditEvent(s.actx(), nil, v2, entities.EscrowAuditVersionActivated)
	s.Require().NoError(err)
	err = repo.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{
		{Version: deact[0], ExpectedState: entities.EscrowKeyActive},
		{Version: v2, ExpectedState: entities.EscrowKeyActive}, // wrong: v2 is STAGED in the database
	}, []*entities.EscrowKeyAuditEvent{ev1, ev2})
	s.ErrorIs(err, entities.ErrEscrowKeyVersionConflict)

	got, err = repo.GetByID(ctx, scope, v1.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowKeyActive, got.State, "the first update was rolled back with the second")
	var auditAfter int64
	s.Require().NoError(tx.Model(&EscrowKeyAuditEventRecord{}).Where("party_id = ?", eve.ID).Count(&auditAfter).Error)
	s.Equal(auditBefore, auditAfter, "no audit row for a rolled-back change")
	s.Equal(outboxBefore, s.countOutbox(tx, v1.ID.String())+s.countOutbox(tx, v2.ID.String()), "no outbox event for a rolled-back change")

	// The same rotation with correct expectations commits both, with events.
	err = repo.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{
		{Version: deact[0], ExpectedState: entities.EscrowKeyActive},
		{Version: v2, ExpectedState: entities.EscrowKeyStaged},
	}, []*entities.EscrowKeyAuditEvent{ev1, ev2})
	s.Require().NoError(err)
	got, err = repo.GetByID(ctx, scope, v1.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowKeyHistorical, got.State)
	s.Equal(int64(3), s.countOutbox(tx, v1.ID.String()), "v1: imported, activated, deactivated — one event per committed change")

	// Without an audit event nothing is written.
	s.Error(repo.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{{Version: got, ExpectedState: entities.EscrowKeyHistorical}}, nil))
}

func (s *EscrowKeysSuite) TestArrangements_LiveRevisionsAndScope() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewEscrowArrangementRepository(tx)
	a, b := s.op("karA"), s.op("karB")
	platformScope := entities.PlatformEscrowKeyScope(entities.NewPlatformScope())
	scopeA := entities.OperatorEscrowKeyScope(a)

	eve := s.createParty(tx, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	rspA := s.createParty(tx, entities.OperatorKeyOwner(a), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	rspA2 := s.createParty(tx, entities.OperatorKeyOwner(a), entities.EscrowPartyRSP, entities.EscrowPartyExternal)

	replace := func(prev *entities.EscrowArrangement, spec entities.EscrowArrangementSpec) (*entities.EscrowArrangement, error) {
		spec.Direction, spec.At, spec.Previous = entities.EscrowArrangementInbound, s.now, prev
		next, err := entities.NewEscrowArrangement(spec)
		s.Require().NoError(err)
		if prev != nil {
			prevCopy := *prev
			prev = &prevCopy
		}
		ev, err := entities.NewEscrowArrangementAuditEvent(s.actx(), next, entities.EscrowAuditArrangementChanged)
		s.Require().NoError(err)
		return next, repo.Replace(ctx, prev, next, ev)
	}

	// The platform level may already hold a live row from another test run on a
	// shared database; supersede it so this test owns the slot.
	if live, err := repo.GetLive(ctx, platformScope, entities.EscrowArrangementPlatform, "", entities.EscrowArrangementInbound); err == nil {
		_, err := replace(live, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementPlatform, Receiver: eve})
		s.Require().NoError(err)
	} else {
		_, err := replace(nil, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementPlatform, Receiver: eve})
		s.Require().NoError(err)
	}

	op1, err := replace(nil, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: a, Depositor: rspA})
	s.Require().NoError(err)
	_, err = replace(nil, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: a, Depositor: rspA2})
	s.ErrorIs(err, entities.ErrEscrowArrangementConflict, "a second live row for the same slot is rejected")

	op2, err := replace(op1, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: a, Depositor: rspA2})
	s.Require().NoError(err)
	s.Equal(2, op2.Revision)
	_, err = replace(op1, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: a, Depositor: rspA})
	s.ErrorIs(err, entities.ErrEscrowArrangementConflict, "superseding a row that is no longer live fails")

	live, err := repo.GetLive(ctx, scopeA, entities.EscrowArrangementOperator, "", entities.EscrowArrangementInbound)
	s.Require().NoError(err)
	s.Equal(op2.ID, live.ID)
	s.Equal(rspA2.ID, *live.DepositorPartyID)

	_, err = repo.GetLive(ctx, entities.OperatorEscrowKeyScope(b), entities.EscrowArrangementOperator, "", entities.EscrowArrangementInbound)
	s.ErrorIs(err, entities.ErrEscrowArrangementNotFound, "operator B cannot read operator A's defaults")
	_, err = repo.GetLive(ctx, platformScope, entities.EscrowArrangementOperator, "", entities.EscrowArrangementInbound)
	s.ErrorIs(err, entities.ErrEscrowArrangementNotFound)
	pl, err := repo.GetLive(ctx, scopeA, entities.EscrowArrangementPlatform, "", entities.EscrowArrangementInbound)
	s.Require().NoError(err, "operators read the platform default they inherit")
	s.Equal(eve.ID, *pl.ReceiverPartyID)

	tldRow, err := replace(nil, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementTLD, Operator: a, TLD: "example", Depositor: rspA})
	s.Require().NoError(err)
	overrides, err := repo.ListLiveTLDOverrides(ctx, a, entities.EscrowArrangementInbound)
	s.Require().NoError(err)
	s.Len(overrides, 1)
	used, err := repo.ListLiveReferencing(ctx, scopeA, rspA.ID)
	s.Require().NoError(err)
	s.Len(used, 1, "rspA is only live in the TLD override now")

	s.Require().NoError(tldRow.Supersede(s.now))
	ev, err := entities.NewEscrowArrangementAuditEvent(s.actx(), tldRow, entities.EscrowAuditArrangementRemoved)
	s.Require().NoError(err)
	s.Require().NoError(repo.Remove(ctx, tldRow, ev))
	_, err = repo.GetLive(ctx, scopeA, entities.EscrowArrangementTLD, "example", entities.EscrowArrangementInbound)
	s.ErrorIs(err, entities.ErrEscrowArrangementNotFound, "removal makes the TLD inherit again")
	s.ErrorIs(repo.Remove(ctx, tldRow, ev), entities.ErrEscrowArrangementConflict)
}

func (s *EscrowKeysSuite) TestAudit_ListByPartyIsScopedAndPaginated() {
	tx := s.db.Begin()
	defer tx.Rollback()
	ctx := context.Background()
	a := s.op("kauA")
	rsp := s.createParty(tx, entities.OperatorKeyOwner(a), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	v := s.newVersion(rsp, entities.EscrowKeyPurposeVerifyInbound, 1, ekFP2)
	s.Require().NoError(s.createVersion(tx, v))

	repo := NewEscrowKeyAuditRepository(tx)
	page1, next, err := repo.ListByParty(ctx, entities.OperatorEscrowKeyScope(a), rsp.ID, queries.ListItemsQuery{PageSize: 1})
	s.Require().NoError(err)
	s.Len(page1, 1)
	s.NotEmpty(next)
	page2, next, err := repo.ListByParty(ctx, entities.OperatorEscrowKeyScope(a), rsp.ID, queries.ListItemsQuery{PageSize: 1, PageCursor: next})
	s.Require().NoError(err)
	s.Len(page2, 1)
	s.Empty(next)
	s.NotEqual(page1[0].ID, page2[0].ID)
	s.Equal("trace-1", page1[0].TraceID)

	other, _, err := repo.ListByParty(ctx, entities.OperatorEscrowKeyScope(s.op("kauB")), rsp.ID, queries.ListItemsQuery{PageSize: 10})
	s.Require().NoError(err)
	s.Empty(other, "another operator cannot read the trail")
}
