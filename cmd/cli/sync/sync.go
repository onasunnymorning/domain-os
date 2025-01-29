package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
				Action:      syncRegistrars,
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

	ianaRars, err := activities.GetIANARegistrars(correlationID, baseURL, bearerToken, 100)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Get the registrars currently in the platform
	log.Println("Getting existing Registrars...")
	rars, err := activities.GetRegistrarListItems(correlationID, baseURL, bearerToken)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Compare the two lists and update the platform as necessary
	log.Println("Comparing IANA registrars with platform registrars...")
	for _, ianaRar := range ianaRars {
		// Create a ClID for the IANA registrar using our naming convention
		clid, _ := ianaRar.CreateClID()
		found := false
		for _, rar := range rars {
			if clid == rar.ClID {
				// Found the registrar
				found = true
				// compare statuses
				cmd := commands.CompareIANARegistrarStatusWithRarStatus(ianaRar, rar)
				if cmd != nil {
					log.Printf("Updating registrar %s status from %s to %s\n", cmd.ClID, cmd.OldStatus, cmd.NewStatus)

					// update the registrar status
					err := activities.SetRegistrarStatus(correlationID, cmd.ClID, cmd.NewStatus)
					if err != nil {
						return cli.Exit(err, 1)
					}
				}

				// Only one match is expected
				break
			}
		}

		if !found {
			if strings.EqualFold(ianaRar.Status.String(), string(entities.IANARegistrarStatusReserved)) {
				log.Printf("found new IANARegistrar: %s, but it is reserved, skipping\n", clid)
				continue
			}

			log.Printf("found new IANARegistrar: %s, creating it\n", clid)

			// Create our Create command
			cmd, err := commands.CreateCreateRegistrarCommandFromIANARegistrar(ianaRar)
			if err != nil {
				return cli.Exit(err, 1)
			}

			// create the registrar
			_, err = activities.CreateRegistrar(correlationID, *cmd)
			if err != nil {
				return cli.Exit(err, 1)
			}

		}
	}

	return nil
}
