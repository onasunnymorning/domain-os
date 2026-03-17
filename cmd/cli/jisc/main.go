package main

import (
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/interface/cli/jisc"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "jisc",
		Usage: "JISC Domain Analysis Tool",
		Commands: []*cli.Command{
			jisc.GetAnalyzeCommand(),
			jisc.GetGenerateDBCommand(),
			jisc.GetImportCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
