package ianaregistrars

import "encoding/xml"

// Top level structure for IANA XML Registry
type IanaXmlRegistry struct {
	XMLName  xml.Name `xml:"registry" json:"-"`
	Title    string   `xml:"title"`
	Updated  string   `xml:"updated"`
	Registry Registry `xml:"registry"`
}

// Second level structure for IANA XML Registry
type Registry struct {
	XMLName xml.Name          `xml:"registry" json:"-"`
	Id      string            `xml:"id,attr"`
	Title   string            `xml:"title"`
	RegRule string            `xml:"registration_rule"`
	Records []RegistrarRecord `xml:"record"`
}

// Third level structure for IANA XML Registry
type RegistrarRecord struct {
	XMLName xml.Name `xml:"record" json:"-"`
	Updated string   `xml:"updated,attr"`
	Value   string   `xml:"value"`
	Name    string   `xml:"name"`
	Status  string   `xml:"status"`
	RdapURL RdapURL  `xml:"rdapurl"`
}

// RDAP URL structure for IANA XML Registry
type RdapURL struct {
	XMLName xml.Name `xml:"rdapurl" json:"rdapurl" json:"-"`
	Server  string   `xml:"server" json:"server"`
}
