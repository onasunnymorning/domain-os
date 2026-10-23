package epp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/epp/schema"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// refusals map the registry's refusals of object commands to EPP result
// codes. A refusal's own text, which names the object or rule involved and
// never a secret, follows the standard message. Anything not listed is
// 2400, with the detail kept in the log.
var refusals = []struct {
	err  error
	code int
}{
	{entities.ErrInvalidDomain, epplib.StatusValueSyntaxError},
	{entities.ErrDomainNotFound, epplib.StatusObjectDoesNotExist},
	{entities.ErrContactNotFound, epplib.StatusObjectDoesNotExist},
	{entities.ErrHostNotFound, epplib.StatusObjectDoesNotExist},
	{entities.ErrEPPNotSponsor, epplib.StatusAuthorizationError},
	{services.ErrRegistrarNotAccredited, epplib.StatusAuthorizationError},
	{services.ErrDomainExists, epplib.StatusObjectExists},
	{services.ErrDomainBlocked, epplib.StatusDataManagementPolicyViolation},
	{entities.ErrEPPInvalidPeriod, epplib.StatusValueRangeError},
	{entities.ErrEPPContactRequired, epplib.StatusMissingParameter},
	{entities.ErrEPPContactProhibited, epplib.StatusParameterPolicyError},
	{entities.ErrEPPTLDNotServed, epplib.StatusParameterPolicyError},
	{entities.ErrEPPTLDNotOpen, epplib.StatusParameterPolicyError},
	{entities.ErrLabelNotValidInPhase, epplib.StatusParameterPolicyError},
	{entities.ErrMaxHostsPerDomainExceeded, epplib.StatusParameterPolicyError},
	{entities.ErrDuplicateHost, epplib.StatusParameterPolicyError},
	{entities.ErrEPPMaxSigLife, epplib.StatusUnimplementedOption},
	{entities.ErrEPPAuthInfoExt, epplib.StatusUnimplementedOption},
	{entities.ErrEPPKeyData, epplib.StatusParameterPolicyError},
	{entities.ErrEPPDSAlgorithm, epplib.StatusParameterPolicyError},
	{entities.ErrEPPDSDigestType, epplib.StatusParameterPolicyError},
	{entities.ErrEPPTooManyDS, epplib.StatusParameterPolicyError},
	{entities.ErrDuplicateDSRecord, epplib.StatusParameterPolicyError},
	{entities.ErrInvalidDSRecord, epplib.StatusValueSyntaxError},
	{entities.ErrEPPHostAttributes, epplib.StatusParameterPolicyError},
	{entities.ErrEPPPeriodNotWholeYears, epplib.StatusParameterPolicyError},
	{entities.ErrEPPContactTypeMissing, epplib.StatusMissingParameter},
	{entities.ErrEPPDuplicateContactType, epplib.StatusParameterPolicyError},
	{entities.ErrInvalidAuthInfo, epplib.StatusParameterPolicyError},
	// Domain update, renew and delete.
	{entities.ErrEPPDomainContactNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPNameserverNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPDSNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPDNSSECUrgent, epplib.StatusUnimplementedOption},
	{entities.ErrEPPAuthInfoMismatch, epplib.StatusInvalidAuthorizationInformation},
	{entities.ErrAuthInfoNotSecure, epplib.StatusInvalidAuthorizationInformation},
	{entities.ErrEPPCurExpDate, epplib.StatusParameterPolicyError},
	{entities.ErrEPPSubordinateHosts, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrEPPSuperordinatePendingDelete, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrInBailiwickHostsMustHaveAddress, epplib.StatusParameterPolicyError},
	{entities.ErrDomainUpdateNotAllowed, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrDomainRenewExceedsMaxHorizon, epplib.StatusParameterPolicyError},
	{entities.ErrDomainRenewNotAllowed, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrDomainDeleteNotAllowed, epplib.StatusObjectStatusProhibitsOperation},
	// Domain transfer (RFC 5731 §3.2.4) and poll (RFC 5730 §2.9.2.3).
	{entities.ErrTransferToSponsor, epplib.StatusNotEligibleForTransfer},
	{entities.ErrTransferNotEligible, epplib.StatusNotEligibleForTransfer},
	{entities.ErrTransferPending, epplib.StatusObjectPendingTransfer},
	{entities.ErrTransferNotPending, epplib.StatusObjectNotPendingTransfer},
	{entities.ErrTransferProhibited, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrTransferExceedsMaxHorizon, epplib.StatusParameterPolicyError},
	{entities.ErrPollMessageNotFound, epplib.StatusObjectDoesNotExist},
	// Domain restore (RFC 3915 §4.2.5).
	{entities.ErrDomainRestoreNotAllowed, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrRestoreNotPending, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrRestoreWithChanges, epplib.StatusParameterPolicyError},
	{entities.ErrRestoreReportIncomplete, epplib.StatusMissingParameter},
	{entities.ErrRestoreReportTimes, epplib.StatusValueRangeError},
	// Host and contact commands (RFC 5732, RFC 5733).
	{entities.ErrEPPStatusNotClient, epplib.StatusParameterPolicyError},
	{entities.ErrEPPStatusAlreadySet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPStatusNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPObjectPending, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPDeleteProhibited, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPInvalidHostName, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidIP, epplib.StatusValueSyntaxError},
	{entities.ErrEPPHostAddressVersion, epplib.StatusValueSyntaxError},
	{entities.ErrHostAlreadyExists, epplib.StatusObjectExists},
	{entities.ErrEPPSuperordinateDomainNotFound, epplib.StatusObjectDoesNotExist},
	{entities.ErrEPPHostAddressNotPublic, epplib.StatusParameterPolicyError},
	{entities.ErrEPPExternalHostAddress, epplib.StatusParameterPolicyError},
	{entities.ErrDuplicateHostAddress, epplib.StatusParameterPolicyError},
	{entities.ErrMaxAddressesPerHostExceeded, epplib.StatusParameterPolicyError},
	{entities.ErrEPPHostAddressNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrHostUpdateProhibited, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPHostLinked, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrEPPHostLinkedByOthers, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrEPPHostNeedsAddress, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrContactAlreadyExists, epplib.StatusObjectExists},
	{entities.ErrInvalidEmail, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidCountryCode, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidE164Type, epplib.StatusValueSyntaxError},
	{entities.ErrEPPPhoneExtension, epplib.StatusUnimplementedOption},
	{entities.ErrEPPEmptyPhone, epplib.StatusParameterPolicyError},
	{entities.ErrEPPDuplicatePostalInfo, epplib.StatusParameterPolicyError},
	{entities.ErrEPPPostalInfoIncomplete, epplib.StatusMissingParameter},
	{entities.ErrInvalidASCIIInIntAddress, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidContactPostalInfo, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidPostalLineType, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidOptPostalLineType, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidPCType, epplib.StatusValueSyntaxError},
	{entities.ErrInvalidClIDType, epplib.StatusValueSyntaxError},
	{entities.ErrContactUpdateNotAllowed, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPContactLinked, epplib.StatusObjectAssociationProhibitsOperation},
}

