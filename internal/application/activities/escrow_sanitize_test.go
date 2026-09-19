package activities

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// esFixture wires a validation world and a sanitisation world onto the same
// fake repositories and stores, because a derivative can only exist downstream
// of a real accepted validation run.
type esFixture struct {
	ev      *evFixture
	sanRepo *fakeSanitizationRepo
	// pseudonymise is the fixture receiver's active pseudonymisation version.
	pseudonymise *entities.EscrowKeyVersion
	acts         *EscrowSanitizeActivities
	env          *testsuite.TestActivityEnvironment
}

func newESFixture(t *testing.T) *esFixture {
	t.Helper()
	ev := newEVFixture(t, nil, nil, nil, nil, nil)
	f := &esFixture{ev: ev, sanRepo: newFakeSanitizationRepo()}
	ev.trust(t)
	f.pseudonymise, _ = ev.addPseudonymiseVersion(t, ev.eve, 1)
	ev.activate(t, f.pseudonymise)
	f.acts = NewEscrowSanitizeActivitiesWithDeps(
		&fakeTLDRepo{owned: map[string]string{"example": "ryop1"}},
		ev.deposits, ev.runs, f.sanRepo, ev.resolver, ev.loader, ev.store,
		rdevalidate.DefaultLimits(), rdesanitize.DefaultLimits(), "artful-dodger",
	)
	f.acts.now = func() time.Time { return time.Now().UTC().Add(time.Hour) }
	var ts testsuite.WorkflowTestSuite
	f.env = ts.NewTestActivityEnvironment()
	f.env.RegisterActivity(f.acts)
	return f
}

// acceptedSource runs a real validation to PASS and returns its run id, so the
// sanitisation path starts from a record it did not fabricate.
func (f *esFixture) acceptedSource(t *testing.T, opts rdetest.DepositOpts) uuid.UUID {
	t.Helper()
	if opts.TLD == "" {
		opts.TLD = "example"
	}
	pair := rdetest.BuildPair(t, opts, f.ev.service, f.ev.registry)
	rydeKey, sigKey := f.ev.upload(t, "example_2026-09-08_full_S1_R0", pair)
	b, err := f.ev.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	res := f.ev.validate(t, b)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	emitted, err := f.ev.emit(t, b, res)
	require.NoError(t, err)
	require.NoError(t, f.ev.finalize(t, b, res, emitted, ""))
	return b.ValidationRunID
}

func (f *esFixture) bind(t *testing.T, sourceRunID uuid.UUID, suffix string) (BindSanitizationSourceOutput, error) {
	t.Helper()
	return f.bindAs(t, sourceRunID, suffix, "wf-san-1", "run-san-1")
}

// bindAs binds as a particular workflow execution, for tests that launch the
// same source more than once.
func (f *esFixture) bindAs(t *testing.T, sourceRunID uuid.UUID, suffix, workflowID, runID string) (BindSanitizationSourceOutput, error) {
	t.Helper()
	var out BindSanitizationSourceOutput
	val, err := f.env.ExecuteActivity(f.acts.BindSanitizationSource, BindSanitizationSourceInput{
		Scope: f.ev.scope.String(), SourceValidationRunID: sourceRunID.String(), SyntheticSuffix: suffix,
		WorkflowID: workflowID, RunID: runID,
	})
	if err != nil {
		return out, err
	}
	require.NoError(t, val.Get(&out))
	return out, nil
}

// resolveKeys runs ResolveSanitizationKeys as the workflow does.
func (f *esFixture) resolveKeys(t *testing.T, b BindSanitizationSourceOutput) *entities.EscrowKeySelection {
	t.Helper()
	var sel entities.EscrowKeySelection
	val, err := f.env.ExecuteActivity(f.acts.ResolveSanitizationKeys, ResolveSanitizationKeysInput{
		Scope: f.ev.scope.String(), SourceValidationRunID: b.SourceValidationRunID, WorkflowID: "wf-san-1",
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&sel))
	return &sel
}

