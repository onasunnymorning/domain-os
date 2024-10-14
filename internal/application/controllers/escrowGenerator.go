package controllers

type EscrowGenerator struct {
	Params EscrowGeneratorParams
}

// NewEscrowGenerator creates a new instance of EscrowGenerator
func NewEscrowGenerator(params EscrowGeneratorParams) *EscrowGenerator {
	return &EscrowGenerator{
		Params: params,
	}
}

// EscrowGeneratorParams is a struct to hold the parameters for the escrow generator
type EscrowGeneratorParams struct {
	Tld            string
	OutputFile     string
	BatchSize      int
	MaxConcurrency int
}

// Generate creates an escrow deposit file
func (c *EscrowGenerator) Generate() error {

	return nil
}
