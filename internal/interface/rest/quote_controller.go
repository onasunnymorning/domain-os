package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
)

// QuoteController is the controller for
type QuoteController struct {
	QuoteService interfaces.QuoteService
}

// NewQuoteController returns a new QuoteController
func NewQuoteController(e *gin.Engine, quoteService interfaces.QuoteService, handler gin.HandlerFunc) *QuoteController {
	ctrl := &QuoteController{
		QuoteService: quoteService,
	}

	e.POST("/quotes", handler, ctrl.GetQuote)

	return ctrl
}

// GetQuote godoc
// @Summary returns a quote for a transaction
// @Description Takes a QuoteRequest and returns a Quote for the transaction including a breakdown of costs
// @ID get-quote
// @Tags Quotes
// @Accept  json
// @Produce  json
// @Param quoteRequest body queries.QuoteRequest true "QuoteRequest"
// @Success 200 {object} entities.Quote
// @Failure 400
// @Router /quotes [post]
func (ctrl *QuoteController) GetQuote(ctx *gin.Context) {
	var qr queries.QuoteRequest
	if err := ctx.ShouldBindJSON(&qr); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	quote, err := ctrl.QuoteService.GetQuote(ctx, &qr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, quote)
}
