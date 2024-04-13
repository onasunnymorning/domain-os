package escrow

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// EscrowAnalysisController is a controller for escrow analysis
type EscrowAnalysisController struct {
	svc *services.XMLEscrowAnalysisService
}

// NewEscrowAnalysisController creates a new instance of EscrowAnalysisController
func NewEscrowAnalysisController(escrowAnalysisService *services.XMLEscrowAnalysisService) *EscrowAnalysisController {
	return &EscrowAnalysisController{
		svc: escrowAnalysisService,
	}
}

// Analyze calls the escrow analysis service to analyze the deposit and header tags and prints the results
func (c *EscrowAnalysisController) Analyze() error {
	log.Println("Analyzing escrow file")

	if err := c.svc.AnalyzeDepostTag(); err != nil {
		return err
	}

	if err := c.svc.AnalyzeHeaderTag(); err != nil {
		return err
	}

	c.svc.UnlinkedContactCheck()

	if err := c.svc.AnalyzeRegistrarTags(c.svc.Header.RegistrarCount()); err != nil {
		return err
	}

	if err := c.svc.AnalyzeIDNTableRefTags(c.svc.Header.IDNCount()); err != nil {
		return err
	}

	if err := c.svc.ExtractContacts(); err != nil {
		return err
	}

	if err := c.svc.ExtractDomains(); err != nil {
		return err
	}

	if err := c.svc.ExtractHosts(); err != nil {
		return err
	}

	if err := c.svc.ExtractNNDNS(); err != nil {
		return err
	}

	if err := c.svc.LookForMissingContacts(); err != nil {
		return err
	}

	log.Println("Analysis complete")

	return nil
}
