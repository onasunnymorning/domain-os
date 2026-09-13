package activities

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/escrowkeys/escrowkeystest"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

var probeNow = time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)

type probeFixture struct {
	eve, rsp *entities.EscrowParty
	store    *secrets.MemorySecretStore
}

func newProbeFixture(t *testing.T) probeFixture {
	t.Helper()
	eve, err := entities.NewEscrowParty(entities.PlatformKeyOwner(), "EVE", entities.EscrowPartyDEA, entities.EscrowPartySelf, "t", probeNow)
	require.NoError(t, err)
	op, _ := entities.NewOperatorID("ryop1")
	rsp, err := entities.NewEscrowParty(entities.OperatorKeyOwner(op), "Acme", entities.EscrowPartyRSP, entities.EscrowPartyExternal, "t", probeNow)
	require.NoError(t, err)
	return probeFixture{eve: eve, rsp: rsp, store: secrets.NewMemorySecretStore()}
}

func (f probeFixture) decryptVersion(t *testing.T, kp rdetest.KeyPair, fingerprint string) *entities.EscrowKeyVersion {
	t.Helper()
	value, err := secrets.EncodeOpenPGPSecret(rdetest.EncryptedArmoredPrivate(t, kp, "pw"), "pw")
	require.NoError(t, err)
	ref, err := f.store.Put(context.Background(), uuid.NewString(), nil, value)
	require.NoError(t, err)
	v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: f.eve, Purpose: entities.EscrowKeyPurposeDecryptInbound, Version: 1,
		Fingerprint: fingerprint, ArmoredPublicKey: kp.ArmoredPublic, SecretRef: &ref, At: probeNow})
	require.NoError(t, err)
	return v
}

func runProbe(t *testing.T, acts *EscrowKeyActivities, owner EscrowKeyOwnerRef, id uuid.UUID) (ProbeEscrowKeyVersionOutput, error) {
	t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestActivityEnvironment()
	env.RegisterActivity(acts)
	val, err := env.ExecuteActivity(acts.ProbeEscrowKeyVersion, ProbeEscrowKeyVersionInput{Owner: owner, VersionID: id.String(), WorkflowID: "wf"})
	if err != nil {
		return ProbeEscrowKeyVersionOutput{}, err
	}
	var out ProbeEscrowKeyVersionOutput
	require.NoError(t, val.Get(&out))
	return out, nil
}

