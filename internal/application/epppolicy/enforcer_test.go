package epppolicy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// fakeRates answers Take with take and ReserveQuota with ok, recording what
// it was asked.
type fakeRates struct {
	take     entities.EPPRateTake
	takeErr  error
	ok       bool
	buckets  []entities.EPPRateBucket
	cost     int
	maxWait  time.Duration
	quotaKey string
	limit    int
	released int
}

func (f *fakeRates) Take(_ context.Context, b []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error) {
	f.buckets, f.cost, f.maxWait = b, cost, maxWait
	return f.take, f.takeErr
}

func (f *fakeRates) ReserveQuota(_ context.Context, key string, limit int, _ time.Duration) (func(context.Context), bool, error) {
	f.quotaKey, f.limit = key, limit
	if !f.ok {
		return nil, false, nil
	}
	return func(context.Context) { f.released++ }, true, nil
}

func TestEnforce(t *testing.T) {
	ctx := context.Background()
	rw := entities.EPPLoginRoleTransform
	gold := &entities.EPPPolicy{Name: "gold", Limits: entities.DefaultEPPPolicyLimits()}
	famPolicy := &entities.EPPPolicy{Name: "fam", Limits: entities.ProbationEPPPolicyLimits()}
	alone := entities.EPPRegistrarPolicy{Policy: gold}
	inFamily := entities.EPPRegistrarPolicy{Policy: gold, Family: &entities.RegistrarFamily{Name: "acme"}, FamilyPolicy: famPolicy}
	enforcer := func(f *fakeRates) *Enforcer {
		e := NewEnforcer(f, &fakeFailed{}, &fakeOutcomes{})
		e.now = func() time.Time { return time.Date(2026, 10, 23, 23, 59, 0, 0, time.FixedZone("x", 3600)) }
		return e
	}

	t.Run("a check takes a token per name from the registrar's and the family's check buckets", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Taken: true}}
		d, release := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, inFamily, Command{Name: "check", Objects: 7})
		assert.Equal(t, Allow, d.Action)
		assert.Nil(t, release)
		assert.Equal(t, 7, f.cost)
		assert.Equal(t, MaxRateDelay, f.maxWait)
		assert.Equal(t, []entities.EPPRateBucket{
			{Key: "reg:rar-a:check", Rate: gold.Limits.Rates.Check},
			{Key: "fam:acme:check", Rate: famPolicy.Limits.FamilyRates.Check},
		}, f.buckets, "the family's bucket has its own policy's family rates")
	})
	t.Run("every other command takes one token from its class's bucket", func(t *testing.T) {
		for c, class := range map[Command]Class{
			{Name: "info"}: ClassInfo, {Name: "transfer", Op: "query"}: ClassInfo,
			{Name: "update"}: ClassTransform, {Name: "poll", Op: "req"}: ClassPoll,
		} {
			f := &fakeRates{take: entities.EPPRateTake{Taken: true}}
			enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, c)
			assert.Equal(t, 1, f.cost, "%+v", c)
			require.Len(t, f.buckets, 1, "no family, no family bucket")
			assert.Equal(t, "reg:rar-a:"+string(class), f.buckets[0].Key)
			assert.Equal(t, RateFor(gold.Limits.Rates, class), f.buckets[0].Rate)
		}
	})
	t.Run("session commands and refusals by Decide take nothing", func(t *testing.T) {
		f := &fakeRates{}
		d, _ := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, Command{Name: "logout"})
		assert.Equal(t, Allow, d.Action)
		d, _ = enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: entities.EPPLoginRoleReadonly}, alone, Command{Name: "create", Object: "domain"})
		assert.Equal(t, RuleRole, d.Rule)
		d, _ = enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, Command{Name: "check", Objects: 51})
		assert.Equal(t, RuleCheckNames, d.Rule)
		assert.Nil(t, f.buckets)
	})
	t.Run("a check larger than the family's check bucket is refused for its size", func(t *testing.T) {
		small := &entities.EPPPolicy{Name: "small", Limits: entities.DefaultEPPPolicyLimits()}
		small.Limits.FamilyRates.Check = entities.EPPRate{PerSecond: 1, Burst: 20}
		rp := entities.EPPRegistrarPolicy{Policy: gold, Family: &entities.RegistrarFamily{Name: "acme"}, FamilyPolicy: small}
		f := &fakeRates{take: entities.EPPRateTake{Taken: true}}
		d, _ := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, rp, Command{Name: "check", Objects: 21})
		assert.Equal(t, Reject, d.Action)
		assert.Equal(t, RuleCheckNames, d.Rule)
		assert.Equal(t, 2306, d.Code)
		assert.Contains(t, d.Msg, "at most 20 names")
		assert.Nil(t, f.buckets, "nothing taken")
		d, _ = enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, rp, Command{Name: "check", Objects: 20})
		assert.Equal(t, Allow, d.Action)
	})
	t.Run("a short wait holds the command", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Taken: true, Wait: 300 * time.Millisecond, Limiting: 1}}
		d, _ := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, inFamily, Command{Name: "info"})
		assert.Equal(t, Decision{Action: Delay, Rule: RuleFamilyRate, Wait: 300 * time.Millisecond}, d)
	})
	t.Run("a longer one refuses it with 2306 and a machine-readable reason", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Wait: 1234 * time.Millisecond}}
		d, _ := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, Command{Name: "update"})
		assert.Equal(t, Reject, d.Action)
		assert.Equal(t, RuleRate, d.Rule)
		assert.Equal(t, 2306, d.Code)
		assert.Equal(t, "Parameter value policy error; the registrar's transform rate is exceeded, try again in 1.3s", d.Msg)
		assert.Equal(t, "rate-limit; class=transform; scope=registrar; retry-after-ms=1300", d.Reason)
		f = &fakeRates{take: entities.EPPRateTake{Wait: 2 * time.Second, Limiting: 1}}
		d, _ = enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, inFamily, Command{Name: "check", Objects: 1})
		assert.Equal(t, "rate-limit; class=check; scope=family; retry-after-ms=2000", d.Reason)
	})
	t.Run("an unanswered limiter lets the command through", func(t *testing.T) {
		f := &fakeRates{takeErr: errors.New("down")}
		d, _ := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, Command{Name: "update"})
		assert.Equal(t, Allow, d.Action)
	})

	probation := entities.EPPRegistrarPolicy{Policy: &entities.EPPPolicy{Name: "probation", Limits: entities.ProbationEPPPolicyLimits()}}
	t.Run("a domain create counts against the day's quota, by UTC day", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Taken: true}, ok: true}
		d, release := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, probation, Command{Name: "create", Object: "domain"})
		assert.Equal(t, Allow, d.Action)
		require.NotNil(t, release)
		assert.Equal(t, "creates:rar-a:2026-10-23", f.quotaKey, "23:59 at UTC+1 is 22:59 UTC")
		assert.Equal(t, 100, f.limit)
		release(ctx)
		assert.Equal(t, 1, f.released)
	})
	t.Run("over the quota it is refused with 2308", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Taken: true}}
		d, release := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, probation, Command{Name: "create", Object: "domain"})
		assert.Equal(t, Reject, d.Action)
		assert.Equal(t, RuleCreateQuota, d.Rule)
		assert.Equal(t, 2308, d.Code)
		assert.Contains(t, d.Msg, "100 domains a day")
		assert.Nil(t, release)
	})
	t.Run("only domain creates count, and only under a cap", func(t *testing.T) {
		f := &fakeRates{take: entities.EPPRateTake{Taken: true}, ok: true}
		_, release := enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, probation, Command{Name: "create", Object: "host"})
		assert.Nil(t, release)
		_, release = enforcer(f).Enforce(ctx, Caller{ClID: "rar-a", Login: "login-a", Role: rw}, alone, Command{Name: "create", Object: "domain"})
		assert.Nil(t, release, "default has no cap")
		assert.Empty(t, f.quotaKey)
	})
}

