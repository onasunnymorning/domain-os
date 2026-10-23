package epp

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/epppolicy"
)

// policyAction is what the policy decides for a command.
type policyAction int

const (
	policyAllow policyAction = iota
	// policyDelay holds the command for a while, then lets it through: a
	// tarpit.
	policyDelay
	// policyReject answers the command with an error and keeps the session.
	policyReject
	// policyClose answers the command with an error and ends the session.
	policyClose
)

func (a policyAction) String() string {
	switch a {
	case policyAllow:
		return "allow"
	case policyDelay:
		return "delay"
	case policyReject:
		return "reject"
	case policyClose:
		return "close"
	default:
		return "unknown"
	}
}

// policyDecision is the policy's answer for one command. rule names the
// rule that decided, for epp_policy_decisions_total.
type policyDecision struct {
	action policyAction
	rule   string
	// wait is how long a delay holds the command.
	wait time.Duration
	// code and msg answer a rejected or closed command.
	code int
	msg  string
	// release gives back what the command counted against a quota, if it
	// does not succeed.
	release func(context.Context)
}

// commandPolicy decides, after a command has been validated and before its
// handler runs, whether it may proceed (docs/epp/ARCHITECTURE.md §5). It
// runs on the session's goroutine and must not block for long: a delay is
// returned, not slept.
type commandPolicy interface {
	decide(ctx context.Context, cmd *command) policyDecision
}

// registrarPolicies is the policy engine (docs/epp/ARCHITECTURE.md §5): a
// logged-in session's commands are decided by its registrar's EPP policy
// (epppolicy.Enforcer), from the set the node last loaded, with the
// command rates and create quotas counted across every node. Commands
// before login are outside it.
type registrarPolicies struct{ s *Server }

func (p registrarPolicies) decide(ctx context.Context, cmd *command) policyDecision {
	auth := cmd.session.auth.Load()
	if auth == nil {
		return policyDecision{action: policyAllow, rule: epppolicy.RuleNone}
	}
	c := epppolicy.Command{Name: cmd.frame.command, Op: cmd.op(), Object: objectType(cmd.frame.objectNS), Objects: len(cmd.objects)}
	if epppolicy.ClassOf(c) == epppolicy.ClassCheck {
		p.s.metrics.checkNames.Observe(float64(c.Objects))
	}
	rp := p.s.policies.Load().ForRegistrar(auth.ClID)
	d, release := p.s.enforcer.Enforce(ctx, auth.ClID, auth.Role, rp, c)
	switch d.Action {
	case epppolicy.Reject:
		return policyDecision{action: policyReject, rule: d.Rule, code: d.Code, msg: d.Msg}
	case epppolicy.Delay:
		return policyDecision{action: policyDelay, rule: d.Rule, wait: d.Wait, release: release}
	default:
		return policyDecision{action: policyAllow, rule: d.Rule, release: release}
	}
}

// objectType is the object an object namespace is for.
func objectType(ns string) string {
	switch ns {
	case nsDomain:
		return "domain"
	case nsHost:
		return "host"
	case nsContact:
		return "contact"
	default:
		return ""
	}
}

// applyPolicy asks the policy about cmd, carries out its decision, and
// reports whether the command may go on to its handler. A command that
// goes on carries the decision's release, for dispatch to call if it
// fails.
func (s *Server) applyPolicy(ctx context.Context, cmd *command) bool {
	d := s.policy.decide(ctx, cmd)
	s.metrics.policyDecisions.WithLabelValues(d.rule, d.action.String()).Inc()
	switch d.action {
	case policyAllow:
		cmd.release = d.release
		return true
	case policyDelay:
		s.metrics.policyDelay.Observe(d.wait.Seconds())
		t := time.NewTimer(d.wait)
		defer t.Stop()
		select {
		case <-t.C:
			cmd.release = d.release
			return true
		case <-ctx.Done():
			if d.release != nil {
				d.release(context.WithoutCancel(ctx))
			}
			cmd.respond(2400, "Command failed; the session ended while the command was held", nil)
			cmd.closeAfterResponse()
			return false
		}
	case policyClose:
		cmd.respond(d.code, d.msg, nil)
		cmd.closeAfterResponse()
		return false
	default:
		cmd.respond(d.code, d.msg, nil)
		return false
	}
}

// releaseIfFailed gives back what a command counted against a quota if it
// did not succeed (a result code of 2000 or more).
func (s *Server) releaseIfFailed(ctx context.Context, cmd *command) {
	if cmd.release == nil || cmd.code < 2000 {
		return
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedStoreTimeout)
	defer cancel()
	cmd.release(rctx)
}
