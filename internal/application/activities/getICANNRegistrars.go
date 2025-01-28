package activities

import (
	"io"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// GetICANNRegistrars parses a csv file as published here https://www.icann.org/en/accredited-registrars
// and returns the registrars described inside. Unfortunately the page renders the data in a table on the client side so we can't scrape it.
// We store this file in our repository. It only contains public contact information and is not considered sensitive.
// This file conatinas complementary information to the IANA registrar list.
// It is useful only once, during system init, to enrich the IANA registrar list
// before importing registrars for the first time
func GetICANNRegistrars(correlationID string, fileReader io.Reader) ([]entities.IANARegistrar, error) {

	var returnVal []entities.IANARegistrar

	return returnVal, nil
}
