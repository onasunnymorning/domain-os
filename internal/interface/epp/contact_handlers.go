package epp

import (
	"context"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/epp/schema"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// The contact commands (RFC 5733) read the frame as the schema defines its
// values and leave every rule to the registry
// (services.EPPContactService).

// contactCheck answers <contact:check>: whether each ID is free, in the
// order asked. More IDs than the registrar's policy allows is 2306.
func (s *Server) contactCheck(ctx context.Context, cmd *command) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	results, err := s.contacts.Check(ctx, cmd.objects)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", contactCheckData(results))
}

// contactInfo answers <contact:info> (RFC 5733 §3.1.2), to the sponsor
// only: anyone else gets 2201, with or without the authorization
// information (ADR 0006).
func (s *Server) contactInfo(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	c, err := s.contacts.Info(ctx, scope, cmd.objects[0])
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", contactInfoData(c))
}

// contactCreate answers <contact:create> (RFC 5733 §3.2.1).
func (s *Server) contactCreate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	create := cmd.doc.FindElement("/epp/command/create/*")
	c := commands.EPPCreateContactCommand{
		ID:         cmd.objects[0],
		PostalInfo: contactPostalInfos(create),
		Voice:      contactPhone(create.SelectElement("voice")),
		Fax:        contactPhone(create.SelectElement("fax")),
		Email:      schema.TokenValue(create.SelectElement("email").Text()),
		Disclose:   contactDisclose(create.SelectElement("disclose")),
	}
	if pw := create.FindElement("authInfo/pw"); pw != nil {
		c.AuthInfo = schema.NormalizedStringValue(pw.Text())
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	created, err := s.contacts.Create(ctx, scope, c)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", contactCreateData(created))
}

