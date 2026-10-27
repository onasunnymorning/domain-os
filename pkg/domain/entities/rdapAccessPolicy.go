package entities

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"time"
)

// An RDAP access policy is a named set of request limits, assigned to
// networks (ADR 0022). A client in no assigned network has "anonymous"; the
// most specific network that contains it wins. Each request takes from two
// buckets at once: the client's address (an IPv4 /32, an IPv6 /64) and its
// aggregate (an IPv4 /24, an IPv6 /48). A lookup that finds nothing costs
// more, to slow enumeration.

// Built-in RDAP access policies.
const (
	RDAPAccessAnonymous  = "anonymous"
	RDAPAccessMonitoring = "monitoring"
)

// Outbox events for RDAP access policy changes.
const (
	EventRDAPAccessPolicySaved     = "rdap.access_policy.saved"
	EventRDAPAccessPolicyDeleted   = "rdap.access_policy.deleted"
	EventRDAPAccessNetworkAssigned = "rdap.access_network.assigned"
)

// RDAP access policy errors.
var (
	ErrInvalidRDAPAccessPolicy  = errors.New("invalid RDAP access policy")
	ErrRDAPAccessPolicyNotFound = errors.New("RDAP access policy not found")
	ErrRDAPAccessPolicyBuiltIn  = errors.New("a built-in RDAP access policy cannot be deleted")
	ErrRDAPAccessPolicyInUse    = errors.New("the RDAP access policy is assigned")
	ErrInvalidRDAPAccessNetwork = errors.New("invalid RDAP access network")
)

// RDAPAccessLimits are a policy's request limits.
type RDAPAccessLimits struct {
	// PerAddress is the bucket of one client address.
	PerAddress EPPRate `json:"perAddress"`
	// PerAggregate is the bucket of the address's aggregate. A zero rate
	// is no aggregate limit.
	PerAggregate EPPRate `json:"perAggregate"`
	// NotFoundCost is the tokens a lookup that finds nothing takes, at
	// least 1.
	NotFoundCost int `json:"notFoundCost"`
}

