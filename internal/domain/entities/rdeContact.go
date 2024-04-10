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

type RDEContactPostalInfo struct {
	XMLName xml.Name `xml:"postalInfo"`
	Type    string   `xml:"type,attr"`
	Org     string   `xml:"org"`
	Address RDEAddress
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
