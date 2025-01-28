package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

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
				Name:        "registrars",
				Aliases:     []string{"rar"},
				Usage:       "initially import registrars from IANA + ICANN data",
				Description: "use this to populate the data at first run, it will error if you are trying to import into a system that already has registrars populated",
				Action:      importRegistrars,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "filename",
						Aliases:  []string{"f"},
						Usage:    "the CSV file containing the ICANN registrar data downloaded from here: https://www.icann.org/en/accredited-registrars",
						Required: true,
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

func notImplemented(c *cli.Context) error {
	return cli.Exit("Not implemented", 1)
}

func importRegistrars(c *cli.Context) error {
	correlationID := "cli-import-registrars-" + time.Now().Format("20060102150405")
	log.Println("Correlation ID:", correlationID)
	// Get a count of the registrars in the system
	count, err := activities.CountRegistrars(correlationID)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// if count > 0 exit
	if count.Count > 0 {
		return cli.Exit("Found at least one existing registrar, cannot continue", 1)
	}

	// Get the ICANN registrars
	log.Printf("Getting ICANN registrars from file: %s\n", c.String("filename"))
	icannRars, err := activities.GetICANNRegistrars(correlationID, c.String("filename"))
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Sync our IANA registrar list
	log.Println("Syncing IANA registrars...")
	syncErr := activities.SyncIanaRegistrars(correlationID)
	if syncErr != nil {
		return cli.Exit(err, 1)
	}

	// Get the IANA registrars
	log.Println("Getting IANA registrars...")
	baseURL := fmt.Sprintf("http://%s:%s", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	bearerToken := fmt.Sprintf("Bearer %s", os.Getenv("API_TOKEN"))

	ianaRars, err := activities.GetIANARegistrars(correlationID, baseURL, bearerToken)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Get the create commands
	log.Println("Creating registrar CREATE commands...")
	createCommands, err := activities.MakeCreateRegistrarCommands(correlationID, icannRars, ianaRars)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Execute the create commands
	log.Println("Creating registrars ... ")
	// create a progress bar
	pbar := progressbar.New(len(createCommands))
	for _, cmd := range createCommands {
		_, err := activities.CreateRegistrar(correlationID, cmd)
		if err != nil {
			return cli.Exit(err, 1)
		}
		pbar.Add(1)
	}

	log.Printf("\n%d Registrars imported successfully\n", len(createCommands))
	return nil
}
