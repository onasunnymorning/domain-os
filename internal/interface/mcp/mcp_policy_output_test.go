package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestMapPhase_ZeroDurationsAreKept: 0 is a setting (ADR 0023), so a
// zero-day period is serialised, not omitted.
func TestMapPhase_ZeroDurationsAreKept(t *testing.T) {
	zero := 0
	p := &entities.Phase{Name: "ote", Type: entities.PhaseTypeGA, LifecyclePolicyName: "ote"}
	p.LifecyclePolicy = &entities.LifecyclePolicy{Name: "ote", Settings: entities.LifecycleSettings{
		RegistrationGP: &zero, RedemptionGP: &zero, TransferLockPeriod: &zero, PendingDeleteGP: &zero,
	}}
	b, err := json.Marshal(mapPhase(p))
	require.NoError(t, err)
	for _, want := range []string{`"registrationGP":0`, `"redemptionGP":0`, `"transferLockPeriod":0`, `"pendingDeleteGP":0`, `"lifecyclePolicy":"ote"`, `"maxHorizon":10`} {
		assert.Contains(t, string(b), want)
	}
}
