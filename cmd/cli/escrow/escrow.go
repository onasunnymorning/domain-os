package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/cli/escrow"
	"github.com/urfave/cli/v2"
)

const APP_VERSION = "0.3.0"

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
						Name:  "streaming",
						Usage: "Use optimized single-pass streaming analysis (recommended for large files)",
					},
				},
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
			escrow.CreateConvertToSQLiteCommand(),
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

// Modified analyzeDeposit function to support both approaches
func analyzeDeposit(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("please provide a filename")
	}

	filename := c.Args().First()
	useStreaming := c.Bool("streaming")

	if useStreaming {
		// Use the optimized streaming approach
		streamingController, err := escrow.NewStreamingEscrowAnalysisController(filename)
		if err != nil {
			return err
		}

		return streamingController.AnalyzeStreaming()
	} else {
		// Use the original approach
		escrowService, err := services.NewXMLEscrowService(filename)
		if err != nil {
			return err
		}

		escrowController := escrow.NewEscrowAnalysisController(escrowService)
		return escrowController.Analyze()
	}
}

func generateDeposit(c *cli.Context) error {
	log.Println("Generate command - not implemented")
	return nil
}

func printVersion(c *cli.Context) error {
	fmt.Printf("Version %s\n", APP_VERSION)
	return nil
}
