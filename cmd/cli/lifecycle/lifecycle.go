package main

// This CLI tool allows you to run domain lifecycle operations.

import (
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
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
