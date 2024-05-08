package mosapi

type StateResponse struct {
	TLD             string `json:"tld"`
	LastUpdateApiDb int    `json:"lastUpdateApiDatabase"` // a JSON number that contains the Unix time stamp of the date and time that the monitoring information provided in the MoSAPI was last updated from the monitoring system central database.
	// Status: the current status of the Service. The possible values are:
	// Up: all of the monitored Services are up.
	// Down: one or more of the monitored Services are down.
	// Up-inconclusive: the SLA monitoring system is under maintenance, therefore all the monitored Services of the TLD are considered to be up by default. Note: if the status is
	// Up-inconclusive
	Status         string `json:"status"`
	TestedServices map[string]TestedService
	Version        int `json:"version"`
}

// TestedService is a struct that represents a tested service in the MOSAPI
type TestedService struct {
	// 	"status", a JSON string that contains the status of the Service as seen from the monitoring system. The "status" field can contain one of the following values:
	// · Up: the monitored Service is up.
	// · Down: the monitored Service is down.
	// · Disabled: the Service is not being monitored.
	// · UP-inconclusive-no-data: indicates that there are enough probe nodes online, but not enough raw data points were received to make a determination.
	// · UP-inconclusive-no-probes: indicates that there are not enough probe nodes online to make a determination.
	// · UP-inconclusive-reconfig: indicates that the system is undergoing a reconfiguration for the particular TLD and service.
	Status string `json:"status"`
	// "emergencyThreshold", a JSON number that contains the current percentage of the Emergency Threshold of the Service. Note: the value "0" specifies that the are no Incidents affecting the Emergency Threshold of the Service.
	// Emergency Threshold: downtime threshold that if reached by any of the monitored Services may cause the TLD's Services emergency transition to an interim Registry Operator. To reach an Emergency Threshold a Service must accumulate X hours of total downtime during the last 7 days (i.e., rolling week).
	// For DNS X=4 (4h per rolling week), for RDDS and RDAP X=24 (24h per rolling week)
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
