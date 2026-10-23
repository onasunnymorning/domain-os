package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const transferTestCode = "Tr4nsf3r-C0de-Xq9#Lm2!" // #nosec G101 -- a test fixture's authInfo, not a credential

// transferTestDomain is a domain sponsored by losing, a year old, with
// transferTestCode set and out of its transfer lock.
func transferTestDomain(t *testing.T, now time.Time) *Domain {
	t.Helper()
	h, err := HashAuthInfo(transferTestCode)
	require.NoError(t, err)
	return &Domain{
		RoID: "1_DOM-TEST", Name: "move.example", TLDName: "example", ClID: "losing",
		CreatedAt: now.AddDate(-1, 0, 0), ExpiryDate: now.AddDate(0, 2, 0),
		AuthInfo: h, Status: DomainStatus{OK: true},
		RGPStatus: DomainRGPStatus{TransferLockPeriodEnd: now.AddDate(0, -10, 0)},
	}
}

func transferTestPhase() *Phase {
	return &Phase{Name: "GA", Policy: NewPhasePolicy()}
}

func TestDomain_RequestTransfer(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	phase := transferTestPhase()

	t.Run("a valid request: pendingTransfer, and a pending transfer to answer within the pending period", func(t *testing.T) {
		d := transferTestDomain(t, now)
		tr, err := d.RequestTransfer("gaining", transferTestCode, 0, phase, now)
		require.NoError(t, err)
		assert.True(t, d.Status.PendingTransfer)
		assert.False(t, d.Status.OK)
		assert.Equal(t, ClIDType("losing"), d.ClID, "the sponsor changes only on approval")
		assert.Equal(t, TransferStatusPending, tr.Status)
		assert.Equal(t, ClIDType("gaining"), tr.RequestingClID)
		assert.Equal(t, ClIDType("losing"), tr.LosingClID)
		assert.Equal(t, ClIDType("losing"), tr.ActingClID, "acID: the registrar that should act")
		assert.Equal(t, now.AddDate(0, 0, PendingTransferPeriod), tr.ActionDate)
		assert.Equal(t, 1, tr.Years, "one year when no period is given")
		assert.Equal(t, d.ExpiryDate.AddDate(1, 0, 0), tr.ExpiryDate)
	})

	refused := func(t *testing.T, d *Domain, gaining ClIDType, code string, years int, want error) {
		t.Helper()
		before := d.DeepCopy()
		_, err := d.RequestTransfer(gaining, code, years, phase, now)
		require.ErrorIs(t, err, want)
		assert.Equal(t, before, d, "a refused request changes nothing")
	}
	t.Run("the sponsor cannot transfer to itself: 2106", func(t *testing.T) {
		refused(t, transferTestDomain(t, now), "losing", transferTestCode, 1, ErrTransferToSponsor)
	})
	t.Run("a wrong, empty or unset code: 2202, before any status check", func(t *testing.T) {
		d := transferTestDomain(t, now)
		refused(t, d, "gaining", transferTestCode+"x", 1, ErrEPPAuthInfoMismatch)
		refused(t, d, "gaining", "", 1, ErrEPPAuthInfoMismatch)
		d.Status.ServerTransferProhibited = true
		refused(t, d, "gaining", "wrong-code", 1, ErrEPPAuthInfoMismatch)
		d = transferTestDomain(t, now)
		d.AuthInfo = ""
		refused(t, d, "gaining", transferTestCode, 1, ErrEPPAuthInfoMismatch)
	})
	t.Run("already pending transfer: 2300", func(t *testing.T) {
		d := transferTestDomain(t, now)
		d.Status = DomainStatus{PendingTransfer: true}
		refused(t, d, "gaining", transferTestCode, 1, ErrTransferPending)
	})
	t.Run("another pending action, or a transfer prohibition: 2304", func(t *testing.T) {
		for _, st := range []DomainStatus{{PendingDelete: true}, {PendingDelete: true, PendingRestore: true}, {ClientTransferProhibited: true}, {ServerTransferProhibited: true}} {
			d := transferTestDomain(t, now)
			d.Status = st
			refused(t, d, "gaining", transferTestCode, 1, ErrTransferProhibited)
		}
	})
	t.Run("an update prohibition does not stop a transfer", func(t *testing.T) {
		d := transferTestDomain(t, now)
		d.Status = DomainStatus{ClientUpdateProhibited: true, ServerUpdateProhibited: true}
		_, err := d.RequestTransfer("gaining", transferTestCode, 1, phase, now)
		require.NoError(t, err)
	})
	t.Run("in the transfer lock period: 2106", func(t *testing.T) {
		d := transferTestDomain(t, now)
		d.RGPStatus.TransferLockPeriodEnd = now.Add(time.Hour)
		refused(t, d, "gaining", transferTestCode, 1, ErrTransferNotEligible)
	})
	t.Run("a period taking the expiry beyond the maximum horizon: 2306", func(t *testing.T) {
		refused(t, transferTestDomain(t, now), "gaining", transferTestCode, 10, ErrTransferExceedsMaxHorizon)
		_, err := transferTestDomain(t, now).RequestTransfer("gaining", transferTestCode, 9, phase, now)
		assert.NoError(t, err, "nine years on an expiry two months out stays within ten")
	})
}

