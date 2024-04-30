package escrow

import (
	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// EscrowImportController is a controller for escrow analysis
type EscrowImportController struct {
	svc *services.XMLEscrowService
}

// NewEscrowImportController creates a new instance of EscrowAnalysisController
func NewEscrowImportController(escrowService *services.XMLEscrowService) *EscrowImportController {
	return &EscrowImportController{
		svc: escrowService,
	}
}

// Import calls the escrow analysis service to import the data into the database
func (c *EscrowImportController) Import(analysisFile, depositFile string) error {
	// Load the analysis file
	err := c.svc.LoadDepostiAnalysis(analysisFile, depositFile)
	if err != nil {
		return err
	}

	// Load the unique contact IDs from file
	err = c.svc.LoadUniqueContactIDs()
	if err != nil {
		return err
	}

	// Check the TLD is in a state allowing import
	// TODO: Implement this
	// e.g. check if the TLD has no domains/contacts/hosts or create a flag that indicates that import is possible

	// Import the Contacts
	contactCmds, err := c.svc.ExtractContacts(true)
	if err != nil {
		return err
	}
	err = c.svc.CreateContacts(contactCmds)
	if err != nil {
		return err
	}

	// Import the Hosts
	hostCmds, err := c.svc.ExtractHosts(true)
	if err != nil {
		return err
	}
	err = c.svc.CreateHosts(hostCmds)

	// Import the Domains

	// QA the import was successful

	// Log the result in a file
	err = c.svc.SaveImportResult()
	if err != nil {
		return err
	}

	return nil
}
