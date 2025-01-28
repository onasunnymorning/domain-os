package entities

import (
	"fmt"
	"strings"
	"time"
)

const (
	IANARegistrarStatusAccredited IANARegistrarStatus = "Accredited"
	IANARegistrarStatusReserved   IANARegistrarStatus = "Reserved"
	IANARegistrarStatusTerminated IANARegistrarStatus = "Terminated"
)

// IANARegistrarStatus is a string representing the status of an IANA Registrar
type IANARegistrarStatus string

func (s IANARegistrarStatus) String() string {
	return string(s)
}

// IANARegistrar is a struct representing an IANA Registrar
type IANARegistrar struct {
	GurID     int
	Name      string
	Status    IANARegistrarStatus
	RdapURL   string
	CreatedAt time.Time
}

// CreateClID uses the GurID and Name of an IANARegistrar to create a valid ClID
func (r IANARegistrar) CreateClID() (ClIDType, error) {
	// split the r.Name string by comma ',' and return the frist part
	slug := strings.Split(r.Name, ",")[0]
	// lowercase the string
	slug = strings.ToLower(slug)
	// Remove all Non-ASCII characters
	slug = RemoveNonASCII(slug)
	// replace all spaces ' ' with dashes '-'
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove all Non-AlphaNumeric characters
	slug = RemoveNonAlphaNumeric(slug)
	// remove all dots '.'
	slug = strings.ReplaceAll(slug, ".", "")
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// prepend the IANAID to the slug
	slug = fmt.Sprintf("%d-%s", r.GurID, slug)
	// if the string is longer than 16 characters, truncate it
	if len(slug) > 16 {
		slug = slug[:16]
	}
	// if the string starts or ends with a dash, remove it
	slug = strings.Trim(slug, "-")
	// validate as a ClIDType
	return NewClIDType(slug)
}
