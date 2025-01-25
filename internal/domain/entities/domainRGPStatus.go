package entities

import "time"

// DomainRGPStatus value object
type DomainRGPStatus struct {
	// AddPeriodEnd is the end of the period in which the domain may be deleted by the registrars after a registration.
	AddPeriodEnd time.Time `json:"addPeriodEnd"`
	// RenewPeriodEnd is the end of the period in which the domain may be deleted by the registrars after a renew.
	RenewPeriodEnd time.Time `json:"renewPeriodEnd"`
	// AutoRenewPeriodEnd is the end of the period in which the domain may be deleted by the registrar after an auto-renew without causing a charge. This should be handled by the billing system.
	AutoRenewPeriodEnd time.Time `json:"autoRenewPeriodEnd"`
	// TransferLockPeriodEnd is the end of the period in which the domain cannot be transferred. This applies after a registration and a transfer.
	TransferLockPeriodEnd time.Time `json:"transferLockPeriodEnd"`
	// RedemptionPeriodEnd is the date after which the domain will no longer be restorable it will remain in the repository until the purge date.
	RedemptionPeriodEnd time.Time `json:"redemptionPeriodEnd"`
	// PurgeDate is date after which the domain should be purged and become available for registration again.
	PurgeDate time.Time `json:"purgeDate" gorm:"index"`
}

// IsNil checks if the DomainRGPStatus object is nil
func (d *DomainRGPStatus) IsNil() bool {
	return d.AddPeriodEnd.IsZero() && d.RenewPeriodEnd.IsZero() && d.AutoRenewPeriodEnd.IsZero() && d.TransferLockPeriodEnd.IsZero() && d.RedemptionPeriodEnd.IsZero() && d.PurgeDate.IsZero()
}
