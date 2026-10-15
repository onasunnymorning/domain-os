package epp

import (
	"context"
	"errors"
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
	// Host commands (RFC 5732).
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
	{entities.ErrEPPHostStatusNotClient, epplib.StatusParameterPolicyError},
	{entities.ErrEPPHostStatusAlreadySet, epplib.StatusParameterPolicyError},
	{entities.ErrEPPHostStatusNotSet, epplib.StatusParameterPolicyError},
	{entities.ErrHostUpdateProhibited, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPHostDeleteProhibited, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPHostPending, epplib.StatusObjectStatusProhibitsOperation},
	{entities.ErrEPPHostLinked, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrEPPHostLinkedByOthers, epplib.StatusObjectAssociationProhibitsOperation},
	{entities.ErrEPPHostNeedsAddress, epplib.StatusObjectAssociationProhibitsOperation},
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

// domainInfo answers <domain:info> (RFC 5731 §3.1.2). Only the sponsor is
// answered; anyone else gets 2201 (ADR 0006), whether or not they present
// the domain's authorization information. The RGP statuses are added when
// the session logged in with RGP.
func (s *Server) domainInfo(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	name := cmd.doc.FindElement("/epp/command/info/*/name")
	hosts := name.SelectAttrValue("hosts", "all")
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	info, err := s.domains.Info(ctx, scope, cmd.objects[0])
	if err != nil {
		s.refuse(cmd, err)
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
