package main

import (
	"flag"
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
)

func main() {
	// FLAGS
	filename := flag.String("f", "", "(path to) filename")
	flag.Parse()

	if *filename == "" {
		log.Fatal("Please provide a filename")
	}

	escrowService, err := services.NewXMLEscrowService(*filename)
	if err != nil {
		log.Fatal(err)
	}

	escrowController := escrow.NewEscrowAnalysisController(escrowService)

	err = escrowController.Analyze()
	if err != nil {
		log.Fatal(err)
	}

}
