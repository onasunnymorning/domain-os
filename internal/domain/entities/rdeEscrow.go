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
