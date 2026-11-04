package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applicationPhase is a landrush phase that collects applications, started
// an hour ago, requiring validation when validated is set.
func applicationPhase(t *testing.T, validated bool) *Phase {
	t.Helper()
	p, err := NewPhase("landrush", string(PhaseTypeLaunch), time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)
	p.Launch = LaunchSettings{Mode: LaunchModeLandrush, Model: RegistrationModelApplication, AcceptSignedMarks: true}
	p.Policy.RequiresValidation = &validated
	return p
}

const testAppID = RoidType("7_APP-TEST")

func TestNewLaunchApplication(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	t.Run("files pendingAllocation, with the code hashed", func(t *testing.T) {
		a, err := NewLaunchApplication(testAppID, "Rushed.Test", "rar-one", "Tr4nsf3r-C0de-Xq9#Lm2!", applicationPhase(t, false), 2, now)
		require.NoError(t, err)
		assert.Equal(t, ApplicationStatusPendingAllocation, a.Status)
		assert.Equal(t, DomainName("rushed.test"), a.Name)
		assert.Equal(t, DomainName("test"), a.TLDName)
		assert.Equal(t, ClIDType("landrush"), a.PhaseName)
		assert.Equal(t, ClIDType("rar-one"), a.ClID)
		assert.Equal(t, 2, a.Years)
		assert.True(t, a.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"))
		assert.Equal(t, now, a.CreatedAt)
		assert.NoError(t, a.Validate())
	})
	t.Run("files pendingValidation when the phase requires validation", func(t *testing.T) {
		a, err := NewLaunchApplication(testAppID, "rushed.test", "rar-one", "", applicationPhase(t, true), 1, now)
		require.NoError(t, err)
		assert.Equal(t, ApplicationStatusPendingValidation, a.Status)
		assert.False(t, a.AuthInfo.IsSet(), "no code given, none set")
	})
	t.Run("refusals", func(t *testing.T) {
		firstCome := applicationPhase(t, false)
		firstCome.Launch.Model = RegistrationModelFirstCome
		sunrise := applicationPhase(t, false)
		sunrise.Launch.Mode = LaunchModeSunrise
		for _, tc := range []struct {
			name         string
			id           RoidType
			domain, clid string
			phase        *Phase
			years        int
			err          error
		}{
			{"no phase", testAppID, "rushed.test", "rar-one", nil, 1, ErrPhaseNotProvided},
			{"a first-come phase", testAppID, "rushed.test", "rar-one", firstCome, 1, ErrPhaseRegistersAtOnce},
			{"a sunrise", testAppID, "rushed.test", "rar-one", sunrise, 1, ErrLaunchModeUnsupported},
			{"a TLD as the name", testAppID, "test", "rar-one", applicationPhase(t, false), 1, ErrTLDAsDomain},
			{"zero years", testAppID, "rushed.test", "rar-one", applicationPhase(t, false), 0, ErrZeroRenewalPeriod},
			{"an invalid ID", RoidType("not a roid"), "rushed.test", "rar-one", applicationPhase(t, false), 1, ErrInvalidApplication},
			{"an invalid registrar", testAppID, "rushed.test", "x", applicationPhase(t, false), 1, ErrInvalidClIDType},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewLaunchApplication(tc.id, tc.domain, tc.clid, "", tc.phase, tc.years, now)
				assert.ErrorIs(t, err, tc.err)
			})
		}
	})
}

func TestLaunchApplication_SetHostNames(t *testing.T) {
	a := &LaunchApplication{}
	require.NoError(t, a.SetHostNames([]string{"NS1.Example.NET", "ns2.example.net"}))
	assert.Equal(t, []string{"ns1.example.net", "ns2.example.net"}, a.HostNames)
	assert.ErrorIs(t, a.SetHostNames([]string{"ns1.example.net", "NS1.example.net"}), ErrDuplicateHost)
	many := make([]string, MaxHostsPerDomain+1)
	for i := range many {
		many[i] = "ns" + string(rune('a'+i)) + ".example.net"
	}
	assert.ErrorIs(t, a.SetHostNames(many), ErrMaxHostsPerDomainExceeded)
	require.NoError(t, a.SetHostNames(nil))
	assert.Nil(t, a.HostNames)
}