func TestProbeEscrowKeyVersion(t *testing.T) {
	f := newProbeFixture(t)
	kp := rdetest.NewKeyPair(t, "eve")
	platform := OwnerRefFor(entities.PlatformKeyOwner())

	t.Run("private key round trip passes", func(t *testing.T) {
		v := f.decryptVersion(t, kp, kp.Fingerprint)
		acts := NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(f.store, time.Minute))
		out, err := runProbe(t, acts, platform, v.ID)
		require.NoError(t, err)
		assert.True(t, out.OK)
		assert.Equal(t, EscrowKeyProbeOK, out.Code)
	})

	t.Run("a stored key that is not the recorded one is refused", func(t *testing.T) {
		other := rdetest.NewKeyPair(t, "other")
		v := f.decryptVersion(t, other, kp.Fingerprint)
		acts := NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(f.store, time.Minute))
		out, err := runProbe(t, acts, platform, v.ID)
		require.NoError(t, err)
		assert.False(t, out.OK)
		assert.Equal(t, EscrowKeyProbeFingerprintMismatch, out.Code)
	})

	t.Run("no key store is a verdict, an outage is retryable", func(t *testing.T) {
		v := f.decryptVersion(t, kp, kp.Fingerprint)
		out, err := runProbe(t, NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(nil, time.Minute)), platform, v.ID)
		require.NoError(t, err)
		assert.Equal(t, EscrowKeyProbeStoreNotConfigured, out.Code)

		down := secrets.NewMemorySecretStore()
		down.FailWith = entities.ErrEscrowKeyStoreUnavailable
		_, err = runProbe(t, NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(down, time.Minute)), platform, v.ID)
		var appErr *temporal.ApplicationError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, EscrowKeyStoreUnavailableErrorType, appErr.Type())
		assert.False(t, appErr.NonRetryable())
	})

	t.Run("symmetric key", func(t *testing.T) {
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		fp, err := rdesanitize.MasterKeyFingerprint(key)
		require.NoError(t, err)
		value, err := secrets.EncodeSymmetricSecret(key)
		require.NoError(t, err)
		ref, err := f.store.Put(context.Background(), uuid.NewString(), nil, value)
		require.NoError(t, err)
		v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: f.eve, Purpose: entities.EscrowKeyPurposePseudonymise, Version: 1, Fingerprint: fp, SecretRef: &ref, At: probeNow})
		require.NoError(t, err)
		out, err := runProbe(t, NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(f.store, time.Minute)), platform, v.ID)
		require.NoError(t, err)
		assert.True(t, out.OK)
	})

	t.Run("public key needs no store", func(t *testing.T) {
		v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: f.rsp, Purpose: entities.EscrowKeyPurposeVerifyInbound, Version: 1,
			Fingerprint: kp.Fingerprint, ArmoredPublicKey: kp.ArmoredPublic, At: probeNow})
		require.NoError(t, err)
		out, err := runProbe(t, NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(nil, time.Minute)),
			EscrowKeyOwnerRef{Kind: "operator", Operator: "ryop1"}, v.ID)
		require.NoError(t, err)
		assert.True(t, out.OK)
	})

	t.Run("scope: an operator cannot probe a platform key, a revoked key cannot be probed", func(t *testing.T) {
		v := f.decryptVersion(t, kp, kp.Fingerprint)
		acts := NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(f.store, time.Minute))
		_, err := runProbe(t, acts, EscrowKeyOwnerRef{Kind: "operator", Operator: "ryop1"}, v.ID)
		var appErr *temporal.ApplicationError
		require.True(t, errors.As(err, &appErr))
		assert.True(t, appErr.NonRetryable())

		require.NoError(t, v.Revoke("test", true, probeNow))
		_, err = runProbe(t, NewEscrowKeyActivitiesWithDeps(escrowkeystest.NewVersions(v), secrets.NewEscrowKeyLoader(f.store, time.Minute)), platform, v.ID)
		require.True(t, errors.As(err, &appErr))
		assert.True(t, appErr.NonRetryable())
	})
}

func TestRecordEscrowKeyProbe(t *testing.T) {
	f := newProbeFixture(t)
	kp := rdetest.NewKeyPair(t, "eve")
	v := f.decryptVersion(t, kp, kp.Fingerprint)
	repo := escrowkeystest.NewVersions(v)
	acts := NewEscrowKeyActivitiesWithDeps(repo, secrets.NewEscrowKeyLoader(f.store, time.Minute))
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestActivityEnvironment()
	env.RegisterActivity(acts)

	_, err := env.ExecuteActivity(acts.RecordEscrowKeyProbe, RecordEscrowKeyProbeInput{
		Owner: OwnerRefFor(entities.PlatformKeyOwner()), VersionID: v.ID.String(), OK: true, Code: EscrowKeyProbeOK,
		Actor: "auth0|ops", WorkflowID: "wf-1", TemporalRunID: "run-1", At: probeNow,
	})
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), entities.PlatformEscrowKeyScope(entities.NewPlatformScope()), v.ID)
	require.NoError(t, err)
	assert.True(t, got.LastProbeOK)
	assert.Equal(t, entities.EscrowKeyStaged, got.State, "a probe never changes state")
	require.Len(t, repo.Audits, 1)
	a := repo.Audits[0]
	assert.Equal(t, entities.EscrowAuditVersionProbePassed, a.Action)
	assert.Equal(t, EscrowKeyProbeOK, a.Reason)
	assert.Equal(t, "wf-1", a.CorrelationID, "INV-16: correlation is the workflow id")
	assert.Equal(t, "run-1", a.TraceID)
	assert.Equal(t, "auth0|ops", a.Actor)
	require.NoError(t, got.Activate(probeNow), "a recorded probe unlocks activation")
}