// refuse answers cmd with the result code for err. An error that is not a
// known refusal is answered 2400, and logged.
func (s *Server) refuse(cmd *command, err error) {
	for _, r := range refusals {
		if errors.Is(err, r.err) {
			cmd.respond(r.code, epplib.StatusText(r.code)+"; "+firstLine(err.Error()), nil)
			return
		}
	}
	s.logger.Error("EPP command failed", "connection_id", cmd.session.id, "svtrid", cmd.svTRID,
		"command", cmd.frame.label(), "error", err)
	cmd.respond(epplib.StatusCommandFailed, "", nil)
}

// firstLine keeps a joined error's message on one line, as <msg> is.
func firstLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// registrarScope is the session's registrar as the scope ADR 0006 requires
// of commands on sponsored objects: from the authenticated login, never
// from the command.
func registrarScope(cmd *command) (entities.RegistrarClID, error) {
	return entities.NewRegistrarClID(cmd.session.registrar())
}

// withActor marks ctx as acting for the session's login, so the events a
// command writes name it, and correlates them with the command's svTRID.
func withActor(ctx context.Context, cmd *command) context.Context {
	ctx = appcontext.WithUserID(ctx, "epp:"+cmd.session.loggedInAs())
	ctx = appcontext.WithRegistrarID(ctx, cmd.session.registrar())
	return appcontext.WithCorrelationID(ctx, cmd.svTRID)
}

