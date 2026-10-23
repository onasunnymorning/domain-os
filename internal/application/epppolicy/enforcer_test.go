package epppolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestClassOf(t *testing.T) {
	for c, want := range map[Command]Class{
		{Name: "check"}:                   ClassCheck,
		{Name: "info"}:                    ClassInfo,
		{Name: "transfer", Op: "query"}:   ClassInfo,
		{Name: "transfer", Op: "request"}: ClassTransform,
		{Name: "transfer", Op: "approve"}: ClassTransform,
		{Name: "create"}:                  ClassTransform,
		{Name: "update"}:                  ClassTransform,
		{Name: "delete"}:                  ClassTransform,
		{Name: "renew"}:                   ClassTransform,
		{Name: "poll", Op: "ack"}:         ClassPoll,
		{Name: "login"}:                   ClassSession,
		{Name: "logout"}:                  ClassSession,
	} {
		assert.Equal(t, want, ClassOf(c), "%+v", c)
	}
}

func TestDecide(t *testing.T) {
	limits := entities.DefaultEPPPolicyLimits()
	rw, ro := entities.EPPLoginRoleTransform, entities.EPPLoginRoleReadonly

	t.Run("a readonly login reads and polls but does not transform (2201)", func(t *testing.T) {
		for _, c := range []Command{{Name: "check", Objects: 1}, {Name: "info"}, {Name: "transfer", Op: "query"}, {Name: "poll", Op: "req"}, {Name: "poll", Op: "ack"}, {Name: "logout"}} {
			assert.Equal(t, Allow, Decide(ro, limits, c).Action, "%+v", c)
		}
		for _, c := range []Command{{Name: "create"}, {Name: "update"}, {Name: "delete"}, {Name: "renew"}, {Name: "transfer", Op: "request"}, {Name: "transfer", Op: "cancel"}} {
			d := Decide(ro, limits, c)
			assert.Equal(t, Reject, d.Action, "%+v", c)
			assert.Equal(t, RuleRole, d.Rule)
			assert.Equal(t, 2201, d.Code)
		}
		assert.Equal(t, Allow, Decide(rw, limits, Command{Name: "create"}).Action)
	})
	t.Run("a check over the policy's names (2306)", func(t *testing.T) {
		assert.Equal(t, Allow, Decide(rw, limits, Command{Name: "check", Objects: limits.CheckNames}).Action)
		d := Decide(rw, limits, Command{Name: "check", Objects: limits.CheckNames + 1})
		assert.Equal(t, Reject, d.Action)
		assert.Equal(t, RuleCheckNames, d.Rule)
		assert.Equal(t, 2306, d.Code)
		assert.Contains(t, d.Msg, "at most 50 names")
		small := limits
		small.CheckNames = 5
		assert.Equal(t, Reject, Decide(rw, small, Command{Name: "check", Objects: 6}).Action, "the policy's own number")
	})
	t.Run("the role rule comes first", func(t *testing.T) {
		assert.Equal(t, RuleNone, Decide(ro, limits, Command{Name: "check", Objects: 1}).Rule)
	})
}
