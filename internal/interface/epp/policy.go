package epp

import (
	"context"
	"time"
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
}

// commandPolicy decides, after a command has been validated and before its
// handler runs, whether it may proceed (docs/epp/ARCHITECTURE.md §5). It
// runs on the session's goroutine and must not block for long: a delay is
// returned, not slept.
type commandPolicy interface {
	decide(ctx context.Context, cmd *command) policyDecision
}

// allowAll is the policy until the policy engine arrives (Phase 5): every
// command proceeds.
type allowAll struct{}

func (allowAll) decide(context.Context, *command) policyDecision {
	return policyDecision{action: policyAllow, rule: "none"}
}

// applyPolicy asks the policy about cmd, carries out its decision, and
// reports whether the command may go on to its handler.
func (s *Server) applyPolicy(ctx context.Context, cmd *command) bool {
	d := s.policy.decide(ctx, cmd)
	s.metrics.policyDecisions.WithLabelValues(d.rule, d.action.String()).Inc()
	switch d.action {
	case policyAllow:
		return true
	case policyDelay:
		t := time.NewTimer(d.wait)
		defer t.Stop()
		select {
		case <-t.C:
			return true
		case <-ctx.Done():
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
