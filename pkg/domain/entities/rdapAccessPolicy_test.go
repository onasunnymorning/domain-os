package entities

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInRDAPAccessPoliciesAreValid(t *testing.T) {
	for _, p := range BuiltInRDAPAccessPolicies() {
		assert.NoError(t, p.Validate(), p.Name)
	}
	assert.Equal(t, EPPRate{PerSecond: 2, Burst: 10}, AnonymousRDAPAccessLimits().PerAddress, "the documented starting limits")
	assert.Equal(t, 3, AnonymousRDAPAccessLimits().NotFoundCost)
}

func TestNewRDAPAccessPolicyRejects(t *testing.T) {
	ok := AnonymousRDAPAccessLimits()
	for name, l := range map[string]RDAPAccessLimits{
		"no address rate":        {PerAddress: EPPRate{}, NotFoundCost: 1},
		"half an aggregate":      {PerAddress: ok.PerAddress, PerAggregate: EPPRate{PerSecond: 5}, NotFoundCost: 1},
		"free not-found":         {PerAddress: ok.PerAddress, NotFoundCost: 0},
		"not-found over a burst": {PerAddress: EPPRate{PerSecond: 1, Burst: 2}, NotFoundCost: 3},
	} {
		_, err := NewRDAPAccessPolicy("p", "", l)
		assert.ErrorIs(t, err, ErrInvalidRDAPAccessPolicy, name)
	}
	_, err := NewRDAPAccessPolicy("Bad", "", ok)
	assert.ErrorIs(t, err, ErrInvalidRDAPAccessPolicy)
	p, err := NewRDAPAccessPolicy("partner", "", RDAPAccessLimits{PerAddress: EPPRate{PerSecond: 10, Burst: 20}, NotFoundCost: 1})
	require.NoError(t, err)
	assert.Equal(t, "partner", p.Name)
}

func TestRDAPAccessSetForAddress(t *testing.T) {
	partner := &RDAPAccessPolicy{Name: "partner"}
	s := NewRDAPAccessSet([]*RDAPAccessPolicy{partner}, []RDAPAccessNetwork{
		{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: RDAPAccessMonitoring},
		{Network: netip.MustParsePrefix("192.0.2.128/25"), Policy: "partner"},
		{Network: netip.MustParsePrefix("2001:db8::/32"), Policy: "gone"},
	}, time.Time{})

	assert.Equal(t, "partner", s.ForAddress(netip.MustParseAddr("192.0.2.200")).Name, "the most specific network wins")
	assert.Equal(t, RDAPAccessMonitoring, s.ForAddress(netip.MustParseAddr("192.0.2.1")).Name)
	assert.Equal(t, RDAPAccessMonitoring, s.ForAddress(netip.MustParseAddr("::ffff:192.0.2.1")).Name, "a mapped IPv4 address is its IPv4 address")
	assert.Equal(t, RDAPAccessAnonymous, s.ForAddress(netip.MustParseAddr("198.51.100.1")).Name)
	assert.Equal(t, RDAPAccessAnonymous, s.ForAddress(netip.MustParseAddr("2001:db8::1")).Name, "a network assigned a missing policy falls back")
}

func TestRDAPAddressBuckets(t *testing.T) {
	a, agg := RDAPAddressBuckets(netip.MustParseAddr("192.0.2.77"))
	assert.Equal(t, "192.0.2.77/32", a.String())
	assert.Equal(t, "192.0.2.0/24", agg.String())
	a, agg = RDAPAddressBuckets(netip.MustParseAddr("2001:db8:1:2:3::4"))
	assert.Equal(t, "2001:db8:1:2::/64", a.String(), "one IPv6 subscriber often holds a whole /64")
	assert.Equal(t, "2001:db8:1::/48", agg.String())
}

func TestParseRDAPAccessNetwork(t *testing.T) {
	p, err := ParseRDAPAccessNetwork("192.0.2.9/24")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.0/24", p.String(), "masked")
	p, err = ParseRDAPAccessNetwork("2001:db8::1")
	require.NoError(t, err)
	assert.Equal(t, "2001:db8::1/128", p.String())
	_, err = ParseRDAPAccessNetwork("not-a-network")
	assert.ErrorIs(t, err, ErrInvalidRDAPAccessNetwork)
}
