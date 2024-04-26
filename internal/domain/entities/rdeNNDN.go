package entities

import "encoding/xml"

var (
	RdeNNDNCSVHeader = []string{"AName", "UName", "IDNTableID", "OriginalName", "NameState", "CrDate"}
)

type RDENNDN struct {
	XMLName      xml.Name `xml:"NNDN" json:"-"`
	AName        string   `xml:"aName"`
	UName        string   `xml:"uName"`
	IDNTableID   string   `xml:"idnTableId"`
	OriginalName string   `xml:"originalName"`
	NameState    string   `xml:"nameState"`
	CrDate       string   `xml:"crDate"`
}

// ToCSV converts the RDENNDN to a slice of strings ([]string) for CSV export. The fields are defined in RdeNNDNCSVHeader
func (n *RDENNDN) ToCSV() []string {
	return []string{n.AName, n.UName, n.IDNTableID, n.OriginalName, n.NameState, n.CrDate}
}
