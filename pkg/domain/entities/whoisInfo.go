package entities

// WhoisInfo Value Object
type WhoisInfo struct {
	Name DomainName `json:"Name" example:"whois.apex.domains" extensions:"x-order=0"`        // name of the registrar WHOIS server listening on TCP port 43
	URL  URL        `json:"URL" example:"https://apex.domains/whois" extensions:"x-order=1"` // URL of the registrar WHOIS server listening on TCP port 80/443
}

// NewWhoisInfo returns a validated WhoisInfo object. It returns an error if any of the input parameters fail validation.
func NewWhoisInfo(name, url string) (*WhoisInfo, error) {
	wi := &WhoisInfo{}
	var dn *DomainName
	var u *URL
	var err error
	if name != "" {
		dn, err = NewDomainName(name)
		if err != nil {
			return nil, err
		}
		wi.Name = *dn
	}
	if url != "" {
		u, err = NewURL(url)
		if err != nil {
			return nil, err
		}
		wi.URL = *u
	}
	return wi, nil
}

// Validate returns a boolean representing the validity of the WhoisInfo object
func (w *WhoisInfo) Validate() error {
	if URL(w.URL.String()) != "" {
		if err := w.URL.Validate(); err != nil {
			return err
		}
	}
	if w.Name.String() != "" {
		if err := w.Name.Validate(); err != nil {
			return err
		}
	}
	return nil
}
