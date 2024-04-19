package entities

import "time"

type RDEHost struct {
	Name   string          `xml:"name"`
	RoID   string          `xml:"roid"`
	Status []RDEHostStatus `xml:"status"`
	Addr   []RDEHostAddr   `xml:"addr"`
	ClID   string          `xml:"clID"`
	CrRr   string          `xml:"crRr"`
	CrDate string          `xml:"crDate"`
	UpRr   string          `xml:"upRr"`
	UpDate string          `xml:"upDate"`
}

// ToEntity converts the RDEHost to an Host entity
func (h *RDEHost) ToEntity() (*Host, error) {
	host, err := NewHost(h.Name, h.RoID, h.ClID)
	if err != nil {
		return nil, err
	}
	// Set the optional fields
	if h.CrRr != "" {
		crrr, err := NewClIDType(h.CrRr)
		if err != nil {
			return nil, err
		}
		host.CrRr = crrr
	}
	if h.UpRr != "" {
		uprr, err := NewClIDType(h.UpRr)
		if err != nil {
			return nil, err
		}
		host.UpRr = uprr
	}
	if h.CrDate != "" {
		date, err := time.Parse(time.RFC3339, h.CrDate)
		if err != nil {
			return nil, err
		}
		host.CreatedAt = date
	}
	if h.UpDate != "" {
		date, err := time.Parse(time.RFC3339, h.UpDate)
		if err != nil {
			return nil, err
		}
		host.UpdatedAt = date
	}

	// set the statusses
	for _, status := range h.Status {
		err := host.SetStatus(status.S)
		if err != nil {
			return nil, err
		}
	}
	// Add the addresses
	for _, addr := range h.Addr {
		_, err := host.AddAddress(addr.IP)
		if err != nil {
			return nil, err
		}
	}

	// Validate the host and return it
	if err := host.Validate(); err != nil {
		return nil, err
	}

	return host, nil
}

type RDEHostStatus struct {
	S string `xml:"s,attr"`
}

type RDEHostAddr struct {
	IP string `xml:"ip,attr"`
	ID string `xml:",chardata"`
}
