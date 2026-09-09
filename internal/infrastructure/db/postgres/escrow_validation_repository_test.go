package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

const (
	evSHA1 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	evSHA2 = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	evKey  = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQ==\n-----END PGP PUBLIC KEY BLOCK-----"
)

type EscrowValidationSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestEscrowValidationSuite(t *testing.T) {
	suite.Run(t, new(EscrowValidationSuite))
}

func (s *EscrowValidationSuite) SetupSuite() {
	s.db = setupTestDB()
	s.Require().NoError(AutoMigrate(s.db))
}

func (s *EscrowValidationSuite) scope(id string) entities.OperatorID {
	sc, err := entities.NewOperatorID(id)
	s.Require().NoError(err)
	return sc
}

func (s *EscrowValidationSuite) newDeposit(scope entities.OperatorID, tld, ryde, sig string) *entities.EscrowDeposit {
	d, err := entities.NewEscrowDeposit(scope, tld, time.Now().UTC(), "tester", "ref", "k/"+ryde[:8]+".ryde", "k/"+sig[:8]+".sig", ryde, sig, 100, 10)
	s.Require().NoError(err)
	return d
}

func (s *EscrowValidationSuite) TestDeposits_TenantIsolationAndDigests() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewEscrowDepositRepository(tx)
	ctx := context.Background()
	a, b := s.scope("opA"), s.scope("opB")

	d := s.newDeposit(a, "example", evSHA1, evSHA2)
	s.Require().NoError(repo.Create(ctx, d))

	got, err := repo.GetByID(ctx, a, d.ID)
	s.Require().NoError(err)
	s.Equal(d.ID, got.ID)
	s.Equal(a, got.TenantID)

	_, err = repo.GetByID(ctx, b, d.ID)
	s.True(errors.Is(err, entities.ErrEscrowDepositNotFound), "another tenant cannot see the deposit")

	found, err := repo.FindByDigests(ctx, a, "example", evSHA1, evSHA2)
	s.Require().NoError(err)
	s.Equal(d.ID, found.ID)
	_, err = repo.FindByDigests(ctx, a, "example", evSHA2, evSHA1)
	s.True(errors.Is(err, entities.ErrEscrowDepositNotFound))
	_, err = repo.FindByDigests(ctx, b, "example", evSHA1, evSHA2)
	s.True(errors.Is(err, entities.ErrEscrowDepositNotFound), "digest lookup is tenant-scoped")

	// The unique index rejects a second record for the same pair.
	dup := s.newDeposit(a, "example", evSHA1, evSHA2)
	s.Error(repo.Create(ctx, dup))
}

func (s *EscrowValidationSuite) TestDeposits_ListPaginates() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewEscrowDepositRepository(tx)
	ctx := context.Background()
	a := s.scope("opA")
	for i := 0; i < 5; i++ {
		d := s.newDeposit(a, "example", evSHA1[:60]+string(rune('a'+i))+"aaa", evSHA2)
		d.ReceivedAt = time.Now().UTC().Add(time.Duration(i) * time.Minute)
		s.Require().NoError(repo.Create(ctx, d))
	}
	s.Require().NoError(repo.Create(ctx, s.newDeposit(a, "other", evSHA2, evSHA1)))

	page1, cursor, err := repo.List(ctx, a, queries.ListItemsQuery{PageSize: 2, Filter: queries.ListEscrowDepositsFilter{TLDEquals: "example"}})
	s.Require().NoError(err)
	s.Len(page1, 2)
	s.NotEmpty(cursor)
	page2, cursor2, err := repo.List(ctx, a, queries.ListItemsQuery{PageSize: 2, PageCursor: cursor, Filter: queries.ListEscrowDepositsFilter{TLDEquals: "example"}})
	s.Require().NoError(err)
	s.Len(page2, 2)
	s.NotEmpty(cursor2)
	page3, cursor3, err := repo.List(ctx, a, queries.ListItemsQuery{PageSize: 2, PageCursor: cursor2, Filter: queries.ListEscrowDepositsFilter{TLDEquals: "example"}})
	s.Require().NoError(err)
	s.Len(page3, 1)
	s.Empty(cursor3)
	s.True(page1[0].ReceivedAt.After(page1[1].ReceivedAt), "newest first")

	none, _, err := repo.List(ctx, s.scope("opB"), queries.ListItemsQuery{PageSize: 10})
	s.Require().NoError(err)
	s.Empty(none)
}

