package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDomainRGPStatus_IsNil(t *testing.T) {
	rgp := DomainRGPStatus{}
	require.True(t, rgp.IsNil())

	rgp = DomainRGPStatus{
		AddPeriodEnd: time.Now().UTC(),
	}
	require.False(t, rgp.IsNil())
}
