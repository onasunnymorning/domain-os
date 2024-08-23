package main

import (
	"flag"
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
)

// This script assumes you have run the escrowAnalyzer and have a files to import

func main() {

	// FLAGS
	filename := flag.String("f", "", "(path to) the XML escrow filename")
	analysisFile := flag.String("a", "", "(path to) the analysis file produced by the escrow analyzer")
	flag.Parse()
	// Flag validation
	if *filename == "" || *analysisFile == "" {
		log.Fatal("Please provide a filename for the escrow and the analysis file produced by the escrow analyzer")
	}

	// Set up the escrow service
	escrowService, err := services.NewXMLEscrowService(*filename)
	if err != nil {
		log.Fatal(err)
	}
	// Create the controller
	importController := escrow.NewEscrowImportController(escrowService)

	// Import the data
	err = importController.Import(*analysisFile, *filename)
	if err != nil {
		log.Fatal(err)
	}

}
