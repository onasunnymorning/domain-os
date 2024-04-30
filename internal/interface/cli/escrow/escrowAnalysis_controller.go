package escrow

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// EscrowAnalysisController is a controller for escrow analysis
type EscrowAnalysisController struct {
	svc *services.XMLEscrowService
}

// NewEscrowAnalysisController creates a new instance of EscrowAnalysisController
func NewEscrowAnalysisController(escrowAnalysisService *services.XMLEscrowService) *EscrowAnalysisController {
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

	if err := c.svc.ExtractDomains(); err != nil {
		return err
	}

	if _, err := c.svc.ExtractContacts(false); err != nil {
		return err
	}

	if _, err := c.svc.ExtractHosts(false); err != nil {
		return err
	}

	if err := c.svc.ExtractNNDNS(); err != nil {
		return err
	}

	if err := c.svc.LookForMissingContacts(); err != nil {
		return err
	}

	if err := c.svc.MapRegistrars(); err != nil {
		return err
	}

	log.Println("Analysis complete")

	if err := c.svc.SaveAnalysis(); err != nil {
		return err
	}

	return nil
}
