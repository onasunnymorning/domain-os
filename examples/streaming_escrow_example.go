package examples
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
)

// Example of how to use the streaming escrow analysis controller
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: streaming_example <xml_file>")
	}

	xmlFile := os.Args[1]

	// Create the streaming controller
	streamingController, err := escrow.NewStreamingEscrowAnalysisController(xmlFile)
	if err != nil {
		log.Fatalf("Failed to create streaming controller: %v", err)
	}

	// Perform streaming analysis (single pass through the XML file)
	fmt.Println("🚀 Starting optimized single-pass XML analysis...")
	
	if err := streamingController.AnalyzeStreaming(true); err != nil {
		log.Fatalf("Streaming analysis failed: %v", err)
	}

	fmt.Println("✅ Streaming analysis completed successfully!")
}
