package entities

import "time"

// DomainRGPStatus value object
type DomainRGPStatus struct {
	AddPeriodEnd           time.Time `json:"AddPeriodEnd"`
	RenewPeriodEnd         time.Time `json:"RenewPeriodEnd"`
	AutoRenewPeriodEnd     time.Time `json:"AutoRenewPeriodEnd"`
	TransferPeriodEnd      time.Time `json:"RransferPeriodEnd"`
	RedemptionPeriodEnd    time.Time `json:"RedemptionPeriodEnd"`
	PendingDeletePeriodEnd time.Time `json:"PendingDeletePeriodEnd"`
}

// IsNil checks if the DomainRGPStatus object is nil
func (d *DomainRGPStatus) IsNil() bool {
	return d.AddPeriodEnd.IsZero() && d.RenewPeriodEnd.IsZero() && d.AutoRenewPeriodEnd.IsZero() && d.TransferPeriodEnd.IsZero() && d.RedemptionPeriodEnd.IsZero() && d.PendingDeletePeriodEnd.IsZero()
}
