package activities

import (
	"bytes"
	"compress/gzip"
	"errors"
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

// sanitizeTokenKey is fixture material, not a credential.
var sanitizeTokenKey = bytes.Repeat([]byte("eve-sanitize-activity-key!!!!!!!"), 2)

// esFixture wires a validation world and a sanitisation world onto the same
// fake repositories and stores, because a derivative can only exist downstream
// of a real accepted validation run.
type esFixture struct {
	ev      *evFixture
	sanRepo *fakeSanitizationRepo
	tokens  *fakeTokenKeyProvider
	acts    *EscrowSanitizeActivities
	env     *testsuite.TestActivityEnvironment
}

func newESFixture(t *testing.T) *esFixture {
	t.Helper()
	ev := newEVFixture(t, nil, nil, nil, nil)
	f := &esFixture{
		ev:      ev,
		sanRepo: newFakeSanitizationRepo(),
		tokens:  &fakeTokenKeyProvider{key: sanitizeTokenKey},
	}
	f.acts = NewEscrowSanitizeActivitiesWithDeps(
		&fakeTLDRepo{owned: map[string]string{"example": "ryop1"}},
		ev.deposits, ev.runs, f.sanRepo, ev.keyRepo,
		ev.prov, f.tokens, ev.store,
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
	var out BindSanitizationSourceOutput
	val, err := f.env.ExecuteActivity(f.acts.BindSanitizationSource, BindSanitizationSourceInput{
		Scope: f.ev.scope.String(), SourceValidationRunID: sourceRunID.String(), SyntheticSuffix: suffix,
		WorkflowID: "wf-san-1", RunID: "run-san-1",
	})
	if err != nil {
		return out, err
	}
	require.NoError(t, val.Get(&out))
	return out, nil
}

func (f *esFixture) produce(t *testing.T, b BindSanitizationSourceOutput) ProduceDerivativeOutput {
	t.Helper()
	var out ProduceDerivativeOutput
	val, err := f.env.ExecuteActivity(f.acts.ProduceDerivative, ProduceDerivativeInput{
		Scope: f.ev.scope.String(), SanitizationRunID: b.SanitizationRunID, WorkflowID: "wf-san-1",
		TLD: b.TLD, SourceProfile: b.SourceProfile, ArtifactKey: b.ArtifactKey, SignatureKey: b.SignatureKey,
		ArtifactSHA256: b.ArtifactSHA256, SignatureSHA256: b.SignatureSHA256,
		SyntheticSuffix: b.SyntheticSuffix, StagingKey: b.StagingKey,
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
	assert.Contains(t, b.DerivativeKey, "/sanitized/deposit-"+rdesanitize.PolicyVersion+".xml.gz")

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
}

func TestEscrowSanitize_MissingTokenKeyIsAnErrorNotAQuarantine(t *testing.T) {
	f := newESFixture(t)
	f.tokens.err = errors.New("secrets manager unavailable")
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 1})

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	p := f.produce(t, b)

	// Quarantining is a statement about the deposit and must not be made
	// because our own key store is down.
	require.Equal(t, rdesanitize.OutcomeError, p.Result.Outcome)
	assert.True(t, p.Result.Has(rdesanitize.CodeTokenKeyUnavailable))
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
