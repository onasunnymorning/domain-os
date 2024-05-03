package mosapi

// {
// 	"version": 1,
// 	"lastUpdateApiDatabase": 1422492450,
// 	"alarmed": "Yes"
//    }

// AlarmResponse is a struct that represents the response from the alarm endpoint of the MOSAPI
type AlarmResponse struct {
	Version         int    `json:"version"`
	LastUpdateApiDb int    `json:"lastUpdateApiDatabase"`
	Alarmed         string `json:"alarmed"`
}

// IsAlarmed returns true if the service is alarmed
func (a *AlarmResponse) IsAlarmed() bool {
	return a.Alarmed == "Yes"
}
