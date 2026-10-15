package epp

import (
	"context"
	"fmt"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/epp/schema"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// The host commands (RFC 5732) read the frame as the schema defines its
// values and leave every rule to the registry (services.EPPHostService).
// Host objects belong to a registrar, so every command, check included, is
// scoped to the session's registrar (ADR 0006).

// hostCheck answers <host:check>: whether the registrar can create each
// name, in the order asked. More names than EPP_MAX_CHECK_NAMES is 2306.
func (s *Server) hostCheck(ctx context.Context, cmd *command) {
	if !s.checkSizeAllowed(cmd) {
		return
	}
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	results, err := s.hosts.Check(ctx, scope, cmd.objects)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", hostCheckData(results))
}

// hostInfo answers <host:info> (RFC 5732 §3.1.2) for the registrar's own
// host object.
func (s *Server) hostInfo(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	host, err := s.hosts.Info(ctx, scope, cmd.objects[0])
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", hostInfoData(host))
}

// hostCreate answers <host:create> (RFC 5732 §3.2.1).
func (s *Server) hostCreate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	create := cmd.doc.FindElement("/epp/command/create/*")
	c := commands.EPPCreateHostCommand{Name: cmd.objects[0], Addresses: hostAddrs(create)}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	host, err := s.hosts.Create(ctx, scope, c)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", hostCreateData(host))
}

// hostUpdate answers <host:update> (RFC 5732 §3.2.5).
func (s *Server) hostUpdate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	update := cmd.doc.FindElement("/epp/command/update/*")
	if update.SelectElement("add") == nil && update.SelectElement("rem") == nil && update.SelectElement("chg") == nil {
		// RFC 5732 §3.2.5: one of them is required, as no host update
		// extension is offered.
		cmd.respond(epplib.StatusMissingParameter, "Required parameter missing; give <host:add>, <host:rem> or <host:chg>", nil)
		return
	}
	c := commands.EPPUpdateHostCommand{Name: cmd.objects[0]}
	if add := update.SelectElement("add"); add != nil {
		c.AddAddresses, c.AddStatuses = hostAddrs(add), hostStatuses(add)
	}
	if rem := update.SelectElement("rem"); rem != nil {
		c.RemoveAddresses, c.RemoveStatuses = hostAddrs(rem), hostStatuses(rem)
	}
	if name := update.FindElement("chg/name"); name != nil {
		c.NewName = schema.TokenValue(name.Text())
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	if _, err := s.hosts.Update(ctx, scope, c); err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// hostDelete answers <host:delete> (RFC 5732 §3.2.2).
func (s *Server) hostDelete(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	if err := s.hosts.Delete(ctx, scope, cmd.objects[0]); err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// hostAddrs reads the <host:addr> children of el. The ip attribute
// defaults to v4, as the schema declares.
func hostAddrs(el *etree.Element) []commands.EPPHostAddress {
	var out []commands.EPPHostAddress
	for _, a := range el.SelectElements("addr") {
		out = append(out, commands.EPPHostAddress{
			Version: schema.TokenValue(a.SelectAttrValue("ip", "v4")),
			Address: schema.TokenValue(a.Text()),
		})
	}
	return out
}

// hostStatuses reads the s attributes of the <host:status> children of el.
func hostStatuses(el *etree.Element) []string {
	var out []string
	for _, st := range el.SelectElements("status") {
		out = append(out, schema.TokenValue(st.SelectAttrValue("s", "")))
	}
	return out
}

// checkSizeAllowed answers a <check> with more names than
// EPP_MAX_CHECK_NAMES 2306, and reports whether it may go on.
func (s *Server) checkSizeAllowed(cmd *command) bool {
	s.metrics.checkNames.Observe(float64(len(cmd.objects)))
	if max := s.cfg.Commands.MaxCheckNames; len(cmd.objects) > max {
		cmd.respond(epplib.StatusParameterPolicyError, fmt.Sprintf("Parameter value policy error; at most %d names per check", max), nil)
		return false
	}
	return true
}

// hostCheckData builds <host:chkData> from check results, in request order.
func hostCheckData(results []*queries.HostCheckResult) *etree.Element {
	chk := etree.NewElement("host:chkData")
	chk.CreateAttr("xmlns:host", nsHost)
	for _, r := range results {
		cd := chk.CreateElement("host:cd")
		name := cd.CreateElement("host:name")
		avail := "0"
		if r.Available {
			avail = "1"
		}
		name.CreateAttr("avail", avail)
		name.SetText(r.Name)
		if !r.Available && r.Reason != "" {
			cd.CreateElement("host:reason").SetText(r.Reason)
		}
	}
	return chk
}

// hostInfoData builds <host:infData> (RFC 5732 §3.1.2).
func hostInfoData(h *entities.Host) *etree.Element {
	inf := etree.NewElement("host:infData")
	inf.CreateAttr("xmlns:host", nsHost)
	inf.CreateElement("host:name").SetText(h.Name.String())
	inf.CreateElement("host:roid").SetText(h.RoID.String())
	for _, st := range h.Status.StringSlice() {
		inf.CreateElement("host:status").CreateAttr("s", st)
	}
	for _, a := range h.Addresses {
		el := inf.CreateElement("host:addr")
		ip := "v4"
		if a.Is6() {
			ip = "v6"
		}
		el.CreateAttr("ip", ip)
		el.SetText(a.String())
	}
	inf.CreateElement("host:clID").SetText(h.ClID.String())
	crID := h.CrRr
	if crID == "" {
		crID = h.ClID
	}
	inf.CreateElement("host:crID").SetText(crID.String())
	inf.CreateElement("host:crDate").SetText(eppTime(h.CreatedAt))
	if h.UpRr != "" {
		inf.CreateElement("host:upID").SetText(h.UpRr.String())
		inf.CreateElement("host:upDate").SetText(eppTime(h.UpdatedAt))
	}
	return inf
}

// hostCreateData builds <host:creData> (RFC 5732 §3.2.1).
func hostCreateData(h *entities.Host) *etree.Element {
	cre := etree.NewElement("host:creData")
	cre.CreateAttr("xmlns:host", nsHost)
	cre.CreateElement("host:name").SetText(h.Name.String())
	cre.CreateElement("host:crDate").SetText(eppTime(h.CreatedAt))
	return cre
}
