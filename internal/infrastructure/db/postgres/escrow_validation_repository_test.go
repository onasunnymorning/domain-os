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
	d, err := entities.NewEscrowDeposit(scope, tld, entities.EscrowProfileRydeSig, time.Now().UTC(), "tester", "ref",
		"k/"+ryde[:8]+".ryde", ryde, 100, "k/"+sig[:8]+".sig", sig, 10)
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

	found, err := repo.FindByDigests(ctx, a, "example", entities.EscrowProfileRydeSig, evSHA1, evSHA2)
	s.Require().NoError(err)
	s.Equal(d.ID, found.ID)
	_, err = repo.FindByDigests(ctx, a, "example", entities.EscrowProfileRydeSig, evSHA2, evSHA1)
	s.True(errors.Is(err, entities.ErrEscrowDepositNotFound))
	_, err = repo.FindByDigests(ctx, b, "example", entities.EscrowProfileRydeSig, evSHA1, evSHA2)
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

// ---------------------------------------------------------------------------
// Sanitization runs (issue #415)
// ---------------------------------------------------------------------------

func (s *EscrowValidationSuite) newSanitizationRun(scope entities.OperatorID, sourceRunID, depositID uuid.UUID, policy string) *entities.EscrowSanitizationRun {
	r, err := entities.NewEscrowSanitizationRun(scope, "example", sourceRunID, depositID, evSHA1,
		policy, "escrow-sanitize/1", "artful-dodger", "wf-s1", "run-s1", time.Now().UTC().Truncate(time.Microsecond))
	s.Require().NoError(err)
	return r
}

func (s *EscrowValidationSuite) TestSanitizationRuns_OneDerivativePerSourceAndPolicy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewEscrowSanitizationRunRepository(tx)
	ctx := context.Background()
	a, b := s.scope("opA"), s.scope("opB")
	sourceRun, deposit := uuid.New(), uuid.New()

	first := s.newSanitizationRun(a, sourceRun, deposit, "rde-baseline-v1")
	s.Require().NoError(repo.Create(ctx, first))

	// The same source under the same policy is refused by the unique index:
	// a derivative is never silently replaced. The violation aborts the
	// enclosing transaction, so it runs inside a savepoint.
	s.Require().NoError(tx.SavePoint("before_dup").Error)
	dup := s.newSanitizationRun(a, sourceRun, deposit, "rde-baseline-v1")
	s.Error(repo.Create(ctx, dup))
	s.Require().NoError(tx.RollbackTo("before_dup").Error)

	// A different policy version is a separate, separately traceable derivative.
	next := s.newSanitizationRun(a, sourceRun, deposit, "rde-baseline-v2")
	s.Require().NoError(repo.Create(ctx, next))

	found, err := repo.FindBySourceAndPolicy(ctx, a, sourceRun, "rde-baseline-v1")
	s.Require().NoError(err)
	s.Equal(first.ID, found.ID)
	s.Equal("artful-dodger", found.SyntheticSuffix)

	_, err = repo.FindBySourceAndPolicy(ctx, a, sourceRun, "rde-baseline-v3")
	s.True(errors.Is(err, entities.ErrEscrowSanitizationRunNotFound))
	_, err = repo.FindBySourceAndPolicy(ctx, b, sourceRun, "rde-baseline-v1")
	s.True(errors.Is(err, entities.ErrEscrowSanitizationRunNotFound), "another tenant sees nothing")
	_, err = repo.GetByID(ctx, b, first.ID)
	s.True(errors.Is(err, entities.ErrEscrowSanitizationRunNotFound))

	runs, _, err := repo.List(ctx, a, queries.ListItemsQuery{PageSize: 10, Filter: queries.ListEscrowSanitizationRunsFilter{
		TLDEquals: "example", SourceValidationRunIDEquals: sourceRun.String(),
	}})
	s.Require().NoError(err)
	s.Len(runs, 2)
	none, _, err := repo.List(ctx, b, queries.ListItemsQuery{PageSize: 10})
	s.Require().NoError(err)
	s.Empty(none)
}

func (s *EscrowValidationSuite) TestSanitizationRuns_FinalizeOnce() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewEscrowSanitizationRunRepository(tx)
	ctx := context.Background()
	a := s.scope("opA")

	run := s.newSanitizationRun(a, uuid.New(), uuid.New(), "rde-baseline-v1")
	s.Require().NoError(repo.Create(ctx, run))

	// Finalising a still-RUNNING entity is rejected before touching the DB.
	s.True(errors.Is(repo.Finalize(ctx, a, run), entities.ErrInvalidEscrowSanitizationRun))

	s.Require().NoError(run.Finalize(entities.EscrowSanitizationFinalization{
		Outcome: entities.EscrowSanitizationPass, StageReached: "verify",
		DerivativeObjectKey: "k/deposit.xml.gz", DerivativeSHA256: evSHA2, DerivativeBytes: 128,
		ManifestObjectKey: "k/manifest.json",
		Counts:            entities.EscrowSanitizationCounts{Kept: 10, Tokenized: 3, ObjectsByType: map[string]int64{entities.DOMAIN_URI: 2}},
		FindingTally: []entities.EscrowFindingTally{
			{Code: "POLICY_UNKNOWN_ATTRIBUTE", Severity: "ERROR", Stage: "rewrite", Object: "rdeDomain:status@vendorFlag", Count: 43313},
		},
		CompletedAt: run.StartedAt.Add(time.Minute),
	}))
	s.Require().NoError(repo.Finalize(ctx, a, run))

	got, err := repo.GetByID(ctx, a, run.ID)
	s.Require().NoError(err)
	s.Equal(entities.EscrowSanitizationPass, got.Outcome)
	s.Equal(evSHA2, got.DerivativeSHA256)
	s.Equal(int64(3), got.Counts.Tokenized)
	s.Equal(int64(2), got.Counts.ObjectsByType[entities.DOMAIN_URI])
	s.Require().NotNil(got.CompletedAt)

	// The tally is the only exact account of a quarantined source, so it has
	// to survive the jsonb round trip with its object and its count intact.
	s.Require().Len(got.FindingTally, 1)
	s.Equal("rdeDomain:status@vendorFlag", got.FindingTally[0].Object)
	s.Equal(43313, got.FindingTally[0].Count)

	// A second conditional UPDATE matches no RUNNING row.
	s.True(errors.Is(repo.Finalize(ctx, a, run), entities.ErrEscrowSanitizationRunAlreadyFinal))
}
