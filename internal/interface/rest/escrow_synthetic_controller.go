package rest

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/rdesynth"
)

// SyntheticSeedHeader carries the seed a synthetic deposit was generated from,
// so a download made without one can be reproduced.
const SyntheticSeedHeader = "X-Synthetic-Seed"

// generateSyntheticDepositRequest shapes POST /escrow/synthetic. The bounds
// are rdesynth's, enforced by rdesynth.Params.Validate.
type generateSyntheticDepositRequest struct {
	TLD               string  `json:"tld" binding:"required"`
	Domains           int     `json:"domains"`
	ContactsPerDomain int     `json:"contactsPerDomain"`
	AvgHostsPerDomain float64 `json:"avgHostsPerDomain"`
	NNDNs             int     `json:"nndns"`
	// Seed makes the output reproducible. Omitted, the server picks one and
	// returns it in X-Synthetic-Seed.
	Seed *uint64 `json:"seed,omitempty"`
}

// GenerateSynthetic streams a synthetic FULL RDE deposit as .xml.gz.
//
// @Summary Generate a synthetic RDE deposit (.xml.gz)
// @Description Builds a made-up FULL deposit for the given TLD — registrars, domains with unique contacts and subordinate hosts, NNDNs — that the escrow validator accepts without findings, and streams it as a gzip download. Nothing in it is real data.
// @Tags Escrow
// @Accept json
// @Produce application/gzip
// @Param body body generateSyntheticDepositRequest true "Deposit shape"
// @Success 200 {file} file "The deposit, gzip-compressed XML"
// @Failure 400 {object} map[string]string
// @Router /escrow/synthetic [post]
func (c *EscrowController) GenerateSynthetic(ctx *gin.Context) {
	var req generateSyntheticDepositRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld is required"})
		return
	}
	// Below 2^53 so the seed survives a round trip through a JavaScript number:
	// the UI shows it, and pasting it back has to reproduce the same deposit.
	seed := rand.Uint64N(1 << 53) // #nosec G404 -- a seed for synthetic test data, not a secret
	if req.Seed != nil {
		seed = *req.Seed
	}
	p := rdesynth.Params{
		TLD:               req.TLD,
		Domains:           req.Domains,
		ContactsPerDomain: req.ContactsPerDomain,
		AvgHostsPerDomain: req.AvgHostsPerDomain,
		NNDNs:             req.NNDNs,
		Seed:              seed,
	}
	if err := p.Validate(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-Type", "application/gzip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", p.Filename()))
	ctx.Header(SyntheticSeedHeader, strconv.FormatUint(seed, 10))
	ctx.Status(http.StatusOK)

	// The status line is gone once the first byte is: a failure from here on
	// — almost always the client hanging up — can only be logged, and the
	// truncated gzip stream is what tells the client it did not get a deposit.
	if err := rdesynth.Generate(ctx.Request.Context(), ctx.Writer, p); err != nil {
		slog.WarnContext(ctx.Request.Context(), "escrow: synthetic deposit generation stopped",
			slog.String("tld", p.TLD), slog.Int("domains", p.Domains), slog.Any("error", err))
	}
}
