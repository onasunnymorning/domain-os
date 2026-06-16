package jisc

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/urfave/cli/v2"
)

// JiscController handles JISC CLI commands
type JiscController struct {
	svc *services.JiscService
}

// NewJiscController creates a new JiscController
func NewJiscController() *JiscController {
	return &JiscController{
		svc: services.NewJiscService(),
	}
}

// Analyze handles the analyze command
func (c *JiscController) Analyze(ctx *cli.Context) error {
	jsonFile := ctx.Args().First()
	if jsonFile == "" {
		return cli.Exit("Error: JSON file argument is required", 1)
	}

	outputDB := ctx.String("output")

	log.Printf("Starting analysis for: %s", jsonFile)
	if err := c.svc.Analyze(jsonFile, outputDB); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

// GetAnalyzeCommand returns the CLI command definition
func GetAnalyzeCommand() *cli.Command {
	ctrl := NewJiscController()

	return &cli.Command{
		Name:      "analyze",
		Aliases:   []string{"a"},
		Usage:     "Analyze a JISC JSON domain export",
		ArgsUsage: "[json-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Path to output SQLite database (optional, defaults to in-memory)",
			},
		},
		Action: ctrl.Analyze,
	}
}

// GenerateDB handles the database generation command
func (c *JiscController) GenerateDB(ctx *cli.Context) error {
	jsonFile := ctx.Args().First()
	if jsonFile == "" {
		return cli.Exit("Error: JSON file argument is required", 1)
	}

	if err := c.svc.GenerateEscrowDB(jsonFile); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

// GetGenerateDBCommand returns the CLI command definition
func GetGenerateDBCommand() *cli.Command {
	ctrl := NewJiscController()

	return &cli.Command{
		Name:      "generatedb",
		Aliases:   []string{"gen"},
		Usage:     "Generate a standard escrow SQLite DB from JISC export",
		ArgsUsage: "[json-file]",
		Action:    ctrl.GenerateDB,
	}
}

// Import handles the import command
func (c *JiscController) Import(ctx *cli.Context) error {
	jsonFile := ctx.Args().First()
	if jsonFile == "" {
		return cli.Exit("Error: JSON file argument is required", 1)
	}

	// We rely on standard PG env vars for connection, which NewDirectDBImporter uses.
	// DB_HOST, DB_USER, DB_PASS, DB_NAME, DB_PORT
	// Ensure these are set or user is aware.

	log.Printf("Starting Direct DB Import from %s...", jsonFile)
	if err := c.svc.ImportToDirectDB(jsonFile); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

// GetImportCommand returns the CLI command definition
func GetImportCommand() *cli.Command {
	ctrl := NewJiscController()

	return &cli.Command{
		Name:      "import",
		Aliases:   []string{"imp"},
		Usage:     "Import JISC JSON data directly to Postgres (via SQLite escrow stage)",
		ArgsUsage: "[json-file]",
		Flags:     []cli.Flag{
			// No flags needed, strictly uses ENV for DB connection
		},
		Action: ctrl.Import,
	}
}
