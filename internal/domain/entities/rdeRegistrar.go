package entities

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrInvalidPostalInfoCount = errors.New("invalid postal info count")
)

// RDEWhoisInfo facilitates the parsing of the whoisInfo element in the RDE XML
type RDEWhoisInfo struct {
	XMLName xml.Name `xml:"whoisInfo" json:"-"`
	Name    string   `xml:"name"`
	URL     string   `xml:"url"`
}

// ToEntity converts the RDEWhoisInfo to a WhoisInfo entity. Relies on the constructor to create a new WhoisInfo object and validate it.
func (w *RDEWhoisInfo) ToEntity() (*WhoisInfo, error) {
	wi, err := NewWhoisInfo(w.Name, w.URL)
	if err != nil {
		return nil, err
	}
	return wi, nil
}

// RDERegistrarPostalInfo facilitates the parsing of the postalInfo element in the RDE XML
type RDERegistrarPostalInfo struct {
	XMLName xml.Name `xml:"postalInfo" json:"-"`
	Type    string   `xml:"type,attr"`
	Address RDEAddress
}

// ToEntity converts the RDERegistrarPostalInfo to an Address entity. Relies on the constructor to create a new Address object and validate it.
func (r *RDERegistrarPostalInfo) ToEntity() (*RegistrarPostalInfo, error) {
	a, err := r.Address.ToEntity()
	if err != nil {
		return nil, err
	}
	// TODO: FIXME: remove this - if we get a dirty deposit that has non-ASCII characters in an INT postalinfo, we override the int postalinfo to loc
	isASCII, _ := a.IsASCII()
	if !isASCII {
		r.Type = "loc"
	}
	rpi, err := NewRegistrarPostalInfo(r.Type, a)
	if err != nil {
		return nil, err
	}
	return rpi, nil
}

// RDEAddress facilitates the parsing of the addr element in the RDE XML
type RDEAddress struct {
	XMLName       xml.Name `xml:"addr" json:"-"`
	Street        []string `xml:"street"`
	City          string   `xml:"city"`
	StateProvince string   `xml:"sp"`
	PostalCode    string   `xml:"pc"`
	CountryCode   string   `xml:"cc"`
}

// ToEntity converts the RDEAddress to an Address entity. Relies on the constructor to create a new Address object and validate it.
func (a *RDEAddress) ToEntity() (*Address, error) {
	addr, err := NewAddress(a.City, a.CountryCode)
	if err != nil {
		return nil, err
	}
	if a.StateProvince != "" {
		sp, err := NewOptPostalLineType(a.StateProvince)
		if err != nil {
			return nil, err
		}
		addr.StateProvince = *sp
	}
	if a.PostalCode != "" {
		pc, err := NewPCType(a.PostalCode)
		if err != nil {
			return nil, err
		}
		addr.PostalCode = *pc
	}
	if len(a.Street) == 0 {
		return addr, nil
	}
	if len(a.Street) > 3 {
		return nil, ErrInvalidStreetCount
	}
	for i, street := range a.Street {
		if street != "" {
			sl, err := NewOptPostalLineType(street)
			if err != nil {
				return nil, err
			}
			switch i {
			case 0:
				addr.Street1 = *sl
			case 1:
				addr.Street2 = *sl
			case 2:
				addr.Street3 = *sl
			}
		}
	}
	return addr, nil
}

// RDERegistrar facilitates the parsing of the registrar element in the RDE XML
type RDERegistrar struct {
	XMLName    xml.Name                 `xml:"registrar" json:"-"`
	ID         string                   `xml:"id"`
	Name       string                   `xml:"name"`
	GurID      int                      `xml:"gurid"`
	Status     RDERegistrarStatus       `xml:"status"`
	PostalInfo []RDERegistrarPostalInfo `xml:"postalInfo"`
	Voice      string                   `xml:"voice"`
	Fax        string                   `xml:"fax"`
	Email      string                   `xml:"email"`
	URL        string                   `xml:"url"`
	WhoisInfo  RDEWhoisInfo
	CrDate     string `xml:"crDate"`
	UpDate     string `xml:"upDate"`
}

// ToEntity converts the RDERegistrar to a Registrar entity. Relies on the constructor to create a new Registrar object and validate it.
func (r *RDERegistrar) ToEntity() (*Registrar, error) {
	if len(r.PostalInfo) > 2 {
		return nil, ErrInvalidPostalInfoCount

	}
	var rarPi [2]*RegistrarPostalInfo
	for i, pi := range r.PostalInfo {
		pi, err := pi.ToEntity()
		if err != nil {
			return nil, err
		}
		rarPi[i] = pi

	}
	// TODO: FIXME: remove this - Sometimes we get multiple email addresses in the email field separated by comma
	if len(strings.Split(r.Email, ",")) > 1 {
		r.Email = strings.Split(r.Email, ",")[0]
	}
	rar, err := NewRegistrar(r.ID, r.Name, r.Email, r.GurID, rarPi)
	if err != nil {
		return nil, err
	}

	if r.Voice != "" {
		rar.Voice = E164Type(r.Voice)
	}
	if r.Fax != "" {
		rar.Fax = E164Type(r.Fax)
	}
	if r.URL != "" {
		rar.URL = URL(r.URL)
	}
	if r.CrDate != "" {
		date, err := time.Parse(time.RFC3339, r.CrDate)
		if err != nil {
			return nil, err
		}
		rar.CreatedAt = date
	}
	if r.UpDate != "" {
		date, err := time.Parse(time.RFC3339, r.UpDate)
		if err != nil {
			return nil, err
		}
		rar.UpdatedAt = date
	}
	wi, err := r.WhoisInfo.ToEntity()
	if err != nil {
		return nil, err
	}
	rar.WhoisInfo = *wi

	// Often there is no status so we set it to ok if it is empty
	if r.Status.S == "" {
		rar.Status = RegistrarStatusOK
	} else {
		rar.Status = RegistrarStatus(r.Status.S)
	}

	err = rar.Validate()
	if err != nil {
		return nil, err
	}

	return rar, nil
}

type RDERegistrarStatus struct {
	S string `xml:"s,attr"`
}
