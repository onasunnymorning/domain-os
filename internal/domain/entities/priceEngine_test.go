package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPriceEngine(t *testing.T) {
	phase := Phase{Name: "GA"}
	domain := Domain{Name: "example.com"}
	fx := FX{}
	pl := []*PremiumLabel{}

	pe := NewPriceEngine(phase, domain, fx, pl)
	require.NotNil(t, pe, "PriceEngine is nil")
}
