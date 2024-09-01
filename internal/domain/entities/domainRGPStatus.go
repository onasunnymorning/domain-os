package entities

import "time"

// DomainRGPStatus value object
type DomainRGPStatus struct {
	AddPeriodEnd time.Time `json:"addPeriodEnd"`
	// Is it nescessary to have a different field for Renew/AutoRenew
	// Isn't it enough to have one and the period differs based on the TLDPhasePolicy RenewalGP and AutoRenewalGP?
	RenewPeriodEnd        time.Time `json:"renewPeriodEnd"`
	AutoRenewPeriodEnd    time.Time `json:"autoRenewPeriodEnd"`
	TransferLockPeriodEnd time.Time `json:"transferLockPeriodEnd"`
	RedemptionPeriodEnd   time.Time `json:"redemptionPeriodEnd"`
	PurgeDate             time.Time `json:"purgeDate" gorm:"index"` // Previously PendingDeleteGPEnd (thought that was a bit too long)
}

// IsNil checks if the DomainRGPStatus object is nil
func (d *DomainRGPStatus) IsNil() bool {
	return d.AddPeriodEnd.IsZero() && d.RenewPeriodEnd.IsZero() && d.AutoRenewPeriodEnd.IsZero() && d.TransferLockPeriodEnd.IsZero() && d.RedemptionPeriodEnd.IsZero() && d.PurgeDate.IsZero()
}
