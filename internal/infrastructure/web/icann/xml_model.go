package icann

import (
	"encoding/xml"
)

// Top level structure for IANA XML Registry
type IcannXmlSpec5Registry struct {
	XMLName    xml.Name   `xml:"registry" json:"-"`
	Id         string     `xml:"id,attr"`
	Title      string     `xml:"title"`
	Created    string     `xml:"created"`
	Updated    string     `xml:"updated"`
	Note       string     `xml:"note"`
	Registries []Registry `xml:"registry"`
}

// Second level structure for IANA XML Registry
type Registry struct {
	XMLName xml.Name `xml:"registry" json:"-"`
	Id      string   `xml:"id,attr"`
	Title   string   `xml:"title"`
	Xref    struct {
		Type string `xml:"type,attr"`
		Data string `xml:"data,attr"`
	} `xml:"xref"`
	Description string   `xml:"description"`
	Records     []Record `xml:"record"`
}

// Record-level structure for IANA XML Registry
type Record struct {
	XMLName xml.Name `xml:"record" json:"-"`
	Name    string   `xml:"name"`
	Label1  string   `xml:"label1"`
	Label2  string   `xml:"label2"`
}
