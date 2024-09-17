package entities

import (
	"strconv"
	"time"
)

// WhoisResponse represents the WHOIS response
type WhoisResponse struct {
	DomainName                 string    `json:"domain_name"`
	RegistryDomainID           string    `json:"registry_domain_id"`
	RegistrarWhoisServer       string    `json:"registrar_whois_server"`
	RegistrarURL               string    `json:"registrar_url"`
	UpdatedDate                time.Time `json:"updated_date"`
	CreationDate               time.Time `json:"creation_date"`
	RegistryExpiryDate         time.Time `json:"registry_expiry_date"`
	Registrar                  string    `json:"registrar"`
	RegistrarIANAID            string    `json:"registrar_iana_id"`
	RegistrarAbuseContactEmail string    `json:"registrar_abuse_contact_email"`
	RegistrarAbuseContactPhone string    `json:"registrar_abuse_contact_phone"`
	DomainStatus               []string  `json:"domain_status"`
	NameServers                []string  `json:"name_servers"`
	DNSSEC                     string    `json:"dnssec"`
	ICANNComplaintURL          string    `json:"icann_complaint_url"`
	LastWhoisUpdate            time.Time `json:"last_whois_update"`
}

// String returns the string representation of the WhoisResponse
func (w WhoisResponse) String() string {
	var resp string
	resp += "Domain Name: " + w.DomainName + "\n"
	resp += "Registry Domain ID: " + w.RegistryDomainID + "\n"
	resp += "Registrar WHOIS Server: " + w.RegistrarWhoisServer + "\n"
	resp += "Registrar URL: " + w.RegistrarURL + "\n"
	resp += "Updated Date: " + w.UpdatedDate.String() + "\n"
	resp += "Creation Date: " + w.CreationDate.String() + "\n"
	resp += "Registry Expiry Date: " + w.RegistryExpiryDate.String() + "\n"
	resp += "Registrar: " + w.Registrar + "\n"
	resp += "Registrar IANA ID: " + w.RegistrarIANAID + "\n"
	resp += "Registrar Abuse Contact Email: " + w.RegistrarAbuseContactEmail + "\n"
	resp += "Registrar Abuse Contact Phone: " + w.RegistrarAbuseContactPhone + "\n"
	for _, d := range w.DomainStatus {
		resp += "Domain Status: " + d + "\n"
	}
	for _, d := range w.NameServers {
		resp += "DNSSEC Data: " + d + "\n"
	}
	resp += "DNSSEC: " + w.DNSSEC + "\n"
	resp += "ICANN Complaint URL: " + w.ICANNComplaintURL + "\n"
	resp += ">>> Last update of whois database:" + w.LastWhoisUpdate.String() + " <<<\n"
	return resp
}

// NewWhoisResponse creates a new instance of WhoisResponse
func NewWhoisResponse(dom *Domain, rar *Registrar) (*WhoisResponse, error) {
	w := &WhoisResponse{
		DomainName:                 dom.Name.String(),
		RegistryDomainID:           dom.RoID.String(),
		RegistrarWhoisServer:       rar.WhoisInfo.Name.String(),
		RegistrarURL:               rar.URL.String(),
		UpdatedDate:                dom.UpdatedAt,
		CreationDate:               dom.CreatedAt,
		RegistryExpiryDate:         dom.ExpiryDate,
		Registrar:                  rar.Name,
		RegistrarIANAID:            strconv.Itoa(rar.GurID),
		RegistrarAbuseContactEmail: rar.Email,
		RegistrarAbuseContactPhone: rar.Voice.String(),
		DomainStatus:               dom.Status.StringSlice(),
		NameServers:                dom.GetHostsAsStringSlice(),
		DNSSEC:                     "unsigned",
		ICANNComplaintURL:          "URL of the ICANN Whois Inaccuracy Complaint Form: https://www.icann.org/wicf/",
		LastWhoisUpdate:            time.Now(),
	}
	return w, nil
}
