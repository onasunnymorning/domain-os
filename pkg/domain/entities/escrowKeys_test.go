package entities

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSymFP = "ABCDEFGHIJKLMNOP"

var keyNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func mustParty(t *testing.T, owner EscrowKeyOwner, kind EscrowPartyKind, side EscrowPartySide) *EscrowParty {
	t.Helper()
	p, err := NewEscrowParty(owner, "party "+string(kind)+" "+string(side), kind, side, "tester", keyNow)
	require.NoError(t, err)
	return p
}

func mustVersion(t *testing.T, p *EscrowParty, purpose EscrowKeyPurpose, n int) *EscrowKeyVersion {
	t.Helper()
	spec := EscrowKeyVersionSpec{Party: p, Purpose: purpose, Version: n, CreatedBy: "tester", At: keyNow}
	switch escrowKeyPurposePolicies[purpose].Material {
	case EscrowKeyMaterialOpenPGPPublic:
		spec.Fingerprint, spec.ArmoredPublicKey = testFP, testKey
	case EscrowKeyMaterialOpenPGPPrivate:
		spec.Fingerprint, spec.ArmoredPublicKey = testFP, testKey
		spec.SecretRef = &EscrowSecretRef{Name: "escrow-keys/platform/x/y"}
	case EscrowKeyMaterialSymmetric:
		spec.Fingerprint = testSymFP
		spec.SecretRef = &EscrowSecretRef{Name: "escrow-keys/platform/x/z"}
	}
	v, err := NewEscrowKeyVersion(spec)
	require.NoError(t, err)
	return v
}

func TestPlatformScope(t *testing.T) {
	var zero PlatformScope
	assert.ErrorIs(t, zero.Validate(), ErrInvalidPlatformScope, "the zero value is not a grant")
	assert.NoError(t, NewPlatformScope().Validate())
}

func TestEscrowKeyOwner(t *testing.T) {
	a, b := testScope(t), OperatorID("ryop2")
	assert.NoError(t, PlatformKeyOwner().Validate())
	assert.NoError(t, OperatorKeyOwner(a).Validate())
	assert.ErrorIs(t, EscrowKeyOwner{Kind: EscrowKeyOwnerOperator}.Validate(), ErrInvalidEscrowKeyOwner)
	assert.ErrorIs(t, EscrowKeyOwner{Kind: EscrowKeyOwnerPlatform, Operator: a}.Validate(), ErrInvalidEscrowKeyOwner)
	assert.ErrorIs(t, EscrowKeyOwner{}.Validate(), ErrInvalidEscrowKeyOwner)

	assert.True(t, PlatformKeyOwner().VisibleTo(a), "platform parties are visible to every operator")
	assert.True(t, OperatorKeyOwner(a).VisibleTo(a))
	assert.False(t, OperatorKeyOwner(a).VisibleTo(b), "never another operator's")
	assert.Equal(t, "operator:ryop1", OperatorKeyOwner(a).String())
}

func TestNewEscrowParty_PurposesDerivedFromRole(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	assert.Equal(t, []EscrowKeyPurpose{EscrowKeyPurposeDecryptInbound, EscrowKeyPurposePseudonymise}, eve.Purposes())
	assert.True(t, eve.CanReceive())
	assert.False(t, eve.CanDeposit())
	assert.NoError(t, eve.AllowsPurpose(EscrowKeyPurposePseudonymise))
	assert.ErrorIs(t, eve.AllowsPurpose(EscrowKeyPurposeVerifyInbound), ErrEscrowKeyPurposeNotAllowed)

	rsp := mustParty(t, OperatorKeyOwner(testScope(t)), EscrowPartyRSP, EscrowPartyExternal)
	assert.Equal(t, []EscrowKeyPurpose{EscrowKeyPurposeVerifyInbound}, rsp.Purposes())
	assert.True(t, rsp.CanDeposit())

	_, err := NewEscrowParty(PlatformKeyOwner(), "our rsp", EscrowPartyRSP, EscrowPartySelf, "t", keyNow)
	assert.ErrorIs(t, err, ErrEscrowPartyRoleNotSupported, "outbound roles arrive with escrow targets")
	_, err = NewEscrowParty(PlatformKeyOwner(), "a dea", EscrowPartyDEA, EscrowPartyExternal, "t", keyNow)
	assert.ErrorIs(t, err, ErrEscrowPartyRoleNotSupported)
	_, err = NewEscrowParty(PlatformKeyOwner(), " ", EscrowPartyDEA, EscrowPartySelf, "t", keyNow)
	assert.ErrorIs(t, err, ErrInvalidEscrowParty)
	_, err = NewEscrowParty(PlatformKeyOwner(), "x", "BANK", EscrowPartySelf, "t", keyNow)
	assert.ErrorIs(t, err, ErrInvalidEscrowParty)
}

