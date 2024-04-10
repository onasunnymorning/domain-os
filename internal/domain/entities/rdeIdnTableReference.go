package entities

import "encoding/xml"

type RDEIdnTableReference struct {
	XMLName   xml.Name `xml:"idnTableRef"`
	ID        string   `xml:"id,attr"`
	Url       string   `xml:"url"`
	UrlPolicy string   `xml:"urlPolicy"`
}
