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
func TestSetQuoteParams(t *testing.T) {
	phase := Phase{Name: "GA"}
	domain := Domain{Name: "example.com"}
	fx := FX{}
	pl := []*PremiumLabel{}
	pe := PriceEngine{
		Phase:          phase,
		PremiumEntries: pl,
		FXRate:         fx,
		Domain:         domain,
	}
	q := &Quote{}
	pe.setQuoteParams()
	require.Equal(t, &fx, q.FXRate, "FXRate is not set correctly")
	require.Equal(t, domain.Name, q.DomainName, "DomainName is not set correctly")
	require.Equal(t, &phase, q.Phase, "Phase is not set correctly")
}
