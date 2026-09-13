package activities

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// The escrow pipelines on the key registry (issue #429, ADR-0009 §7).
// ---------------------------------------------------------------------------

func (f *evFixture) bindAt(t *testing.T, rydeKey, sigKey string, receivedAt time.Time) BindDepositOutput {
	t.Helper()
	f.trust(t)
	var out BindDepositOutput
	val, err := f.env.ExecuteActivity(f.acts.BindDeposit, BindDepositInput{
		Scope: f.scope.String(), TLD: f.tld, ArtifactObjectKey: rydeKey, SignatureObjectKey: sigKey,
		SubmittedBy: "tester", ReceivedAt: receivedAt, WorkflowID: "wf-keys", RunID: "run-keys",
	})
	require.NoError(t, err)
	require.NoError(t, val.Get(&out))
	return out
}

func (f *evFixture) resolveKeys(t *testing.T, b BindDepositOutput) *entities.EscrowKeySelection {
	t.Helper()
	var sel entities.EscrowKeySelection
	val, err := f.env.ExecuteActivity(f.acts.ResolveEscrowKeys, ResolveEscrowKeysInput{Scope: f.scope.String(), DepositID: b.DepositID, WorkflowID: "wf-keys"})
	require.NoError(t, err)
	require.NoError(t, val.Get(&sel))
	return &sel
}

func (f *evFixture) validateWith(t *testing.T, b BindDepositOutput, keys *entities.EscrowKeySelection) (rdevalidate.Result, error) {
	t.Helper()
	var res rdevalidate.Result
	val, err := f.env.ExecuteActivity(f.acts.ValidateArtifacts, ValidateArtifactsInput{
		Scope: f.scope.String(), TLD: f.tld, DepositID: b.DepositID, ValidationRunID: b.ValidationRunID, WorkflowID: "wf-keys",
		Profile: b.Profile, ArtifactKey: b.ArtifactKey, SignatureKey: b.SignatureKey, ArtifactSHA256: b.ArtifactSHA256, SignatureSHA256: b.SignatureSHA256,
		Keys: keys,
	})
	if err != nil {
		return res, err
	}
	require.NoError(t, val.Get(&res))
	return res, nil
}

func (f *evFixture) transition(t *testing.T, v *entities.EscrowKeyVersion, change func(*entities.EscrowKeyVersion) error, action entities.EscrowKeyAuditAction) {
	t.Helper()
	before := v.Clone()
	require.NoError(t, change(v))
	ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), before, v, action)
	require.NoError(t, err)
	require.NoError(t, f.versions.ApplyTransitions(t.Context(), []entities.EscrowKeyVersionTransition{{Version: v, ExpectedState: before.State}}, []*entities.EscrowKeyAuditEvent{ev}))
}

func TestEscrowKeysPipeline_RegistryReceiverDecryptsAndTheRunRecordsItsKeys(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	eve, versions := f.receiver(t, f.service)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "registry", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())

	keys := f.resolveKeys(t, b)
	require.Len(t, keys.Decrypt, 1)
	require.Len(t, keys.Verify, 1)
	assert.Equal(t, 2, keys.Arrangement.TLDRevision, "trust wrote revision 1, the receiver revision 2")

	res, err := f.validateWith(t, b, keys)
	require.NoError(t, err)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)

	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	_, err = f.env.ExecuteActivity(f.acts.FinalizeValidationRun, FinalizeRunInput{
		Scope: f.scope.String(), ValidationRunID: b.ValidationRunID, WorkflowID: "wf-keys", Result: res,
		SummaryKey: emitted.SummaryKey, ReportKey: emitted.ReportKey, NotificationKey: emitted.NotificationKey,
		NotificationStatus: emitted.NotificationStatus, CompletedAt: time.Now().UTC().Add(2 * time.Hour), Keys: keys,
	})
	require.NoError(t, err)

	run, err := f.runs.GetByID(t.Context(), f.scope, b.ValidationRunID)
	require.NoError(t, err)
	require.NotNil(t, run.Keys.DecryptionKeyVersionID)
	assert.Equal(t, versions[0].ID, *run.Keys.DecryptionKeyVersionID)
	require.NotNil(t, run.Keys.SigningKeyVersionID)
	assert.Equal(t, keys.Verify[0].ID, *run.Keys.SigningKeyVersionID)
	require.NotNil(t, run.Keys.ReceiverPartyID)
	assert.Equal(t, eve.ID, *run.Keys.ReceiverPartyID)
	assert.Equal(t, 2, run.Keys.TLDArrangementRevision)
	assert.Equal(t, f.service.Fingerprint, run.DecryptionKeyFingerprint, "fingerprints are still recorded")
}

