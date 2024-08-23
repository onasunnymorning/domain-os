package entities

import (
	"encoding/xml"
	"reflect"
	"time"
)

var (
	RdeDomainCSVHeader = []string{"Name", "RoID", "UName", "IdnTableId", "OriginalName", "Registrant", "ClID", "CrRr", "CrDate", "ExDate", "UpRr", "UpDate"}
)

type RDEDomain struct {
	XMLName      xml.Name             `xml:"domain" json:"-"`
	Name         DomainName           `xml:"name"` // element that contains the fully qualified name of the domain name object. For IDNs, the A-label is used
	RoID         string               `xml:"roid"` // element that contains the ROID assigned to the domain name object when it was created
	UName        string               `xml:"uName"`
	IdnTableId   string               `xml:"idnTableId"`
	OriginalName string               `xml:"originalName"`
	Status       []RDEDomainStatus    `xml:"status"`
	RgpStatus    []RDEDomainRGPStatus `xml:"rgpStatus"`
	Registrant   string               `xml:"registrant"`
	Contact      []RDEDomainContact   `xml:"contact"`
	Ns           []RDEDomainHost      `xml:"ns"`
	ClID         string               `xml:"clID"`
	CrRr         string               `xml:"crRr"`
	CrDate       string               `xml:"crDate"`
	ExDate       string               `xml:"exDate"`
	UpRr         string               `xml:"upRr"`
	UpDate       string               `xml:"upDate"`
	SecDNS       RDESecDNS            `xml:"secDNS"`
	TrnData      TrnData              `xml:"trnData"`
}

// ToCSV converts the RDEDomain to a slice of strings ([]string) for CSV export. The fields are defined in RdeDomainCSVHeader
func (d *RDEDomain) ToCSV() []string {
	return []string{string(d.Name), d.RoID, d.UName, d.IdnTableId, d.OriginalName, d.Registrant, d.ClID, d.CrRr, d.CrDate, d.ExDate, d.UpRr, d.UpDate}
}

// ToEntity converts the RDEDomain to a Domain entity
func (d *RDEDomain) ToEntity() (*Domain, error) {
	// Since the Escrow specification (RFC 9022) does not specify the authInfo field, we will generate a random one to import the data
	aInfo, err := NewAuthInfoType("escr0W1mP*rt")
	if err != nil {
		return nil, err // Untestable, just catching the error incase we edit two lines above
	}
	domain, err := NewDomain(d.RoID, d.Name.String(), d.ClID, string(aInfo))
	if err != nil {
		return nil, err
	}

	// Set the ExpiryDate
	domain.ExpiryDate, err = time.Parse(time.RFC3339, d.ExDate)
	if err != nil {
		return nil, err
	}

	// Set the optional fields
	if d.UName != "" {
		domain.UName = DomainName(d.UName)
	}
	// TODO: Set this when the Domain has IDN support
	// if d.IdnTableId != "" {
	// 	domain.IdnTableId = d.IdnTableId
	// }
	if d.OriginalName != "" {
		domain.OriginalName = DomainName(d.OriginalName)
	}
	if d.CrRr != "" {
		crrr, err := NewClIDType(d.CrRr)
		if err != nil {
			return nil, err
		}
		domain.CrRr = crrr
	}
	if d.CrDate != "" {
		date, err := time.Parse(time.RFC3339, d.CrDate)
		if err != nil {
			return nil, err
		}
		domain.CreatedAt = date
	}
	if d.UpRr != "" {
		uprr, err := NewClIDType(d.UpRr)
		if err != nil {
			return nil, err
		}
		domain.UpRr = uprr
	}
	if d.UpDate != "" {
		date, err := time.Parse(time.RFC3339, d.UpDate)
		if err != nil {
			return nil, err
		}
		domain.UpdatedAt = date
	}
	// Set contact information
	if d.Registrant != "" {
		c, err := NewClIDType(d.Registrant)
		if err != nil {
			return nil, err
		}
		domain.RegistrantID = c
	}
	if len(d.Contact) > 0 {
		for _, contact := range d.Contact {
			switch contact.Type {
			case "admin":
				c, err := NewClIDType(contact.ID)
				if err != nil {
					return nil, err
				}
				domain.AdminID = c
			case "tech":
				c, err := NewClIDType(contact.ID)
				if err != nil {
					return nil, err
				}
				domain.TechID = c
			case "billing":
				c, err := NewClIDType(contact.ID)
				if err != nil {
					return nil, err
				}
				domain.BillingID = c
			default:
				return nil, ErrInvalidContact
			}
		}
	}
	// Set the status
	ds, err := GetDomainStatusFromRDEDomainStatus(d.Status)
	if err != nil {
		return nil, err
	}
	domain.Status = ds
	// NOTE: If you experience deltas importing escrows, might want to investigate the following line
	domain.SetUnsetInactiveStatus() // this is needed because we just overwrote the status with what we got from the RDE, But Inactive which might not meet ou

	err = domain.Status.Validate()
	if err != nil {
		return nil, err
	}

	// TODO: FIXME: Add the nameservers

	err = domain.Validate()
	if err != nil {
		return nil, err
	}

	return domain, nil

}

