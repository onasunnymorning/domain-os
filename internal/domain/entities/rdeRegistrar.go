// WhoisInfo Value Object
package entities

import (
	"encoding/xml"
)

type RDEWhoisInfo struct {
	XMLName xml.Name `xml:"whoisInfo"`
	Name    string   `xml:"name"`
	URL     string   `xml:"url"`
}

type RDERegistrarPostalInfo struct {
	XMLName xml.Name `xml:"postalInfo"`
	Type    string   `xml:"type,attr"`
	Address RDEAddress
}

type RDEAddress struct {
	XMLName       xml.Name `xml:"addr"`
	Street        []string `xml:"street"`
	City          string   `xml:"city"`
	StateProvince string   `xml:"sp"`
	PostalCode    string   `xml:"pc"`
	CountryCode   string   `xml:"cc"`
}

// Registrar Entity
type RDERegistrar struct {
	XMLName    xml.Name                 `xml:"registrar"`
	ID         string                   `xml:"id"`
	Name       string                   `xml:"name"`
	GurID      int                      `xml:"gurid"`
	Status     []RDERegistrarStatus     `xml:"status"`
	PostalInfo []RDERegistrarPostalInfo `xml:"postalInfo"`
	Voice      string                   `xml:"voice"`
	Fax        string                   `xml:"fax"`
	Email      string                   `xml:"email"`
	URL        string                   `xml:"url"`
	WhoisInfo  RDEWhoisInfo
	CrDate     string `xml:"crDate"`
	UpDate     string `xml:"upDate"`
}

type RDERegistrarStatus struct {
	S string `xml:"s,attr"`
}