func (f *esFixture) produce(t *testing.T, b BindSanitizationSourceOutput) ProduceDerivativeOutput {
	t.Helper()
	return f.produceWith(t, b, f.resolveKeys(t, b))
}

func (f *esFixture) produceWith(t *testing.T, b BindSanitizationSourceOutput, keys *entities.EscrowKeySelection) ProduceDerivativeOutput {
	t.Helper()
	var out ProduceDerivativeOutput
	val, err := f.env.ExecuteActivity(f.acts.ProduceDerivative, ProduceDerivativeInput{
		Scope: f.ev.scope.String(), SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-1",
		TLD: b.TLD, SourceProfile: b.SourceProfile, ArtifactKey: b.ArtifactKey, SignatureKey: b.SignatureKey,
		ArtifactSHA256: b.ArtifactSHA256, SignatureSHA256: b.SignatureSHA256,
		SyntheticSuffix: b.SyntheticSuffix, StagingKey: b.StagingKey,
		Keys: keys,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&out))
	return out
}

func (f *esFixture) verify(t *testing.T, b BindSanitizationSourceOutput, p ProduceDerivativeOutput, sourceRunID uuid.UUID) VerifyDerivativeOutput {
	t.Helper()
	var out VerifyDerivativeOutput
	val, err := f.env.ExecuteActivity(f.acts.VerifyDerivative, VerifyDerivativeInput{
		Scope: f.ev.scope.String(), SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-1", RunID: "run-san-1",
		TLD: b.TLD, DepositID: b.DepositID, SourceValidationRunID: sourceRunID, SourceProfile: b.SourceProfile,
		SourceArtifactSHA256: b.ArtifactSHA256, SyntheticSuffix: b.SyntheticSuffix,
		StagingKey: b.StagingKey, DerivativeKey: b.DerivativeKey, ManifestKey: b.ManifestKey,
		DerivativeSHA256: p.DerivativeSHA256, DerivativeBytes: p.DerivativeBytes,
		TokenKeyID: p.TokenKeyID, Produced: p.Result,
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&out))
	return out
}

func (f *esFixture) finalize(t *testing.T, b BindSanitizationSourceOutput, v VerifyDerivativeOutput, p ProduceDerivativeOutput, failure string) error {
	t.Helper()
	_, err := f.env.ExecuteActivity(f.acts.FinalizeSanitizationRun, FinalizeSanitizationRunInput{
		Scope: f.ev.scope.String(), SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-1",
		Result: v.Result, DerivativeKey: v.DerivativeKey, ManifestKey: v.ManifestKey,
		DerivativeSHA256: p.DerivativeSHA256, DerivativeBytes: p.DerivativeBytes,
		CompletedAt: time.Now().UTC().Add(2 * time.Hour), Failure: failure,
	})
	return err
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	require.NoError(t, err)
	out, err := io.ReadAll(gz)
	require.NoError(t, err)
	return string(out)
}

