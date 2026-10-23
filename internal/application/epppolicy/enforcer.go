// Package epppolicy decides, for each EPP command, what the registrar's
// policy allows (docs/epp/ARCHITECTURE.md §5, ADR 0019). Decide applies
// the rules that need nothing but the login, its policy and the command,
// as a pure function; Enforcer adds the command rates and the create
// quota, which are counted across every node. The EPP adapter carries out
// the decision.
package epppolicy

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Class is a command's class, which the policy's rules are written for.
type Class string

const (
	// ClassSession is <hello>, <login> and <logout>: outside every rule.
	ClassSession Class = "session"
	// ClassCheck is <check>, metered per name.
	ClassCheck Class = "check"
	// ClassInfo is <info> and <transfer op="query">: reading an object.
	ClassInfo Class = "info"
	// ClassTransform is every command that changes the registry: create,
	// update, delete, renew, and transfer request, approve, reject and
	// cancel.
	ClassTransform Class = "transform"
	// ClassPoll is <poll>, the registrar's own message queue.
	ClassPoll Class = "poll"
)

// Command is what Decide needs to know about a command.
type Command struct {
	// Name is <command>'s first child: check, info, create, poll, ...
	Name string
	// Op is the op attribute of <transfer> and <poll>, if any.
	Op string
	// Object is the object type an object command is about: domain, host
	// or contact.
	Object string
	// Objects is how many objects the command names: the names of a check.
	Objects int
}

// ClassOf is c's class.
func ClassOf(c Command) Class {
	switch c.Name {
	case "check":
		return ClassCheck
	case "info":
		return ClassInfo
	case "transfer":
		if c.Op == "query" {
			return ClassInfo
		}
		return ClassTransform
	case "create", "update", "delete", "renew":
		return ClassTransform
	case "poll":
		return ClassPoll
	default:
		return ClassSession
	}
}

// Action is what Decide says to do with a command.
type Action int

const (
	// Allow lets the command through to its handler.
	Allow Action = iota
	// Reject answers the command with Code and Msg; the session continues.
	Reject
	// Delay holds the command for Wait, then lets it through.
	Delay
)

func (a Action) String() string {
	switch a {
	case Reject:
		return "reject"
	case Delay:
		return "delay"
	default:
		return "allow"
	}
}

// EPP result codes the rules answer with (RFC 5730 §3).
const (
	codeAuthorizationError   = 2201
	codeParameterPolicyError = 2306
	codeDataPolicyViolation  = 2308
)

// Rule names, for epp_policy_decisions_total{rule}.
const (
	RuleNone       = "none"
	RuleRole       = "role"
	RuleCheckNames = "check_names"
	// RuleRate is the registrar's bucket for the command's class.
	RuleRate = "rate"
	// RuleFamilyRate is the family's bucket for the command's class.
	RuleFamilyRate = "family_rate"
	// RuleCreateQuota is the registrar's domain creates per day.
	RuleCreateQuota = "create_quota"
)

// Decision is Decide's answer. Rule names the rule that decided.
type Decision struct {
	Action Action
	Rule   string
	Code   int
	Msg    string
	// Wait is how long a Delay holds the command.
	Wait time.Duration
	// Reason, for a rate refusal, is a machine-readable reason for the
	// response's <extValue>: "rate-limit; class=...; scope=...;
	// retry-after-ms=...".
	Reason string
}

// Decide applies the policy's rules to a command of a logged-in session, in
// order:
//
//  1. role: a readonly login may not transform (2201). It may check,
//     read, query transfers and poll.
//  2. check_names: a check may name at most the policy's CheckNames
//     objects (2306).
//
// Commands outside every rule (session commands) are allowed.
func Decide(role entities.EPPLoginRole, limits entities.EPPPolicyLimits, c Command) Decision {
	class := ClassOf(c)
	switch {
	case class == ClassTransform && role == entities.EPPLoginRoleReadonly:
		return Decision{Action: Reject, Rule: RuleRole, Code: codeAuthorizationError,
			Msg: "Authorization error; this login is read-only and cannot change the registry"}
	case class == ClassCheck && c.Objects > limits.CheckNames:
		return Decision{Action: Reject, Rule: RuleCheckNames, Code: codeParameterPolicyError,
			Msg: fmt.Sprintf("Parameter value policy error; at most %d names per check", limits.CheckNames)}
	}
	return Decision{Action: Allow, Rule: RuleNone}
}

// MaxRateDelay is the longest a command is held for its tokens. A command
// that would wait longer is refused instead, and takes nothing. Holding a
// command slows a session down to its rate without failing anything; a
// client that runs many sessions at once past its rate gets refusals.
const MaxRateDelay = time.Second

// A session refused for its rate RefusalsBeforeClose times within
// RefusalWindow is closed with 2502: its client is not backing off.
const (
	RefusalsBeforeClose = 10
	RefusalWindow       = 10 * time.Second
)

// RefusalStreak counts one session's rate refusals. The zero value is
// ready; it is not safe for concurrent use (a session runs one command at
// a time).
type RefusalStreak struct{ times []time.Time }

