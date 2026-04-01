package jisc

// JiscDomain represents a domain record from the JISC JSON export
type JiscDomain struct {
	DomainID           int            `json:"domain_id"`
	DomainName         string         `json:"domain_name"`
	RegisteredDate     string         `json:"registered_date"`
	RegisterExpireDate string         `json:"registerexpire_date"`
	Status             string         `json:"status"`
	DNS1               string         `json:"dns1"`
	DNS2               string         `json:"dns2"`
	DNS3               string         `json:"dns3"`
	DNS4               string         `json:"dns4"`
	Registrant         JiscRegistrant `json:"registrant"`
	Registrar          JiscRegistrar  `json:"registrar"`
	DSKeys             []interface{}  `json:"ds_keys"` // Using interface{} as the example showed empty array, adjust if structure is known
	Notes              string         `json:"notes"`
	Private            bool           `json:"private"`
}

// JiscRegistrant represents the registrant information
type JiscRegistrant struct {
	ContactID   int    `json:"contact_id"`
	Name        string `json:"name"`
	CompanyName string `json:"company_name"`
	JoID        int    `json:"joid"`
	Address     string `json:"address"`
	PCode       string `json:"pcode"`
	City        string `json:"city"` // Note: Can be null based on example
	Country     string `json:"country"`
	Phone       string `json:"phone"`
	Fax         string `json:"fax"`
	Email       string `json:"email"`
}

// JiscRegistrar represents the registrar information
type JiscRegistrar struct {
	JoID int    `json:"joid"`
	Name string `json:"name"`
}