func TestNewEscrowKeyVersion_MaterialRules(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	rsp := mustParty(t, PlatformKeyOwner(), EscrowPartyRSP, EscrowPartyExternal)

	v := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 1)
	assert.Equal(t, EscrowKeyStaged, v.State)
	assert.Equal(t, eve.Owner, v.Owner, "owner is copied from the party")

	cases := []struct {
		name string
		spec EscrowKeyVersionSpec
		want error
	}{
		{"private without secret ref", EscrowKeyVersionSpec{Party: eve, Purpose: EscrowKeyPurposeDecryptInbound, Version: 1, Fingerprint: testFP, ArmoredPublicKey: testKey, At: keyNow}, ErrInvalidEscrowKeyVersion},
		{"private without public half", EscrowKeyVersionSpec{Party: eve, Purpose: EscrowKeyPurposeDecryptInbound, Version: 1, Fingerprint: testFP, SecretRef: &EscrowSecretRef{Name: "n"}, At: keyNow}, ErrInvalidEscrowKeyVersion},
		{"public with secret ref", EscrowKeyVersionSpec{Party: rsp, Purpose: EscrowKeyPurposeVerifyInbound, Version: 1, Fingerprint: testFP, ArmoredPublicKey: testKey, SecretRef: &EscrowSecretRef{Name: "n"}, At: keyNow}, ErrEscrowKeyMaterialNotApplicable},
		{"symmetric with armored key", EscrowKeyVersionSpec{Party: eve, Purpose: EscrowKeyPurposePseudonymise, Version: 1, Fingerprint: testSymFP, ArmoredPublicKey: testKey, SecretRef: &EscrowSecretRef{Name: "n"}, At: keyNow}, ErrEscrowKeyMaterialNotApplicable},
		{"symmetric with an OpenPGP fingerprint", EscrowKeyVersionSpec{Party: eve, Purpose: EscrowKeyPurposePseudonymise, Version: 1, Fingerprint: testFP, SecretRef: &EscrowSecretRef{Name: "n"}, At: keyNow}, ErrInvalidEscrowKeyVersion},
		{"purpose not allowed for party", EscrowKeyVersionSpec{Party: rsp, Purpose: EscrowKeyPurposeDecryptInbound, Version: 1, Fingerprint: testFP, ArmoredPublicKey: testKey, SecretRef: &EscrowSecretRef{Name: "n"}, At: keyNow}, ErrEscrowKeyPurposeNotAllowed},
		{"version zero", EscrowKeyVersionSpec{Party: rsp, Purpose: EscrowKeyPurposeVerifyInbound, Version: 0, Fingerprint: testFP, ArmoredPublicKey: testKey, At: keyNow}, ErrInvalidEscrowKeyVersion},
		{"inverted window", EscrowKeyVersionSpec{Party: rsp, Purpose: EscrowKeyPurposeVerifyInbound, Version: 1, Fingerprint: testFP, ArmoredPublicKey: testKey, NotBefore: &keyNow, NotAfter: ptrTime(keyNow.Add(-time.Hour)), At: keyNow}, ErrEscrowKeyInvalidWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEscrowKeyVersion(tc.spec)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestEscrowKeyVersion_Lifecycle(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	v := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 1)

	assert.ErrorIs(t, v.Activate(keyNow), ErrEscrowKeyNotProbed, "private material must be proven on a worker first")
	require.NoError(t, v.RecordProbe(false, keyNow))
	assert.ErrorIs(t, v.Activate(keyNow), ErrEscrowKeyNotProbed, "a failed probe is not a probe")
	require.NoError(t, v.RecordProbe(true, keyNow))
	require.NoError(t, v.Activate(keyNow))
	assert.ErrorIs(t, v.Activate(keyNow), ErrEscrowKeyInvalidTransition)
	assert.ErrorIs(t, v.Destroy(keyNow), ErrEscrowKeyInvalidTransition, "an active version must be deactivated or revoked first")

	deactivated := keyNow.Add(24 * time.Hour)
	require.NoError(t, v.Deactivate(deactivated))
	assert.Equal(t, EscrowKeyHistorical, v.State)

	assert.ErrorIs(t, v.Revoke(" ", true, keyNow), ErrInvalidEscrowKeyVersion, "a reason is required")
	require.NoError(t, v.Revoke("key leaked", true, deactivated))
	assert.True(t, v.Compromised)
	assert.ErrorIs(t, v.RecordProbe(true, keyNow), ErrEscrowKeyInvalidTransition)
	require.NoError(t, v.Destroy(deactivated))
	assert.Equal(t, EscrowKeyDestroyed, v.State)
	assert.ErrorIs(t, v.Revoke("again", false, keyNow), ErrEscrowKeyInvalidTransition)

	rsp := mustParty(t, PlatformKeyOwner(), EscrowPartyRSP, EscrowPartyExternal)
	pub := mustVersion(t, rsp, EscrowKeyPurposeVerifyInbound, 1)
	assert.NoError(t, pub.Activate(keyNow), "public keys need no worker probe")
}

func TestEscrowKeyVersion_Usability(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	before, deactivatedAt, after := keyNow.Add(-time.Hour), keyNow, keyNow.Add(time.Hour)

	dec := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 1)
	assert.False(t, dec.UsableForDeposit(before), "staged is never selected")
	assert.True(t, dec.UsableForRecordedRun(), "but a retry that recorded it may continue")
	require.NoError(t, dec.RecordProbe(true, before))
	require.NoError(t, dec.Activate(before.Add(-time.Hour)))
	assert.True(t, dec.UsableForDeposit(after))
	require.NoError(t, dec.Deactivate(deactivatedAt))
	assert.True(t, dec.UsableForDeposit(before), "historical still opens deposits received before deactivation")
	assert.False(t, dec.UsableForDeposit(after), "but not deposits received after it")
	assert.False(t, dec.UsableForNewWork())
	require.NoError(t, dec.Revoke("rotated early", false, after))
	assert.False(t, dec.UsableForDeposit(before), "revocation wins")
	assert.False(t, dec.UsableForRecordedRun(), "including for retries")

	pseudo := mustVersion(t, eve, EscrowKeyPurposePseudonymise, 1)
	require.NoError(t, pseudo.RecordProbe(true, before))
	require.NoError(t, pseudo.Activate(before))
	require.NoError(t, pseudo.Deactivate(deactivatedAt))
	assert.False(t, pseudo.UsableForDeposit(before), "a historical pseudonymisation key is never reselected by receipt time")
	assert.True(t, pseudo.UsableForRecordedRun())

	rsp := mustParty(t, PlatformKeyOwner(), EscrowPartyRSP, EscrowPartyExternal)
	windowed, err := NewEscrowKeyVersion(EscrowKeyVersionSpec{Party: rsp, Purpose: EscrowKeyPurposeVerifyInbound, Version: 1,
		Fingerprint: testFP, ArmoredPublicKey: testKey, NotBefore: &deactivatedAt, At: keyNow})
	require.NoError(t, err)
	require.NoError(t, windowed.Activate(keyNow))
	assert.False(t, windowed.UsableForDeposit(before), "outside NotBefore")
	assert.True(t, windowed.UsableForDeposit(after))
}

