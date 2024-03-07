package entities

// WhoisInfo Value Object
type WhoisInfo struct {
	Name DomainName `json:"Name" example:"whois.apex.domains" extensions:"x-order=0"`        // name of the registrar WHOIS server listening on TCP port 43
	URL  URL        `json:"URL" example:"https://apex.domains/whois" extensions:"x-order=1"` // URL of the registrar WHOIS server listening on TCP port 80/443
}

// NewWhoisInfo returns a validated WhoisInfo object. It returns an error if any of the input parameters fail validation.
func NewWhoisInfo(name, url string) (*WhoisInfo, error) {
	dn, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	u, err := NewURL(url)
	if err != nil {
		return nil, err
	}
	return &WhoisInfo{
		Name: *dn,
		URL:  *u,
	}, nil
}

// Validate returns a boolean representing the validity of the WhoisInfo object
func (w *WhoisInfo) Validate() error {
	if err := w.URL.Validate(); err != nil {
		return err
	}
	if err := w.Name.Validate(); err != nil {
		return err
	}
	return nil
}