func TestEscrowKeysPipeline_HistoricalKeyOpensOnlyDepositsReceivedBeforeDeactivation(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	_, versions := f.receiver(t, f.service)
	deactivatedAt := time.Now().UTC()
	f.transition(t, versions[0], func(v *entities.EscrowKeyVersion) error { return v.Deactivate(deactivatedAt) }, entities.EscrowAuditVersionDeactivated)

	old := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Domains: 2}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "old", old)
	bOld := f.bindAt(t, rydeKey, sigKey, deactivatedAt.Add(-time.Hour))
	keys := f.resolveKeys(t, bOld)
	require.Len(t, keys.Decrypt, 1, "a retained deposit received before deactivation still selects the key")
	res, err := f.validateWith(t, bOld, keys)
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)

	fresh := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example", Domains: 3}, f.service, f.registry)
	rydeKey, sigKey = f.upload(t, "new", fresh)
	bNew := f.bindAt(t, rydeKey, sigKey, deactivatedAt.Add(time.Hour))
	keys = f.resolveKeys(t, bNew)
	assert.Empty(t, keys.Decrypt, "a new deposit never selects a historical key")
	res, err = f.validateWith(t, bNew, keys)
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomeError, res.Outcome, "our missing key is ERROR, never a DVFN claim about the registry")
	assert.Contains(t, res.Codes(), rdevalidate.CodeDecryptKeyUnavailable)
}

func TestEscrowKeysPipeline_RevocationReachesARetryOfASelectedRun(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	_, versions := f.receiver(t, f.service)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "revoked", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())
	keys := f.resolveKeys(t, b)
	require.Len(t, keys.Decrypt, 1)

	// Revoked after the workflow recorded its selection: the retry must not use it.
	f.transition(t, versions[0], func(v *entities.EscrowKeyVersion) error {
		return v.Revoke("private key exposed", true, time.Now().UTC())
	}, entities.EscrowAuditVersionRevoked)
	res, err := f.validateWith(t, b, keys)
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomeError, res.Outcome)
	assert.Contains(t, res.Codes(), rdevalidate.CodeDecryptKeyUnavailable)
}

func TestEscrowKeysPipeline_RevokedSigningKeyIsNoLongerTrusted(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "untrusted", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())
	keys := f.resolveKeys(t, b)
	require.Len(t, keys.Verify, 1)
	current, err := f.versions.GetByIDs(t.Context(), entities.OperatorEscrowKeyScope(f.scope), []uuid.UUID{keys.Verify[0].ID})
	require.NoError(t, err)
	require.Len(t, current, 1)
	f.transition(t, current[0], func(v *entities.EscrowKeyVersion) error {
		return v.Revoke("registry reported compromise", true, time.Now().UTC())
	}, entities.EscrowAuditVersionRevoked)

	res, err := f.validateWith(t, b, keys)
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomeFail, res.Outcome)
	assert.Contains(t, res.Codes(), rdevalidate.CodeSigKeyUntrusted)
	assert.Empty(t, res.Digests.PlaintextSHA256, "never decrypted")
}

