package main

// This CLI tool allows you to run domain lifecycle operations.

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/schedules"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
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
		Name:  "lifecycle",
		Usage: "execute domain lifecycle operations",
		Commands: []*cli.Command{
			{
				Name:    "purge",
				Aliases: []string{"p", "pu"},
				Usage:   "purge pendingDelete domains past the grace period",
				Action:  purge,
			},
			{
				Name:    "expire",
				Aliases: []string{"exp", "ex"},
				Usage:   "process expired domains",
				Action:  expire,
			},
			{
				Name:      "schedule",
				Aliases:   []string{"s", "sch"},
				Usage:     "manage temporal schedules for domain lifecycle operations",
				UsageText: "Use this command to created/delete temporal schedules for domain lifecycle operations",
				Subcommands: []*cli.Command{
					{
						Name:    "create",
						Aliases: []string{"c", "cr"},
						Usage:   "create temporal schedules (expiry and purge)",
						Action:  createTemporalSchedules,
					},
					{
						Name:    "delete",
						Aliases: []string{"d", "del"},
						Usage:   "delete temporal schedules (expiry and purge)",
						Action:  deleteTemporalSchedules,
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

func expire(c *cli.Context) error {
	// Query the API for the amount of expired domains
	log.Println("Querying expired domain count...")
	countResult, err := activities.GetExpiredDomainCount(queries.ExpiringDomainsQuery{})
	if err != nil {
		return err
	}

	log.Println("Found", countResult.Count, "expired domains")

	// If there are no expired domains, exit
	if countResult.Count == 0 {
		log.Println("No expired domains to process, nothing to do")
		os.Exit(0)
	}

	// Query the API for a batch of 25 expired domains
	q, err := queries.NewExpiringDomainsQuery("", "", "")
	if err != nil {
		return err
	}
	log.Println("Querying expired domains...")
	domains, err := activities.ListExpiringDomains(*q)
	if err != nil {
		return err
	}
	log.Println("Found", len(domains), "expired domains")

	// Process the batch of expired domains
	log.Println("Processing expired domains...")
	for _, domain := range domains {
		// Try and auto-renew the domain
		err := activities.AutoRenewDomain(domain.Name)
		if err != nil {
			// If the domain is not eligible for auto-renew, it should be marked for deletion
			if strings.Contains(err.Error(), "auto renew is not enabled") {
				log.Println("Domain", domain.Name, "is not eligible for auto-renew, marking for deletion")
				err := activities.MarkDomainForDeletion(domain.Name)
				if err != nil {
					log.Printf("Failed to mark domain %s for deletion: %s\n", domain.Name, err)
					continue
				}
				log.Println("Domain", domain.Name, "marked for deletion")
				continue
			}
			// If another error occurred, log it and continue
			log.Println("Failed to auto-renew domain", domain.Name, ":", err)
		}
		log.Println("Domain", domain.Name, "auto-renewed")
	}

	return nil
}

func purge(c *cli.Context) error {

	// Query the API for the amount of purgeable domains
	log.Println("Querying purgeable domain count...")
	countResult, err := activities.GetPurgeableDomainCount(queries.PurgeableDomainsQuery{})
	if err != nil {
		return err
	}
	log.Println("Found", countResult.Count, "purgeable domains")

	// If there are no purgeable domains, exit
	if countResult.Count == 0 {
		log.Println("No purgeable domains to process, nothing to do")
		os.Exit(0)
	}

	// Query the API for a batch of 25 purgeable domains
	q, err := queries.NewPurgeableDomainsQuery("", "", "")
	if err != nil {
		return err
	}
	log.Println("Querying purgeable domains...")

	domains, err := activities.ListPurgeableDomains(*q)
	if err != nil {
		return err
	}
	log.Println("Found", len(domains), "purgeable domains")

	// Process the batch of purgeable domains
	log.Println("Processing purgeable domains...")
	for _, domain := range domains {
		// Premanently delete the domain
		err := activities.PurgeDomain(domain.Name)
		if err != nil {
			log.Printf("Failed to delete domain %s: %s\n", domain.Name, err)
			continue
		}
		log.Println("Domain", domain.Name, "Purged")
	}

	return nil
}

// createTemporalExpirySchedule automates the creation of a temporal schedule as defined in schedules.CreateExpiryScheduleHourly. Use this to set up the schedules when deploying an instance of the application. Note that the environment variables must be set for this to work and there is no facility yet to updated/delete schedules. Use the temporal web UI to manage schedules.
func createTemporalExpirySchedule(cfg *temporal.TemporalClientconfig) error {
	// Create the schedule
	scheduleID, err := schedules.CreateExpiryScheduleHourly(*cfg)
	if err != nil {
		return err
	}

	log.Println("Created schedule with ID:", scheduleID)

	return nil
}

// createTemporalPurgeSchedule automates the creation of a temporal schedule as defined in schedules.CreatePurgeScheduleHourly. Use this to set up the schedules when deploying an instance of the application. Note that the environment variables must be set for this to work and there is no facility yet to updated/delete schedules. Use the temporal web UI to manage schedules.
func createTemporalPurgeSchedule(cfg *temporal.TemporalClientconfig) error {
	// Create the schedule
	scheduleID, err := schedules.CreatePurgeScheduleHourly(*cfg)
	if err != nil {
		return err
	}

	log.Println("Created schedule with ID:", scheduleID)

	return nil
}

// createTemporalUpdateFXSchedule automates the creation of a temporal schedule as defined in schedules.CreateUpdateFXScheduleDaily. Use this to set up the schedules when deploying an instance of the application. Note that the environment variables must be set for this to work and there is no facility yet to updated/delete schedules. Use the temporal web UI to manage schedules.
func createTemporalUpdateFXSchedule(cfg *temporal.TemporalClientconfig) error {
	// Create the schedule
	scheduleID, err := schedules.CreateUpdateFXScheduleDaily(*cfg)
	if err != nil {
		return err
	}

	log.Println("Created schedule with ID:", scheduleID)

	return nil
}

// createTemporalSchedules is a CLI command that creates a temporal schedule for domain lifecycle operations. It takes a single argument, either 'expiry' or 'purge', to specify the type of schedule to create.
func createTemporalSchedules(c *cli.Context) error {
	// Check if the first argument is a valid schedule (expiry or purge)
	if c.Args().First() != "expiry" && c.Args().First() != "purge" && c.Args().First() != "updatefx" {
		log.Println("Invalid schedule type. Must be 'expiry' or 'purge'")
		return cli.ShowCommandHelp(c, "create")
	}

	// Create a temporal client config
	cfg := &temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: os.Getenv("TMPIO_QUEUE"),
	}

	switch c.Args().First() {
	case "expiry":
		return createTemporalExpirySchedule(cfg)
	case "purge":
		return createTemporalPurgeSchedule(cfg)
	case "updatefx":
		return createTemporalUpdateFXSchedule(cfg)
	}

	return errors.New("invalid schedule type")
}

func deleteTemporalSchedules(c *cli.Context) error {
	fmt.Println("Not implemented")
	return nil
}
