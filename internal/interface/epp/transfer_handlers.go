package epp

import (
	"context"
	"strconv"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/epp/schema"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// domainTransfer answers <transfer> on a domain (RFC 5731 §3.2.4 and
// §3.1.3). Who may use which op, and the rules of each, are
// services.EPPDomainService's; entities.DomainTransfer describes the
// process. A request answers 1001, the transfer pending; the other ops
// answer 1000. Each answer carries the transfer as <domain:trnData>.
func (s *Server) domainTransfer(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	obj := cmd.doc.FindElement("/epp/command/transfer/*")
	// Only a password can be checked: the registry keeps nothing else
	// (2102, as on info, create and update).
	if obj.FindElement("authInfo/ext") != nil {
		s.refuse(cmd, entities.ErrEPPAuthInfoExt)
		return
	}
	var authInfo *string
	if pw := obj.FindElement("authInfo/pw"); pw != nil {
		v := schema.NormalizedStringValue(pw.Text())
		authInfo = &v
	}
	name := cmd.objects[0]
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()

	var tr *entities.DomainTransfer
	code := epplib.StatusSuccess
	switch cmd.op() {
	case "request":
		c := commands.EPPTransferDomainCommand{Name: name, AuthInfo: authInfo}
		if p := obj.SelectElement("period"); p != nil {
			c.Period, _ = strconv.Atoi(schema.TokenValue(p.Text())) // the schema has checked it is 1 to 99
			c.PeriodUnit = schema.TokenValue(p.SelectAttrValue("unit", ""))
		}
		tr, err = s.domains.TransferRequest(ctx, scope, c)
		code = epplib.StatusActionPending
	case "query":
		tr, err = s.domains.TransferQuery(ctx, scope, name, authInfo)
	case "approve":
		tr, err = s.domains.TransferApprove(ctx, scope, name)
	case "reject":
		tr, err = s.domains.TransferReject(ctx, scope, name)
	case "cancel":
		tr, err = s.domains.TransferCancel(ctx, scope, name)
	}
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(code, "", transferData(tr))
}

// poll answers <poll> (RFC 5730 §2.9.2.3) from the session's registrar's
// own queue. op="req" answers 1301 with the oldest message and the queue's
// size, or 1300 when the queue is empty; op="ack" removes the message
// msgID names and answers 1000 with how many remain.
func (s *Server) poll(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	switch cmd.op() {
	case "req":
		msg, count, err := s.polls.Request(ctx, scope)
		switch {
		case err != nil:
			s.refuse(cmd, err)
		case msg == nil:
			cmd.respond(epplib.StatusNoMessage, "", nil)
		default:
			var resData *etree.Element
			if msg.Transfer != nil {
				resData = transferData(msg.Transfer)
			}
			cmd.respondWithQueue(epplib.StatusAckToDequeue, messageQueue(count, msg), resData)
		}
	case "ack":
		id := schema.TokenValue(cmd.doc.FindElement("/epp/command/poll").SelectAttrValue("msgID", ""))
		if id == "" {
			cmd.respond(epplib.StatusMissingParameter, epplib.StatusText(epplib.StatusMissingParameter)+"; msgID names the message to acknowledge", nil)
			return
		}
		count, err := s.polls.Ack(ctx, scope, id)
		if err != nil {
			s.refuse(cmd, err)
			return
		}
		q := etree.NewElement("msgQ")
		q.CreateAttr("count", strconv.FormatInt(count, 10))
		q.CreateAttr("id", id)
		cmd.respondWithQueue(epplib.StatusSuccess, q, nil)
	}
}

// transferData builds <domain:trnData> (RFC 5731 §3.2.4). exDate is left
// out when the transfer changes no expiry: rejected or cancelled.
func transferData(t *entities.DomainTransfer) *etree.Element {
	trn := etree.NewElement("domain:trnData")
	trn.CreateAttr("xmlns:domain", nsDomain)
	trn.CreateElement("domain:name").SetText(t.DomainName.String())
	trn.CreateElement("domain:trStatus").SetText(string(t.Status))
	trn.CreateElement("domain:reID").SetText(t.RequestingClID.String())
	trn.CreateElement("domain:reDate").SetText(eppTime(t.RequestedAt))
	trn.CreateElement("domain:acID").SetText(t.ActingClID.String())
	trn.CreateElement("domain:acDate").SetText(eppTime(t.ActionDate))
	if !t.ExpiryDate.IsZero() {
		trn.CreateElement("domain:exDate").SetText(eppTime(t.ExpiryDate))
	}
	return trn
}

// messageQueue builds the <msgQ> of a <poll op="req"> response (RFC 5730
// §2.6): the queue's size, and the message's ID, date and text.
func messageQueue(count int64, m *entities.PollMessage) *etree.Element {
	q := etree.NewElement("msgQ")
	q.CreateAttr("count", strconv.FormatInt(count, 10))
	q.CreateAttr("id", strconv.FormatInt(m.ID, 10))
	q.CreateElement("qDate").SetText(eppTime(m.CreatedAt))
	q.CreateElement("msg").SetText(m.Message)
	return q
}
