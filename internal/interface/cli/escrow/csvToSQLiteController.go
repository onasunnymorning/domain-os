package escrow

import (
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/urfave/cli/v2"
)

// ConvertToSQLiteController handles the conversion of CSV files to SQLite database
type ConvertToSQLiteController struct {
}

// NewConvertToSQLiteController creates a new controller for CSV to SQLite conversion
func NewConvertToSQLiteController() *ConvertToSQLiteController {
	return &ConvertToSQLiteController{}
}

// ConvertCSVsToSQLite converts escrow CSV files to SQLite database
func (ctrl *ConvertToSQLiteController) ConvertCSVsToSQLite(baseFilename, outputDB string) error {
	log.Printf("Starting CSV to SQLite conversion...")
	log.Printf("Base filename: %s", baseFilename)
	log.Printf("Output database: %s", outputDB)

	// Create the CSV to SQLite service
	service := services.NewCSVToSQLiteService(baseFilename)

	// Convert CSV files to SQLite
	if err := service.ConvertToSQLite(outputDB); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	log.Printf("✅ Conversion completed successfully!")
	log.Printf("SQLite database created: %s", outputDB)

	// Show some basic stats
	if err := ctrl.showDatabaseStats(outputDB); err != nil {
		log.Printf("⚠️  Could not retrieve database stats: %v", err)
	}

	return nil
}

// showDatabaseStats displays basic statistics about the created database
func (ctrl *ConvertToSQLiteController) showDatabaseStats(dbPath string) error {
	db, err := services.OpenEscrowDatabase(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	stats, err := db.GetDatabaseStats()
	if err != nil {
		return err
	}

	log.Printf("📊 Database Statistics:")
	for table, count := range stats {
		log.Printf("   %s: %d records", table, count)
	}
	log.Printf("💡 Database ready for fast domain/host lookups!")

	return nil
}

// CreateConvertToSQLiteCommand creates the urfave/cli command for CSV to SQLite conversion
func CreateConvertToSQLiteCommand() *cli.Command {
	ctrl := NewConvertToSQLiteController()

	return &cli.Command{
		Name:    "csv-to-sqlite",
		Aliases: []string{"csv2db", "c2s"},
		Usage:   "Convert escrow CSV files to SQLite database",
		Description: `Convert escrow CSV files to SQLite database for fast queries.

This command takes the base filename of your escrow CSV files and creates
an SQLite database with optimized schema and indexes for fast domain/host lookups.

Example:
  escrow csv-to-sqlite co_2025-09-29_full_S1_R0
  
This will read:
  - co_2025-09-29_full_S1_R0-domains.csv
  - co_2025-09-29_full_S1_R0-hosts.csv  
  - co_2025-09-29_full_S1_R0-domainNameservers.csv
  - etc.
  
And create: co_2025-09-29_full_S1_R0.db`,
		ArgsUsage: "[base-filename]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output SQLite database file (default: [base-filename].db)",
			},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() != 1 {
				return fmt.Errorf("exactly one argument required: base filename")
			}

			baseFilename := ctx.Args().Get(0)

			// Generate output database name
			outputDB := ctx.String("output")

			// If no output specified, use base filename + .db
			if outputDB == "" {
				outputDB = baseFilename + ".db"
			}

			return ctrl.ConvertCSVsToSQLite(baseFilename, outputDB)
		},
	}
}