func TestPlanEscrowKeyActivation(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	probedActive := func(v *EscrowKeyVersion) *EscrowKeyVersion {
		require.NoError(t, v.RecordProbe(true, keyNow))
		require.NoError(t, v.Activate(keyNow))
		return v
	}

	t.Run("rollover purposes allow overlap", func(t *testing.T) {
		v1 := probedActive(mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 1))
		v2 := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 2)
		require.NoError(t, v2.RecordProbe(true, keyNow))
		deact, err := PlanEscrowKeyActivation(v2, []*EscrowKeyVersion{v1, v2}, false, keyNow)
		require.NoError(t, err)
		assert.Empty(t, deact)
		assert.Equal(t, EscrowKeyActive, v1.State)
		assert.Equal(t, EscrowKeyActive, v2.State)
	})

	t.Run("pseudonymisation replaces only when confirmed", func(t *testing.T) {
		v1 := probedActive(mustVersion(t, eve, EscrowKeyPurposePseudonymise, 1))
		v2 := mustVersion(t, eve, EscrowKeyPurposePseudonymise, 2)
		require.NoError(t, v2.RecordProbe(true, keyNow))

		_, err := PlanEscrowKeyActivation(v2, []*EscrowKeyVersion{v1}, false, keyNow)
		assert.ErrorIs(t, err, ErrEscrowKeyReplaceNotConfirmed)
		assert.Equal(t, EscrowKeyStaged, v2.State, "an unconfirmed activation leaves nothing half-done")
		assert.Nil(t, v2.ActivatedAt)
		assert.Equal(t, EscrowKeyActive, v1.State)

		deact, err := PlanEscrowKeyActivation(v2, []*EscrowKeyVersion{v1}, true, keyNow)
		require.NoError(t, err)
		require.Len(t, deact, 1)
		assert.Equal(t, v1.ID, deact[0].ID)
		assert.Equal(t, EscrowKeyHistorical, v1.State)
		assert.Equal(t, EscrowKeyActive, v2.State)
	})

	t.Run("siblings must share party and purpose", func(t *testing.T) {
		a := mustVersion(t, eve, EscrowKeyPurposePseudonymise, 1)
		require.NoError(t, a.RecordProbe(true, keyNow))
		other := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 1)
		_, err := PlanEscrowKeyActivation(a, []*EscrowKeyVersion{other}, true, keyNow)
		assert.ErrorIs(t, err, ErrInvalidEscrowKeyVersion)
	})
}

