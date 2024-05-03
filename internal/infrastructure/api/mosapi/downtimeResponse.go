package mosapi

type DowntimeResponse struct {
	Version         int `json:"version"`
	LastUpdateApiDb int `json:"lastUpdateApiDatabase"`
	Downtime        int `json:"downtime"`
}