// contactUpdate answers <contact:update> (RFC 5733 §3.2.5).
func (s *Server) contactUpdate(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	update := cmd.doc.FindElement("/epp/command/update/*")
	add, rem, chg := update.SelectElement("add"), update.SelectElement("rem"), update.SelectElement("chg")
	if add == nil && rem == nil && chg == nil {
		// RFC 5733 §3.2.5: one of them is required, as no contact update
		// extension is offered.
		cmd.respond(epplib.StatusMissingParameter, "Required parameter missing; give <contact:add>, <contact:rem> or <contact:chg>", nil)
		return
	}
	c := commands.EPPUpdateContactCommand{ID: cmd.objects[0]}
	if add != nil {
		c.AddStatuses = statusValues(add)
	}
	if rem != nil {
		c.RemoveStatuses = statusValues(rem)
	}
	if chg != nil {
		c.PostalInfo = contactPostalInfos(chg)
		c.Voice = contactPhone(chg.SelectElement("voice"))
		c.Fax = contactPhone(chg.SelectElement("fax"))
		if el := chg.SelectElement("email"); el != nil {
			email := schema.TokenValue(el.Text())
			c.Email = &email
		}
		if el := chg.SelectElement("authInfo"); el != nil {
			pw := ""
			if p := el.SelectElement("pw"); p != nil {
				pw = schema.NormalizedStringValue(p.Text())
			}
			c.AuthInfo = &pw
		}
		c.Disclose = contactDisclose(chg.SelectElement("disclose"))
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	if _, err := s.contacts.Update(ctx, scope, c); err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// contactDelete answers <contact:delete> (RFC 5733 §3.2.2).
func (s *Server) contactDelete(ctx context.Context, cmd *command) {
	scope, err := registrarScope(cmd)
	if err != nil {
		s.refuse(cmd, err)
		return
	}
	ctx, cancel := context.WithTimeout(withActor(ctx, cmd), commandTimeout)
	defer cancel()
	if err := s.contacts.Delete(ctx, scope, cmd.objects[0]); err != nil {
		s.refuse(cmd, err)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// statusValues reads the s attributes of el's <status> children.
func statusValues(el *etree.Element) []string {
	var out []string
	for _, st := range el.SelectElements("status") {
		out = append(out, schema.TokenValue(st.SelectAttrValue("s", "")))
	}
	return out
}

// contactPostalInfos reads el's <contact:postalInfo> children. A part that
// is absent is nil.
func contactPostalInfos(el *etree.Element) []commands.EPPPostalInfo {
	var out []commands.EPPPostalInfo
	for _, pi := range el.SelectElements("postalInfo") {
		p := commands.EPPPostalInfo{Type: schema.TokenValue(pi.SelectAttrValue("type", ""))}
		if n := pi.SelectElement("name"); n != nil {
			v := schema.NormalizedStringValue(n.Text())
			p.Name = &v
		}
		if o := pi.SelectElement("org"); o != nil {
			v := schema.NormalizedStringValue(o.Text())
			p.Org = &v
		}
		if a := pi.SelectElement("addr"); a != nil {
			addr := &commands.EPPContactAddress{
				City: schema.NormalizedStringValue(a.SelectElement("city").Text()),
				CC:   schema.TokenValue(a.SelectElement("cc").Text()),
			}
			for _, st := range a.SelectElements("street") {
				addr.Streets = append(addr.Streets, schema.NormalizedStringValue(st.Text()))
			}
			if sp := a.SelectElement("sp"); sp != nil {
				addr.SP = schema.NormalizedStringValue(sp.Text())
			}
			if pc := a.SelectElement("pc"); pc != nil {
				addr.PC = schema.TokenValue(pc.Text())
			}
			p.Addr = addr
		}
		out = append(out, p)
	}
	return out
}

// contactPhone reads a <contact:voice> or <contact:fax>, or nil if el is.
func contactPhone(el *etree.Element) *commands.EPPPhone {
	if el == nil {
		return nil
	}
	p := &commands.EPPPhone{Number: schema.TokenValue(el.Text())}
	if x := el.SelectAttr("x"); x != nil {
		ext := schema.TokenValue(x.Value)
		p.Ext = &ext
	}
	return p
}

// contactDisclose reads a <contact:disclose>, or nil if el is.
func contactDisclose(el *etree.Element) *commands.EPPDisclose {
	if el == nil {
		return nil
	}
	flag := schema.TokenValue(el.SelectAttrValue("flag", ""))
	d := &commands.EPPDisclose{Flag: flag == "1" || flag == "true"}
	types := func(name string) []string {
		var out []string
		for _, e := range el.SelectElements(name) {
			out = append(out, schema.TokenValue(e.SelectAttrValue("type", "")))
		}
		return out
	}
	d.Names, d.Orgs, d.Addrs = types("name"), types("org"), types("addr")
	d.Voice, d.Fax, d.Email = el.SelectElement("voice") != nil, el.SelectElement("fax") != nil, el.SelectElement("email") != nil
	return d
}

// contactCheckData builds <contact:chkData> from check results, in request
// order.
func contactCheckData(results []*queries.ContactCheckResult) *etree.Element {
	chk := etree.NewElement("contact:chkData")
	chk.CreateAttr("xmlns:contact", nsContact)
	for _, r := range results {
		cd := chk.CreateElement("contact:cd")
		id := cd.CreateElement("contact:id")
		avail := "0"
		if r.Available {
			avail = "1"
		}
		id.CreateAttr("avail", avail)
		id.SetText(r.ID)
		if !r.Available && r.Reason != "" {
			cd.CreateElement("contact:reason").SetText(r.Reason)
		}
	}
	return chk
}

// contactStatuses are a contact's statuses as RFC 5733 names them.
func contactStatuses(s entities.ContactStatus) []string {
	var out []string
	for _, st := range []struct {
		set  bool
		name entities.ContactStatusType
	}{
		{s.OK, entities.ContactStatusOK},
		{s.Linked, entities.ContactStatusLinked},
		{s.PendingCreate, entities.ContactStatusPendingCreate},
		{s.PendingDelete, entities.ContactStatusPendingDelete},
		{s.PendingTransfer, entities.ContactStatusPendingTransfer},
		{s.PendingUpdate, entities.ContactStatusPendingUpdate},
		{s.ClientDeleteProhibited, entities.ContactStatusClientDeleteProhibited},
		{s.ClientTransferProhibited, entities.ContactStatusClientTransferProhibited},
		{s.ClientUpdateProhibited, entities.ContactStatusClientUpdateProhibited},
		{s.ServerDeleteProhibited, entities.ContactStatusServerDeleteProhibited},
		{s.ServerTransferProhibited, entities.ContactStatusServerTransferProhibited},
		{s.ServerUpdateProhibited, entities.ContactStatusServerUpdateProhibited},
	} {
		if st.set {
			out = append(out, string(st.name))
		}
	}
	return out
}

// contactInfoData builds <contact:infData> (RFC 5733 §3.1.2). Info is
// answered only to the sponsor, so the authorization information is
// theirs to see.
func contactInfoData(c *entities.Contact) *etree.Element {
	inf := etree.NewElement("contact:infData")
	inf.CreateAttr("xmlns:contact", nsContact)
	inf.CreateElement("contact:id").SetText(c.ID.String())
	inf.CreateElement("contact:roid").SetText(c.RoID.String())
	for _, st := range contactStatuses(c.Status) {
		inf.CreateElement("contact:status").CreateAttr("s", st)
	}
	for _, pi := range c.PostalInfo {
		if pi == nil {
			continue
		}
		el := inf.CreateElement("contact:postalInfo")
		el.CreateAttr("type", string(pi.Type))
		el.CreateElement("contact:name").SetText(pi.Name.String())
		if pi.Org != "" {
			el.CreateElement("contact:org").SetText(pi.Org.String())
		}
		addr := el.CreateElement("contact:addr")
		for _, st := range []entities.OptPostalLineType{pi.Address.Street1, pi.Address.Street2, pi.Address.Street3} {
			if st != "" {
				addr.CreateElement("contact:street").SetText(st.String())
			}
		}
		addr.CreateElement("contact:city").SetText(pi.Address.City.String())
		if pi.Address.StateProvince != "" {
			addr.CreateElement("contact:sp").SetText(pi.Address.StateProvince.String())
		}
		if pi.Address.PostalCode != "" {
			addr.CreateElement("contact:pc").SetText(pi.Address.PostalCode.String())
		}
		addr.CreateElement("contact:cc").SetText(string(pi.Address.CountryCode))
	}
	if c.Voice != "" {
		inf.CreateElement("contact:voice").SetText(c.Voice.String())
	}
	if c.Fax != "" {
		inf.CreateElement("contact:fax").SetText(c.Fax.String())
	}
	inf.CreateElement("contact:email").SetText(c.Email)
	inf.CreateElement("contact:clID").SetText(c.ClID.String())
	crID := c.CrRr
	if crID == "" {
		crID = c.ClID
	}
	inf.CreateElement("contact:crID").SetText(crID.String())
	inf.CreateElement("contact:crDate").SetText(eppTime(c.CreatedAt))
	if c.UpRr != "" {
		inf.CreateElement("contact:upID").SetText(c.UpRr.String())
		inf.CreateElement("contact:upDate").SetText(eppTime(c.UpdatedAt))
	}
	inf.CreateElement("contact:authInfo").CreateElement("contact:pw").SetText(c.AuthInfo.String())
	if d := discloseData(c.Disclose); d != nil {
		inf.AddChild(d)
	}
	return inf
}

// discloseData builds the <contact:disclose flag="1"> listing what the
// contact discloses, or nil if it discloses nothing: by default nothing is.
func discloseData(d entities.ContactDisclose) *etree.Element {
	if d.IsNil() {
		return nil
	}
	el := etree.NewElement("contact:disclose")
	el.CreateAttr("flag", "1")
	for _, x := range []struct {
		name       string
		intF, locF bool
	}{{"name", d.NameInt, d.NameLoc}, {"org", d.OrgInt, d.OrgLoc}, {"addr", d.AddrInt, d.AddrLoc}} {
		if x.intF {
			el.CreateElement("contact:"+x.name).CreateAttr("type", "int")
		}
		if x.locF {
			el.CreateElement("contact:"+x.name).CreateAttr("type", "loc")
		}
	}
	for _, x := range []struct {
		name string
		set  bool
	}{{"voice", d.Voice}, {"fax", d.Fax}, {"email", d.Email}} {
		if x.set {
			el.CreateElement("contact:" + x.name)
		}
	}
	return el
}

// contactCreateData builds <contact:creData> (RFC 5733 §3.2.1).
func contactCreateData(c *entities.Contact) *etree.Element {
	cre := etree.NewElement("contact:creData")
	cre.CreateAttr("xmlns:contact", nsContact)
	cre.CreateElement("contact:id").SetText(c.ID.String())
	cre.CreateElement("contact:crDate").SetText(eppTime(c.CreatedAt))
	return cre
}
