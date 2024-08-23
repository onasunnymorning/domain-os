package entities

import (
	"encoding/xml"
	"time"
)

const (
	RDEReportTypeFULL = "FULL"
	RDEReportTypeDIFF = "DIFF"
	RDEReportTypeINCR = "INCR"
	XMLNS             = "urn:ietf:params:xml:ns:rdeReport-1.0"
	RYDE_ESCROW_RFC   = "RFC8909"
	RYDE_MAPPING_RFC  = "RFC9022"
)

// RDEReport represents the <rdeReport:report> object
type RDEReport struct {
	XMLName xml.Name `xml:"rdeReport:report"`
	XMLNS   string   `xml:"xmlns:rdeReport,attr"`

	ID              string    `xml:"id"`                        // must match the ID in the RDEDeposit
	Version         int       `xml:"version"`                   // must be 1
	RydeSpecEscrow  string    `xml:"rydeSpecEscrow"`            // Which RFC is being used - 8909
	RydeSpecMapping string    `xml:"rydeSpecMapping,omitempty"` // Optional field Which RFC do we use for object mapping - 9022
	Resend          int       `xml:"resend"`                    // must match the resend field in the RDEDeposit
	CrDate          time.Time `xml:"crDate"`                    // Creation date of the deposit
	Kind            string    `xml:"kind"`                      // FULL, DIFF, INCR
	Watermark       time.Time `xml:"watermark"`                 // must match the watermark field in the RDEDeposit
	RdeHeader       RDEHeader `xml:"rdeHeader:header"`          // must match the header in the RDEDeposit
}

// NewRDEReport creates a new RDEReport object
func NewRDEReport(id string, resend int, crDate, watermark time.Time, header RDEHeader) *RDEReport {
	return &RDEReport{
		XMLNS:           XMLNS,
		ID:              id,
		Version:         1,
		RydeSpecEscrow:  RYDE_ESCROW_RFC,
		RydeSpecMapping: RYDE_MAPPING_RFC,
		Resend:          resend,
		CrDate:          crDate,
		Kind:            RDEReportTypeFULL,
		Watermark:       watermark,
		RdeHeader:       header,
	}

}
