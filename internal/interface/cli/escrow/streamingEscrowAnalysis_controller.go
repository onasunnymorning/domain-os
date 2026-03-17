package escrow

import (
	"fmt"
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// StreamingEscrowAnalysisController is an optimized controller for escrow analysis using single-pass streaming
type StreamingEscrowAnalysisController struct {
	svc *services.StreamingXMLEscrowService
}

// NewStreamingEscrowAnalysisController creates a new instance of StreamingEscrowAnalysisController
func NewStreamingEscrowAnalysisController(xmlFilename string) (*StreamingEscrowAnalysisController, error) {
	streamingService, err := services.NewStreamingXMLEscrowService(xmlFilename)
	if err != nil {
		return nil, err
	}

	return &StreamingEscrowAnalysisController{
		svc: streamingService,
	}, nil
}

// AnalyzeStreaming performs optimized single-pass analysis of the escrow file
func (c *StreamingEscrowAnalysisController) AnalyzeStreaming(mapRegistrars bool) error {
	// Perform single-pass streaming analysis
	token := fmt.Sprintf("Bearer %s", os.Getenv("ADMIN_TOKEN"))
	if err := c.svc.StreamAnalyze(mapRegistrars, token, nil); err != nil {
		return err
	}

	// Display results
	log.Printf("Analysis completed successfully!")
	log.Printf("📄 Deposit: %s", c.svc.GetDepositJSON())
	log.Printf("📊 Header: %s", c.svc.GetHeaderJSON())

	return nil
}

// GetBaseService returns the underlying XMLEscrowService for compatibility
func (c *StreamingEscrowAnalysisController) GetBaseService() *services.XMLEscrowService {
	return c.svc.XMLEscrowService
}
