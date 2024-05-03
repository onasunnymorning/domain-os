package mosapi

// Working CURL version
// curl --cert icann-tls.crt.pem --key icann-tls.private.key https://mosapi-ote.icann.org/ry/example56/v2/monitoring/state

// This struct is used to represent the response from the state endpoint of the MOSAPI
// Ref:
//
//	{
//		"tld": "example",
//		"lastUpdateApiDatabase": 1496923082,
//		"status": "Down",
//		"testedServices": {
//		"DNS": {
//		"status": "Down",
//		"emergencyThreshold": 10.0000,
//		"incidents": [{
//		"incidentID": "1495811850.1700",
//		"endTime": null,
//		"startTime": 1495811850,
//		"falsePositive": false,
//		"state": "Active"
//		}]
//		},
//		"DNSSEC": {
//		"status": "Down",
//		"emergencyThreshold": 10.0000,
//		"incidents": [{
//		"incidentID": "1495811790.1694",
//		"endTime": null,
//		"startTime": 1495811790,
//		"falsePositive": false,
//		"state": "Active"
//		}]
//		},
//		"EPP": {
//		"status": "Disabled"
//		},
//		"RDDS": {
//		13
//		"status": "Disabled"
//		},
//		"RDAP": {
//		"status": "Disabled"
//		}
//		},
//		"version": 2
//		}
type StateResponse struct {
	TLD             string `json:"tld"`
	LastUpdateApiDb int    `json:"lastUpdateApiDatabase"`
	Status          string `json:"status"`
	TestedServices  map[string]TestedService
	Version         int `json:"version"`
}

// TestedService is a struct that represents a tested service in the MOSAPI
type TestedService struct {
	Status             string     `json:"status"`
	EmergencyThreshold float64    `json:"emergencyThreshold"`
	Incidents          []Incident `json:"incidents"`
}

// Incident is a struct that represents an incident in the MOSAPI
type Incident struct {
	IncidentID    string `json:"incidentID"`
	EndTime       *int   `json:"endTime"`
	StartTime     int    `json:"startTime"`
	FalsePositive bool   `json:"falsePositive"`
	State         string `json:"state"`
}