// domainInfo answers <domain:info> (RFC 5731 §3.1.2). The sponsor gets the
// full answer. Another registrar gets the limited view with the domain's
// transfer code, 2202 with a wrong one and 2201 with none (RFC 9154 §5.3,
// ADR 0006 as amended). Authorization information other than a password is
// 2102. The RGP statuses are added when the session logged in with RGP.
func (s *Server) domainInfo(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	name := cmd.doc.FindElement("/epp/command/info/*/name")
	hosts := name.SelectAttrValue("hosts", "all")
	// Only a password can be checked: the registry keeps nothing else
	// (2102, as on create and update).
	if cmd.doc.FindElement("/epp/command/info/*/authInfo/ext") != nil {
		s.refuse(cmd, entities.ErrEPPAuthInfoExt)
		return
	}
	// A code given (RFC 9154 §5.3): an empty <domain:pw/> is given, and
	// matches nothing.
	var authInfo *string
	if pw := cmd.doc.FindElement("/epp/command/info/*/authInfo/pw"); pw != nil {
		v := schema.NormalizedStringValue(pw.Text())
		authInfo = &v
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	info, err := s.domains.Info(ctx, scope, cmd.objects[0], authInfo)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	if info.Limited {
		cmd.respond(epplib.StatusSuccess, "", limitedInfoData(info.Domain))
		return
	}
	// Each extension's data is added only for a session that asked for it
	// at login.
	var secDNS, rgp *etree.Element
	if slices.Contains(cmd.session.extensions, nsSecDNS) {
		secDNS = secDNSInfoData(info.Domain)
	}
	if slices.Contains(cmd.session.extensions, nsRGP) {
		rgp = rgpInfoData(info.Domain, time.Now())
	}
	cmd.respond(epplib.StatusSuccess, "", infoData(info, hosts), secDNS, rgp)
}

// domainCreate answers <domain:create> (RFC 5731 §3.2.1). It reads the
// frame as the schema defines its values, and leaves every rule to the
// registry (services.EPPDomainService.Create), which applies them in one
// order.
func (s *Server) domainCreate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	create := cmd.doc.FindElement("/epp/command/create/*")
	c := commands.EPPCreateDomainCommand{Name: cmd.objects[0]}
	if p := create.SelectElement("period"); p != nil {
		c.Period, _ = strconv.Atoi(schema.TokenValue(p.Text())) // the schema has checked it is 1 to 99
		c.PeriodUnit = schema.TokenValue(p.SelectAttrValue("unit", ""))
	}
	if ns := create.SelectElement("ns"); ns != nil {
		c.HostAttributes = len(ns.SelectElements("hostAttr")) > 0
		for _, h := range ns.SelectElements("hostObj") {
			c.HostNames = append(c.HostNames, schema.TokenValue(h.Text()))
		}
	}
	if r := create.SelectElement("registrant"); r != nil {
		c.RegistrantID = schema.TokenValue(r.Text())
	}
	for _, el := range create.SelectElements("contact") {
		c.Contacts = append(c.Contacts, commands.EPPDomainContact{
			Type: schema.TokenValue(el.SelectAttrValue("type", "")),
			ID:   schema.TokenValue(el.Text()),
		})
	}
	if create.FindElement("authInfo/ext") != nil {
		s.refuse(cmd, entities.ErrEPPAuthInfoExt)
		return
	}
	if pw := create.FindElement("authInfo/pw"); pw != nil {
		// eppcom:pwAuthInfoType is a normalizedString: the password is the
		// text with tabs and line breaks as spaces, as it was validated.
		c.AuthInfo = schema.NormalizedStringValue(pw.Text())
	}
	readSecDNSCreate(cmd.doc, &c)

	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	dom, err := s.domains.Create(ctx, scope, c)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	code := epplib.StatusSuccess
	if dom.Status.PendingCreate {
		code = epplib.StatusActionPending
	}
	cmd.respond(code, "", createData(dom))
}

