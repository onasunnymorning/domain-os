package entities

import "time"

// DomainRGPStatus value object
type DomainRGPStatus struct {
	// AddPeriodEnd is the end of the period in which the domain may be deleted by the registrars after a registration with no financial penalty.
	AddPeriodEnd time.Time `json:"addPeriodEnd"`
	// RenewPeriodEnd is the end of the period in which the domain may be deleted by the registrars after a renew.
	RenewPeriodEnd time.Time `json:"renewPeriodEnd"`
	// AutoRenewPeriodEnd is the end of the period in which the domain may be deleted by the registrar after an auto-renew without causing a charge. This should be handled by the billing system.
	AutoRenewPeriodEnd time.Time `json:"autoRenewPeriodEnd"`
	// TransferPeriodEnd is the end of the transfer grace period
	// (RFC 3915 transferPeriod): the days after a transfer in which the
	// gaining registrar may delete the domain for a refund.
	TransferPeriodEnd time.Time `json:"transferPeriodEnd"`
	// TransferLockPeriodEnd is the end of the period in which the domain cannot be transferred. This applies after a registration and a transfer.
	TransferLockPeriodEnd time.Time `json:"transferLockPeriodEnd"`
	// RedemptionPeriodEnd is the date after which the domain will no longer be restorable it will remain in the repository until the purge date.
	RedemptionPeriodEnd time.Time `json:"redemptionPeriodEnd"`
	// PurgeDate is date after which the domain should be purged and become available for registration again.
	PurgeDate time.Time `json:"purgeDate" gorm:"index"`
}

// IsNil checks if the DomainRGPStatus object is nil
func (d *DomainRGPStatus) IsNil() bool {
	return d.AddPeriodEnd.IsZero() && d.RenewPeriodEnd.IsZero() && d.AutoRenewPeriodEnd.IsZero() && d.TransferPeriodEnd.IsZero() && d.TransferLockPeriodEnd.IsZero() && d.RedemptionPeriodEnd.IsZero() && d.PurgeDate.IsZero()
}

// RFC 3915 grace period statuses.
const (
	RGPStatusAddPeriod        = "addPeriod"
	RGPStatusAutoRenewPeriod  = "autoRenewPeriod"
	RGPStatusRenewPeriod      = "renewPeriod"
	RGPStatusTransferPeriod   = "transferPeriod"
	RGPStatusRedemptionPeriod = "redemptionPeriod"
	RGPStatusPendingRestore   = "pendingRestore"
	RGPStatusPendingDelete    = "pendingDelete"
)

// RGPStatuses returns the RFC 3915 grace period statuses of d at now. A
// domain whose restore has been requested is pendingRestore: Restore clears
// pendingDelete as it sets pendingRestore, and the two never hold together.
// A domain pending delete is in redemptionPeriod while still restorable,
// then pendingDelete until it is purged. Otherwise it is in each of the
// add, renew, auto-renew and transfer grace periods that has not ended.
func (d *Domain) RGPStatuses(now time.Time) []string {
	r := d.RGPStatus
	if d.Status.PendingRestore {
		return []string{RGPStatusPendingRestore}
	}
	if d.Status.PendingDelete {
		switch {
		case now.Before(r.RedemptionPeriodEnd):
			return []string{RGPStatusRedemptionPeriod}
		default:
			return []string{RGPStatusPendingDelete}
		}
	}
	var out []string
	for _, p := range []struct {
		end    time.Time
		status string
	}{
		{r.AddPeriodEnd, RGPStatusAddPeriod},
		{r.RenewPeriodEnd, RGPStatusRenewPeriod},
		{r.AutoRenewPeriodEnd, RGPStatusAutoRenewPeriod},
		{r.TransferPeriodEnd, RGPStatusTransferPeriod},
	} {
		if now.Before(p.end) {
			out = append(out, p.status)
		}
	}
	return out
}
