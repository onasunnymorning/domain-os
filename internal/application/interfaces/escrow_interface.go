package interfaces

// XMLEscrowAnalysisService is an interface for escrow service
type XMLEscrowAnalysisService interface {
	AnalyzeDepostTag() error
	GetDepositJSON() string
	AnalyzeHeaderTag() error
}
