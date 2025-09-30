// Example modification to cmd/cli/escrow/escrow.go to support streaming analysis

// Add a new flag for streaming analysis
func getAnalyzeCommand() *cli.Command {
	return &cli.Command{
		Name:  "analyze",
		Usage: "Analyze an XML escrow deposit file",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "map-registrars",
				Usage: "Map registrars from the API",
			},
			&cli.BoolFlag{
				Name:  "streaming",
				Usage: "Use optimized single-pass streaming analysis (recommended for large files)",
			},
		},
		Action: analyzeDeposit,
	}
}

// Modified analyzeDeposit function to support both approaches
func analyzeDeposit(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("please provide a filename")
	}

	filename := c.Args().First()
	mapRegistrars := c.Bool("map-registrars")
	useStreaming := c.Bool("streaming")

	if useStreaming {
		// Use the optimized streaming approach
		streamingController, err := escrow.NewStreamingEscrowAnalysisController(filename)
		if err != nil {
			return err
		}

		return streamingController.AnalyzeStreaming(mapRegistrars)
	} else {
		// Use the original approach
		escrowService, err := services.NewXMLEscrowService(filename)
		if err != nil {
			return err
		}

		escrowController := escrow.NewEscrowAnalysisController(escrowService)
		return escrowController.Analyze(mapRegistrars)
	}
}

// Usage examples:
// ./domain-os escrow analyze large-file.xml --streaming
// ./domain-os escrow analyze large-file.xml --map-registrars --streaming
