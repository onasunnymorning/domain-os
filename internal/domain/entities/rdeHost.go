package entities

import (
	"reflect"
	"strings"
	"time"
)

var (
	RdeHostCSVHeader = []string{"Name", "RoID", "ClID", "CrRr", "CrDate", "UpRr", "UpDate"}
)

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

// ToCSV converts the RDEHost to a slice of strings ([]string) for CSV export. The fields are defined in RdeHostCSVHeader
func (h *RDEHost) ToCSV() []string {
	return []string{h.Name, h.RoID, h.ClID, h.CrRr, h.CrDate, h.UpRr, h.UpDate}
}

// IsLinked returns true if the host is linked to a domain (contains status "linked")
func (h *RDEHost) IsLinked() bool {
	for _, status := range h.Status {
		if status.S == "linked" {
			return true
		}
	}
	return false
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

	// Add the addresses
	for _, addr := range h.Addr {
		_, err := host.AddAddress(addr.ID)
		if err != nil {
			return nil, err
		}
	}
	// set the statusses
	// for _, status := range h.Status {
	// 	err := host.SetStatus(status.S)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	hs, err := GetHostStatusFromRDEHostStatus(h.Status) // We use this instead of SetStatus because we can't guarantee the order of the statuses, which may break in case a prohibition is set first
	if err != nil {
		return nil, err
	}
	host.Status = hs
	host.SetOKIfNeeded()

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

// GetHostStatusFromRDEHostStatus returns a HostStatus from a []RDEHostStatus slice
func GetHostStatusFromRDEHostStatus(statuses []RDEHostStatus) (HostStatus, error) {
	var hs HostStatus
	for _, status := range statuses {
		// pointer to struct - addressable
		ps := reflect.ValueOf(&hs)
		// struct
		s := ps.Elem()
		if s.Kind() == reflect.Struct {
			// exported field
			var f reflect.Value
			if strings.ToLower(status.S) == "ok" {
				f = s.FieldByName(strings.ToUpper(string(status.S))) // uppercase OK completely
			} else {
				f = s.FieldByName(strings.ToUpper(string(status.S[0])) + status.S[1:]) // uppercase the first character to match the struct field
			}
			if f.IsValid() {
				// A Value can be changed only if it is
				// addressable and was not obtained by
				// the use of unexported struct fields.
				if f.CanSet() {
					// change value of N
					if f.Kind() == reflect.Bool {
						f.SetBool(true)
					}
				}
			} else {
				return hs, ErrInvalidHostStatus
			}
		}
	}
	return hs, nil
}
