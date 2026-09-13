package escrowkeys_test

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/escrowkeys"
	"github.com/onasunnymorning/domain-os/internal/application/escrowkeys/escrowkeystest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fpA = "0123456789ABCDEF0123456789ABCDEF01234567"
	fpB = "FEDCBA9876543210FEDCBA9876543210FEDCBA98"
	fpC = "AAAABBBBCCCCDDDDEEEEFFFF0000111122223333"
	pub = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQ==\n-----END PGP PUBLIC KEY BLOCK-----"
)

var now = time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

type world struct {
	versions     *escrowkeystest.Versions
	arrangements *escrowkeystest.Arrangements
	resolver     *escrowkeys.Resolver
}

func newWorld() world {
	v, a := escrowkeystest.NewVersions(), escrowkeystest.NewArrangements()
	return world{versions: v, arrangements: a, resolver: escrowkeys.NewResolver(v, a)}
}

func party(t *testing.T, owner entities.EscrowKeyOwner, kind entities.EscrowPartyKind, side entities.EscrowPartySide) *entities.EscrowParty {
	t.Helper()
	p, err := entities.NewEscrowParty(owner, "p", kind, side, "t", now)
	require.NoError(t, err)
	return p
}

func (w world) version(t *testing.T, p *entities.EscrowParty, purpose entities.EscrowKeyPurpose, n int, fp string, state entities.EscrowKeyVersionState, at time.Time) *entities.EscrowKeyVersion {
	t.Helper()
	spec := entities.EscrowKeyVersionSpec{Party: p, Purpose: purpose, Version: n, Fingerprint: fp, ArmoredPublicKey: pub, At: now}
	if purpose == entities.EscrowKeyPurposeDecryptInbound {
		spec.SecretRef = &entities.EscrowSecretRef{Name: "n/" + fp}
	}
	v, err := entities.NewEscrowKeyVersion(spec)
	require.NoError(t, err)
	v.LastProbeOK = true
	if state != entities.EscrowKeyStaged {
		require.NoError(t, v.Activate(at.Add(-time.Hour)))
	}
	if state == entities.EscrowKeyHistorical {
		require.NoError(t, v.Deactivate(at))
	}
	w.versions.Put(v)
	return v
}