func TestNewEscrowArrangement(t *testing.T) {
	opA, opB := testScope(t), OperatorID("ryop2")
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	catalogueRSP := mustParty(t, PlatformKeyOwner(), EscrowPartyRSP, EscrowPartyExternal)
	rspA := mustParty(t, OperatorKeyOwner(opA), EscrowPartyRSP, EscrowPartyExternal)

	platform, err := NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementPlatform, Direction: EscrowArrangementInbound, Receiver: eve, At: keyNow})
	require.NoError(t, err)
	assert.Equal(t, 1, platform.Revision)

	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementPlatform, Direction: EscrowArrangementInbound, Depositor: rspA, At: keyNow})
	assert.ErrorIs(t, err, ErrEscrowKeyOwnerMismatch, "a platform default cannot point at an operator's party")

	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opB, Direction: EscrowArrangementInbound, Depositor: rspA, At: keyNow})
	assert.ErrorIs(t, err, ErrEscrowKeyOwnerMismatch, "operator B cannot use operator A's party")

	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, Direction: EscrowArrangementInbound, Depositor: eve, At: keyNow})
	assert.ErrorIs(t, err, ErrInvalidEscrowArrangement, "roles are checked: a DEA cannot deposit")

	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementTLD, Operator: opA, Direction: EscrowArrangementInbound, At: keyNow, TLD: "example"})
	assert.ErrorIs(t, err, ErrInvalidEscrowArrangement, "an empty override is a removal, not a row")

	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, TLD: "example", Direction: EscrowArrangementInbound, Depositor: rspA, At: keyNow})
	assert.ErrorIs(t, err, ErrInvalidEscrowArrangement)

	op1, err := NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, Direction: EscrowArrangementInbound, Depositor: catalogueRSP, At: keyNow})
	require.NoError(t, err, "operators may use platform catalogue parties")
	op2, err := NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, Direction: EscrowArrangementInbound, Depositor: rspA, Previous: op1, At: keyNow})
	require.NoError(t, err)
	assert.Equal(t, 2, op2.Revision)

	require.NoError(t, op1.Supersede(keyNow))
	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, Direction: EscrowArrangementInbound, Depositor: rspA, Previous: op1, At: keyNow})
	assert.ErrorIs(t, err, ErrEscrowArrangementConflict, "cannot follow a revision that is no longer live")

	tld, err := NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementTLD, Operator: opA, TLD: "Example.", Direction: EscrowArrangementInbound, Depositor: rspA, At: keyNow})
	require.NoError(t, err)
	assert.Equal(t, "example", tld.TLD)
	_, err = NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementTLD, Operator: opA, TLD: "example", Direction: EscrowArrangementInbound, Depositor: rspA, Previous: op2, At: keyNow})
	assert.ErrorIs(t, err, ErrInvalidEscrowArrangement, "previous must be at the same level")
}

func TestResolveEscrowArrangement(t *testing.T) {
	opA := testScope(t)
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	opRSP := mustParty(t, OperatorKeyOwner(opA), EscrowPartyRSP, EscrowPartyExternal)
	tldRSP := mustParty(t, OperatorKeyOwner(opA), EscrowPartyRSP, EscrowPartyExternal)
	mk := func(spec EscrowArrangementSpec) *EscrowArrangement {
		spec.Direction, spec.At = EscrowArrangementInbound, keyNow
		a, err := NewEscrowArrangement(spec)
		require.NoError(t, err)
		return a
	}
	platform := mk(EscrowArrangementSpec{Level: EscrowArrangementPlatform, Receiver: eve})
	operator := mk(EscrowArrangementSpec{Level: EscrowArrangementOperator, Operator: opA, Depositor: opRSP})
	override := mk(EscrowArrangementSpec{Level: EscrowArrangementTLD, Operator: opA, TLD: "example", Depositor: tldRSP})

	eff := ResolveEscrowArrangement(nil, operator, platform)
	require.NotNil(t, eff.Depositor)
	require.NotNil(t, eff.Receiver)
	assert.Equal(t, opRSP.ID, eff.Depositor.PartyID)
	assert.Equal(t, EscrowArrangementOperator, eff.Depositor.From)
	assert.Equal(t, eve.ID, eff.Receiver.PartyID)
	assert.Equal(t, EscrowArrangementPlatform, eff.Receiver.From)
	assert.Equal(t, 0, eff.TLDRevision)

	eff = ResolveEscrowArrangement(override, operator, platform)
	assert.Equal(t, tldRSP.ID, eff.Depositor.PartyID, "the TLD override wins for its side")
	assert.Equal(t, EscrowArrangementTLD, eff.Depositor.From)
	assert.Equal(t, eve.ID, eff.Receiver.PartyID, "and the other side still inherits")
	assert.Equal(t, 1, eff.TLDRevision)
	assert.Equal(t, 1, eff.OperatorRevision)
	assert.Equal(t, 1, eff.PlatformRevision)

	assert.Equal(t, EffectiveEscrowArrangement{}, ResolveEscrowArrangement(nil, nil, nil))
}

