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
	SecDNS       []RDESecDNS          `xml:"secDNS"`
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

type RDESecDNS struct {
	KeyTag     int    `xml:"keyTag"`
	Alg        int    `xml:"alg"`
	DigestType int    `xml:"digestType"`
	Digest     string `xml:"digest"`
}
