package main

import (
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/jisc"
	"github.com/urfave/cli/v2"
)

func main() {
	// The importer mints ROIDs on the process's node.
	if id, set, err := config.LoadSnowflakeNodeID(); err != nil {
		log.Fatal(err)
	} else if set {
		if err := snowflakeidgenerator.SetNodeID(id); err != nil {
			log.Fatal(err)
		}
	}
	app := &cli.App{
		Name:  "jisc",
		Usage: "JISC Domain Analysis Tool",
		Commands: []*cli.Command{
			jisc.GetAnalyzeCommand(),
			jisc.GetGenerateDBCommand(),
			jisc.GetImportCommand(config.LoadRepositoryID, config.LoadDatabase),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