func TestRefusalStreak(t *testing.T) {
	var r RefusalStreak
	now := time.Unix(1_700_000_000, 0)
	for i := 1; i < RefusalsBeforeClose; i++ {
		assert.False(t, r.Record(now), "refusal %d", i)
		now = now.Add(RefusalWindow / RefusalsBeforeClose / 2)
	}
	assert.True(t, r.Record(now), "the tenth within the window")

	var spread RefusalStreak
	now = time.Unix(1_700_000_000, 0)
	for i := range 3 * RefusalsBeforeClose {
		assert.False(t, spread.Record(now), "refusal %d, spread out", i)
		now = now.Add(RefusalWindow / (RefusalsBeforeClose - 2))
	}
}

// fakeFailed answers Assess with standing, recording what it was asked.
type fakeFailed struct {
	standing  entities.EPPFailedCreateStanding
	suspended time.Duration
	asked     *entities.EPPFailedCreateCheck
	recorded  []string
	failed    []bool
}

func (f *fakeFailed) Assess(_ context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error) {
	f.asked = &c
	return f.standing, nil
}

func (f *fakeFailed) Record(_ context.Context, scopes []string, failed bool) error {
	f.recorded, f.failed = scopes, append(f.failed, failed)
	return nil
}

func (f *fakeFailed) Suspended(context.Context, string) (time.Duration, error) {
	return f.suspended, nil
}

type fakeOutcomes struct{ failed, succeeded int }

func (f *fakeOutcomes) Add(_ entities.ClIDType, failed bool) {
	if failed {
		f.failed++
	} else {
		f.succeeded++
	}
}

