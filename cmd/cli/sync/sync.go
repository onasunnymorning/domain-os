package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
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
				Name:        "spec5",
				Aliases:     []string{"spec"},
				Usage:       "sync spec5 data",
				Description: "pulls the XML spec5 provided by ICANN and replaces our local copy in the database",
				Action:      notImplemented,
			},
			{
				Name:        "registrars",
				Aliases:     []string{"rar"},
				Usage:       "sync registrar status with the IANA repository",
				Description: "pulls the iana registrar repository and ensures that our registrar status is up to date",
				Action:      notImplemented,
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

func syncRegistrars(c *cli.Context) error {
	correlationID := "cli-sync-registrars-" + time.Now().Format("20060102150405")
	log.Println("Correlation ID:", correlationID)

	// Get a count of the registrars in the system
	count, err := activities.CountRegistrars(correlationID)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// if count = 0 exit
	if count.Count == 0 {
		return cli.Exit("no registrars found in the system, consider importing", 1)
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

	// Get the registrars currently in the platform
	fmt.Println(ianaRars)
	// Compare the two lists and update the platform as necessary

	return nil
}
