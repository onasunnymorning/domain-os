package entities

import "fmt"

var (
	ErrHeaderCountNotFount = fmt.Errorf("count for this object not found, have you analyzed the header?")
)

type RDEHeader struct {
	TLD       string     `xml:"tld"`
	Registrar int        `xml:"registrar"`
	PPSP      int        `xml:"ppsp"`
	Count     []RDECount `xml:"count"`
}

type RDECount struct {
	Uri string `xml:"uri,attr"`
	ID  int    `xml:",chardata"`
}

func (h *RDEHeader) IDNCount() int {
	for _, count := range h.Count {
		if count.Uri == IDN_URI {
			return count.ID
		}
	}
	return 0
}

func (h *RDEHeader) ContactCount() int {
	for _, count := range h.Count {
		if count.Uri == CONTACT_URI {
			return count.ID
		}
	}
	return 0
}

func (h *RDEHeader) DomainCount() int {
	for _, count := range h.Count {
		if count.Uri == DOMAIN_URI {
			return count.ID
		}
	}
	return 0
}

func (h *RDEHeader) HostCount() int {
	for _, count := range h.Count {
		if count.Uri == HOST_URI {
			return count.ID
		}
	}
	return 0
}

func (h *RDEHeader) NNDNCount() int {
	for _, count := range h.Count {
		if count.Uri == NNDN_URI {
			return count.ID
		}
	}
	return 0
}

func (h *RDEHeader) RegistrarCount() int {
	for _, count := range h.Count {
		if count.Uri == REGISTRAR_URI {
			return count.ID
		}
	}
	return 0
}