func TestFailedCreateEscalation(t *testing.T) {
	ctx := context.Background()
	who := Caller{ClID: "rar-a", Login: "login-a", Role: entities.EPPLoginRoleTransform}
	gold := &entities.EPPPolicy{Name: "gold", Limits: entities.DefaultEPPPolicyLimits()}
	famPolicy := &entities.EPPPolicy{Name: "fam", Limits: entities.ProbationEPPPolicyLimits()}
	inFamily := entities.EPPRegistrarPolicy{Policy: gold, Family: &entities.RegistrarFamily{Name: "acme"}, FamilyPolicy: famPolicy}
	create := Command{Name: "create", Object: "domain"}
	run := func(st entities.EPPFailedCreateStanding, rates entities.EPPRateTake) (Decision, *fakeFailed) {
		failed := &fakeFailed{standing: st}
		d, _ := NewEnforcer(&fakeRates{take: rates}, failed, &fakeOutcomes{}).Enforce(ctx, who, inFamily, create)
		return d, failed
	}

	t.Run("within budget, nothing happens; the check holds the create to the registrar's and the family's budgets", func(t *testing.T) {
		d, failed := run(entities.EPPFailedCreateStanding{}, entities.EPPRateTake{Taken: true})
		assert.Equal(t, Allow, d.Action)
		require.NotNil(t, failed.asked)
		assert.Equal(t, entities.EPPFailedCreateCheck{
			Login: "login-a", Strikes: "rar-a", SuspendAfter: 20, SuspendFor: 15 * time.Minute,
			Scopes: []entities.EPPFailedCreateScope{
				{Key: "reg:rar-a", Budget: gold.Limits.FailedCreates},
				{Key: "fam:acme", Budget: famPolicy.Limits.FamilyFailedCreates},
			},
		}, *failed.asked)
	})
	t.Run("the first strikes hold the create a second each, on top of any rate wait", func(t *testing.T) {
		d, _ := run(entities.EPPFailedCreateStanding{Strikes: 3}, entities.EPPRateTake{Taken: true, Wait: 200 * time.Millisecond})
		assert.Equal(t, Decision{Action: Delay, Rule: RuleFailedCreates, Wait: 3200 * time.Millisecond}, d)
		d, _ = run(entities.EPPFailedCreateStanding{Strikes: DefaultEscalation.TarpitStrikes}, entities.EPPRateTake{Taken: true})
		assert.Equal(t, 10*time.Second, d.Wait)
	})
	t.Run("then they close the session", func(t *testing.T) {
		d, _ := run(entities.EPPFailedCreateStanding{Strikes: DefaultEscalation.TarpitStrikes + 1, Over: 1}, entities.EPPRateTake{Taken: true})
		assert.Equal(t, Close, d.Action)
		assert.Equal(t, 2502, d.Code)
		assert.Contains(t, d.Msg, "the registrar family's creates of registered names are over their budget")
		assert.Equal(t, "failed-creates; scope=family; suspended-for-ms=0", d.Reason)
		assert.False(t, d.Suspended)
	})
	t.Run("and a suspended login is closed", func(t *testing.T) {
		d, _ := run(entities.EPPFailedCreateStanding{Suspended: 15 * time.Minute, NewlySuspended: true, Strikes: 21}, entities.EPPRateTake{Taken: true})
		assert.Equal(t, Close, d.Action)
		assert.True(t, d.Suspended, "newly: the caller ends the login's other sessions and raises the alert")
		assert.Contains(t, d.Msg, "suspended for 15m0s")
		assert.Equal(t, "failed-creates; scope=registrar; suspended-for-ms=900000", d.Reason)
	})
	t.Run("other commands are not assessed", func(t *testing.T) {
		failed := &fakeFailed{}
		NewEnforcer(&fakeRates{take: entities.EPPRateTake{Taken: true}}, failed, &fakeOutcomes{}).Enforce(ctx, who, inFamily, Command{Name: "create", Object: "host"})
		assert.Nil(t, failed.asked)
	})
}

func TestRecorded(t *testing.T) {
	ctx := context.Background()
	gold := &entities.EPPPolicy{Name: "gold", Limits: entities.DefaultEPPPolicyLimits()}
	rp := entities.EPPRegistrarPolicy{Policy: gold, Family: &entities.RegistrarFamily{Name: "acme"}, FamilyPolicy: gold}
	failed, outcomes := &fakeFailed{}, &fakeOutcomes{}
	e := NewEnforcer(&fakeRates{}, failed, outcomes)
	create := Command{Name: "create", Object: "domain"}

	for _, code := range []int{1000, 1001, 2302, 2302, 2201, 2306, 2400} {
		e.Recorded(ctx, "rar-a", rp, create, code)
	}
	e.Recorded(ctx, "rar-a", rp, Command{Name: "create", Object: "host"}, 2302)
	assert.Equal(t, []bool{false, false, true, true}, failed.failed, "created, or refused because the name is registered; nothing else")
	assert.Equal(t, []string{"reg:rar-a", "fam:acme"}, failed.recorded)
	assert.Equal(t, 2, outcomes.failed)
	assert.Equal(t, 2, outcomes.succeeded)
}
