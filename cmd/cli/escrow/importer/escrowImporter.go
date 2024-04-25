package main

import (
	"flag"
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
)

// This script assumes you have run the escrowAnalyzer and have a files to import

func main() {
	// FLAGS
	dirName := flag.String("d", "", "(path to) directory containing escrow analysis files")
	flag.Parse()

	if *dirName == "" {
		log.Fatal("Please provide a directory containing escrow analysis files")
	}

	escrowService, err := services.NewXMLEscrowService(*dirName)
	if err != nil {
		log.Fatal(err)
	}

	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		panic(err)
	}
	roidService := services.NewRoidService(idGenerator)

	importController := escrow.NewEscrowImportController(escrowService, roidService)

	err = importController.Import()
	if err != nil {
		log.Fatal(err)
	}

}
