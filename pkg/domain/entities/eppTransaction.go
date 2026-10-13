package entities

import (
	"net/netip"
	"strings"
	"time"
)

// EPPTransaction is one EPP command and its response, as the audit log
// records it (docs/epp/ARCHITECTURE.md §6). It is a durable record of what a
// registrar asked for and what the registry answered, keyed by svTRID; it is
// not telemetry (INV-01), so it is never sampled.
//
// The frames are stored redacted: passwords (<pw>, <newPW>) and object
// authorization information (<authInfo>) never reach the log.
type EPPTransaction struct {
	SvTRID string
	ClTRID string
	// SessionID and Node identify the session that sent the command.
	SessionID string
	Node      string
	// ClID and LoginID are who the session had logged in as. For a <login>,
	// LoginID is the login it presented, whether or not it was accepted.
	ClID     ClIDType
	LoginID  ClIDType
	ClientIP netip.Addr
	// Command names the command as the metrics do: "login",
	// "domain:check", and so on.
	Command string
	// Objects are the object identifiers the command names, such as the
	// domain names in a <check>.
	Objects    []string
	ResultCode int
	ReceivedAt time.Time
	Duration   time.Duration
	// Request is the redacted command frame. It is empty when the frame
	// could not be parsed: a frame that cannot be parsed cannot be redacted.
	Request string
	// Response is the redacted response frame. It may be left out of
	// queries by configuration.
	Response string
}

// EPPCommandIsQuery reports whether a command, named as in
// EPPTransaction.Command, only reads: <hello>, <check>, <info> and
// <poll op="req">, plus frames that never named a command. Everything else
// is a transform, and its audit record is written before the response is
// sent.
func EPPCommandIsQuery(command string, pollReq bool) bool {
	switch command {
	case "hello", "unknown":
		return true
	case "poll":
		return pollReq
	}
	return strings.HasSuffix(command, ":check") || strings.HasSuffix(command, ":info")
}
