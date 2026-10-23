// Package epppolicy decides, for each EPP command, what the registrar's
// policy allows (docs/epp/ARCHITECTURE.md §5, ADR 0019). Decide is a pure
// function of the login, its policy and the command, so every rule can be
// tested without a server; the EPP adapter carries out the decision.
package epppolicy

import (
	"fmt"

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
)

func (a Action) String() string {
	if a == Reject {
		return "reject"
	}
	return "allow"
}

// EPP result codes the rules answer with (RFC 5730 §3).
const (
	codeAuthorizationError   = 2201
	codeParameterPolicyError = 2306
)

// Rule names, for epp_policy_decisions_total{rule}.
const (
	RuleNone       = "none"
	RuleRole       = "role"
	RuleCheckNames = "check_names"
)

// Decision is Decide's answer. Rule names the rule that decided.
type Decision struct {
	Action Action
	Rule   string
	Code   int
	Msg    string
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
