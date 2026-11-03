package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func TestLifecycleSettings_Resolve(t *testing.T) {
	assert.Equal(t, DefaultLifecycle(), LifecycleSettings{}.Resolve(), "nil is the default")
	r := LifecycleSettings{RegistrationGP: intPtr(0), TransferLockPeriod: intPtr(0), RestoreReportRequired: boolPtr(false), MaxHorizon: intPtr(2)}.Resolve()
	assert.Equal(t, 0, r.RegistrationGP, "0 is zero days")
	assert.Equal(t, 0, r.TransferLockPeriod)
	assert.False(t, r.RestoreReportRequired)
	assert.Equal(t, 2, r.MaxHorizon)
	assert.Equal(t, RedemptionGP, r.RedemptionGP, "the rest stays the default")
	assert.Equal(t, DefaultLifecycle(), DefaultLifecycle().Settings().Resolve(), "Settings round-trips")
}

func TestLifecycleSettings_Validate(t *testing.T) {
	assert.NoError(t, LifecycleSettings{}.Validate())
	assert.NoError(t, DefaultLifecycle().Settings().Validate())
	for name, s := range map[string]LifecycleSettings{
		"negative":               {RenewalGP: intPtr(-1)},
		"grace over a year":      {AutoRenewalGP: intPtr(MaxGraceDays + 1)},
		"lock over a year":       {TransferLockPeriod: intPtr(366)},
		"pending transfer":       {PendingTransferPeriod: intPtr(31)},
		"redemption":             {RedemptionGP: intPtr(400)},
		"pending delete":         {PendingDeleteGP: intPtr(91)},
		"pending restore":        {PendingRestorePeriod: intPtr(31)},
		"horizon zero":           {MaxHorizon: intPtr(0)},
		"horizon over 10 years":  {MaxHorizon: intPtr(11)},
		"no window for a report": {PendingRestorePeriod: intPtr(0)},
	} {
		t.Run(name, func(t *testing.T) { assert.ErrorIs(t, s.Validate(), ErrInvalidLifecyclePolicy) })
	}
	assert.NoError(t, LifecycleSettings{PendingRestorePeriod: intPtr(0), RestoreReportRequired: boolPtr(false)}.Validate(), "a zero window with no report to send")
	err := LifecycleSettings{RenewalGP: intPtr(-1), MaxHorizon: intPtr(0)}.Validate()
	assert.ErrorContains(t, err, "renewalGP must be 0 to 365; maxHorizon must be 1 to 10", "every fault, named")
}

func TestNewLifecyclePolicy(t *testing.T) {
	p, err := NewLifecyclePolicy("short-redemption", "  demo  ", LifecycleSettings{RedemptionGP: intPtr(3)})
	require.NoError(t, err)
	assert.Equal(t, "demo", p.Description)
	assert.Equal(t, 3, p.RedemptionDays())
	assert.Equal(t, PendingDeleteGP, p.PendingDeleteDays())
	assert.False(t, p.BuiltIn())

	_, err = NewLifecyclePolicy("Bad Name", "", LifecycleSettings{})
	assert.ErrorIs(t, err, ErrInvalidLifecyclePolicy)
	_, err = NewLifecyclePolicy("ok", "", LifecycleSettings{RedemptionGP: intPtr(-1)})
	assert.ErrorIs(t, err, ErrInvalidLifecyclePolicy)

	std := StandardLifecyclePolicy()
	assert.True(t, std.BuiltIn())
	assert.Equal(t, DefaultLifecycle(), std.Resolved())
	assert.NoError(t, std.Settings.Validate())
}

func TestLifecyclePolicy_NilIsTheDefault(t *testing.T) {
	var p *LifecyclePolicy
	assert.Equal(t, TransferLockPeriod, p.TransferLockDays())
	assert.True(t, p.RequiresRestoreReport())
	assert.True(t, p.AutoRenewAllowed())
	assert.Equal(t, MaxHorizon, p.MaxHorizonYears())
	phase := &Phase{}
	assert.Equal(t, DefaultLifecycle(), phase.Lifecycle().Resolved(), "a phase without a loaded policy follows gtld-standard")
	custom := &LifecyclePolicy{Name: "x", Settings: LifecycleSettings{RedemptionGP: intPtr(1)}}
	phase.SetLifecyclePolicy(custom)
	assert.Equal(t, "x", phase.LifecyclePolicyName)
	assert.Equal(t, 1, phase.Lifecycle().RedemptionDays())
}

func TestLifecyclePolicy_JSON(t *testing.T) {
	p, err := NewLifecyclePolicy("demo", "", LifecycleSettings{RedemptionGP: intPtr(3)})
	require.NoError(t, err)
	b, err := json.Marshal(p)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Contains(t, string(m["settings"]), `"renewalGP":null`, "stored settings, nil as null")
	assert.Contains(t, string(m["resolved"]), `"renewalGP":5`, "and resolved beside them")
	var back LifecyclePolicy
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, p.Settings, back.Settings)
	assert.Equal(t, p.Name, back.Name)
}

func TestNewPhase_NamesTheStandardLifecycle(t *testing.T) {
	p, err := NewPhase("ga-1", "GA", time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, LifecyclePolicyStandard, p.LifecyclePolicyName)
	assert.Nil(t, p.LifecyclePolicy)
}
