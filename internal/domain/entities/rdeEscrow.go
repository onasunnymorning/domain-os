package entities

var (
	RDE_DOMAIN_CSV_HEADER             = []string{"Name", "RoID", "Uname", "IdnTableId", "OriginalName", "Registrant", "ClID", "CrRr", "CrDate", "ExDate", "UpRr", "UpDate"}
	RDE_HOST_CSV_HEADER               = []string{"Name", "RoID", "ClID", "CrRr", "CrDate", "UpRr", "UpDate"}
	RDE_CONTACT_CSV_HEADER            = []string{"ID", "RoID", "Voice", "Fax", "Email", "ClID", "CrRr", "CrDate", "UpRr", "UpDate"}
	RDE_REGISTRAR_CSV_HEADER          = []string{"ID", "Name", "GurID", "Email", "Voice", "Fax", "URL"}
	RDE_NNDN_CSV_HEADER               = []string{"AName", "UName", "IDNTableID", "OriginalName", "NameState", "CrDate"}
	RDE_REGISTRAR_MAPPING_CSV_HEARDER = []string{"ClID", "Name", "GurID", "RegistrarID", "DomainCount", "HostCound", "ContactCount"}

	IDN_URI       = "urn:ietf:params:xml:ns:rdeIDN-1.0"
	CONTACT_URI   = "urn:ietf:params:xml:ns:rdeContact-1.0"
	DOMAIN_URI    = "urn:ietf:params:xml:ns:rdeDomain-1.0"
	HOST_URI      = "urn:ietf:params:xml:ns:rdeHost-1.0"
	NNDN_URI      = "urn:ietf:params:xml:ns:rdeNNDN-1.0"
	REGISTRAR_URI = "urn:ietf:params:xml:ns:rdeRegistrar-1.0"
)

// RegsitrarMapping maps the ID of the registrar in the RDE Escrow file to the RdeRegistrarInfo
type RegsitrarMapping map[string]RdeRegistrarInfo

// RegistrarInfo holds counters for the objects associated with a registrar as found in an RDE Escrow file when analyzing that file
type RdeRegistrarInfo struct {
	Name          string   `json:"name"`
	GurID         int      `json:"gurID"`
	RegistrarClID ClIDType `json:"registrarClID"`
	DomainCount   int      `json:"domainCount"`
	HostCount     int      `json:"hostCount"`
	ContactCount  int      `json:"contactCount"`
}

// NewRegistrarMapping creates a new, empty RegistrarMapping
func NewRegistrarMapping() RegsitrarMapping {
	return make(RegsitrarMapping)
}

// EscrowAnalysis holds errors and warnings as well as files generated and counters
type EscrowAnalysis struct {
	// Errors          []string      `json:"errors"`
	// Warnings        []string      `json:"warnings"`
	// Files           []FileCounter `json:"files"`
	MissingContacts []string `json:"missingContacts"`
}