// domainUpdate answers <domain:update> (RFC 5731 §3.2.5), with a
// <secDNS:update> (RFC 5910 §5.2.5) if the session logged in with
// secDNS-1.1, or an <rgp:update> (RFC 3915 §4.2.5), a restore, if it
// logged in with RGP. A restore is answered with the domain's RGP statuses
// in <rgp:upData>, when it has any: pendingRestore after a request.
func (s *Server) domainUpdate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	update := cmd.doc.FindElement("/epp/command/update/*")
	add, rem, chg := update.SelectElement("add"), update.SelectElement("rem"), update.SelectElement("chg")
	secDNS := secDNSUpdate(cmd.doc)
	restore, err := rgpRestore(cmd.doc)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	if add == nil && rem == nil && chg == nil && secDNS == nil && restore == nil {
		// RFC 5731 §3.2.5: one of them is required unless the command is
		// extended.
		cmd.respond(epplib.StatusMissingParameter, "Required parameter missing; give <domain:add>, <domain:rem>, <domain:chg> or an extension", nil)
		return
	}
	c := commands.EPPUpdateDomainCommand{Name: cmd.objects[0], DNSSEC: secDNS, Restore: restore}
	if add != nil {
		c.AddHostNames, c.AddContacts, c.AddStatuses = domainAddRem(add, &c.HostAttributes)
	}
	if rem != nil {
		c.RemoveHostNames, c.RemoveContacts, c.RemoveStatuses = domainAddRem(rem, &c.HostAttributes)
	}
	if chg != nil {
		if r := chg.SelectElement("registrant"); r != nil {
			v := schema.TokenValue(r.Text())
			c.Registrant = &v
		}
		if a := chg.SelectElement("authInfo"); a != nil {
			switch {
			case a.SelectElement("null") != nil:
				c.AuthInfoNull = true
			case a.SelectElement("pw") != nil:
				v := schema.NormalizedStringValue(a.SelectElement("pw").Text())
				c.AuthInfo = &v
			default: // <domain:ext>: not a password, so nothing to hash
				s.refuse(cmd, entities.ErrEPPAuthInfoExt)
				return
			}
		}
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	dom, err := s.domains.Update(ctx, scope, c)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	if restore != nil {
		cmd.respond(epplib.StatusSuccess, "", nil, rgpData("rgp:upData", dom, time.Now()))
		return
	}
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// rgpRestore reads an <rgp:update> extension (RFC 3915 §4.2.5), or nil if
// the command has none. A report's free-text parts are kept as sent, XML
// included; its times are read as the schema defines them, and one the
// registry cannot hold is entities.ErrRestoreReportTimes.
func rgpRestore(doc *etree.Document) (*commands.EPPRestore, error) {
	ext := doc.FindElement("/epp/command/extension")
	if ext == nil {
		return nil, nil
	}
	for _, el := range ext.ChildElements() {
		if el.NamespaceURI() != nsRGP || el.Tag != "update" {
			continue
		}
		restore := el.SelectElement("restore")
		r := &commands.EPPRestore{}
		if schema.TokenValue(restore.SelectAttrValue("op", "")) != "report" {
			return r, nil
		}
		rep := restore.SelectElement("report")
		if rep == nil {
			// The schema allows op="report" without one (RFC 3915 §4.2.5
			// requires it).
			return nil, fmt.Errorf("%w: give <rgp:report>", entities.ErrRestoreReportIncomplete)
		}
		report := &entities.RestoreReport{
			PreData:   innerXML(rep.SelectElement("preData")),
			PostData:  innerXML(rep.SelectElement("postData")),
			ResReason: innerXML(rep.SelectElement("resReason")),
		}
		for _, st := range rep.SelectElements("statement") {
			report.Statements = append(report.Statements, innerXML(st))
		}
		if o := rep.SelectElement("other"); o != nil {
			report.Other = innerXML(o)
		}
		for _, t := range []struct {
			name string
			into *time.Time
		}{{"delTime", &report.DelTime}, {"resTime", &report.ResTime}} {
			v, err := schema.DateTimeValue(rep.SelectElement(t.name).Text())
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %w", entities.ErrRestoreReportTimes, t.name, err)
			}
			*t.into = v.UTC()
		}
		r.Report = report
		return r, nil
	}
	return nil, nil
}

