package entities

import (
	"errors"
	"time"
)

// ErrPollMessageNotFound: an acknowledgement of a message that is not in
// the registrar's queue (RFC 5730 §2.9.2.3: 2303). Another registrar's
// message is not in it.
var ErrPollMessageNotFound = errors.New("no such message in the registrar's queue")

// PollMessage is a service message in a registrar's queue (RFC 5730
// §2.9.2.3). A registrar reads the oldest with <poll op="req"> and removes
// it with <poll op="ack">. A message belongs to one registrar, and is seen
// and acknowledged only by it (ADR 0006).
type PollMessage struct {
	// ID is the message's identifier (<msgQ id>), increasing in the order
	// messages are queued.
	ID int64
	// ClID is the registrar whose queue it is in.
	ClID ClIDType
	// CreatedAt is when it was queued (<qDate>).
	CreatedAt time.Time
	// Message is its human-readable text (<msg>).
	Message string
	// Transfer is the transfer the message is about, as it stood when the
	// message was queued: its <domain:trnData>. Every message is about a
	// transfer for now.
	Transfer *DomainTransfer
}

// Poll message texts, by what the transfer has become.
const (
	PollMsgTransferRequested       = "Transfer requested"
	PollMsgTransferApproved        = "Transfer approved"
	PollMsgTransferRejected        = "Transfer rejected"
	PollMsgTransferCancelled       = "Transfer cancelled"
	PollMsgTransferServerApproved  = "Transfer approved by the registry"
	PollMsgTransferServerCancelled = "Transfer cancelled by the registry"
	// pollMsgReplaceContacts is added to the gaining registrar's message on
	// approval: the domain keeps the losing registrar's contacts (ADR 0006,
	// amended 2026-10-22).
	pollMsgReplaceContacts = "; the domain's contacts are still the losing registrar's: replace them with your own"
)

// NewTransferPollMessages returns the messages a step of transfer t queues:
// one for each of its two registrars that did not take the step. actor is
// the registrar that did, or empty when the registry did, in which case
// both are told. t is copied as it stands.
func NewTransferPollMessages(t *DomainTransfer, actor ClIDType, now time.Time) []*PollMessage {
	var text string
	switch t.Status {
	case TransferStatusPending:
		text = PollMsgTransferRequested
	case TransferStatusClientApproved:
		text = PollMsgTransferApproved
	case TransferStatusClientRejected:
		text = PollMsgTransferRejected
	case TransferStatusClientCancelled:
		text = PollMsgTransferCancelled
	case TransferStatusServerApproved:
		text = PollMsgTransferServerApproved
	case TransferStatusServerCancelled:
		text = PollMsgTransferServerCancelled
	}
	var out []*PollMessage
	for _, to := range []ClIDType{t.LosingClID, t.RequestingClID} {
		if to == actor {
			continue
		}
		msg := text
		if to == t.RequestingClID && t.Status.IsApproved() {
			msg += pollMsgReplaceContacts
		}
		snapshot := *t
		out = append(out, &PollMessage{ClID: to, CreatedAt: now, Message: msg, Transfer: &snapshot})
	}
	return out
}
