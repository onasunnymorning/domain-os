package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEPPPolicy(t *testing.T) {
	_, err := NewEPPPolicy("gold-2", "  Large registrars  ", DefaultEPPPolicyLimits())
	require.NoError(t, err)
	for _, name := range []string{"", "Gold", "2gold", "gold plus", "a-very-long-policy-name-of-33-chr"} {
		_, err := NewEPPPolicy(name, "", DefaultEPPPolicyLimits())
		assert.ErrorIs(t, err, ErrInvalidEPPPolicy, name)
	}
	for _, edit := range []func(*EPPPolicyLimits){
		func(l *EPPPolicyLimits) { l.SessionsPerRegistrar = 0 },
		func(l *EPPPolicyLimits) { l.SessionsPerLogin = -1 },
		func(l *EPPPolicyLimits) { l.SessionsPerFamily = 0 },
		func(l *EPPPolicyLimits) { l.CheckNames = 0 },
		func(l *EPPPolicyLimits) { l.Rates.Info.PerSecond = 0 },
		func(l *EPPPolicyLimits) { l.Rates.Poll.Burst = 0 },
		func(l *EPPPolicyLimits) { l.FamilyRates.Transform.PerSecond = 0 },
		func(l *EPPPolicyLimits) { l.Rates.Check.Burst = l.CheckNames - 1 },
		func(l *EPPPolicyLimits) { l.FamilyRates.Check.Burst = l.CheckNames - 1 },
		func(l *EPPPolicyLimits) { l.CreatesPerDay = -1 },
		func(l *EPPPolicyLimits) { l.FailedCreates.Failures = 0 },
		func(l *EPPPolicyLimits) { l.FailedCreates.RatioPercent = 101 },
		func(l *EPPPolicyLimits) { l.FamilyFailedCreates.RatioPercent = 0 },
	} {
		l := DefaultEPPPolicyLimits()
		edit(&l)
		_, err := NewEPPPolicy("gold", "", l)
		assert.ErrorIs(t, err, ErrInvalidEPPPolicy)
	}
	l := DefaultEPPPolicyLimits()
	l.CreatesPerDay, l.Rates.Check.Burst = 0, l.CheckNames
	_, err = NewEPPPolicy("gold", "", l)
	assert.NoError(t, err, "no create cap, and a check burst of exactly one full check")
	_, err = NewEPPPolicy("gold", "", EPPPolicyLimits{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rates.check.per_second must be at least 1", "every bad limit is named")
	assert.True(t, (&EPPPolicy{Name: EPPPolicyProbation}).BuiltIn())
	assert.False(t, (&EPPPolicy{Name: "gold"}).BuiltIn())
}

func TestStartingLimits(t *testing.T) {
	for _, p := range BuiltInEPPPolicies() {
		require.NoError(t, p.Limits.Validate(), p.Name)
		assert.Equal(t, p.Limits.Rates.Times(3), p.Limits.FamilyRates, "%s: a family's rates start at 3× a registrar's", p.Name)
	}
	assert.Zero(t, DefaultEPPPolicyLimits().CreatesPerDay, "default has no create cap")
	assert.Equal(t, 100, ProbationEPPPolicyLimits().CreatesPerDay)
	assert.Equal(t, EPPRate{PerSecond: 250, Burst: 1000}, DefaultEPPPolicyLimits().Rates.Check)
}

func TestFailedCreateBudget(t *testing.T) {
	b := EPPFailedCreateBudget{Failures: 100, RatioPercent: 90}
	assert.True(t, b.Over(100, 0))
	assert.True(t, b.Over(900, 100), "exactly 90%")
	assert.False(t, b.Over(99, 0), "too few")
	assert.False(t, b.Over(899, 101), "a busy registrar with failures, under 90%")
}

func TestNewRegistrarFamily(t *testing.T) {
	f, err := NewRegistrarFamily("acme", "", EPPPolicyDefault, []ClIDType{"rar-b", "rar-a", "rar-b"})
	require.NoError(t, err)
	assert.Equal(t, []ClIDType{"rar-a", "rar-b"}, f.Members)
	assert.Equal(t, EPPPolicyDefault, f.PolicyName())
	assert.Equal(t, EPPPolicyDefault, (&RegistrarFamily{}).PolicyName(), "none is default")
	_, err = NewRegistrarFamily("Acme", "", "", nil)
	assert.ErrorIs(t, err, ErrInvalidRegistrarFamily)
	_, err = NewRegistrarFamily("acme", "", "", []ClIDType{"x"})
	assert.ErrorIs(t, err, ErrInvalidRegistrarFamily, "a ClID too short")
}

func TestEPPPolicySet(t *testing.T) {
	gold := &EPPPolicy{Name: "gold", Limits: EPPPolicyLimits{SessionsPerRegistrar: 20, SessionsPerLogin: 5, SessionsPerFamily: 3, CheckNames: 25}}
	set := NewEPPPolicySet([]*EPPPolicy{gold},
		map[ClIDType]string{"rar-a": "gold", "rar-z": "deleted-meanwhile"},
		[]*RegistrarFamily{{Name: "acme", Policy: "gold", Members: []ClIDType{"rar-b"}}}, time.Now())

	assert.Equal(t, "gold", set.ForRegistrar("rar-a").Policy.Name)
	assert.Equal(t, EPPPolicyDefault, set.ForRegistrar("rar-c").Policy.Name, "not assigned")
	assert.Equal(t, DefaultEPPPolicyLimits(), set.ForRegistrar("rar-c").Policy.Limits, "a built-in policy not stored has its starting limits")
	assert.Equal(t, EPPPolicyDefault, set.ForRegistrar("rar-z").Policy.Name, "an assignment to a missing policy falls back to default")

	b := set.ForRegistrar("rar-b")
	require.NotNil(t, b.Family)
	assert.Equal(t, EPPSessionLimits{PerRegistrar: 10, PerLogin: 10, Family: "acme", PerFamily: 3}, b.SessionLimits(0))
	assert.Equal(t, EPPSessionLimits{PerRegistrar: 20, PerLogin: 2}, set.ForRegistrar("rar-a").SessionLimits(2))

	empty := DefaultEPPPolicySet()
	assert.Equal(t, DefaultEPPPolicyLimits(), empty.ForRegistrar("anyone").Policy.Limits)
	assert.Len(t, empty.Policies(), 2)
}

func TestNoteExpiringPins(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	pin := func(notAfter time.Time, notice int) *EPPCertificatePin {
		p := &EPPCertificatePin{Subject: "CN=x", NotAfter: notAfter, ExpiryNotice: notice}
		p.ID = uuid.New()
		return p
	}
	far, in20, in5, gone := pin(now.AddDate(0, 0, 60), 0), pin(now.AddDate(0, 0, 20), 0), pin(now.AddDate(0, 0, 5), 30), pin(now.Add(-time.Hour), 0)
	a := &EPPRegistrarAccess{ClID: "rar-a", CertificatePins: []*EPPCertificatePin{far, in20, in5, gone}}

	events := a.NoteExpiringPins(now, "test")
	require.Len(t, events, 2)
	assert.Equal(t, 30, events[0].Data.(EPPCertificateExpiringData).Days)
	assert.Equal(t, 7, events[1].Data.(EPPCertificateExpiringData).Days)
	assert.Equal(t, "system", events[0].Actor)
	assert.Equal(t, 30, in20.ExpiryNotice)
	assert.Equal(t, 7, in5.ExpiryNotice)
	assert.Zero(t, far.ExpiryNotice, "not due yet")
	assert.Zero(t, gone.ExpiryNotice, "already expired: no notice")
	assert.Empty(t, a.NoteExpiringPins(now, "test"), "each notice once")
	assert.Len(t, a.NoteExpiringPins(now.AddDate(0, 0, 14), "test"), 1, "the 20-day pin's 7-day notice")
}
