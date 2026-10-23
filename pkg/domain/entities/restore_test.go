package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deletedDomain is a domain deleted a day before now: pendingDelete, in its
// 30-day redemption period, expiring in expiry.
func deletedDomain(t *testing.T, now time.Time, expiry time.Time) *Domain {
	t.Helper()
	d, err := NewDomain("12345_DOM-APEX", "restore.domaintesttld", "GoMamma", "")
	require.NoError(t, err)
	d.ExpiryDate = expiry
	d.Status = DomainStatus{PendingDelete: true, Inactive: true}
	d.RGPStatus.RedemptionPeriodEnd = now.AddDate(0, 0, 29)
	d.RGPStatus.PurgeDate = now.AddDate(0, 0, 34)
	require.NoError(t, d.Status.Validate())
	return d
}

func goodReport(now time.Time) *RestoreReport {
	return &RestoreReport{
		PreData: "before", PostData: "after",
		DelTime: now.Add(-48 * time.Hour), ResTime: now.Add(-time.Hour),
		ResReason: "Registrant error.",
		Statements: []string{
			"This registrar has not restored the Registered Name in order to assume the rights to use or sell the Registered Name for itself or for any third party.",
			"The information in this report is true to best of this registrar's knowledge.",
		},
	}
}

func TestRequestRestore(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	phase := &Phase{Policy: NewPhasePolicy()}

	t.Run("a deleted domain in redemption: pendingRestore, still pendingDelete", func(t *testing.T) {
		d := deletedDomain(t, now, now.AddDate(0, 3, 0))
		require.NoError(t, d.RequestRestore(phase, now))
		assert.True(t, d.Status.PendingRestore)
		assert.True(t, d.Status.PendingDelete, "the EPP status stays pendingDelete (RFC 3915 §4)")
		assert.Equal(t, now.AddDate(0, 0, PendingRestorePeriod), d.RGPStatus.PendingRestoreEnd)
		assert.Equal(t, []string{RGPStatusPendingRestore}, d.RGPStatuses(now))
		assert.Contains(t, d.Status.StringSlice(), DomainStatusPendingDelete)
		assert.True(t, d.RestorePending(now))
		assert.False(t, d.RestoreLapsed(now))
	})
	t.Run("the phase sets the pending restore period", func(t *testing.T) {
		d := deletedDomain(t, now, now.AddDate(0, 3, 0))
		p := &Phase{Policy: NewPhasePolicy()}
		p.Policy.PendingRestorePeriod = 3
		require.NoError(t, d.RequestRestore(p, now))
		assert.Equal(t, now.AddDate(0, 0, 3), d.RGPStatus.PendingRestoreEnd)
	})
	t.Run("refused: 2304", func(t *testing.T) {
		live := deletedDomain(t, now, now.AddDate(1, 0, 0))
		live.Status = DomainStatus{OK: true}
		past := deletedDomain(t, now, now)
		past.RGPStatus.RedemptionPeriodEnd = now.Add(-time.Minute)
		again := deletedDomain(t, now, now)
		require.NoError(t, again.RequestRestore(phase, now))
		for name, d := range map[string]*Domain{"not deleted": live, "redemption over": past, "already requested": again} {
			t.Run(name, func(t *testing.T) {
				before := *d
				assert.ErrorIs(t, d.RequestRestore(phase, now), ErrDomainRestoreNotAllowed)
				assert.Equal(t, before.Status, d.Status)
				assert.Equal(t, before.RGPStatus, d.RGPStatus)
			})
		}
	})
}

