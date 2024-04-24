package entities

import "encoding/xml"

type RDEIdnTableReference struct {
	XMLName   xml.Name `xml:"idnTableRef" json:"-"`
	ID        string   `xml:"id,attr"`
	Url       string   `xml:"url"`
	UrlPolicy string   `xml:"urlPolicy"`
}
