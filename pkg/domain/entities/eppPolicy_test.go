package entities

import (
	"testing"
	"time"

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
	} {
		l := DefaultEPPPolicyLimits()
		edit(&l)
		_, err := NewEPPPolicy("gold", "", l)
		assert.ErrorIs(t, err, ErrInvalidEPPPolicy)
	}
	assert.True(t, (&EPPPolicy{Name: EPPPolicyProbation}).BuiltIn())
	assert.False(t, (&EPPPolicy{Name: "gold"}).BuiltIn())
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
