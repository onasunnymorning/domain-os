package entities

import (
	"encoding/xml"
)

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

type RDEDisclose struct {
	Flag  bool                 `xml:"flag,attr"`
	Name  []RDEContactWithType `xml:"name"`
	Org   []RDEContactWithType `xml:"org"`
	Addr  []RDEContactWithType `xml:"addr"`
	Voice []string             `xml:"voice"`
	Fax   []string             `xml:"fax"`
	Email []string             `xml:"email"`
}

type RDEContactWithType struct {
	Type string `xml:"type,attr"`
}
