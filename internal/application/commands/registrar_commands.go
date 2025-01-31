package commands

import (
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type CreateRegistrarCommand struct {
	ClID        string                           `json:"ClID" binding:"required"`
	Name        string                           `json:"Name" binding:"required"`
	Email       string                           `json:"Email" binding:"required"`
	PostalInfo  [2]*entities.RegistrarPostalInfo `json:"PostalInfo" binding:"required"`
	GurID       int                              `json:"GurID"`
	Voice       string                           `json:"Voice"`
	Fax         string                           `json:"Fax"`
	URL         string                           `json:"URL"`
	RdapBaseURL string                           `json:"RdapBaseURL"`
	WhoisInfo   *entities.WhoisInfo              `json:"WhoisInfo"`
}

type CreateRegistrarCommandResult struct {
	Result entities.Registrar
}

// UpdateRegistrarStatusCommand represents a command to update the status of a registrar.
type UpdateRegistrarStatusCommand struct {
	ClID      string
	NewStatus string
	OldStatus string
}

// ChunkCreateRegistrarCommands returns a channel that yields slices of size chunkSize.
func ChunkCreateRegistrarCommands(cmds []CreateRegistrarCommand, chunkSize int) <-chan []CreateRegistrarCommand {
	ch := make(chan []CreateRegistrarCommand)

	go func() {
		defer close(ch)

		if chunkSize <= 0 {
			// Fallback to 1 if invalid chunkSize
			chunkSize = 1
		}

		for i := 0; i < len(cmds); i += chunkSize {
			end := i + chunkSize
			if end > len(cmds) {
				end = len(cmds)
			}
			// Send the chunk to the channel
			ch <- cmds[i:end]
		}
	}()

	return ch
}

// CompareIANARegistrarStatusWithRarStatus compares the status of an IANA registrar with a platform registrar.
// If the status is different, it returns a command to update the status.
func CompareIANARegistrarStatusWithRarStatus(ianaRar entities.IANARegistrar, rar entities.RegistrarListItem) *UpdateRegistrarStatusCommand {
	// if the status is the same, return nil
	if strings.EqualFold(ianaRar.Status.String(), rar.Status.String()) {
		return nil
	}
	// if the status is accredited (iana) and ok (platform), return nil
	if strings.EqualFold(ianaRar.Status.String(), "accredited") && strings.EqualFold(rar.Status.String(), "ok") {
		return nil
	}

	// if the status is different, return a command to update the status
	newStatus := strings.ToLower(ianaRar.Status.String())
	// IANA uses "accredited" for "ok" status
	if newStatus == "accredited" {
		newStatus = "ok"
	}

	return &UpdateRegistrarStatusCommand{
		ClID:      rar.ClID.String(),
		NewStatus: newStatus,
		OldStatus: rar.Status.String(),
	}
}

func CreateCreateRegistrarCommandFromIANARegistrar(ianaRar entities.IANARegistrar) (*CreateRegistrarCommand, error) {
	if ianaRar.GurID < 0 {
		return nil, fmt.Errorf("invalid GurID for registrar %s: %d", ianaRar.Name, ianaRar.GurID)
	}

	// Create a ClID for the IANA registrar using our naming convention
	clid, err := ianaRar.CreateClID()
	if err != nil {
		return nil, fmt.Errorf("error creating ClID for registrar %d - %s: %v", ianaRar.GurID, ianaRar.Name, err)
	}

	pi, err := createDummyPostalInfo()
	if err != nil {
		return nil, fmt.Errorf("error creating postalinfo for registrar %d - %s: %v", ianaRar.GurID, ianaRar.Name, err)
	}

	// Create the command with dummy information
	cmd := CreateRegistrarCommand{
		ClID:        clid.String(),
		Name:        ianaRar.Name,
		GurID:       ianaRar.GurID,
		RdapBaseURL: ianaRar.RdapURL,
		Email:       "i.need@2be.replaced",
		PostalInfo: [2]*entities.RegistrarPostalInfo{
			pi,
		},
	}

	return &cmd, nil
}

func createDummyPostalInfo() (*entities.RegistrarPostalInfo, error) {
	// Create a dummy postalinfo that will be overwritten if there is data, otherwise it will make it easy to find the missing data
	a, err := entities.NewAddress("Replaceme", "PE")
	if err != nil {
		return nil, fmt.Errorf("error creating address: %v", err)
	}
	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, a)
	if err != nil {
		return nil, fmt.Errorf("error creating postalinfo: %v", err)
	}

	return pi, nil
}