func TestDomainTransfer_Resolve(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	pending := func() *DomainTransfer {
		tr, err := transferTestDomain(t, now).RequestTransfer("gaining", transferTestCode, 1, transferTestPhase(), now)
		require.NoError(t, err)
		return tr
	}
	later := now.Add(time.Hour)

	tr := pending()
	require.NoError(t, tr.Resolve(TransferStatusClientApproved, "losing", later))
	assert.Equal(t, ClIDType("losing"), tr.ActingClID)
	assert.Equal(t, later, tr.ActionDate, "acDate: when it was answered")
	assert.False(t, tr.ExpiryDate.IsZero(), "an approval changes the expiry")
	assert.ErrorIs(t, tr.Resolve(TransferStatusClientRejected, "losing", later), ErrTransferNotPending, "answered once")

	tr = pending()
	require.NoError(t, tr.Resolve(TransferStatusClientCancelled, "gaining", later))
	assert.Equal(t, ClIDType("gaining"), tr.ActingClID, "acID: the registrar that acted")
	assert.True(t, tr.ExpiryDate.IsZero(), "no exDate: a cancellation changes no expiry")

	tr = pending()
	require.NoError(t, tr.Resolve(TransferStatusServerApproved, "", later))
	assert.Equal(t, ClIDType("losing"), tr.ActingClID, "the registry acted: acID stays the losing registrar")

	assert.ErrorIs(t, pending().Resolve(TransferStatusPending, "losing", later), ErrInvalidTransferStatus)
	assert.ErrorIs(t, pending().Resolve(TransferStatusClientApproved, "", later), ErrInvalidTransferStatus)
}

