package interfaces

// XMLEscrowAnalysisService is an interface for escrow service
type XMLEscrowAnalysisService interface {
	AnalyzeDepostTag() error
	AnalyzeHeaderTag() error
	AnalyzeRegistrarTags() error
	GetDepositJSON() string
	GetHeaderJSON() string
}