// innerXML is el's content as sent: its text and child elements, as XML.
func innerXML(el *etree.Element) string {
	var b strings.Builder
	var settings etree.WriteSettings
	for _, tok := range el.Child {
		tok.WriteTo(&b, &settings)
	}
	return b.String()
}

// domainAddRem reads a <domain:add> or <domain:rem>: its host objects,
// contacts and statuses. Host attributes set attrs.
func domainAddRem(el *etree.Element, attrs *bool) (hosts []string, contacts []commands.EPPDomainContact, statuses []string) {
	if ns := el.SelectElement("ns"); ns != nil {
		if len(ns.SelectElements("hostAttr")) > 0 {
			*attrs = true
		}
		for _, h := range ns.SelectElements("hostObj") {
			hosts = append(hosts, schema.TokenValue(h.Text()))
		}
	}
	for _, ct := range el.SelectElements("contact") {
		contacts = append(contacts, commands.EPPDomainContact{
			Type: schema.TokenValue(ct.SelectAttrValue("type", "")),
			ID:   schema.TokenValue(ct.Text()),
		})
	}
	return hosts, contacts, statusValues(el)
}

// secDNSUpdate reads a <secDNS:update> extension (RFC 5910 §5.2.5), or nil
// if the command has none.
func secDNSUpdate(doc *etree.Document) *commands.EPPDNSSECUpdate {
	ext := doc.FindElement("/epp/command/extension")
	if ext == nil {
		return nil
	}
	for _, el := range ext.ChildElements() {
		if el.NamespaceURI() != nsSecDNS || el.Tag != "update" {
			continue
		}
		urgent := schema.TokenValue(el.SelectAttrValue("urgent", "false"))
		u := &commands.EPPDNSSECUpdate{Urgent: urgent == "true" || urgent == "1"}
		if rem := el.SelectElement("rem"); rem != nil {
			if all := rem.SelectElement("all"); all != nil {
				v := schema.TokenValue(all.Text())
				u.RemoveAll = v == "true" || v == "1"
			}
			u.RemoveDS = dsDataOf(rem)
			u.KeyData = u.KeyData || len(rem.SelectElements("keyData")) > 0
		}
		if add := el.SelectElement("add"); add != nil {
			u.AddDS = dsDataOf(add)
			u.KeyData = u.KeyData || len(add.SelectElements("keyData")) > 0
			u.MaxSigLife = maxSigLifeOf(add, u.MaxSigLife)
		}
		if chg := el.SelectElement("chg"); chg != nil {
			u.MaxSigLife = maxSigLifeOf(chg, u.MaxSigLife)
		}
		return u
	}
	return nil
}

