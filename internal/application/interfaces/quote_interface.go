package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// QuoteService is the interface for the quote service
type QuoteService interface {
	// GetQuote returns quotes for transactions
	GetQuote(ctx context.Context, q *queries.QuoteRequest) (*entities.Quote, error)
}
