package entities

import (
	"encoding/xml"
	"time"
)

// RDEContact  is a struct that facilitates the parsing of the Contact elements in the RDE XML
type RDEContact struct {
	XMLName    xml.Name               `xml:"contact"`
	ID         string                 `xml:"id"`
	RoID       string                 `xml:"roid"`
	Status     []RDEContactStatus     `xml:"status"`
	PostalInfo []RDEContactPostalInfo `xml:"postalInfo"`
	Voice      string                 `xml:"voice"`
	Fax        string                 `xml:"fax"`
	Email      string                 `xml:"email"`
	ClID       string                 `xml:"clID"`
	CrRr       string                 `xml:"crRr"`
	CrDate     string                 `xml:"crDate"`
	UpRr       string                 `xml:"upRr"`
	UpDate     string                 `xml:"upDate"`
	Disclose   RDEDisclose            `xml:"disclose"`
}

// ToEntity converts the RDEContact to an Contact entity
func (c *RDEContact) ToEntity() (*Contact, error) {
	postalInfos := [2]*ContactPostalInfo{}
	for i, postal := range c.PostalInfo {
		postalInfo, err := postal.ToEntity()
		if err != nil {
			return nil, err
		}
		postalInfos[i] = postalInfo
	}
	disclose, err := c.Disclose.ToEntity()
	if err != nil {
		return nil, err // Untestable until implementation of Disclose.ToEntity()
	}
	// Since the Escrow specification (RFC 9022) does not specify the authInfo field, we will generate a random one to import the data
	aInfo, err := NewAuthInfoType("escr0W1mP*rt")
	if err != nil {
		return nil, err // Untestable, just catching the error incase our AuthInfoType is validation changes
	}
	// Create a new contact object
	contact, err := NewContact(c.ID, c.RoID, c.Email, aInfo.String(), c.ClID)
	if err != nil {
		return nil, err
	}
	// Add the postal info and disclose to the contact
	contact.PostalInfo = postalInfos
	contact.Disclose = *disclose

	// Set the optional fields
	if c.Voice != "" {
		v, err := NewE164Type(c.Voice)
		if err != nil {
			return nil, err
		}
		contact.Voice = *v
	}
	if c.Fax != "" {
		f, err := NewE164Type(c.Fax)
		if err != nil {
			return nil, err
		}
		contact.Fax = *f
	}
	if c.CrDate != "" {
		date, err := time.Parse(time.RFC3339, c.CrDate)
		if err != nil {
			return nil, err
		}
		contact.CreatedAt = date
	}
	if c.UpDate != "" {
		date, err := time.Parse(time.RFC3339, c.UpDate)
		if err != nil {
			return nil, err
		}
		contact.UpdatedAt = date
	}

	if c.CrRr != "" {
		crrr, err := NewClIDType(c.CrRr)
		if err != nil {
			return nil, err
		}
		contact.CrRr = crrr
	}
	if c.UpRr != "" {
		uprr, err := NewClIDType(c.UpRr)
		if err != nil {
			return nil, err
		}
		contact.UpRr = uprr
	}

	// Set the statuses
	for _, status := range c.Status {
		err := contact.SetStatus(ContactStatusType(status.S))
		if err != nil {
			return nil, err
		}
	}

	// Validate the contact and return it
	if _, err := contact.IsValid(); err != nil {
		return nil, err
	}
	return contact, nil

}

// RDEContactPostalInfo is a struct that facilitates the parsing of the postalInfo element in the RDE XML
type RDEContactPostalInfo struct {
	XMLName xml.Name `xml:"postalInfo"`
	Type    string   `xml:"type,attr"`
	Name    string   `xml:"name"`
	Org     string   `xml:"org"`
	Address RDEAddress
}

// ToEntity converts the RDEContactPostalInfo to an ContactPostalInfo entity
func (p *RDEContactPostalInfo) ToEntity() (*ContactPostalInfo, error) {
	addr, err := p.Address.ToEntity()
	if err != nil {
		return nil, err
	}
	cpi, err := NewContactPostalInfo(p.Type, p.Name, addr)
	if err != nil {
		return nil, err
	}
	if p.Org != "" {
		org, err := NewOptPostalLineType(p.Org)
		if err != nil {
			return nil, err
		}
		cpi.Org = *org
	}
	return cpi, nil
}

type RDEContactStatus struct {
	S string `xml:"s,attr"`
}

// RDEDisclose is a struct that facilitates the parsing of the disclose element in the RDE XML
type RDEDisclose struct {
	Flag  bool                 `xml:"flag,attr"`
	Name  []RDEContactWithType `xml:"name"`
	Org   []RDEContactWithType `xml:"org"`
	Addr  []RDEContactWithType `xml:"addr"`
	Voice []string             `xml:"voice"`
	Fax   []string             `xml:"fax"`
	Email []string             `xml:"email"`
}

// ToEntity converts the RDEDisclose to an Disclose entity
func (d *RDEDisclose) ToEntity() (*ContactDisclose, error) {
	// our main data policy is not to disclose
	cd := NewDiscloseStruct(false)
	// Since our default is not to disclose, and some elements need to be set to false, we might skip this as everything is already set to false
	if d.Flag {
		// If the xml element is present, set the Disclose Property in question to equal the d.Flag (in this case true since we already handled the false case)
		// TODO: implement this
	}
	return cd, nil
}

type RDEContactWithType struct {
	Type string `xml:"type,attr"`
}
