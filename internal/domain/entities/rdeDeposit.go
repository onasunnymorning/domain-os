package entities

import "encoding/xml"

type RDEDeposit struct {
	XMLName   xml.Name `xml:"deposit"`
	Type      string   `xml:"type,attr"`
	ID        string   `xml:"id,attr"`
	PrevID    string   `xml:"prevId,attr"`
	Resend    int      `xml:"resend,attr"`
	Watermark string   `xml:"watermark"`
	FileName  string
}
