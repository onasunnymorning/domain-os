package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
	"github.com/urfave/cli/v2"
)

const APP_VERSION = "0.1.0"

func main() {
	start := time.Now()
	// Keep track of memory usage
	// Channel to signal the monitoring goroutine to stop
	done := make(chan struct{})
	var maxAlloc uint64
	// Start a goroutine to monitor memory usage
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var m runtime.MemStats
		for {
			select {
			case <-ticker.C:
				runtime.ReadMemStats(&m)
				if m.Alloc > maxAlloc {
					maxAlloc = m.Alloc
				}
			case <-done:
				return
			}
		}
	}()

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "version",
				Aliases: []string{"v", "ver"},
				Usage:   "Print the version of this escrow tool",
				Action:  printVersion,
			},
			{
				Name:    "analyze",
				Aliases: []string{"an"},
				Usage:   "analyze an RDE escrow deposit file (XML)",
				Action:  analyzeDeposit,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:     "map-registrars",
						Aliases:  []string{"map", "m"},
						Usage:    "try to map registrar IDs to the target system",
						Value:    false,
						Required: false,
					},
				},
			},
			{
				Name:    "import",
				Aliases: []string{"imp"},
				Usage:   "import an RDE escrow deposit file (XML)",
				Action:  importDeposit,
			},
			{
				Name:    "generate",
				Aliases: []string{"gen"},
				Usage:   "export all relevant data from the Database and create an XML escrow deposit file",
				Action:  generateDeposit,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:        "indent",
						Aliases:     []string{"i"},
						Usage:       "amount of indentation to use in the output file",
						Required:    false,
						Value:       0,
						DefaultText: "0 - no indentation - single line of XML",
					},
					&cli.IntFlag{
						Name:        "concurrency",
						Aliases:     []string{"c"},
						Usage:       "amount of goroutines to use for concurrent processing",
						Required:    false,
						Value:       10,
						DefaultText: "10",
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

	// Signal the monitoring goroutine to stop
	close(done)
	// Report the time taken
	log.Printf("[DEBUG] Time taken: %s\n", time.Since(start))
	// Wait a bit to ensure the goroutine exits
	time.Sleep(200 * time.Millisecond)
	// Report the maximum memory usage
	log.Printf("[DEBUG] Maximum memory usage: %d Mbytes\n", maxAlloc/1024/1024)
}

func analyzeDeposit(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("please provide a filename")
	}

	escrowService, err := services.NewXMLEscrowService(c.Args().First())
	if err != nil {
		return err
	}

	escrowController := escrow.NewEscrowAnalysisController(escrowService)

	err = escrowController.Analyze(c.Bool("map-registrars"))
	if err != nil {
		return err
	}

	return nil
}

func importDeposit(c *cli.Context) error {
	filename := c.Args().First()
	if filename == "" {
		return errors.New("please provide a filename")
	}

	// Set up the escrow service
	escrowService, err := services.NewXMLEscrowService(filename)
	if err != nil {
		log.Fatal(err)
	}
	// Create the controller
	importController := escrow.NewEscrowImportController(escrowService)

	// Import the data
	err = importController.Import(strings.TrimSuffix(filename, ".xml")+"-analysis.json", filename)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func generateDeposit(c *cli.Context) error {
	log.Println("Generate command - not implemented")
	return nil
}

func printVersion(c *cli.Context) error {
	fmt.Printf("Version %s\n", APP_VERSION)
	return nil
}
