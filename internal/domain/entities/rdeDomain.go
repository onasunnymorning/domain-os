package entities

import (
	"encoding/xml"
)

type RDEDomain struct {
	XMLName      xml.Name             `xml:"domain"`
	Name         DomainName           `xml:"name"` // element that contains the fully qualified name of the domain name object. For IDNs, the A-label is used
	RoID         string               `xml:"roid"` // element that contains the ROID assigned to the domain name object when it was created
	UName        string               `xml:"uName"`
	IdnTableId   string               `xml:"idnTableId"`
	OriginalName string               `xml:"originalName"`
	Status       []RDEDomainStatus    `xml:"status"`
	RgpStatus    []RDEDomainRGPStatus `xml:"rgpStatus"`
	Registrant   string               `xml:"registrant"`
	Contact      []RDEDomainContact   `xml:"contact"`
	Ns           []RDEDomainHost      `xml:"ns"`
	ClID         string               `xml:"clID"`
	CrRr         string               `xml:"crRr"`
	CrDate       string               `xml:"crDate"`
	ExDate       string               `xml:"exDate"`
	UpRr         string               `xml:"upRr"`
	UpDate       string               `xml:"upDate"`
	SecDNS       RDESecDNS            `xml:"secDNS"`
	TrnData      TrnData              `xml:"trnData"`
}

type RDEDomainStatus struct {
	S string `xml:"s,attr"`
}

type RDEDomainRGPStatus struct {
	S string `xml:"s,attr"`
}

type RDEDomainHost struct {
	HostObjs []string `xml:"hostObj"`
}

type RDEDomainContact struct {
	Type string `xml:"type,attr"`
	ID   string `xml:",chardata"`
}

type DSData struct {
	KeyTag     int    `xml:"keyTag"`
	Alg        int    `xml:"alg"`
	DigestType int    `xml:"digestType"`
	Digest     string `xml:"digest"`
}

type RDESecDNS struct {
	DSData []DSData `xml:"dsData"`
}

type TrnData struct {
	TrStatus TrStatus `xml:"trStatus"`
	ReRr     ReRr     `xml:"reRr"`
	ReDate   string   `xml:"reDate"`
	AcRr     AcRr     `xml:"acRr"`
	AcDate   string   `xml:"acDate"`
	ExDate   string   `xml:"exDate,omitempty"`
}

type TrStatus struct {
	State string `xml:",chardata"`
}

type ReRr struct {
	RegID  string `xml:",chardata"`
	Client string `xml:"client,attr,omitempty"`
}

type AcRr struct {
	RegID  string `xml:",chardata"`
	Client string `xml:"client,attr,omitempty"`
}
