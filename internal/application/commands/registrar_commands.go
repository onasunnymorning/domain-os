package commands

import (
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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
	// Optional initial statuses; if omitted, defaults are applied in entity creation
	Status     string                       `json:"Status,omitempty"`
	IANAStatus entities.IANARegistrarStatus `json:"IANAStatus,omitempty"`
}

// UpdateRegistrarStatusCommand represents a command to update the status of a registrar.
// It can carry updates for platform status, IANA status, or both.
type UpdateRegistrarStatusCommand struct {
	ClID          string
	NewStatus     string
	OldStatus     string
	NewIANAStatus string
	OldIANAStatus string
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

// CompareIANARegistrarStatusWithRarStatus compares both the platform status and IANA status
// of an IANA registrar with a platform registrar. If either status differs, it returns
// a command to update both statuses. Returns nil when both are already in sync.
func CompareIANARegistrarStatusWithRarStatus(ianaRar entities.IANARegistrar, rar entities.RegistrarListItem) *UpdateRegistrarStatusCommand {
	// Determine expected platform status from IANA status
	expectedPlatformStatus := strings.ToLower(ianaRar.Status.String())
	if expectedPlatformStatus == "accredited" {
		expectedPlatformStatus = "ok"
	}

	// Check if platform status needs updating
	platformStatusChanged := !strings.EqualFold(expectedPlatformStatus, rar.Status.String())

	// Check if IANA status needs updating
	ianaStatusChanged := !strings.EqualFold(ianaRar.Status.String(), string(rar.IANAStatus))

	// Nothing to do if both are in sync
	if !platformStatusChanged && !ianaStatusChanged {
		return nil
	}

	cmd := &UpdateRegistrarStatusCommand{
		ClID:          rar.ClID.String(),
		OldStatus:     rar.Status.String(),
		OldIANAStatus: string(rar.IANAStatus),
	}

	if platformStatusChanged {
		cmd.NewStatus = expectedPlatformStatus
	}
	if ianaStatusChanged {
		cmd.NewIANAStatus = ianaRar.Status.String()
	}

	return cmd
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
		// Carry IANA status
		IANAStatus: ianaRar.Status,
	}

	// Map initial platform status from IANA status
	switch ianaRar.Status {
	case entities.IANARegistrarStatusAccredited:
		cmd.Status = string(entities.RegistrarStatusOK)
	case entities.IANARegistrarStatusTerminated:
		cmd.Status = string(entities.RegistrarStatusTerminated)
	case entities.IANARegistrarStatusReserved:
		if ianaRar.GurID == 9995 || ianaRar.GurID == 9996 {
			cmd.Status = string(entities.RegistrarStatusOK)
		}
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
