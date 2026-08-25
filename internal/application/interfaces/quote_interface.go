package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// QuoteService is the interface for the quote service
type QuoteService interface {
	// GetQuote returns quotes for transactions
	GetQuote(ctx context.Context, q *queries.QuoteRequest) (*entities.Quote, error)
}