// dsDataOf reads the <secDNS:dsData> children of el.
func dsDataOf(el *etree.Element) []commands.EPPDSData {
	var out []commands.EPPDSData
	for _, ds := range el.SelectElements("dsData") {
		num := func(name string) int {
			n, _ := strconv.Atoi(schema.TokenValue(ds.SelectElement(name).Text())) // the schema has checked it
			return n
		}
		out = append(out, commands.EPPDSData{
			KeyTag: num("keyTag"), Algorithm: num("alg"), DigestType: num("digestType"),
			Digest: schema.TokenValue(ds.SelectElement("digest").Text()),
		})
	}
	return out
}

// maxSigLifeOf is el's <secDNS:maxSigLife>, or def if it has none.
func maxSigLifeOf(el *etree.Element, def int) int {
	if m := el.SelectElement("maxSigLife"); m != nil {
		n, _ := strconv.Atoi(schema.TokenValue(m.Text())) // the schema has checked it is a positive int
		return n
	}
	return def
}

// domainRenew answers <domain:renew> (RFC 5731 §3.2.3).
func (s *Server) domainRenew(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	renew := cmd.doc.FindElement("/epp/command/renew/*")
	// xs:date may carry a time zone; the expiry date is compared as a day.
	cur := schema.TokenValue(renew.SelectElement("curExpDate").Text())
	c := commands.EPPRenewDomainCommand{Name: cmd.objects[0], CurExpDate: dateOnly(cur)}
	if p := renew.SelectElement("period"); p != nil {
		c.Period, _ = strconv.Atoi(schema.TokenValue(p.Text())) // the schema has checked it is 1 to 99
		c.PeriodUnit = schema.TokenValue(p.SelectAttrValue("unit", ""))
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	dom, err := s.domains.Renew(ctx, scope, c)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", renewData(dom))
}

// dateOnly is an xs:date's YYYY-MM-DD, without its time zone.
func dateOnly(d string) string {
	neg := strings.HasPrefix(d, "-")
	body := strings.TrimPrefix(d, "-")
	// The date is the year, month and day: the first three dash-separated
	// fields; a time zone is Z or follows a + or a fourth dash.
	if i := strings.IndexAny(body, "Z+"); i >= 0 {
		body = body[:i]
	}
	if parts := strings.SplitN(body, "-", 4); len(parts) == 4 {
		body = strings.Join(parts[:3], "-")
	}
	if neg {
		return "-" + body
	}
	return body
}

// domainDelete answers <domain:delete> (RFC 5731 §3.2.2): 1001, the domain
// pendingDelete (RFC 3915), until it is purged.
func (s *Server) domainDelete(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	if _, err := s.domains.Delete(ctx, scope, cmd.objects[0]); err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusActionPending, "", nil)
}

// readSecDNSCreate reads a <secDNS:create> extension (RFC 5910 §5.2.1) into
// c: its DS data, its maxSigLife and whether it used the key data
// interface. A <secDNS:keyData> inside a <secDNS:dsData> is optional
// supporting information and is not kept. The registry decides what to
// accept.
func readSecDNSCreate(doc *etree.Document, c *commands.EPPCreateDomainCommand) {
	ext := doc.FindElement("/epp/command/extension")
	if ext == nil {
		return
	}
	for _, el := range ext.ChildElements() {
		if el.NamespaceURI() != nsSecDNS || el.Tag != "create" {
			continue
		}
		if m := el.SelectElement("maxSigLife"); m != nil {
			c.MaxSigLife, _ = strconv.Atoi(schema.TokenValue(m.Text())) // the schema has checked it is a positive int
		}
		c.KeyData = len(el.SelectElements("keyData")) > 0
		for _, ds := range el.SelectElements("dsData") {
			num := func(name string) int {
				n, _ := strconv.Atoi(schema.TokenValue(ds.SelectElement(name).Text())) // the schema has checked it
				return n
			}
			c.DSData = append(c.DSData, commands.EPPDSData{
				KeyTag: num("keyTag"), Algorithm: num("alg"), DigestType: num("digestType"),
				Digest: schema.TokenValue(ds.SelectElement("digest").Text()),
			})
		}
	}
}
