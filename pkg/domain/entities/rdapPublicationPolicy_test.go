package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allWithheld() map[RDAPField]RDAPFieldRule {
	out := map[RDAPField]RDAPFieldRule{}
	for _, f := range RDAPFields {
		out[f] = RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPRemoval}
	}
	return out
}

func TestBuiltInRDAPPublicationPoliciesAreValid(t *testing.T) {
	for _, p := range BuiltInRDAPPublicationPolicies() {
		assert.NoError(t, p.Validate(), p.Name)
		assert.True(t, IsBuiltInRDAPPublicationPolicy(p.Name))
	}
}

func TestNewRDAPPublicationPolicyRejects(t *testing.T) {
	with := func(f RDAPField, r RDAPFieldRule) map[RDAPRole]map[RDAPField]RDAPFieldRule {
		rules := allWithheld()
		rules[f] = r
		return map[RDAPRole]map[RDAPField]RDAPFieldRule{RDAPRoleRegistrant: rules}
	}
	missing := allWithheld()
	delete(missing, RDAPFieldFax)

	for _, tc := range []struct {
		name  string
		pname string
		roles map[RDAPRole]map[RDAPField]RDAPFieldRule
		uri   string
		want  string
	}{
		{"bad name", "Bad Name", nil, "", "name"},
		{"insecure contact form", "p", nil, "http://x.example/form", "https"},
		{"unknown role", "p", map[RDAPRole]map[RDAPField]RDAPFieldRule{"reseller": allWithheld()}, "", "unknown role"},
		{"a field without a rule", "p", map[RDAPRole]map[RDAPField]RDAPFieldRule{RDAPRoleTech: missing}, "", "no rule for fax"},
		{"unknown rule", "p", with(RDAPFieldName, RDAPFieldRule{Rule: "maybe"}), "", "rule"},
		{"withheld without method", "p", with(RDAPFieldName, RDAPFieldRule{Rule: RDAPWithhold}), "", "method"},
		{"published with a method", "p", with(RDAPFieldName, RDAPFieldRule{Rule: RDAPPublish, Method: RDAPRemoval}), "", "no redaction method"},
		{"replacing a non-email", "p", with(RDAPFieldName, RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPReplacementValue}), "", "only an email"},
		{"handle if disclosed", "p", with(RDAPFieldHandle, RDAPFieldRule{Rule: RDAPPublishIfDisclosed}), "", "disclose flag"},
		{"handle emptied", "p", with(RDAPFieldHandle, RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPEmptyValue}), "", "removed"},
		{"phone emptied", "p", with(RDAPFieldVoice, RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPEmptyValue}), "", "phone or fax is removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRDAPPublicationPolicy(tc.pname, "", tc.roles, tc.uri)
			require.ErrorIs(t, err, ErrInvalidRDAPPublicationPolicy)
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	p, err := NewRDAPPublicationPolicy("thick-open", "", with(RDAPFieldName, RDAPFieldRule{Rule: RDAPPublish}), "https://registry.example/contact")
	require.NoError(t, err)
	assert.Equal(t, "thick-open", p.Name)
}

func TestRDAPPublicationSetForTLD(t *testing.T) {
	custom := &RDAPPublicationPolicy{Name: "custom", Roles: map[RDAPRole]map[RDAPField]RDAPFieldRule{}}
	s := NewRDAPPublicationSet([]*RDAPPublicationPolicy{custom}, map[DomainName]string{"assigned": "custom", "dangling": "gone"}, time.Time{})
	assert.Equal(t, "custom", s.ForTLD("assigned", TLDTypeGTLD).Name)
	assert.Equal(t, RDAPPolicyGTLDRDP2024, s.ForTLD("other", TLDTypeGTLD).Name)
	assert.Equal(t, RDAPPolicyCCTLDMinimal, s.ForTLD("other", TLDTypeCCTLD).Name)
	assert.Equal(t, RDAPPolicyGTLDRDP2024, s.ForTLD("dangling", TLDTypeGTLD).Name, "an assignment to a policy not loaded falls back")
	assert.Len(t, s.Policies(), 3, "the built-ins are always present")
	assert.Equal(t, RDAPPolicyCCTLDMinimal, DefaultRDAPPublicationSet().ForTLD("x", TLDTypeCCTLD).Name)
}

func TestUnresolvedAssignments(t *testing.T) {
	s := NewRDAPPublicationSet(nil, map[DomainName]string{"a": RDAPPolicyCCTLDMinimal, "b": "created-later"}, time.Time{})
	assert.Equal(t, []string{"b=created-later"}, s.UnresolvedAssignments(), "a built-in always resolves; a missing policy does not")
}

func TestRemoveEmptiedPhones(t *testing.T) {
	p := BuiltInRDAPPublicationPolicies()[0]
	p.Roles[RDAPRoleRegistrant][RDAPFieldVoice] = RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPEmptyValue}
	p.Roles[RDAPRoleRegistrant][RDAPFieldFax] = RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPEmptyValue}
	street := p.Roles[RDAPRoleRegistrant][RDAPFieldStreet]
	require.Error(t, p.Validate(), "as stored before the rule")

	assert.True(t, p.RemoveEmptiedPhones())
	assert.Equal(t, RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPRemoval}, p.Roles[RDAPRoleRegistrant][RDAPFieldVoice])
	assert.Equal(t, RDAPFieldRule{Rule: RDAPWithhold, Method: RDAPRemoval}, p.Roles[RDAPRoleRegistrant][RDAPFieldFax])
	assert.Equal(t, street, p.Roles[RDAPRoleRegistrant][RDAPFieldStreet], "nothing else changes")
	assert.NoError(t, p.Validate())
	assert.False(t, p.RemoveEmptiedPhones(), "idempotent")
}