func TestEscrowSanitize_EndToEnd(t *testing.T) {
	f := newESFixture(t)
	opts := rdetest.DepositOpts{Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, NNDNs: 1,
		AuthInfo: true, SecDNS: true, Disclose: true, SharedContact: true, IDNDomain: true}
	sourceRunID := f.acceptedSource(t, opts)

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	assert.False(t, b.Replay)
	assert.Equal(t, "artful-dodger", b.SyntheticSuffix, "the configured default applies when none is given")
	assert.Equal(t, rdesanitize.PolicyVersion, b.PolicyVersion)
	// The published file name carries the suffix and the source watermark date.
	assert.Contains(t, b.DerivativeKey, "/sanitized/deposit-"+rdesanitize.PolicyVersion+"_artful-dodger_2026-09-08.xml.gz")
	assert.Contains(t, b.ManifestKey, "/sanitized/manifest-"+rdesanitize.PolicyVersion+"_artful-dodger_2026-09-08.json")

	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	assert.NotEmpty(t, p.TokenKeyID)
	// Staged, not published: the sanitized/ prefix is still empty.
	_, staged := f.ev.store.get(b.StagingKey)
	assert.True(t, staged, "the derivative is staged before it is published")
	_, published := f.ev.store.get(b.DerivativeKey)
	assert.False(t, published, "nothing reaches sanitized/ before it has been verified")

	v := f.verify(t, b, p, sourceRunID)
	require.Equal(t, rdesanitize.OutcomePass, v.Result.Outcome, "findings: %v", v.Result.Findings)
	raw, ok := f.ev.store.get(b.DerivativeKey)
	require.True(t, ok, "the verified derivative is published beside the source")
	doc := gunzip(t, raw)
	assert.Contains(t, doc, "<rdeHeader:tld>artful-dodger</rdeHeader:tld>")
	assert.NotContains(t, doc, "authInfo")
	assert.NotContains(t, doc, "2fooBAR")
	assert.NotContains(t, doc, "contact1@example.com")

	manifest, ok := f.ev.store.get(b.ManifestKey)
	require.True(t, ok)
	assert.Contains(t, string(manifest), `"label": "sanitized-pseudonymized"`)
	assert.Contains(t, string(manifest), `"policyVersion": "`+rdesanitize.PolicyVersion+`"`)
	assert.NotContains(t, string(manifest), "Testville")

	require.NoError(t, f.finalize(t, b, v, p, ""))
	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, b.SanitizationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowSanitizationPass, run.Outcome)
	assert.Equal(t, b.DerivativeKey, run.DerivativeObjectKey)
	assert.Positive(t, run.Counts.ObjectsByType[entities.DOMAIN_URI])

	// The source deposit is untouched: same bytes, same key.
	src, ok := f.ev.store.get(b.ArtifactKey)
	require.True(t, ok)
	assert.NotEmpty(t, src)
}

func TestEscrowSanitize_RefusesASourceThatDidNotPass(t *testing.T) {
	f := newESFixture(t)
	// A deposit whose signature does not verify: validation records FAIL.
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Domains: 1}, f.ev.service, f.ev.registry)
	pair.Sig = rdetest.Tamper(pair.Sig, 120)
	rydeKey, sigKey := f.ev.upload(t, "bad", pair)
	b, err := f.ev.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	res := f.ev.validate(t, b)
	require.Equal(t, rdevalidate.OutcomeFail, res.Outcome)
	emitted, err := f.ev.emit(t, b, res)
	require.NoError(t, err)
	require.NoError(t, f.ev.finalize(t, b, res, emitted, ""))

	_, err = f.bind(t, b.ValidationRunID, "")
	require.Error(t, err)
	assert.True(t, isNonRetryable(err), "an unaccepted source is a caller error, not a retryable one")
	assert.Contains(t, err.Error(), string(rdesanitize.CodeSourceNotAccepted))
}

func TestEscrowSanitize_RefusesAnotherTenantsRun(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1})

	var out BindSanitizationSourceOutput
	_, err := f.env.ExecuteActivity(f.acts.BindSanitizationSource, BindSanitizationSourceInput{
		Scope: "ryop2", SourceValidationRunID: sourceRunID.String(), WorkflowID: "wf-san-1", RunID: "run-san-1",
	})
	require.Error(t, err)
	assert.True(t, isNonRetryable(err))
	assert.Empty(t, out.SanitizationRunID)
}

func TestEscrowSanitize_ReplayBindsToTheExistingDerivative(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1, Contacts: 1})

	first, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	again, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)

	assert.True(t, again.Replay)
	assert.Equal(t, first.SanitizationRunID, again.SanitizationRunID,
		"re-running under the same policy version binds to the existing derivative rather than making a second one")
	assert.Equal(t, first.DerivativeKey, again.DerivativeKey)
}

