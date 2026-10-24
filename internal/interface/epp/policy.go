package epp

import (
	"context"
	"fmt"
	"time"

	"github.com/beevik/etree"

	"github.com/onasunnymorning/domain-os/internal/application/epppolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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
	// reason, when set, goes in the response's <extValue> (RFC 5730
	// §2.6), so a client can tell what refused it.
	reason string
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
	d, release := p.s.enforcer.Enforce(ctx, epppolicy.Caller{ClID: auth.ClID, Login: string(auth.LoginID), Role: auth.Role}, rp, c)
	if epppolicy.IsDomainCreate(c) {
		cmd.recorded = func(ctx context.Context, code int) { p.s.enforcer.Recorded(ctx, auth.ClID, rp, c, code) }
	}
	switch d.Action {
	case epppolicy.Close:
		if d.Suspended {
			p.s.loginSuspended(ctx, auth, cmd.session, d)
		}
		return policyDecision{action: policyClose, rule: d.Rule, code: d.Code, msg: d.Msg, reason: d.Reason}
	case epppolicy.Reject:
		if (d.Rule == epppolicy.RuleRate || d.Rule == epppolicy.RuleFamilyRate) && cmd.session.rateRefusals.Record(time.Now()) {
			// The client is not backing off: end the session.
			return policyDecision{action: policyClose, rule: d.Rule, code: 2502,
				msg:    fmt.Sprintf("Session limit exceeded; server closing connection; %d command rate refusals within %s", epppolicy.RefusalsBeforeClose, epppolicy.RefusalWindow),
				reason: d.Reason}
		}
		return policyDecision{action: policyReject, rule: d.Rule, code: d.Code, msg: d.Msg, reason: d.Reason}
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
			cmd.respond(2400, "Command failed; the session ended while the command was held", nil)
			cmd.closeAfterResponse()
			cmd.release = d.release
			s.releaseIfFailed(ctx, cmd)
			return false
		}
	case policyClose:
		cmd.respond(d.code, d.msg, nil)
		addReason(cmd, d.reason)
		cmd.closeAfterResponse()
		return false
	default:
		cmd.respond(d.code, d.msg, nil)
		addReason(cmd, d.reason)
		return false
	}
}

// addReason adds reason to the response's result as an <extValue> (RFC
// 5730 §2.6): <value><undef/></value>, since no one element of the command
// is at fault, and the reason. Nothing is added for an empty reason.
func addReason(cmd *command, reason string) {
	if reason == "" {
		return
	}
	result := cmd.response.FindElement("/epp/response/result")
	ext := etree.NewElement("extValue")
	ext.CreateElement("value").CreateElement("undef")
	ext.CreateElement("reason").SetText(reason)
	result.AddChild(ext)
}

// eventSource is the outbox source of what the EPP server records on its
// own, as for <newPW>.
const eventSource = "domain-os/epp"

// loginSuspended raises the alert for a login just suspended for failed
// creates and ends its other sessions on this node; the session that was
// suspended is closed by its command's answer. Other nodes refuse the
// login's next domain create and its next login.
//
// It also records an epp.login.suspended event, with the system as actor.
// The suspension holds whether or not the event is written: Redis holds
// the suspension, and the event is its record, so a failed write is logged
// and counted (epp_events_lost_total), not a reason to let the login go on.
func (s *Server) loginSuspended(ctx context.Context, auth *entities.EPPAuthenticatedLogin, from *session, d epppolicy.Decision) {
	s.metrics.loginsSuspended.WithLabelValues(string(auth.ClID)).Inc()
	s.logger.Error("EPP login suspended for creating registered names past the failed-create budget",
		"clid", auth.ClID, "login", auth.LoginID, "connection_id", from.id, "client_ip", from.clientIP,
		"suspended_for", s.enforcer.Escalation().SuspendFor)
	for _, sess := range s.sessions.all() {
		if sess != from && sess.loggedInAs() == string(auth.LoginID) {
			s.endSession(sess.id, endedSuspended)
		}
	}
	e := entities.NewDomainEvent(eventSource, entities.EventEPPLoginSuspended, string(auth.ClID),
		fmt.Sprintf("EPP login %s suspended for creating registered names past the failed-create budget", auth.LoginID),
		entities.EPPLoginSuspendedData{EPPLoginSuspension: d.Suspension, Strikes: d.Strikes, ClientIP: from.clientIP})
	e.Actor = "system"
	e.AfterState = d.Suspension
	ectx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedStoreTimeout*4)
	defer cancel()
	if err := s.events.Publish(ectx, e); err != nil {
		s.metrics.eventsLost.WithLabelValues(entities.EventEPPLoginSuspended).Inc()
		s.logger.Error("EPP login suspension event not written", "clid", auth.ClID, "login", auth.LoginID, "error", err)
	}
}

// afterHandler counts the outcome of a command that ran, and gives back
// what it counted against a quota if it failed.
func (s *Server) afterHandler(ctx context.Context, cmd *command) {
	if cmd.recorded != nil {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedStoreTimeout)
		cmd.recorded(rctx, cmd.code)
		cancel()
	}
	s.releaseIfFailed(ctx, cmd)
}

// releaseIfFailed gives back what a command counted against a quota if it
// did not succeed (a result code of 2000 or more). The release is bounded,
// so a stalled store cannot hold the session.
func (s *Server) releaseIfFailed(ctx context.Context, cmd *command) {
	if cmd.release == nil || cmd.code < 2000 {
		return
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sharedStoreTimeout)
	defer cancel()
	cmd.release(rctx)
}
