package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainRGPStatus_IsNil(t *testing.T) {
	rgp := DomainRGPStatus{}
	require.True(t, rgp.IsNil())

	rgp = DomainRGPStatus{
		AddPeriodEnd: time.Now().UTC(),
	}
	require.False(t, rgp.IsNil())
}
func TestDomainRGPStatuses(t *testing.T) {
	now := time.Now()
	later, earlier := now.Add(time.Hour), now.Add(-time.Hour)
	cases := []struct {
		name string
		dom  Domain
		want []string
	}{
		{"none", Domain{}, nil},
		{"just registered", Domain{RGPStatus: DomainRGPStatus{AddPeriodEnd: later}}, []string{RGPStatusAddPeriod}},
		{"add period over", Domain{RGPStatus: DomainRGPStatus{AddPeriodEnd: earlier}}, nil},
		{"renewed and auto-renewed", Domain{RGPStatus: DomainRGPStatus{RenewPeriodEnd: later, AutoRenewPeriodEnd: later}}, []string{RGPStatusRenewPeriod, RGPStatusAutoRenewPeriod}},
		{"deleted, restorable", Domain{Status: DomainStatus{PendingDelete: true}, RGPStatus: DomainRGPStatus{RedemptionPeriodEnd: later, AddPeriodEnd: later}}, []string{RGPStatusRedemptionPeriod}},
		{"restore requested", restored(t, later), []string{RGPStatusPendingRestore}},
		{"past redemption", Domain{Status: DomainStatus{PendingDelete: true}, RGPStatus: DomainRGPStatus{RedemptionPeriodEnd: earlier}}, []string{RGPStatusPendingDelete}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.dom.RGPStatuses(now))
		})
	}
}

// restored is a domain whose restore was requested (RequestRestore): still
// pendingDelete, and pendingRestore, inside its redemption period.
func restored(t *testing.T, redemptionEnd time.Time) Domain {
	t.Helper()
	d := Domain{Status: DomainStatus{PendingDelete: true}, RGPStatus: DomainRGPStatus{RedemptionPeriodEnd: redemptionEnd}}
	require.NoError(t, d.RequestRestore(&Phase{Policy: NewPhasePolicy()}, time.Now()))
	require.True(t, d.Status.PendingRestore)
	require.True(t, d.Status.PendingDelete)
	return d
}