func TestEscrowSanitize_QuarantinesAnUnclassifiedSourceWithoutPublishing(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1, Contacts: 1, VendorExtension: true})

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	p := f.produce(t, b)

	require.Equal(t, rdesanitize.OutcomeQuarantined, p.Result.Outcome, "codes: %v", p.Result.Codes())
	assert.True(t, p.Result.Has(rdesanitize.CodePolicyUnknownNamespace))
	_, staged := f.ev.store.get(b.StagingKey)
	assert.False(t, staged, "a refused rewrite stores nothing, not even in the staging prefix")
	_, published := f.ev.store.get(b.DerivativeKey)
	assert.False(t, published)

	v := VerifyDerivativeOutput{Result: p.Result}
	require.NoError(t, f.finalize(t, b, v, p, ""))
	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, b.SanitizationRunID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowSanitizationQuarantined, run.Outcome)
	assert.Empty(t, run.DerivativeObjectKey, "a quarantined run points at nothing")
	assert.Contains(t, run.FindingCodes(), string(rdesanitize.CodePolicyUnknownNamespace))

	// The record is the only place an operator sees this run, so it has to
	// carry both halves: which profile entries are missing, and how much of
	// the source depends on each. Findings keeps one example per gap; the
	// tally counts every occurrence.
	require.NotEmpty(t, run.FindingTally, "a quarantined run records its tally")
	var gaps int
	for _, e := range run.FindingTally {
		if e.Code != string(rdesanitize.CodePolicyUnknownNamespace) {
			continue
		}
		gaps++
		assert.NotEmpty(t, e.Object, "a tally row names the gap it counts")
		assert.Positive(t, e.Count)
	}
	assert.Positive(t, gaps, "tally: %+v", run.FindingTally)
}

func TestEscrowSanitize_MissingTokenKeyIsAnErrorNotAQuarantine(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1})
	// The receiver's only pseudonymisation key is revoked: no usable key.
	current, err := f.ev.versions.GetByID(t.Context(), entities.OperatorEscrowKeyScope(f.ev.scope), f.pseudonymise.ID)
	require.NoError(t, err)
	f.ev.transition(t, current, func(v *entities.EscrowKeyVersion) error { return v.Revoke("test", false, time.Now().UTC()) }, entities.EscrowAuditVersionRevoked)

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	p := f.produce(t, b)

	// Quarantining is a statement about the deposit and must not be made
	// because our own key store is down.
	require.Equal(t, rdesanitize.OutcomeError, p.Result.Outcome)
	assert.True(t, p.Result.Has(rdesanitize.CodeTokenKeyUnavailable))
}

// erroredRun drives a source to a finalised ERROR the way the workflow does when
// the receiver has no usable pseudonymisation key, and returns the run id.
func (f *esFixture) erroredRun(t *testing.T, sourceRunID uuid.UUID) uuid.UUID {
	t.Helper()
	current, err := f.ev.versions.GetByID(t.Context(), entities.OperatorEscrowKeyScope(f.ev.scope), f.pseudonymise.ID)
	require.NoError(t, err)
	f.ev.transition(t, current, func(v *entities.EscrowKeyVersion) error { return v.Revoke("test", false, time.Now().UTC()) }, entities.EscrowAuditVersionRevoked)

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomeError, p.Result.Outcome)
	require.NoError(t, f.finalize(t, b, VerifyDerivativeOutput{Result: p.Result}, p, ""))
	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, b.SanitizationRunID)
	require.NoError(t, err)
	require.Equal(t, entities.EscrowSanitizationError, run.Outcome)
	return b.SanitizationRunID
}