func TestEscrowKeysPipeline_StoredMaterialThatIsNotTheRecordedKeyIsNeverUsed(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	impostor := rdetest.NewKeyPair(t, "impostor")
	eve, _ := f.receiver(t)
	// The version records the service fingerprint, but the store holds another key.
	value, err := secrets.EncodeOpenPGPSecret(impostor.ArmoredPrivate, "")
	require.NoError(t, err)
	ref, err := f.secretStore.Put(t.Context(), "swapped", nil, value)
	require.NoError(t, err)
	v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: eve, Purpose: entities.EscrowKeyPurposeDecryptInbound, Version: 1,
		Fingerprint: f.service.Fingerprint, ArmoredPublicKey: f.service.ArmoredPublic, SecretRef: &ref, At: time.Now().UTC()})
	require.NoError(t, err)
	require.NoError(t, v.RecordProbe(true, time.Now().UTC()))
	require.NoError(t, v.Activate(time.Now().UTC().Add(-time.Hour)))
	ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), nil, v, entities.EscrowAuditVersionImported)
	require.NoError(t, err)
	require.NoError(t, f.versions.Create(t.Context(), v, ev))

	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "swapped", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())
	res, err := f.validateWith(t, b, f.resolveKeys(t, b))
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomeError, res.Outcome)
	assert.Contains(t, res.Codes(), rdevalidate.CodeDecryptKeyUnavailable)
}

func TestEscrowKeysPipeline_KeyStoreOutageIsRetriedNotDecided(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	f.receiver(t, f.service)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "outage", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())
	keys := f.resolveKeys(t, b)

	f.secretStore.FailWith = entities.ErrEscrowKeyStoreUnavailable
	_, err := f.validateWith(t, b, keys)
	require.Error(t, err, "an outage is an activity error for Temporal to retry")
	assert.False(t, isNonRetryable(err))
}

func TestEscrowKeysPipeline_NoReceiverMeansNoDecryptionKey(t *testing.T) {
	f := newEVFixture(t, nil, nil, nil, nil, nil)
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "example"}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "noreceiver", pair)
	b := f.bindAt(t, rydeKey, sigKey, time.Now().UTC())
	f.setArrangement(t, entities.EscrowArrangementSpec{Level: entities.EscrowArrangementTLD, Operator: f.scope, TLD: f.tld, Depositor: f.depositorParty(t)})
	keys := f.resolveKeys(t, b)
	assert.Empty(t, keys.Decrypt)
	res, err := f.validateWith(t, b, keys)
	require.NoError(t, err)
	assert.Equal(t, rdevalidate.OutcomeError, res.Outcome, "no receiver is our misconfiguration: ERROR, never a DVFN")
	assert.Contains(t, res.Codes(), rdevalidate.CodeDecryptKeyUnavailable)
}

// ---- sanitisation ----

func (f *evFixture) addPseudonymiseVersion(t *testing.T, eve *entities.EscrowParty, n int) (*entities.EscrowKeyVersion, string) {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	fp, err := rdesanitize.MasterKeyFingerprint(key)
	require.NoError(t, err)
	value, err := secrets.EncodeSymmetricSecret(key)
	require.NoError(t, err)
	ref, err := f.secretStore.Put(t.Context(), "pseudonymise/"+fp, nil, value)
	require.NoError(t, err)
	v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: eve, Purpose: entities.EscrowKeyPurposePseudonymise, Version: n,
		Fingerprint: fp, SecretRef: &ref, At: time.Now().UTC()})
	require.NoError(t, err)
	require.NoError(t, v.RecordProbe(true, time.Now().UTC()))
	ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), nil, v, entities.EscrowAuditVersionGenerated)
	require.NoError(t, err)
	require.NoError(t, f.versions.Create(t.Context(), v, ev))
	return v, fp
}