func (s *EscrowValidationSuite) TestRuns_FinalizeOnce() {
	tx := s.db.Begin()
	defer tx.Rollback()
	deposits := NewEscrowDepositRepository(tx)
	runs := NewEscrowValidationRunRepository(tx)
	ctx := context.Background()
	a, b := s.scope("opA"), s.scope("opB")

	d := s.newDeposit(a, "example", evSHA1, evSHA2)
	s.Require().NoError(deposits.Create(ctx, d))
	started := time.Now().UTC().Truncate(time.Microsecond)
	run, err := entities.NewEscrowValidationRun(d.ID, a, "example", "wf-1", "run-1", entities.EscrowProfileRydeSig, started)
	s.Require().NoError(err)
	s.Require().NoError(runs.Create(ctx, run))

	got, err := runs.GetByID(ctx, a, run.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowValidationRunning, got.Outcome)
	s.Empty(got.Findings)
	_, err = runs.GetByID(ctx, b, run.ID)
	s.True(errors.Is(err, entities.ErrEscrowValidationRunNotFound))

	// Finalising a still-RUNNING entity is rejected before touching the DB.
	s.True(errors.Is(runs.Finalize(ctx, a, run), entities.ErrInvalidEscrowValidationRun))

	wm := started.Add(-time.Hour)
	s.Require().NoError(run.Finalize(entities.EscrowValidationFinalization{
		Outcome: entities.EscrowValidationFail, StageReached: "rde",
		Findings:              []entities.EscrowFinding{{Code: "RDE_COUNT_MISMATCH", Severity: "ERROR", Stage: "rde", Message: "header declares 4 objects, deposit contains 2", At: started}},
		SigningKeyFingerprint: "AA", DecryptionKeyFingerprint: "BB", PlaintextSHA256: evSHA1,
		RDEDepositID: "20260908001", RDEKind: "FULL", RDEResend: 1, RDEWatermark: &wm,
		ReportObjectKey: "r.xml", NotificationObjectKey: "n.xml", NotificationStatus: entities.EscrowNotificationDVFN,
		CompletedAt: started.Add(time.Minute),
	}))

	// Another tenant cannot finalise it.
	s.True(errors.Is(runs.Finalize(ctx, b, run), entities.ErrEscrowValidationRunAlreadyFinal))
	stillRunning, _ := runs.GetByID(ctx, a, run.ID)
	s.Equal(entities.EscrowValidationRunning, stillRunning.Outcome)

	s.Require().NoError(runs.Finalize(ctx, a, run))
	final, err := runs.GetByID(ctx, a, run.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowValidationFail, final.Outcome)
	s.Equal(entities.EscrowNotificationDVFN, final.NotificationStatus)
	s.Len(final.Findings, 1)
	s.Equal("RDE_COUNT_MISMATCH", final.Findings[0].Code)
	s.Equal(1, final.RDEResend)
	s.NotNil(final.CompletedAt)
	s.Equal("wf-1", final.WorkflowID)

	// A second finalisation never overwrites the first outcome.
	run.Outcome = entities.EscrowValidationPass
	run.NotificationStatus = entities.EscrowNotificationDVPN
	err = runs.Finalize(ctx, a, run)
	s.True(errors.Is(err, entities.ErrEscrowValidationRunAlreadyFinal), "got %v", err)
	again, _ := runs.GetByID(ctx, a, run.ID)
	s.Equal(entities.EscrowValidationFail, again.Outcome)

	byDeposit, err := runs.ListByDeposit(ctx, a, d.ID)
	s.Require().NoError(err)
	s.Len(byDeposit, 1)

	listed, _, err := runs.List(ctx, a, queries.ListItemsQuery{PageSize: 10, Filter: queries.ListEscrowValidationRunsFilter{TLDEquals: "example", OutcomeEquals: "FAIL"}})
	s.Require().NoError(err)
	s.Len(listed, 1)
	listed, _, err = runs.List(ctx, a, queries.ListItemsQuery{PageSize: 10, Filter: queries.ListEscrowValidationRunsFilter{OutcomeEquals: "PASS"}})
	s.Require().NoError(err)
	s.Empty(listed)
}

