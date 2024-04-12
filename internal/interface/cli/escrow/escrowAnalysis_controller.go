package escrow

import (
	"fmt"
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

	if err := c.svc.AnalyzeRegistrarTags(c.svc.Header.RegistrarCount()); err != nil {
		return err
	}

	if err := c.svc.AnalyzeIDNTableRefTags(c.svc.Header.IDNCount()); err != nil {
		return err
	}

	log.Println("Analysis complete")

	fmt.Println(c.svc.GetDepositJSON())
	fmt.Println(c.svc.GetHeaderJSON())
	fmt.Println(c.svc.Registrars)
	fmt.Println(c.svc.IDNs)

	return nil
}
