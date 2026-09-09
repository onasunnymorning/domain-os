package activities

import (
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// TestEscrowSanitize_NoRegistryWriteSideEffect is the behavioural half of the
// acceptance criterion that a derivative run "has no path to invoke
// EscrowImportWorkflow, mutate registry data, or treat the derivative as an
// RDE/escrow release artifact". The structural half is the AST scan in
// TestEscrowValidationDoesNotTouchImport.
//
// It runs the whole chain against real repositories — validate to PASS, then
// bind, produce, verify and finalise a derivative — and asserts that every
// registry table is exactly as it was, that the source object is byte-for-byte
// unchanged, and that the derivative is a separate object under its own prefix.
func TestEscrowSanitize_NoRegistryWriteSideEffect(t *testing.T) {
	db := testDB(t)
	// Shared database, parallel packages: REPEATABLE READ isolates this
	// transaction from concurrent inserts while still seeing its own writes.
	tx := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	defer tx.Rollback()

	ctx := context.Background()
	ryid := "sansidefx"
	ro, err := entities.NewRegistryOperator(ryid, "Sanitize Side-Effect Operator", "ops@example.com")
	require.NoError(t, err)
	_, err = postgres.NewGORMRegistryOperatorRepository(tx).Create(ctx, ro)
	require.NoError(t, err)
	tld, err := entities.NewTLD(ryid, ryid)
	require.NoError(t, err)
	tldRepo := postgres.NewGormTLDRepo(tx)
	require.NoError(t, tldRepo.Create(ctx, tld))

	before := registryCounts(t, tx)

	deposits := postgres.NewEscrowDepositRepository(tx)
	runs := postgres.NewEscrowValidationRunRepository(tx)
	keys := postgres.NewEscrowTrustedKeyRepository(tx)
	sanitizations := postgres.NewEscrowSanitizationRunRepository(tx)

	// 1. A real accepted validation run, produced by the validation activities.
	ev := newEVFixture(t, tldRepo, deposits, runs, keys)
	ev.scope = entities.OperatorID(ryid)
	ev.tld = ryid
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{
		TLD: ryid, Domains: 12, Contacts: 6, Hosts: 4, Registrars: 2, NNDNs: 2,
		AuthInfo: true, SecDNS: true, Disclose: true,
	}, ev.service, ev.registry)
	rydeKey, sigKey := ev.upload(t, ryid+"_2026-09-08_full_S1_R0", pair)
	bound, err := ev.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	vres := ev.validate(t, bound)
	require.Equal(t, rdevalidate.OutcomePass, vres.Outcome, "findings: %v", vres.Findings)
	emitted, err := ev.emit(t, bound, vres)
	require.NoError(t, err)
	require.NoError(t, ev.finalize(t, bound, vres, emitted, ""))

	sourceBytes, ok := ev.store.get(bound.ArtifactKey)
	require.True(t, ok)
	sourceCopy := append([]byte(nil), sourceBytes...)

	// 2. The derivative, against the same repositories and object store.
	acts := NewEscrowSanitizeActivitiesWithDeps(tldRepo, deposits, runs, sanitizations, keys,
		ev.prov, &fakeTokenKeyProvider{key: sanitizeTokenKey}, ev.store,
		rdevalidate.DefaultLimits(), rdesanitize.DefaultLimits(), "artful-dodger")
	acts.now = func() time.Time { return time.Now().UTC().Add(time.Hour) }
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestActivityEnvironment()
	env.RegisterActivity(acts)

	var b BindSanitizationSourceOutput
	val, err := env.ExecuteActivity(acts.BindSanitizationSource, BindSanitizationSourceInput{
		Scope: ryid, SourceValidationRunID: bound.ValidationRunID.String(),
		WorkflowID: "wf-san-sidefx", RunID: "run-san-sidefx",
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&b))

	var p ProduceDerivativeOutput
	val, err = env.ExecuteActivity(acts.ProduceDerivative, ProduceDerivativeInput{
		Scope: ryid, SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-sidefx",
		TLD: b.TLD, SourceProfile: b.SourceProfile, ArtifactKey: b.ArtifactKey, SignatureKey: b.SignatureKey,
		ArtifactSHA256: b.ArtifactSHA256, SignatureSHA256: b.SignatureSHA256,
		SyntheticSuffix: b.SyntheticSuffix, StagingKey: b.StagingKey,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&p))
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)

	var v VerifyDerivativeOutput
	val, err = env.ExecuteActivity(acts.VerifyDerivative, VerifyDerivativeInput{
		Scope: ryid, SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-sidefx", RunID: "run-san-sidefx",
		TLD: b.TLD, DepositID: b.DepositID, SourceValidationRunID: b.SourceValidationRunID,
		SourceProfile: b.SourceProfile, SourceArtifactSHA256: b.ArtifactSHA256,
		SyntheticSuffix: b.SyntheticSuffix, StagingKey: b.StagingKey,
		DerivativeKey: b.DerivativeKey, ManifestKey: b.ManifestKey,
		DerivativeSHA256: p.DerivativeSHA256, DerivativeBytes: p.DerivativeBytes,
		TokenKeyID: p.TokenKeyID, Produced: p.Result,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&v))
	require.Equal(t, rdesanitize.OutcomePass, v.Result.Outcome, "findings: %v", v.Result.Findings)

	_, err = env.ExecuteActivity(acts.FinalizeSanitizationRun, FinalizeSanitizationRunInput{
		Scope: ryid, SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-sidefx",
		Result: v.Result, DerivativeKey: v.DerivativeKey, ManifestKey: v.ManifestKey,
		DerivativeSHA256: p.DerivativeSHA256, DerivativeBytes: p.DerivativeBytes,
		CompletedAt: time.Now().UTC().Add(2 * time.Hour),
	})
	require.NoError(t, err)

	// 3. Nothing in the registry moved.
	require.Equal(t, before, registryCounts(t, tx), "a derivative run must not touch registry data")

	// 4. The source deposit is the authoritative custody artifact and is
	// unchanged: same bytes, same key, same record.
	afterSource, ok := ev.store.get(bound.ArtifactKey)
	require.True(t, ok, "the source object must still exist under its own key")
	require.Equal(t, sourceCopy, afterSource, "the source deposit must be byte-for-byte unchanged")
	dep, err := deposits.GetByID(ctx, entities.OperatorID(ryid), bound.DepositID)
	require.NoError(t, err)
	require.Equal(t, b.ArtifactSHA256, dep.ArtifactSHA256, "the source checksum must be unchanged")

	// 5. The derivative is a separate object under its own prefix, and its
	// record points at it.
	require.NotEqual(t, bound.ArtifactKey, v.DerivativeKey)
	require.Contains(t, v.DerivativeKey, "/sanitized/")
	derivative, ok := ev.store.get(v.DerivativeKey)
	require.True(t, ok)

	gz, err := gzip.NewReader(strings.NewReader(string(derivative)))
	require.NoError(t, err)
	doc, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.NotContains(t, string(doc), "authInfo")
	require.Contains(t, string(doc), "<rdeHeader:tld>artful-dodger</rdeHeader:tld>")

	var outcome string
	require.NoError(t, tx.Table("escrow_sanitization_runs").Select("outcome").Where("id = ?", b.SanitizationRunID).Scan(&outcome).Error)
	require.Equal(t, string(entities.EscrowSanitizationPass), outcome)

	// One derivative per source per policy version, enforced by the database.
	var derivatives int64
	require.NoError(t, tx.Table("escrow_sanitization_runs").
		Where("source_validation_run_id = ? AND policy_version = ?", bound.ValidationRunID, rdesanitize.PolicyVersion).
		Count(&derivatives).Error)
	require.Equal(t, int64(1), derivatives)
}