func TestResolveInbound_InheritancePerSide(t *testing.T) {
	w := newWorld()
	opA := entities.OperatorID("ryopa")
	eve := party(t, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	opRSP := party(t, entities.OperatorKeyOwner(opA), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	tldRSP := party(t, entities.OperatorKeyOwner(opA), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	dec := w.version(t, eve, entities.EscrowKeyPurposeDecryptInbound, 1, fpA, entities.EscrowKeyActive, now)
	w.version(t, opRSP, entities.EscrowKeyPurposeVerifyInbound, 1, fpB, entities.EscrowKeyActive, now)
	tldSig := w.version(t, tldRSP, entities.EscrowKeyPurposeVerifyInbound, 1, fpC, entities.EscrowKeyActive, now)

	_, err := w.arrangements.Set(entities.EscrowArrangementSpec{Level: entities.EscrowArrangementPlatform, Receiver: eve})
	require.NoError(t, err)
	_, err = w.arrangements.Set(entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: opA, Depositor: opRSP})
	require.NoError(t, err)
	_, err = w.arrangements.Set(entities.EscrowArrangementSpec{Level: entities.EscrowArrangementTLD, Operator: opA, TLD: "special", Depositor: tldRSP})
	require.NoError(t, err)

	ctx := context.Background()
	sel, err := w.resolver.ResolveInbound(ctx, opA, "plain", now)
	require.NoError(t, err)
	assert.Equal(t, entities.EscrowArrangementOperator, sel.Arrangement.Depositor.From)
	assert.Equal(t, entities.EscrowArrangementPlatform, sel.Arrangement.Receiver.From)
	require.Len(t, sel.Verify, 1)
	assert.Equal(t, fpB, sel.Verify[0].Fingerprint)
	require.Len(t, sel.Decrypt, 1)
	assert.Equal(t, dec.ID, sel.Decrypt[0].ID, "a platform receiver's key is usable by every operator")

	sel, err = w.resolver.ResolveInbound(ctx, opA, "special", now)
	require.NoError(t, err)
	require.Len(t, sel.Verify, 1)
	assert.Equal(t, tldSig.ID, sel.Verify[0].ID, "the TLD override wins for the depositor")
	assert.Equal(t, dec.ID, sel.Decrypt[0].ID, "and the receiver is still inherited")

	// Another operator inherits only the platform default and sees none of A's parties.
	sel, err = w.resolver.ResolveInbound(ctx, "ryopb", "special", now)
	require.NoError(t, err)
	assert.Nil(t, sel.Arrangement.Depositor)
	assert.Empty(t, sel.Verify)
	assert.Len(t, sel.Decrypt, 1)
}

func TestResolveInbound_NoReceiverSelectsNoDecryptionKeyAndOrdersActiveFirst(t *testing.T) {
	w := newWorld()
	op := entities.OperatorID("ryopa")
	rsp := party(t, entities.OperatorKeyOwner(op), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	w.version(t, rsp, entities.EscrowKeyPurposeVerifyInbound, 1, fpA, entities.EscrowKeyHistorical, now.Add(time.Hour))
	w.version(t, rsp, entities.EscrowKeyPurposeVerifyInbound, 2, fpB, entities.EscrowKeyActive, now)
	w.version(t, rsp, entities.EscrowKeyPurposeVerifyInbound, 3, fpC, entities.EscrowKeyStaged, now)
	_, err := w.arrangements.Set(entities.EscrowArrangementSpec{Level: entities.EscrowArrangementOperator, Operator: op, Depositor: rsp})
	require.NoError(t, err)

	sel, err := w.resolver.ResolveInbound(context.Background(), op, "example", now)
	require.NoError(t, err)
	assert.Empty(t, sel.Decrypt, "no receiver: nothing to decrypt with")
	require.Len(t, sel.Verify, 2, "staged versions are never selected")
	assert.Equal(t, fpB, sel.Verify[0].Fingerprint, "active before historical")
	assert.Equal(t, fpA, sel.Verify[1].Fingerprint)
}

func TestStillUsable_DropsRevokedAndRefingerprintedVersions(t *testing.T) {
	w := newWorld()
	op := entities.OperatorID("ryopa")
	rsp := party(t, entities.OperatorKeyOwner(op), entities.EscrowPartyRSP, entities.EscrowPartyExternal)
	keep := w.version(t, rsp, entities.EscrowKeyPurposeVerifyInbound, 1, fpA, entities.EscrowKeyActive, now)
	revoked := w.version(t, rsp, entities.EscrowKeyPurposeVerifyInbound, 2, fpB, entities.EscrowKeyActive, now)
	require.NoError(t, revoked.Revoke("r", false, now))
	w.versions.Put(revoked)

	wrongFP := keep.Ref()
	wrongFP.Fingerprint = fpC
	got, err := w.resolver.StillUsable(context.Background(), op, []entities.EscrowKeyVersionRef{revoked.Ref(), keep.Ref(), wrongFP})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, keep.ID, got[0].ID)

	other, err := w.resolver.StillUsable(context.Background(), "ryopb", []entities.EscrowKeyVersionRef{keep.Ref()})
	require.NoError(t, err)
	assert.Empty(t, other, "another operator's version is invisible, even by id")
}

func TestResolvePseudonymisation_OnlyTheActiveVersion(t *testing.T) {
	w := newWorld()
	op := entities.OperatorID("ryopa")
	eve := party(t, entities.PlatformKeyOwner(), entities.EscrowPartyDEA, entities.EscrowPartySelf)
	ctx := context.Background()

	ref, _, err := w.resolver.ResolvePseudonymisation(ctx, op, "example")
	require.NoError(t, err)
	assert.Nil(t, ref, "no receiver: nothing to resolve")

	_, err = w.arrangements.Set(entities.EscrowArrangementSpec{Level: entities.EscrowArrangementPlatform, Receiver: eve})
	require.NoError(t, err)
	mk := func(n int, fp string, state entities.EscrowKeyVersionState) *entities.EscrowKeyVersion {
		v, err := entities.NewEscrowKeyVersion(entities.EscrowKeyVersionSpec{Party: eve, Purpose: entities.EscrowKeyPurposePseudonymise, Version: n,
			Fingerprint: fp, SecretRef: &entities.EscrowSecretRef{Name: fp}, At: now})
		require.NoError(t, err)
		v.LastProbeOK = true
		if state != entities.EscrowKeyStaged {
			require.NoError(t, v.Activate(now))
		}
		if state == entities.EscrowKeyHistorical {
			require.NoError(t, v.Deactivate(now))
		}
		w.versions.Put(v)
		return v
	}
	mk(1, "AAAAAAAAAAAAAAAA", entities.EscrowKeyHistorical)
	active := mk(2, "BBBBBBBBBBBBBBBB", entities.EscrowKeyActive)
	mk(3, "CCCCCCCCCCCCCCCC", entities.EscrowKeyStaged)

	ref, eff, err := w.resolver.ResolvePseudonymisation(ctx, op, "example")
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, active.ID, ref.ID)
	assert.Equal(t, eve.ID, eff.Receiver.PartyID)
}