// Record counts a refusal at now and reports whether the session has now
// had RefusalsBeforeClose within RefusalWindow.
func (r *RefusalStreak) Record(now time.Time) bool {
	kept := r.times[:0]
	for _, t := range r.times {
		if now.Sub(t) < RefusalWindow {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	r.times = kept
	return len(r.times) >= RefusalsBeforeClose
}

// quotaTTL keeps a day's create count a day past its end, for diagnosis.
const quotaTTL = 48 * time.Hour

// Enforcer decides commands under every rule: Decide's, then the command
// rates, then the daily create quota (docs/epp/ARCHITECTURE.md §5).
type Enforcer struct {
	rates interfaces.EPPRateLimiter
	now   func() time.Time
}

// NewEnforcer returns an enforcer counting in rates.
func NewEnforcer(rates interfaces.EPPRateLimiter) *Enforcer {
	return &Enforcer{rates: rates, now: time.Now}
}

// Cost is how many tokens c takes from its class's buckets: one per name
// for a check, one for any other command.
func Cost(c Command) int {
	if ClassOf(c) == ClassCheck {
		return max(c.Objects, 1)
	}
	return 1
}

// RateFor is r's rate for class.
func RateFor(r entities.EPPCommandRates, class Class) entities.EPPRate {
	switch class {
	case ClassCheck:
		return r.Check
	case ClassInfo:
		return r.Info
	case ClassTransform:
		return r.Transform
	default:
		return r.Poll
	}
}

// Buckets are the buckets a command of class takes from: the registrar's,
// and its family's when it is in one.
func Buckets(clid entities.ClIDType, rp entities.EPPRegistrarPolicy, class Class) []entities.EPPRateBucket {
	out := []entities.EPPRateBucket{{Key: "reg:" + clid.String() + ":" + string(class), Rate: RateFor(rp.Policy.Limits.Rates, class)}}
	if rp.Family != nil {
		out = append(out, entities.EPPRateBucket{Key: "fam:" + rp.Family.Name + ":" + string(class), Rate: RateFor(rp.FamilyPolicy.Limits.FamilyRates, class)})
	}
	return out
}

// QuotaKey is the registrar's create count for the UTC day of now.
func QuotaKey(clid entities.ClIDType, now time.Time) string {
	return "creates:" + clid.String() + ":" + now.UTC().Format(time.DateOnly)
}

// Enforce decides a logged-in session's command, in order:
//
//  1. Decide's rules: the login's role, the check's names. A check may
//     carry no more names than its family's check bucket holds either, so
//     a check the registrar's policy allows but its family's bucket could
//     never pay for is refused for its size (2306), not as a rate that
//     would never be met.
//  2. Command rates: the command takes Cost tokens from the registrar's
//     bucket for its class and, if it is in a family, the family's, both
//     at once. If they are there within MaxRateDelay it is held until
//     then (Delay); if not it is refused with 2306, with Reason, and takes
//     nothing. Closing a session that keeps being refused
//     (RefusalStreak) is the caller's, which holds the session.
//  3. The create quota: a domain create counts against the registrar's
//     creates for the UTC day, if its policy caps them. Over the cap it is
//     refused with 2308.
//
// release, when not nil, gives the create back to the quota; the caller
// calls it if the create does not succeed. Counting fails open: the
// limiter falls back to the node, and an error from it lets the command
// through.
func (e *Enforcer) Enforce(ctx context.Context, clid entities.ClIDType, role entities.EPPLoginRole, rp entities.EPPRegistrarPolicy, c Command) (d Decision, release func(context.Context)) {
	limits := rp.Policy.Limits
	if rp.Family != nil {
		limits.CheckNames = min(limits.CheckNames, rp.FamilyPolicy.Limits.FamilyRates.Check.Burst)
	}
	if d := Decide(role, limits, c); d.Action != Allow {
		return d, nil
	}
	class := ClassOf(c)
	if class == ClassSession {
		return Decision{Action: Allow, Rule: RuleNone}, nil
	}
	d = Decision{Action: Allow, Rule: RuleNone}
	buckets := Buckets(clid, rp, class)
	take, err := e.rates.Take(ctx, buckets, Cost(c), MaxRateDelay)
	if err == nil {
		rule, whose := RuleRate, "the registrar's"
		if take.Limiting > 0 {
			rule, whose = RuleFamilyRate, "the registrar family's"
		}
		switch {
		case !take.Taken:
			retry := roundUp(take.Wait)
			scope := "registrar"
			if rule == RuleFamilyRate {
				scope = "family"
			}
			return Decision{Action: Reject, Rule: rule, Code: codeParameterPolicyError,
				Msg:    fmt.Sprintf("Parameter value policy error; %s %s rate is exceeded, try again in %s", whose, class, retry),
				Reason: fmt.Sprintf("rate-limit; class=%s; scope=%s; retry-after-ms=%d", class, scope, retry.Milliseconds())}, nil
		case take.Wait > 0:
			d = Decision{Action: Delay, Rule: rule, Wait: take.Wait}
		}
	}
	if c.Name == "create" && c.Object == "domain" && limits.CreatesPerDay > 0 {
		release, ok, err := e.rates.ReserveQuota(ctx, QuotaKey(clid, e.now()), limits.CreatesPerDay, quotaTTL)
		if err == nil && !ok {
			return Decision{Action: Reject, Rule: RuleCreateQuota, Code: codeDataPolicyViolation,
				Msg: fmt.Sprintf("Data management policy violation; the registrar may create %d domains a day (UTC), and has", limits.CreatesPerDay)}, nil
		}
		return d, release
	}
	return d, nil
}

// roundUp is w to the next tenth of a second, at least 0.1s, for a client
// to read.
func roundUp(w time.Duration) time.Duration {
	return max(((w+99*time.Millisecond)/(100*time.Millisecond))*(100*time.Millisecond), 100*time.Millisecond)
}