func (f *evFixture) activate(t *testing.T, v *entities.EscrowKeyVersion, siblings ...*entities.EscrowKeyVersion) {
	t.Helper()
	before := v.Clone()
	var ts []entities.EscrowKeyVersionTransition
	var audits []*entities.EscrowKeyAuditEvent
	for _, s := range siblings {
		ts = append(ts, entities.EscrowKeyVersionTransition{Version: s, ExpectedState: s.State})
	}
	deactivated, err := entities.PlanEscrowKeyActivation(v, siblings, true, time.Now().UTC())
	require.NoError(t, err)
	ts = append(ts, entities.EscrowKeyVersionTransition{Version: v, ExpectedState: before.State})
	for _, d := range deactivated {
		ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), nil, d, entities.EscrowAuditVersionDeactivated)
		require.NoError(t, err)
		audits = append(audits, ev)
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), before, v, entities.EscrowAuditVersionActivated)
	require.NoError(t, err)
	audits = append(audits, ev)
	require.NoError(t, f.versions.ApplyTransitions(t.Context(), ts, audits))
}

func TestEscrowKeysPipeline_DerivativeUsesTheRegistryKeyAndKeepsItAcrossARotation(t *testing.T) {
	f := newESFixture(t)
	eve, _ := f.ev.receiver(t, f.ev.service)
	v1, fp1 := f.ev.addPseudonymiseVersion(t, eve, 1)
	f.ev.activate(t, v1)
	sourceRunID := f.acceptedSource(t, rdetest.DepositOpts{Domains: 2, Contacts: 1})

	b, err := f.bind(t, sourceRunID, "")
	require.NoError(t, err)
	keys := f.resolveKeys(t, b)
	require.NotNil(t, keys.Pseudonymise)
	assert.Equal(t, v1.ID, keys.Pseudonymise.ID)

	p := f.produceWith(t, b, keys)
	require.Equal(t, rdesanitize.OutcomePass, p.Result.Outcome, "findings: %v", p.Result.Findings)
	assert.Equal(t, fp1, p.TokenKeyID)
	assert.Equal(t, v1.ID.String(), p.TokenKeyVersionID)

	// Rotate: v2 replaces v1 (confirmed). A retry of the run that selected v1
	// still tokenises under v1; a new resolution picks v2.
	v2, fp2 := f.ev.addPseudonymiseVersion(t, eve, 2)
	current, err := f.ev.versions.GetByID(t.Context(), entities.OperatorEscrowKeyScope(f.ev.scope), v1.ID)
	require.NoError(t, err)
	f.ev.activate(t, v2, current)

	retry := f.produceWith(t, b, keys)
	require.Equal(t, rdesanitize.OutcomePass, retry.Result.Outcome)
	assert.Equal(t, fp1, retry.TokenKeyID, "a retry keeps its recorded key: identical tokens")

	again := f.resolveKeys(t, b)
	require.NotNil(t, again.Pseudonymise)
	assert.Equal(t, v2.ID, again.Pseudonymise.ID)
	rotated := f.produceWith(t, b, again)
	assert.Equal(t, fp2, rotated.TokenKeyID)
	assert.NotEqual(t, fp1, fp2)

	// Revoking v1 withdraws it even from the run that recorded it.
	current, err = f.ev.versions.GetByID(t.Context(), entities.OperatorEscrowKeyScope(f.ev.scope), v1.ID)
	require.NoError(t, err)
	before := current.Clone()
	require.NoError(t, current.Revoke("rotated after exposure", true, time.Now().UTC()))
	ev, err := entities.NewEscrowKeyVersionAuditEvent(auditCtx(), before, current, entities.EscrowAuditVersionRevoked)
	require.NoError(t, err)
	require.NoError(t, f.ev.versions.ApplyTransitions(t.Context(), []entities.EscrowKeyVersionTransition{{Version: current, ExpectedState: before.State}}, []*entities.EscrowKeyAuditEvent{ev}))
	revoked := f.produceWith(t, b, keys)
	assert.Equal(t, rdesanitize.OutcomeError, revoked.Result.Outcome)
	assert.True(t, revoked.Result.Has(rdesanitize.CodeTokenKeyUnavailable))
}
