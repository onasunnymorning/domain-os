package jisc

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/urfave/cli/v2"
)

// JiscController handles JISC CLI commands
type JiscController struct {
	svc *services.JiscService
	// repositoryID loads the repository ID the import mints ROIDs with, and
	// database the database to import into and the process's connection
	// budget. Both are called only by the import, so analysis does not need
	// them set.
	repositoryID func() (entities.RepositoryID, error)
	database     func() (config.Database, error)
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

	repositoryID, err := c.repositoryID()
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	db, err := c.database()
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	// The importer's pool is this process's only one, so the budget bounds
	// it, as does the importer's own ceiling.
	pool, err := services.OpenImportPool(db.DSN(), min(services.DefaultImportConns, db.MaxOpenConns))
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	defer pool.Close()

	log.Printf("Starting Direct DB Import from %s...", jsonFile)
	if err := c.svc.ImportToDirectDB(jsonFile, repositoryID, pool); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

// GetImportCommand returns the CLI command definition. repositoryID and
// database are config.LoadRepositoryID and config.LoadDatabase, injected
// from the composition root (INV-17).
func GetImportCommand(repositoryID func() (entities.RepositoryID, error), database func() (config.Database, error)) *cli.Command {
	ctrl := NewJiscController()
	ctrl.repositoryID = repositoryID
	ctrl.database = database

	return &cli.Command{
		Name:      "import",
		Aliases:   []string{"imp"},
		Usage:     "Import JISC JSON data directly to Postgres (via SQLite escrow stage)",
		ArgsUsage: "[json-file]",
		Flags:     []cli.Flag{
			// No flags: the database comes from the environment, loaded at start
		},
		Action: ctrl.Import,
	}
}