func TestDomain_CompleteTransfer(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	phase := transferTestPhase()
	d := transferTestDomain(t, now)
	d.Status.ClientUpdateProhibited = true
	d.RGPStatus.AddPeriodEnd = now.Add(time.Hour)
	d.RGPStatus.RenewPeriodEnd = now.Add(time.Hour)
	d.RGPStatus.AutoRenewPeriodEnd = now.Add(time.Hour)
	d.GrandFathering = DomainGrandFathering{GFAmount: 500, GFCurrency: "USD", GFExpiryCondition: GFConditionTransfer}
	d.RegistrantID = "losing-contact"
	expiry := d.ExpiryDate
	tr, err := d.RequestTransfer("gaining", transferTestCode, 2, phase, now)
	require.NoError(t, err)
	at := now.Add(time.Hour)
	require.NoError(t, tr.Resolve(TransferStatusClientApproved, "losing", at))
	require.NoError(t, d.CompleteTransfer(tr, phase, at))

	assert.Equal(t, ClIDType("gaining"), d.ClID)
	assert.Equal(t, expiry.AddDate(2, 0, 0), d.ExpiryDate, "extended by the period")
	assert.Equal(t, d.ExpiryDate, tr.ExpiryDate)
	assert.Equal(t, at, d.TransferredAt)
	assert.False(t, d.AuthInfo.IsSet(), "the code is used once (RFC 9154)")
	assert.False(t, d.Status.PendingTransfer)
	assert.True(t, d.Status.ClientUpdateProhibited, "the client statuses stay")
	assert.Equal(t, []string{RGPStatusTransferPeriod}, d.RGPStatuses(at), "only the transfer grace period: the losing registrar's grace periods end")
	assert.Equal(t, at.AddDate(0, 0, TransferGP), d.RGPStatus.TransferPeriodEnd)
	assert.Equal(t, at.AddDate(0, 0, TransferLockPeriod), d.RGPStatus.TransferLockPeriodEnd)
	assert.False(t, d.IsGrandFathered(), "grandfathering until transfer ends")
	assert.Equal(t, ClIDType("losing-contact"), d.RegistrantID, "contacts stay as they are")

	t.Run("a zero lock leaves the domain transferable at once", func(t *testing.T) {
		p := transferTestPhase()
		zero := 0
		p.Policy.TransferLockPeriod = &zero
		d := transferTestDomain(t, now)
		tr, err := d.RequestTransfer("gaining", transferTestCode, 1, p, now)
		require.NoError(t, err)
		require.NoError(t, tr.Resolve(TransferStatusServerApproved, "", now))
		require.NoError(t, d.CompleteTransfer(tr, p, now))
		d.AuthInfo, _ = HashAuthInfo(transferTestCode)
		_, err = d.RequestTransfer("losing", transferTestCode, 1, p, now.Add(time.Second))
		assert.NoError(t, err)
	})
	t.Run("only an approval completes a transfer", func(t *testing.T) {
		d := transferTestDomain(t, now)
		tr, err := d.RequestTransfer("gaining", transferTestCode, 1, phase, now)
		require.NoError(t, err)
		assert.ErrorIs(t, d.CompleteTransfer(tr, phase, now), ErrInvalidTransferStatus)
	})
}

func TestDomain_EndTransfer(t *testing.T) {
	now := time.Now().UTC()
	d := transferTestDomain(t, now)
	before := d.DeepCopy()
	_, err := d.RequestTransfer("gaining", transferTestCode, 1, transferTestPhase(), now)
	require.NoError(t, err)
	d.EndTransfer()
	assert.Equal(t, before, d, "a rejection or cancellation leaves the domain as it was, code included")
}

func TestNewTransferPollMessages(t *testing.T) {
	now := time.Now().UTC()
	tr, err := transferTestDomain(t, now).RequestTransfer("gaining", transferTestCode, 1, transferTestPhase(), now)
	require.NoError(t, err)

	msgs := NewTransferPollMessages(tr, "gaining", now)
	require.Len(t, msgs, 1, "the requester is answered; the losing registrar is told")
	assert.Equal(t, ClIDType("losing"), msgs[0].ClID)
	assert.Equal(t, PollMsgTransferRequested, msgs[0].Message)

	require.NoError(t, tr.Resolve(TransferStatusClientApproved, "losing", now))
	msgs = NewTransferPollMessages(tr, "losing", now)
	require.Len(t, msgs, 1)
	assert.Equal(t, ClIDType("gaining"), msgs[0].ClID)
	assert.Contains(t, msgs[0].Message, "replace them with your own", "the gaining registrar is told to replace the contacts")

	msgs = NewTransferPollMessages(tr, "", now)
	require.Len(t, msgs, 2, "when the registry acts, both registrars are told")

	snapshot := msgs[0].Transfer
	tr.Status = TransferStatusClientRejected
	assert.Equal(t, TransferStatusClientApproved, snapshot.Status, "a message keeps the transfer as it stood")
}