func TestLaunchApplication_Transitions(t *testing.T) {
	now := time.Now().UTC()
	later := now.Add(time.Minute)
	filed := func(validation bool) *LaunchApplication {
		a, err := NewLaunchApplication(testAppID, "rushed.test", "rar-one", "", applicationPhase(t, validation), 1, now)
		require.NoError(t, err)
		return a
	}

	t.Run("validated, then allocated", func(t *testing.T) {
		a := filed(true)
		assert.ErrorIs(t, a.Allocate("9_DOM-TEST", later), ErrApplicationTransition, "not yet validated")
		require.NoError(t, a.Validated(later))
		assert.Equal(t, ApplicationStatusPendingAllocation, a.Status)
		assert.Equal(t, later, a.UpdatedAt)
		assert.ErrorIs(t, a.Validated(later), ErrApplicationTransition, "validated once")
		require.NoError(t, a.Allocate("9_DOM-TEST", later))
		assert.Equal(t, ApplicationStatusAllocated, a.Status)
		assert.Equal(t, RoidType("9_DOM-TEST"), a.DomainRoID)
		assert.True(t, a.Status.IsFinal())
		assert.ErrorIs(t, a.CanChange(), ErrApplicationFinal)
		assert.ErrorIs(t, a.Reject("late", later), ErrApplicationTransition, "final")
		assert.NoError(t, a.Validate())
	})
	t.Run("allocation needs a valid domain ROID", func(t *testing.T) {
		a := filed(false)
		assert.ErrorIs(t, a.Allocate("", later), ErrInvalidApplication)
		assert.Equal(t, ApplicationStatusPendingAllocation, a.Status, "unchanged")
	})
	t.Run("invalidated", func(t *testing.T) {
		a := filed(true)
		require.NoError(t, a.Invalidate("the applicant is not eligible", later))
		assert.Equal(t, ApplicationStatusInvalid, a.Status)
		assert.Equal(t, "the applicant is not eligible", a.Reason)
		assert.True(t, a.Status.IsFinal())
		a = filed(false)
		assert.ErrorIs(t, a.Invalidate("x", later), ErrApplicationTransition, "only a pendingValidation application is validated")
	})
	t.Run("rejected from either pending status", func(t *testing.T) {
		for _, validation := range []bool{true, false} {
			a := filed(validation)
			require.NoError(t, a.CanChange())
			require.NoError(t, a.Reject("the name went to another application", later))
			assert.Equal(t, ApplicationStatusRejected, a.Status)
			assert.ErrorIs(t, a.CanChange(), ErrApplicationFinal)
		}
	})
}

func TestLaunchApplication_Validate(t *testing.T) {
	now := time.Now().UTC()
	good := func() *LaunchApplication {
		a, err := NewLaunchApplication(testAppID, "rushed.test", "rar-one", "", applicationPhase(t, false), 1, now)
		require.NoError(t, err)
		return a
	}
	for _, tc := range []struct {
		name string
		edit func(*LaunchApplication)
	}{
		{"an ID that is not a roid", func(a *LaunchApplication) { a.ID = "nope nope" }},
		{"a name not under its TLD", func(a *LaunchApplication) { a.TLDName = "other" }},
		{"an unknown status", func(a *LaunchApplication) { a.Status = "validated" }},
		{"zero years", func(a *LaunchApplication) { a.Years = 0 }},
		{"a code that is not a hash", func(a *LaunchApplication) { a.AuthInfo = "plain" }},
		{"allocated without a domain", func(a *LaunchApplication) { a.Status = ApplicationStatusAllocated }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := good()
			tc.edit(a)
			assert.ErrorIs(t, a.Validate(), ErrInvalidApplication)
		})
	}
}

func TestLaunchApplication_DeepCopy(t *testing.T) {
	a := &LaunchApplication{ID: testAppID, HostNames: []string{"ns1.example.net"}, DSRecords: []DSRecord{{KeyTag: 1, Algorithm: 13, DigestType: 2, Digest: "AB"}}}
	c := a.DeepCopy()
	c.HostNames[0] = "ns2.example.net"
	c.DSRecords[0].KeyTag = 2
	assert.Equal(t, "ns1.example.net", a.HostNames[0])
	assert.Equal(t, uint16(1), a.DSRecords[0].KeyTag)
}

func TestNewRoidType_Application(t *testing.T) {
	r, err := NewRoidType(42, RoidTypeApplication, "TEST")
	require.NoError(t, err)
	assert.Equal(t, RoidType("42_APP-TEST"), r)
	assert.NoError(t, r.Validate(), "an application ID is roidType-valid, so it can be echoed as a <domain:roid>")
	assert.True(t, r.IsIssuedHere())
	assert.Equal(t, APPLICATION_ROID_ID, r.ObjectIdentifier())
}
