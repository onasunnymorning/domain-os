package icannregistrars

import (
	"fmt"
	"log"
	"strconv"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// GetCreateCommands takes a slice of CSVRegistrars and a slice of IANARegistrars and returns a slice of CreateRegistrarCommands
func GetCreateCommands(csvRegistrars []CSVRegistrar, icannRegistrars []entities.IANARegistrar) ([]commands.CreateRegistrarCommand, error) {
	skipped := []string{}
	seen := make(map[string]bool)
	var createCommands []commands.CreateRegistrarCommand

	// Create a dummy postalinfo that will be overwritten if there is data, otherwise it will make it easy to find the missing data
	a, err := entities.NewAddress("Replaceme", "PE")
	if err != nil {
		return nil, fmt.Errorf("error creating address: %v", err)
	}
	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, a)
	if err != nil {
		return nil, fmt.Errorf("error creating postalinfo: %v", err)
	}

	// Create a map of IANARegistrars for easy lookup by IANAID
	ianaMap := make(map[int]entities.IANARegistrar)
	for _, irar := range icannRegistrars {
		ianaMap[irar.GurID] = irar
	}

	// Create a map of CSVRegistrars for easy lookup by IANAID
	csvMap := make(map[int]CSVRegistrar)
	for _, crar := range csvRegistrars {
		csvMap[crar.IANAID] = crar
	}

	// Loop over the IANARegistrars and create a CreateRegistrarCommand for each, enriched with the contact information from the CSVRegistrars
	for _, irar := range icannRegistrars {

		// Omit the reserved registrars
		if irar.Status == entities.IANARegistrarStatusReserved {
			log.Printf("[WARN] Registrar %s with GurID %d is reserved, skipping\n", irar.Name, irar.GurID)
			skipped = append(skipped, strconv.Itoa(irar.GurID)+" - "+irar.Name)
			continue
		}

		clid, err := irar.CreateClID()
		if err != nil {
			return nil, fmt.Errorf("error creating ClID for registrar %d - %s: %v", irar.GurID, irar.Name, err)
		}

		if seen[irar.Name] {
			irar.Name = irar.Name + "-2"
		}
		seen[irar.Name] = true

		// Create the command with dummy information
		cmd := commands.CreateRegistrarCommand{
			ClID:  clid.String(),
			GurID: irar.GurID,
			Name:  irar.Name,
			Email: "i.need@2be.replaced",
			PostalInfo: [2]*entities.RegistrarPostalInfo{
				pi,
			},
		}

		// try and enrich the command with the contact information from the CSVRegistrars - only if it exists
		csv, ok := csvMap[irar.GurID]
		if ok {
			a, err := csv.Address()
			if err != nil {
				return nil, fmt.Errorf("error getting address for registrar %s: %v", csv.Name, err)
			}
			// if the Address is ASCII add an int postalinfo, else add a loc postalinfo
			if isacii, _ := a.IsASCII(); isacii {
				cmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
					Type:    entities.PostalInfoEnumTypeINT,
					Address: a,
				}
			} else {
				cmd.PostalInfo[0] = &entities.RegistrarPostalInfo{
					Type:    entities.PostalInfoEnumTypeLOC,
					Address: a,
				}
			}

			cmd.Email = csv.ContactEmail()
			cmd.Voice = csv.ContactPhone()
			cmd.URL = csv.Link

		}

		// Add the command to the slice
		createCommands = append(createCommands, cmd)

	}

	// Log the skipped registrars
	if len(skipped) > 0 {
		log.Printf("[INFO] Skipped %d reserved registrars\n", len(skipped))
	}

	return createCommands, nil
}
