package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/schedules"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

const (
	ScheduleTypeSyncRegistrars = "sync-registrars"
)

var (
	supportedScheduleTypes = []string{
		ScheduleTypeSyncRegistrars,
	}
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
					&cli.IntFlag{
						Name:    "chunksize",
						Aliases: []string{"c"},
						Usage:   "value between 1 and 100 - the number of registrars to create in a single batch",
						Value:   100,
					},
				},
			},
			{
				Name:      "schedule",
				Aliases:   []string{"s", "sch"},
				Usage:     "import schedules {schedulename}",
				UsageText: "Use this command to import (create) temporal schedules",
				Action:    importSchedule,
				// TODO: port this over from the lifecycle cli tool
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
	// check for a chunk size
	if c.Int("chunksize") < 1 || c.Int("chunksize") > 1000 {
		return cli.Exit("[ERROR] Chunk size should be between 1 and 1000", 1)
	}

	correlationID := "cli-import-registrars-" + time.Now().Format("20060102150405")
	log.Println("[INFO] Correlation ID:", correlationID)
	// Get a count of the registrars in the system
	count, err := activities.CountRegistrars(correlationID)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// if count > 0 exit
	if count.Count > 0 {
		return cli.Exit("[ERROR] Found at least one existing registrar, cannot continue", 1)
	}

	// Get the ICANN registrars
	log.Printf("[INFO] Getting ICANN registrars from file: %s\n", c.String("filename"))
	icannRars, err := activities.GetICANNRegistrars(correlationID, c.String("filename"))
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Sync our IANA registrar list
	log.Println("[INFO] Syncing IANA registrars...")
	syncErr := activities.SyncIanaRegistrars(correlationID)
	if syncErr != nil {
		return cli.Exit(err, 1)
	}

	// Get the IANA registrars
	log.Println("[INFO] Getting IANA registrars...")
	baseURL := fmt.Sprintf("http://%s:%s", os.Getenv("API_HOST"), os.Getenv("API_PORT"))
	bearerToken := fmt.Sprintf("Bearer %s", os.Getenv("ADMIN_TOKEN"))

	ianaRars, err := activities.GetIANARegistrars(correlationID, baseURL, bearerToken, 100)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Get the create commands
	log.Println("[INFO] Creating registrar CREATE commands...")
	createCommands, err := activities.MakeCreateRegistrarCommands(correlationID, icannRars, ianaRars)
	if err != nil {
		return cli.Exit(err, 1)
	}

	// Execute the create commands
	log.Println("[INFO] Creating registrars ... ")
	// create a progress bar
	pbar := progressbar.New(len(createCommands))
	// Process the commands in chunks of 100
	for chunk := range commands.ChunkCreateRegistrarCommands(createCommands, c.Int("chunksize")) {
		if err := activities.BulkCreateRegistrars(correlationID, chunk); err != nil {
			return cli.Exit(err, 1)
		}
		pbar.Add(len(chunk))
	}

	log.Println("")
	log.Printf("[INFO] %d Registrars imported successfully\n", len(createCommands))
	return nil
}

func importSchedule(c *cli.Context) error {
	// Validate input
	if !slices.Contains(supportedScheduleTypes, c.Args().First()) {
		return cli.Exit(fmt.Sprintf("[ERROR] Unsupported schedule type: %s. Currently supporting %v", c.Args().First(), supportedScheduleTypes), 1)
	}

	// Create the schedule
	scheduleID, err := schedules.CreateSyncRegistrarScheduleDaily(getTemporalClientConfig())
	if err != nil {
		return err
	}

	log.Println("Created schedule with ID:", scheduleID)

	return nil
}

func getTemporalClientConfig() temporal.TemporalClientconfig {
	// Create a temporal client config
	return temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: os.Getenv("TMPIO_QUEUE"),
	}
}