type RDEDomainStatus struct {
	S string `xml:"s,attr"`
}

type RDEDomainRGPStatus struct {
	S string `xml:"s,attr"`
}

type RDEDomainHost struct {
	HostObjs []string `xml:"hostObj"`
}

type RDEDomainContact struct {
	Type string `xml:"type,attr"`
	ID   string `xml:",chardata"`
}

type DSData struct {
	KeyTag     int    `xml:"keyTag"`
	Alg        int    `xml:"alg"`
	DigestType int    `xml:"digestType"`
	Digest     string `xml:"digest"`
}

type RDESecDNS struct {
	DSData []DSData `xml:"dsData"`
}

type TrnData struct {
	TrStatus TrStatus `xml:"trStatus"`
	ReRr     ReRr     `xml:"reRr"`
	ReDate   string   `xml:"reDate"`
	AcRr     AcRr     `xml:"acRr"`
	AcDate   string   `xml:"acDate"`
	ExDate   string   `xml:"exDate,omitempty"`
}

type TrStatus struct {
	State string `xml:",chardata"`
}

type ReRr struct {
	RegID  string `xml:",chardata"`
	Client string `xml:"client,attr,omitempty"`
}

type AcRr struct {
	RegID  string `xml:",chardata"`
	Client string `xml:"client,attr,omitempty"`
}

// GetDomainStatusFromRDEDomainStatus returns a DomainStatus type from a []RDEDomainStatus slice
// Use this instead of the SetStatus function because we can't guarantee the order of the statuses
func GetDomainStatusFromRDEDomainStatus(statuses []RDEDomainStatus) (DomainStatus, error) {
	var ds DomainStatus
	for _, status := range statuses {
		// pointer to struct - addressable
		ps := reflect.ValueOf(&ds)
		// struct
		s := ps.Elem()
		if s.Kind() == reflect.Struct {
			// exported field
			var f reflect.Value
			switch status.S {
			case DomainStatusClientDeleteProhibited:
				f = s.FieldByName("ClientDeleteProhibited")
			case DomainStatusClientHold:
				f = s.FieldByName("ClientHold")
			case DomainStatusClientRenewProhibited:
				f = s.FieldByName("ClientRenewProhibited")
			case DomainStatusClientTransferProhibited:
				f = s.FieldByName("ClientTransferProhibited")
			case DomainStatusClientUpdateProhibited:
				f = s.FieldByName("ClientUpdateProhibited")
			case DomainStatusInactive:
				f = s.FieldByName("Inactive")
			case DomainStatusOK:
				f = s.FieldByName("OK")
			case DomainStatusPendingCreate:
				f = s.FieldByName("PendingCreate")
			case DomainStatusPendingDelete:
				f = s.FieldByName("PendingDelete")
			case DomainStatusPendingRenew:
				f = s.FieldByName("PendingRenew")
			case DomainStatusPendingTransfer:
				f = s.FieldByName("PendingTransfer")
			case DomainStatusPendingUpdate:
				f = s.FieldByName("PendingUpdate")
			case DomainStatusServerDeleteProhibited:
				f = s.FieldByName("ServerDeleteProhibited")
			case DomainStatusServerHold:
				f = s.FieldByName("ServerHold")
			case DomainStatusServerRenewProhibited:
				f = s.FieldByName("ServerRenewProhibited")
			case DomainStatusServerTransferProhibited:
				f = s.FieldByName("ServerTransferProhibited")
			case DomainStatusServerUpdateProhibited:
				f = s.FieldByName("ServerUpdateProhibited")
			default:
				return ds, ErrInvalidDomainStatus
			}
			if f.IsValid() && f.CanSet() {
				// change value of N
				if f.Kind() == reflect.Bool {
					f.SetBool(true)
				}
			}
		}

	}
	return ds, nil
}
