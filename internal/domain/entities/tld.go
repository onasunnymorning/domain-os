package entities

import (
	"strings"
	"time"

	"golang.org/x/net/idna"
)

// TLDType is a custom type describing the type of TLD
type TLDType string

// String returns the string representation of the TLDType
func (t TLDType) String() string {
	return string(t)
}

// TLDType constants
const (
	TLDTypeGTLD  = "generic"
	TLDTypeCCTLD = "country-code"
	TLDTypeSLD   = "second-level"
)

// TLD is a struct representing a top-level domain
type TLD struct {
	Name      DomainName `json:"name"`  // Name is the ASCII name of the TLD (aka A-label)
	Type      TLDType    `json:"type"`  // Type is the type of TLD (generic, country-code, second-level)
	UName     string     `json:"uname"` // UName is the unicode name of the TLD (aka U-label)
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NewTLD returns a pointer to a TLD struct or an error (ErrInvalidDomainName) if the domain name is invalid. It will set the Uname and TLDType fields.
func NewTLD(name string) (*TLD, error) {
	d, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	tld := &TLD{Name: *d}
	tld.SetUname()
	tld.setTLDType()
	tld.CreatedAt = RoundTime(time.Now().UTC())
	return tld, nil
}

// SetUname sets the unicode name of the TLD based on the name. Uname is always set regardless if the name is an IDN. If the name is not an IDN the Uname will be equal to the name.
func (t *TLD) SetUname() {
	unicode_string, _ := idna.ToUnicode(string(t.Name))
	t.UName = unicode_string
}

// Determines TLD type from the name. If the name is 2 characters long, it's a country-code TLD. If it contains a dot, it's a second-level TLD. Otherwise, it's a generic TLD.
func (t *TLD) setTLDType() {
	if len(string(t.Name)) == 2 {
		t.Type = TLDTypeCCTLD
	} else if strings.Contains(string(t.Name), ".") {
		t.Type = TLDTypeSLD
	} else {
		t.Type = TLDTypeGTLD
	}
}
