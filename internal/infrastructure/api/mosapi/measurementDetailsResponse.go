package mosapi

// Root struct for the main response
type MeasurementDetailsResponse struct {
	Version                  int                     `json:"version"`
	LastUpdateApiDatabase    int64                   `json:"lastUpdateApiDatabase"`
	TLD                      string                  `json:"tld,omitempty"`
	RegistrarID              string                  `json:"registrarID,omitempty"`
	Service                  string                  `json:"service"`
	CycleCalculationDateTime int64                   `json:"cycleCalculationDateTime"`
	MinNameServersUp         *int                    `json:"minNameServersUp,omitempty"`
	NameServerAvailability   *NameServerAvailability `json:"nameServerAvailability,omitempty"`
	Status                   string                  `json:"status"`
	TestedInterface          []TestedInterface       `json:"testedInterface"`
}

// NameServerAvailability struct for name server availability details
type NameServerAvailability struct {
	NameServerStatus []NameServerStatus `json:"nameServerStatus"`
}

// NameServerStatus struct for individual name server statuses
type NameServerStatus struct {
	Status string  `json:"status"`
	Target string  `json:"target"`
	Probes []Probe `json:"probes"`
}

// Probe struct for individual probes
type Probe struct {
	City     string     `json:"city"`
	TestData []TestData `json:"testData"`
}

// TestData struct for the test data of probes
type TestData struct {
	Target  string   `json:"target"`
	Status  string   `json:"status"`
	Metrics []Metric `json:"metrics"`
}

// Metric struct for metric details
type Metric struct {
	TestDateTime int64   `json:"testDateTime"`
	TargetIP     string  `json:"targetIP"`
	RTT          *int64  `json:"rtt,omitempty"`
	Result       string  `json:"result"`
	NSID         *string `json:"nsid,omitempty"`
}

// TestedInterface struct for details of tested interfaces
type TestedInterface struct {
	Interface string      `json:"interface"`
	Probes    []ProbeInfo `json:"probes"`
}

// ProbeInfo struct for details within the tested interface probes
type ProbeInfo struct {
	City       string     `json:"city"`
	TestedName string     `json:"testedName,omitempty"`
	Transport  string     `json:"transport,omitempty"`
	Status     string     `json:"status"`
	TestData   []TestData `json:"testData"`
}