func TestCompleteRestore(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	phase := &Phase{Policy: NewPhasePolicy()}
	requested := func(t *testing.T, expiry time.Time) *Domain {
		d := deletedDomain(t, now.Add(-time.Hour), expiry)
		require.NoError(t, d.RequestRestore(phase, now.Add(-time.Hour)))
		return d
	}

	t.Run("a registrar's delete: a year added to a future expiry", func(t *testing.T) {
		expiry := now.AddDate(0, 6, 0)
		d := requested(t, expiry)
		years, err := d.CompleteRestore(goodReport(now), phase, now)
		require.NoError(t, err)
		assert.Equal(t, 1, years)
		assert.Equal(t, expiry.AddDate(1, 0, 0), d.ExpiryDate)
		assert.False(t, d.Status.PendingDelete)
		assert.False(t, d.Status.PendingRestore)
		assert.True(t, d.Status.Inactive, "no nameservers")
		assert.Empty(t, d.RGPStatuses(now), "out of its end of life, and no renew grace period")
		assert.True(t, d.RGPStatus.RedemptionPeriodEnd.IsZero())
		assert.True(t, d.RGPStatus.PurgeDate.IsZero())
		assert.True(t, d.RGPStatus.PendingRestoreEnd.IsZero())
		assert.Equal(t, 1, d.RenewedYears)
	})
	t.Run("an expired domain: renewed until its expiry is in the future", func(t *testing.T) {
		expiry := now.AddDate(-1, 0, -10) // a year and ten days ago
		d := requested(t, expiry)
		years, err := d.CompleteRestore(goodReport(now), phase, now)
		require.NoError(t, err)
		assert.Equal(t, 2, years)
		assert.Equal(t, expiry.AddDate(2, 0, 0), d.ExpiryDate)
		assert.True(t, d.ExpiryDate.After(now))
	})
	t.Run("no report when the phase requires none", func(t *testing.T) {
		d := requested(t, now.AddDate(0, 1, 0))
		years, err := d.CompleteRestore(nil, phase, now)
		require.NoError(t, err)
		assert.Equal(t, 1, years)
	})
	t.Run("no restore requested: 2304", func(t *testing.T) {
		d := deletedDomain(t, now, now)
		_, err := d.CompleteRestore(goodReport(now), phase, now)
		assert.ErrorIs(t, err, ErrRestoreNotPending)
		assert.True(t, d.Status.PendingDelete)
	})
	t.Run("after the deadline: 2304", func(t *testing.T) {
		d := requested(t, now)
		late := d.RGPStatus.PendingRestoreEnd
		_, err := d.CompleteRestore(goodReport(late), phase, late)
		assert.ErrorIs(t, err, ErrRestoreNotPending)
		assert.True(t, d.Status.PendingRestore)
	})
	t.Run("a report missing a part: 2003", func(t *testing.T) {
		for name, edit := range map[string]func(*RestoreReport){
			"preData":       func(r *RestoreReport) { r.PreData = " \n" },
			"postData":      func(r *RestoreReport) { r.PostData = "" },
			"resReason":     func(r *RestoreReport) { r.ResReason = "" },
			"one statement": func(r *RestoreReport) { r.Statements = r.Statements[:1] },
			"an empty one":  func(r *RestoreReport) { r.Statements[1] = "  " },
		} {
			t.Run(name, func(t *testing.T) {
				d := requested(t, now)
				r := goodReport(now)
				edit(r)
				_, err := d.CompleteRestore(r, phase, now)
				assert.ErrorIs(t, err, ErrRestoreReportIncomplete)
				assert.True(t, d.Status.PendingRestore, "nothing changed")
			})
		}
	})
	t.Run("a report with impossible times: 2004", func(t *testing.T) {
		for name, edit := range map[string]func(*RestoreReport){
			"deleted after the request": func(r *RestoreReport) { r.DelTime = r.ResTime.Add(time.Second) },
			"deleted at the request":    func(r *RestoreReport) { r.DelTime = r.ResTime },
			"requested in the future":   func(r *RestoreReport) { r.ResTime = now.Add(time.Minute) },
		} {
			t.Run(name, func(t *testing.T) {
				d := requested(t, now)
				r := goodReport(now)
				edit(r)
				_, err := d.CompleteRestore(r, phase, now)
				assert.ErrorIs(t, err, ErrRestoreReportTimes)
			})
		}
	})
}

func TestRevertRestore(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	phase := &Phase{Policy: NewPhasePolicy()}
	d := deletedDomain(t, now, now)
	redemptionEnd, purge := d.RGPStatus.RedemptionPeriodEnd, d.RGPStatus.PurgeDate
	require.NoError(t, d.RequestRestore(phase, now))
	deadline := d.RGPStatus.PendingRestoreEnd

	assert.ErrorIs(t, d.RevertRestore(deadline.Add(-time.Second)), ErrRestoreNotPending, "not lapsed yet")
	require.NoError(t, d.RevertRestore(deadline))
	assert.False(t, d.Status.PendingRestore)
	assert.True(t, d.Status.PendingDelete)
	assert.True(t, d.RGPStatus.PendingRestoreEnd.IsZero())
	assert.Equal(t, redemptionEnd, d.RGPStatus.RedemptionPeriodEnd, "the original redemption period")
	assert.Equal(t, purge, d.RGPStatus.PurgeDate, "and purge date")
	assert.Equal(t, []string{RGPStatusRedemptionPeriod}, d.RGPStatuses(deadline))
	assert.ErrorIs(t, d.RevertRestore(deadline), ErrRestoreNotPending, "nothing left to revert")
	require.NoError(t, d.RequestRestore(phase, deadline), "the registrar may ask again within redemption")
}

func TestRestorePolicy(t *testing.T) {
	p := NewPhasePolicy()
	assert.Equal(t, PendingRestorePeriod, p.PendingRestoreDays())
	assert.True(t, p.RequiresRestoreReport())

	old := PhasePolicy{} // a phase stored before the settings existed
	assert.Equal(t, PendingRestorePeriod, old.PendingRestoreDays())
	assert.True(t, old.RequiresRestoreReport())

	no := false
	p.UpdatePolicy(&PhasePolicy{PendingRestorePeriod: 3, RestoreReportRequired: &no})
	assert.Equal(t, 3, p.PendingRestoreDays())
	assert.False(t, p.RequiresRestoreReport())
	p.UpdatePolicy(&PhasePolicy{})
	assert.False(t, p.RequiresRestoreReport(), "left out, the setting is kept")
}

func TestPendingRestoreNeedsPendingDelete(t *testing.T) {
	assert.ErrorIs(t, (&DomainStatus{PendingRestore: true}).Validate(), ErrInvalidDomainStatusCombination)
	assert.NoError(t, (&DomainStatus{PendingRestore: true, PendingDelete: true}).Validate())
	assert.ErrorIs(t, (&DomainStatus{PendingRestore: true, PendingDelete: true, PendingTransfer: true}).Validate(), ErrInvalidDomainStatusCombination)
}