// RDAPAccessPolicy is a named set of limits.
type RDAPAccessPolicy struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Limits      RDAPAccessLimits `json:"limits"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	UpdatedBy   string           `json:"updatedBy,omitempty"`
}

// AnonymousRDAPAccessLimits are anonymous's starting limits: deliberately
// low, to be raised from observed traffic (ADR 0022 decision 5).
func AnonymousRDAPAccessLimits() RDAPAccessLimits {
	return RDAPAccessLimits{PerAddress: EPPRate{PerSecond: 2, Burst: 10}, PerAggregate: EPPRate{PerSecond: 20, Burst: 60}, NotFoundCost: 3}
}

// MonitoringRDAPAccessLimits are monitoring's starting limits, for ICANN's
// SLA probes and RST's test clients: a throttled probe counts against the
// SLA.
func MonitoringRDAPAccessLimits() RDAPAccessLimits {
	return RDAPAccessLimits{PerAddress: EPPRate{PerSecond: 50, Burst: 200}, NotFoundCost: 1}
}

// BuiltInRDAPAccessPolicies are anonymous and monitoring with their
// starting limits. The migration stores them if they are not stored yet;
// their limits can be changed, they cannot be deleted.
func BuiltInRDAPAccessPolicies() []*RDAPAccessPolicy {
	return []*RDAPAccessPolicy{
		{Name: RDAPAccessAnonymous, Description: "Every client in no assigned network", Limits: AnonymousRDAPAccessLimits()},
		{Name: RDAPAccessMonitoring, Description: "ICANN SLA monitoring and RST test clients", Limits: MonitoringRDAPAccessLimits()},
	}
}

// IsBuiltInRDAPAccessPolicy reports whether name is a built-in policy.
func IsBuiltInRDAPAccessPolicy(name string) bool {
	return name == RDAPAccessAnonymous || name == RDAPAccessMonitoring
}

// NewRDAPAccessPolicy returns a validated policy.
func NewRDAPAccessPolicy(name, description string, limits RDAPAccessLimits) (*RDAPAccessPolicy, error) {
	p := &RDAPAccessPolicy{Name: name, Description: description, Limits: limits}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate checks the policy.
func (p *RDAPAccessPolicy) Validate() error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidRDAPAccessPolicy, fmt.Sprintf(format, args...))
	}
	if !policyNamePattern.MatchString(p.Name) {
		return invalid("name %q must be 1 to 32 lower-case letters, digits and hyphens, starting with a letter", p.Name)
	}
	l := p.Limits
	if l.PerAddress.PerSecond < 1 || l.PerAddress.Burst < 1 {
		return invalid("perAddress needs a rate and a burst of at least 1")
	}
	agg := l.PerAggregate
	if (agg.PerSecond == 0) != (agg.Burst == 0) || agg.PerSecond < 0 || agg.Burst < 0 {
		return invalid("perAggregate needs both a rate and a burst of at least 1, or neither for no aggregate limit")
	}
	if l.NotFoundCost < 1 {
		return invalid("notFoundCost must be at least 1")
	}
	// A request that can never fit in a bucket would always be refused.
	if l.NotFoundCost > l.PerAddress.Burst || (agg.Burst > 0 && l.NotFoundCost > agg.Burst) {
		return invalid("notFoundCost (%d) must fit in every burst", l.NotFoundCost)
	}
	return nil
}

// RDAPAccessNetwork is a network assigned a policy.
type RDAPAccessNetwork struct {
	Network netip.Prefix `json:"network"`
	Policy  string       `json:"policy"`
}

// ParseRDAPAccessNetwork reads a CIDR, or a single address as its /32 or
// /128, in its canonical (masked) form. An IPv4-mapped IPv6 prefix
// (::ffff:192.0.2.0/120) is read as the IPv4 prefix it covers, since client
// addresses are matched unmapped.
func ParseRDAPAccessNetwork(s string) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return canonicalRDAPNetwork(p), nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()), nil
	}
	return netip.Prefix{}, fmt.Errorf("%w: %q is neither a CIDR nor an address", ErrInvalidRDAPAccessNetwork, s)
}

// canonicalRDAPNetwork is p masked, with an IPv4-mapped prefix that covers
// only IPv4 addresses written as IPv4 (as ParseEPPAllowedPrefix does).
func canonicalRDAPNetwork(p netip.Prefix) netip.Prefix {
	if p.Addr().Is4In6() && p.Bits() >= 96 {
		p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	return p.Masked()
}

// RDAPAccessSet is every access policy and network assignment, as an RDAP
// node holds them between reloads.
type RDAPAccessSet struct {
	policies map[string]*RDAPAccessPolicy
	networks []RDAPAccessNetwork // most specific first
	LoadedAt time.Time
}

// NewRDAPAccessSet builds a set. A built-in policy not stored is included
// with its starting limits.
func NewRDAPAccessSet(policies []*RDAPAccessPolicy, networks []RDAPAccessNetwork, loadedAt time.Time) *RDAPAccessSet {
	s := &RDAPAccessSet{policies: map[string]*RDAPAccessPolicy{}, LoadedAt: loadedAt}
	for _, b := range BuiltInRDAPAccessPolicies() {
		s.policies[b.Name] = b
	}
	for _, p := range policies {
		s.policies[p.Name] = p
	}
	for _, n := range networks {
		n.Network = canonicalRDAPNetwork(n.Network)
		s.networks = append(s.networks, n)
	}
	sort.SliceStable(s.networks, func(i, j int) bool { return s.networks[i].Network.Bits() > s.networks[j].Network.Bits() })
	return s
}

// DefaultRDAPAccessSet is the set before any is loaded: the built-ins,
// assigned to no network.
func DefaultRDAPAccessSet() *RDAPAccessSet { return NewRDAPAccessSet(nil, nil, time.Time{}) }

// ForAddress is the policy for a client: that of the most specific network
// containing addr, or anonymous.
func (s *RDAPAccessSet) ForAddress(addr netip.Addr) *RDAPAccessPolicy {
	addr = addr.Unmap()
	for _, n := range s.networks {
		if n.Network.Contains(addr) {
			if p, ok := s.policies[n.Policy]; ok {
				return p
			}
		}
	}
	return s.policies[RDAPAccessAnonymous]
}

// UnresolvedNetworks lists, as "network=policy", every network assigned a
// policy the set does not hold. A set with any is inconsistent and must not
// replace the one in force.
func (s *RDAPAccessSet) UnresolvedNetworks() []string {
	var out []string
	for _, n := range s.networks {
		if _, ok := s.policies[n.Policy]; !ok {
			out = append(out, n.Network.String()+"="+n.Policy)
		}
	}
	sort.Strings(out)
	return out
}

// Policies returns every policy, by name.
func (s *RDAPAccessSet) Policies() []*RDAPAccessPolicy {
	out := make([]*RDAPAccessPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Networks returns every assigned network, most specific first.
func (s *RDAPAccessSet) Networks() []RDAPAccessNetwork {
	return append([]RDAPAccessNetwork(nil), s.networks...)
}

// RDAPAddressBuckets names the buckets a request from addr takes from: the
// address (IPv4 /32, IPv6 /64) and its aggregate (IPv4 /24, IPv6 /48).
func RDAPAddressBuckets(addr netip.Addr) (address, aggregate netip.Prefix) {
	addr = addr.Unmap()
	if addr.Is4() {
		address, _ = addr.Prefix(32)
		aggregate, _ = addr.Prefix(24)
		return address, aggregate
	}
	address, _ = addr.Prefix(64)
	aggregate, _ = addr.Prefix(48)
	return address, aggregate
}