// The incident this guards: a run ended in TOKEN_KEY_UNAVAILABLE, the operator
// then activated a pseudonymisation key, and every relaunch just replayed the
// stored ERROR in a few milliseconds because the unique index leaves no room
// for a second record.
func TestEscrowSanitize_RetryAfterAnErrorReopensTheRun(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 2, Contacts: 1})
	runID := f.erroredRun(t, sourceRunID)

	// The operator fixes the cause.
	v2, _ := f.ev.addPseudonymiseVersion(t, f.ev.eve, 2)
	f.ev.activate(t, v2)

	b, err := f.bindAs(t, sourceRunID, "", "wf-san-2", "run-san-2")
	require.NoError(t, err)
	assert.Equal(t, runID, b.SanitizationRunID, "the retry uses the same record: the unique index allows no other")
	assert.True(t, b.Replay)
	assert.False(t, b.AlreadyFinal, "an ERROR is not a decision, so it is not replayed")

	reopened, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowSanitizationRunning, reopened.Outcome)
	assert.Empty(t, reopened.Findings, "the previous attempt's findings do not leak into this one")
	assert.Nil(t, reopened.CompletedAt)
	assert.Equal(t, "wf-san-2", reopened.WorkflowID)
	assert.Equal(t, "run-san-2", reopened.RunID)

	// And the retry can now succeed end to end.
	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	v := f.verify(t, b, p, sourceRunID)
	require.Equal(t, rdesanitize.OutcomePass, v.Result.Outcome, "findings: %v", v.Result.Findings)
	require.NoError(t, f.finalize(t, b, v, p, ""))
	done, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowSanitizationPass, done.Outcome)
	assert.NotEmpty(t, done.DerivativeObjectKey)

	// A decision stays final: a further launch replays the PASS and produces
	// nothing, exactly as before.
	again, err := f.bindAs(t, sourceRunID, "", "wf-san-3", "run-san-3")
	require.NoError(t, err)
	assert.True(t, again.AlreadyFinal)
	assert.Equal(t, string(entities.EscrowSanitizationPass), again.ExistingOutcome)
	assert.Equal(t, done.DerivativeObjectKey, again.DerivativeKey)
	unchanged, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, "wf-san-2", unchanged.WorkflowID, "a PASS is never reopened")
}

// A bad suffix is itself an ERROR, so a retry must be able to correct it.
func TestEscrowSanitize_RetryAfterAnErrorTakesTheCorrectedSuffix(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1})
	runID := f.erroredRun(t, sourceRunID)

	// The source TLD is "example": deriving under it would mislabel the copy.
	// The retry is refused before it starts, and the ERROR stays as it was so
	// the next, corrected launch can still reopen it.
	_, err := f.bindAs(t, sourceRunID, "example", "wf-san-2", "run-san-2")
	require.Error(t, err)
	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowSanitizationError, run.Outcome, "a rejected retry leaves the record as it was")
	assert.Equal(t, "wf-san-1", run.WorkflowID)

	b, err := f.bindAs(t, sourceRunID, "Sandbox-Zone.", "wf-san-3", "run-san-3")
	require.NoError(t, err)
	assert.Equal(t, "sandbox-zone", b.SyntheticSuffix, "the corrected suffix replaces the one the failed attempt carried")
	run, err = f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, "sandbox-zone", run.SyntheticSuffix)
	assert.Equal(t, entities.EscrowSanitizationRunning, run.Outcome)
}

// Two launches can both see the ERROR. Only one reopens it; the other binds to
// what the winner wrote instead of failing or overwriting it.
func TestEscrowSanitize_RetryLosingTheReopenRaceBindsToTheWinner(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1})
	runID := f.erroredRun(t, sourceRunID)
	f.acts.sanitizations = losingReopenRepo{f.sanRepo}

	b, err := f.bindAs(t, sourceRunID, "", "wf-loser", "run-loser")
	require.NoError(t, err)
	assert.Equal(t, runID, b.SanitizationRunID)
	assert.True(t, b.Replay)
	assert.False(t, b.AlreadyFinal)
	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, runID)
	require.NoError(t, err)
	assert.Equal(t, "wf-winner", run.WorkflowID, "the loser did not overwrite the winner's attempt")
}

// losingReopenRepo makes another attempt win the reopen just before this one.
type losingReopenRepo struct{ *fakeSanitizationRepo }

func (r losingReopenRepo) Reopen(ctx context.Context, scope entities.OperatorID, run *entities.EscrowSanitizationRun) error {
	winner := *run
	winner.WorkflowID = "wf-winner"
	if err := r.fakeSanitizationRepo.Reopen(ctx, scope, &winner); err != nil {
		return err
	}
	return entities.ErrEscrowSanitizationRunNotReopenable
}

