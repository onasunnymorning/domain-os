package escrow

import "github.com/onasunnymorning/domain-os/internal/application/services"

// EscrowImportController is a controller for escrow analysis
type EscrowImportController struct {
	svc *services.XMLEscrowService
	rs  *services.RoidService
}

// NewEscrowImportController creates a new instance of EscrowAnalysisController
func NewEscrowImportController(escrowService *services.XMLEscrowService, roidService *services.RoidService) *EscrowImportController {
	return &EscrowImportController{
		svc: escrowService,
		rs:  roidService,
	}
}

// Import calls the escrow analysis service to import the data into the database
func (c *EscrowImportController) Import() error {
	return nil
}
