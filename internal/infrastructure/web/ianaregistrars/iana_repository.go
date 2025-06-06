package ianaregistrars

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

const (
	IANA_REGISTRARS_XML_URL = "https://www.iana.org/assignments/registrar-ids/registrar-ids.xml"
)

var (
	ErrNotimplemented = errors.New("not implemented")
)

// IANARepository implements the IANARepository interface
type IANARepository struct {
	XMLRegistrarURL string
}

// NewIANARRepository returns a new IANARepo
func NewIANARRepository() *IANARepository {
	return &IANARepository{
		XMLRegistrarURL: IANA_REGISTRARS_XML_URL,
	}
}

// ListRegistrars returns a list all IANA Registrars
func (repo *IANARepository) ListRegistrars() ([]*entities.IANARegistrar, error) {
	// Get a http.Client
	client := GetHTTPClient()
	// Download the XML
	resp, err := client.Get(repo.XMLRegistrarURL)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving IANA XML Registry: %v", err)
	}
	defer resp.Body.Close()

	// Read the body
	byteValue, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading IANA XML Registry: %v", err)
	}

	// Parse the XML to a struct
	var registry IanaXmlRegistry
	err = xml.Unmarshal(byteValue, &registry)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling IANA XML Registry: %v", err)
	}

	// Convert the struct to a list of IANARegistrar entities
	registrars := make([]*entities.IANARegistrar, len(registry.Registry.Records))
	for i, record := range registry.Registry.Records {
		registrars[i] = FromIANAXMLRegistrarRecord(&record)
	}

	// Return the list of IANARegistrar entities
	return registrars, nil
}