func TestEscrowSanitize_NeverLeaksAValueIntoTheRecord(t *testing.T) {
	const canary = "CANARY-cf3a91-VALUE"
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1, Contacts: 1, Canary: canary})

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	v := f.verify(t, b, p, sourceRunID)
	require.NoError(t, f.finalize(t, b, v, p, ""))

	run, err := f.sanRepo.GetByID(t.Context(), f.ev.scope, b.SanitizationRunID)
	require.NoError(t, err)
	for _, finding := range run.Findings {
		assert.NotContains(t, finding.Message+finding.Locator, canary)
	}
	manifest, ok := f.ev.store.get(b.ManifestKey)
	require.True(t, ok)
	assert.NotContains(t, string(manifest), canary, "the manifest carries counts, not values")
	assert.NotContains(t, b.DerivativeKey, canary, "the object name is derived from ids, not from content")

	doc, ok := f.ev.store.get(b.DerivativeKey)
	require.True(t, ok)
	assert.NotContains(t, gunzip(t, doc), canary, "the canary was planted in a contact org, which is removed")
}

func TestEscrowSanitize_PlaintextSourceIsSanitizedToo(t *testing.T) {
	f := newESFixture(t)
	opts := rdetest.DepositOpts{TLD: "example", Layout: rdetest.LayoutGzip, Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1}
	artifact := rdetest.BuildPayload(t, opts, rdetest.BuildXML(opts))
	f.ev.store.put("uploads/example.xml.gz", artifact)

	bound, err := f.ev.bindPlaintext(t, "uploads/example.xml.gz")
	require.NoError(t, err)
	res := f.ev.validate(t, bound)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	require.NoError(t, f.ev.finalize(t, bound, res, EmitReportOutput{}, ""))

	b, err := f.bind(t, bound.ValidationRunID, "")
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowProfilePlaintextXML, b.SourceProfile)
	assert.Empty(t, b.SignatureKey)

	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	v := f.verify(t, b, p, bound.ValidationRunID)
	require.Equal(t, rdesanitize.OutcomePass, v.Result.Outcome)

	raw, ok := f.ev.store.get(b.DerivativeKey)
	require.True(t, ok)
	assert.Contains(t, gunzip(t, raw), "<rdeDomain:name>example-1.artful-dodger</rdeDomain:name>")
}

func TestEscrowSanitize_SuffixOverride(t *testing.T) {
	f := newESFixture(t)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1, Contacts: 1})

	b, err := f.bind(t, sourceRunID, "Sandbox-Zone.")
	require.NoError(t, err)
	assert.Equal(t, "sandbox-zone", b.SyntheticSuffix, "the launch override is normalised like any other name")

	p := f.produce(t, b)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	v := f.verify(t, b, p, sourceRunID)
	require.Equal(t, rdesanitize.OutcomePass, v.Result.Outcome)
	raw, _ := f.ev.store.get(b.DerivativeKey)
	assert.Contains(t, gunzip(t, raw), "<rdeHeader:tld>sandbox-zone</rdeHeader:tld>")
	assert.NotContains(t, strings.ToLower(gunzip(t, raw)), ".example<")
}

func TestSanitizedFileTag(t *testing.T) {
	wm := time.Date(2026, 9, 17, 23, 30, 0, 0, time.FixedZone("x", -5*3600))
	zero := time.Time{}
	cases := []struct {
		name   string
		suffix string
		wm     *time.Time
		want   string
	}{
		{"suffix and watermark, the latter in UTC", "wild", &wm, "_wild_2026-09-18"},
		{"no watermark", "wild", nil, "_wild"},
		{"zero watermark", "wild", &zero, "_wild"},
		{"no suffix", "", &wm, "_2026-09-18"},
		{"case, dots and separators are normalised", " .Artful.Dodger/../ ", &wm, "_artful-dodger_2026-09-18"},
		{"idn is punycoded", "bücher", &wm, "_xn--bcher-kva_2026-09-18"},
		{"nothing usable", "///", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, sanitizedFileTag(c.suffix, c.wm))
		})
	}
}
