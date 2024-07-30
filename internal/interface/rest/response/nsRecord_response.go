package response

import "time"

// NSRecord struct holds the response for and NS record for a domain
type NSRecord struct {
	Domain string `json:"domain"`
	NS     string `json:"ns"`
}

// NSRecordResponse struct holds the response for an NS record query
type NSRecordResponse struct {
	TLD       string     `json:"tld"`
	Timestamp time.Time  `json:"timestamp"`
	NSRecords []NSRecord `json:"nsRecords"`
}
