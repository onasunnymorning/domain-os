package icannspec5

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

const (
	ICANN_SPEC5_XML_URL = "https://www.icann.org/sites/default/files/packages/reserved-names/ReservedNames.xml"
)

// ICANNRepository implements the ICANNRepository interface
type ICANNRepository struct {
	XMLSpec5URL string
}

// NewICANNRepo returns a new ICANNSpec5Repo
func NewICANNRepo() *ICANNRepository {
	return &ICANNRepository{
		XMLSpec5URL: ICANN_SPEC5_XML_URL,
	}
}

// ListSpec5Labels returns a list all ICANN Spec5 Labels
func (repo *ICANNRepository) ListSpec5Labels() ([]*entities.Spec5Label, error) {
	// Get a http.Client
	client := GetHTTPClient()
	// Download the XML
	resp, err := client.Get(repo.XMLSpec5URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the body
	byteValue, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading ICANN XML Spec5 Registry: %v", err)
	}

	// Parse the XML to a struct
	var icannSpec5Registry IcannXmlSpec5Registry
	err = xml.Unmarshal(byteValue, &icannSpec5Registry)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling ICANN XML Spec5 Registry, this might indicate the ICANN site is under maintenance or has updated the XML format: %v", err)
	}

	// Convert the struct to a list of IcannSpec5Labels
	spec5Labels := FromIcannXmlSpec5RegistryToSpec5Label(&icannSpec5Registry)

	// Add Spec 5.1
	for _, label := range SPEC5_1 {
		spec5Labels = append(spec5Labels, &entities.Spec5Label{
			Label: label,
			Type:  "spec5_1",
		})
	}

	// Add Spec 5.2
	for _, label := range SPEC5_2 {
		spec5Labels = append(spec5Labels, &entities.Spec5Label{
			Label: label,
			Type:  "spec5_2",
		})
	}

	// Add Spec 5.3 => This is taken care of by label validation

	// Add Spec 5.4
	for _, label := range SPEC5_4 {
		spec5Labels = append(spec5Labels, &entities.Spec5Label{
			Label: label,
			Type:  "spec5_4",
		})
	}

	// Add Spec 5.5
	for _, label := range SPEC5_5 {
		spec5Labels = append(spec5Labels, &entities.Spec5Label{
			Label: label,
			Type:  "spec5_5",
		})
	}

	// Remove any duplicate labels and return the result
	return removeDuplicates(spec5Labels), nil
}

// removeDuplicates removes duplicate elements from a slice of entities.Spec5Label
func removeDuplicates(elements []*entities.Spec5Label) []*entities.Spec5Label {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []*entities.Spec5Label{}

	for v := range elements {
		if encountered[elements[v].Label] {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v].Label] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}