func TestEscrowKeyAuditEvent_OutboxEventCarriesNoMaterial(t *testing.T) {
	eve := mustParty(t, PlatformKeyOwner(), EscrowPartyDEA, EscrowPartySelf)
	before := mustVersion(t, eve, EscrowKeyPurposeDecryptInbound, 3)
	after := before.Clone()
	require.NoError(t, after.Revoke("compromised laptop", true, keyNow))

	ctx := EscrowKeyAuditContext{Actor: "auth0|u1", TraceID: "trace", CorrelationID: "corr", At: keyNow}
	e, err := NewEscrowKeyVersionAuditEvent(ctx, before, after, EscrowAuditVersionRevoked)
	require.NoError(t, err)
	assert.Equal(t, "STAGED", e.StateBefore)
	assert.Equal(t, "REVOKED", e.StateAfter)
	assert.True(t, e.Compromised)

	ev := e.DomainEvent()
	assert.Equal(t, "escrow.key_version.revoked", ev.Type)
	assert.Equal(t, after.ID.String(), ev.Subject)
	assert.Equal(t, "auth0|u1", ev.Actor)
	assert.Equal(t, keyNow, ev.Time)

	raw, err := json.Marshal(ev)
	require.NoError(t, err)
	for _, forbidden := range []string{"PRIVATE KEY", "PUBLIC KEY BLOCK", "passphrase", after.SecretRef.Name} {
		assert.False(t, strings.Contains(string(raw), forbidden), "outbox event must not contain %q", forbidden)
	}

	_, err = NewEscrowKeyVersionAuditEvent(EscrowKeyAuditContext{}, nil, after, EscrowAuditVersionRevoked)
	assert.True(t, errors.Is(err, ErrInvalidEscrowKeyAuditEvent))

	arr, err := NewEscrowArrangement(EscrowArrangementSpec{Level: EscrowArrangementPlatform, Direction: EscrowArrangementInbound, Receiver: eve, At: keyNow})
	require.NoError(t, err)
	ae, err := NewEscrowArrangementAuditEvent(ctx, arr, EscrowAuditArrangementChanged)
	require.NoError(t, err)
	assert.True(t, ae.Owner.IsPlatform())
	assert.NotEqual(t, uuid.Nil, ae.SubjectID)
}

func TestEscrowKeyScope(t *testing.T) {
	opA, opB := testScope(t), OperatorID("ryop2")
	var zero EscrowKeyScope
	assert.Error(t, zero.Validate())
	assert.Error(t, PlatformEscrowKeyScope(PlatformScope{}).Validate(), "an ungranted platform scope is invalid")
	assert.NoError(t, OperatorEscrowKeyScope(opA).Validate())

	platform := PlatformEscrowKeyScope(NewPlatformScope())
	require.NoError(t, platform.Validate())
	assert.True(t, platform.CanSee(PlatformKeyOwner()))
	assert.False(t, platform.CanSee(OperatorKeyOwner(opA)), "the platform scope is not a superuser over operators")
	assert.True(t, platform.CanManage(PlatformKeyOwner()))

	a := OperatorEscrowKeyScope(opA)
	assert.True(t, a.CanSee(PlatformKeyOwner()))
	assert.False(t, a.CanManage(PlatformKeyOwner()), "operators use platform parties but never manage them")
	assert.True(t, a.CanManage(OperatorKeyOwner(opA)))
	assert.False(t, a.CanSee(OperatorKeyOwner(opB)))
	assert.Equal(t, OperatorKeyOwner(opA), a.Owner())
}
