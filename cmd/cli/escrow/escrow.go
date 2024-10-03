package main

import (
	"log"
	"os"
	"runtime"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
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
				Name:    "analyze",
				Aliases: []string{"an"},
				Usage:   "analyze an RDE escrow deposit file (XML)",
				Action:  analyzeDeposit,
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
				Usage:   "export all relevant data from the Database to CSV files that can be used to build an escrow later",
				Action:  generateDeposit,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "output",
						Aliases:  []string{"o"},
						Usage:    "path where the output files will go",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "format",
						Aliases:  []string{"f"},
						Usage:    "format to write the output files in (CSV/XML)",
						Required: true,
					},
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
	// Wait a bit to ensure the goroutine exits
	time.Sleep(200 * time.Millisecond)
	// Report the maximum memory usage
	log.Printf("[DEBUG] Maximum memory usage: %d Mbytes\n", maxAlloc/1024/1024)
}

func analyzeDeposit(c *cli.Context) error {
	log.Println("Analyze command")
	return nil
}

func importDeposit(c *cli.Context) error {
	log.Println("Import command")
	return nil
}

func generateDeposit(c *cli.Context) error {
	log.Println("Export command")
	return nil
}