func (s *EscrowValidationSuite) TestTrustedKeys_WindowsAndRetirement() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewEscrowTrustedKeyRepository(tx)
	ctx := context.Background()
	a, b := s.scope("opA"), s.scope("opB")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.AddDate(0, 6, 0)

	oldKey, err := entities.NewEscrowTrustedKey(a, "example", "AAAA0123456789ABCDEF0123456789ABCDEF0123", evKey, "old", t0, &t1)
	s.Require().NoError(err)
	newKey, err := entities.NewEscrowTrustedKey(a, "example", "BBBB0123456789ABCDEF0123456789ABCDEF0123", evKey, "new", t0.AddDate(0, 5, 0), nil)
	s.Require().NoError(err)
	otherTLD, err := entities.NewEscrowTrustedKey(a, "other", "CCCC0123456789ABCDEF0123456789ABCDEF0123", evKey, "", t0, nil)
	s.Require().NoError(err)
	for _, k := range []*entities.EscrowTrustedKey{oldKey, newKey, otherTLD} {
		s.Require().NoError(repo.Create(ctx, k))
	}

	fps := func(ks []*entities.EscrowTrustedKey) []string {
		var out []string
		for _, k := range ks {
			out = append(out, k.Label)
		}
		return out
	}
	active, err := repo.ListActive(ctx, a, "example", t0.AddDate(0, 1, 0))
	s.Require().NoError(err)
	s.Equal([]string{"old"}, fps(active))
	active, _ = repo.ListActive(ctx, a, "example", t0.AddDate(0, 5, 15))
	s.ElementsMatch([]string{"old", "new"}, fps(active), "overlap window: both keys verify")
	active, _ = repo.ListActive(ctx, a, "example", t1)
	s.Equal([]string{"new"}, fps(active), "validTo is exclusive")
	active, _ = repo.ListActive(ctx, b, "example", t0.AddDate(0, 1, 0))
	s.Empty(active, "other tenant sees nothing")

	retireAt := t0.AddDate(0, 8, 0)
	s.True(errors.Is(repo.Retire(ctx, b, newKey.ID, retireAt), entities.ErrEscrowTrustedKeyNotFound))
	s.Require().NoError(repo.Retire(ctx, a, newKey.ID, retireAt))
	s.True(errors.Is(repo.Retire(ctx, a, newKey.ID, retireAt), entities.ErrEscrowTrustedKeyAlreadyRetired))
	active, _ = repo.ListActive(ctx, a, "example", retireAt.Add(time.Hour))
	s.Empty(active, "retired and expired: nothing left")
	got, err := repo.GetByID(ctx, a, newKey.ID)
	s.Require().NoError(err)
	s.NotNil(got.RetiredAt)

	all, err := repo.List(ctx, a, "example")
	s.Require().NoError(err)
	s.Len(all, 2)
	all, _ = repo.List(ctx, a, "")
	s.Len(all, 3)
	_, err = repo.GetByID(ctx, a, uuid.New())
	s.True(errors.Is(err, entities.ErrEscrowTrustedKeyNotFound))
}
