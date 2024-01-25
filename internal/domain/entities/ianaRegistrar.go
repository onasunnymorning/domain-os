package entities

import "time"

const (
	IANARegistrarStatusAccredited IANARegistrarStatus = "Accredited"
	IANARegistrarStatusReserved   IANARegistrarStatus = "Reserved"
	IANARegistrarStatusTerminated IANARegistrarStatus = "Terminated"
)

// IANARegistrarStatus is a string representing the status of an IANA Registrar
type IANARegistrarStatus string

// IANARegistrar is a struct representing an IANA Registrar
type IANARegistrar struct {
	GurID     int
	Name      string
	Status    IANARegistrarStatus
	RdapURL   string
	CreatedAt time.Time
	UpdateAt  time.Time
}
