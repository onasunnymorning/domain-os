package entities

type RDEEppParameters struct {
	Version      string       `xml:"version"`
	Lang         string       `xml:"lang"`
	ObjUri       []string     `xml:"objURI"`
	SvcExtension RDEExtension `xml:"svcExtension"`
	// TODO: implement DCP
	// DCP          []string       `xml:"dcp"`
}

type RDEExtension struct {
	ExtURI []string `xml:"extURI"`
}
