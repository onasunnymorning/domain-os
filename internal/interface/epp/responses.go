package epp

import (
	"strconv"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// serverID is the <svID> in the greeting.
const serverID = "domain-os EPP"

// supportedObjects are the object services the greeting offers and a
// <login> may ask for. Contact commands are not bound yet and are answered
// 2101.
var supportedObjects = []string{nsDomain, nsContact, nsHost}

// supportedExtensions are the extensions the greeting offers and a <login>
// may ask for. DNSSEC (RFC 5910) is offered for DS data on <domain:create>
// and <domain:info>; RGP (RFC 3915) for the grace period statuses on
// <domain:info>. Their update operations arrive with <domain:update>.
var supportedExtensions = []string{nsSecDNS, nsRGP}

// greetingDocument builds the RFC 5730 §2.4 greeting.
func greetingDocument(now time.Time) *etree.Document {
	doc, epp := newEPPDocument()
	g := epp.CreateElement("greeting")
	g.CreateElement("svID").SetText(serverID)
	g.CreateElement("svDate").SetText(now.UTC().Format(time.RFC3339))
	menu := g.CreateElement("svcMenu")
	menu.CreateElement("version").SetText("1.0")
	menu.CreateElement("lang").SetText("en")
	for _, uri := range supportedObjects {
		menu.CreateElement("objURI").SetText(uri)
	}
	if len(supportedExtensions) > 0 {
		ext := menu.CreateElement("svcExtension")
		for _, uri := range supportedExtensions {
			ext.CreateElement("extURI").SetText(uri)
		}
	}
	addDataCollectionPolicy(g)
	return doc
}

// addDataCollectionPolicy appends the <dcp> RFC 5730 §2.4 requires in every
// greeting. It states the registry's data collection policy, so it is a
// policy statement, not a technical default; docs/epp/POLICY_REFERENCE.md
// records it for review:
//
//   - access: all. A client may access all data it provided.
//   - purpose: admin and prov. Data is used to administer and provision.
//   - recipient: ours and public. The registry, and the public through
//     registration data services.
//   - retention: stated. Retention follows the registry's published policy.
func addDataCollectionPolicy(greeting *etree.Element) {
	dcp := greeting.CreateElement("dcp")
	dcp.CreateElement("access").CreateElement("all")
	stmt := dcp.CreateElement("statement")
	purpose := stmt.CreateElement("purpose")
	purpose.CreateElement("admin")
	purpose.CreateElement("prov")
	recipient := stmt.CreateElement("recipient")
	recipient.CreateElement("ours")
	recipient.CreateElement("public")
	stmt.CreateElement("retention").CreateElement("stated")
}

// responseDocument builds an RFC 5730 §2.6 response. msg defaults to the
// standard text for code. resData, when given, is placed in <resData>, and
// extensions in <extension>. clTRID is echoed only when the client sent one.
func responseDocument(code int, msg, clTRID, svTRID string, resData *etree.Element, extensions ...*etree.Element) *etree.Document {
	if msg == "" {
		msg = epplib.StatusText(code)
	}
	doc, epp := newEPPDocument()
	resp := epp.CreateElement("response")
	result := resp.CreateElement("result")
	result.CreateAttr("code", strconv.Itoa(code))
	result.CreateElement("msg").SetText(msg)
	if resData != nil {
		resp.CreateElement("resData").AddChild(resData)
	}
	if len(extensions) > 0 {
		ext := resp.CreateElement("extension")
		for _, e := range extensions {
			ext.AddChild(e)
		}
	}
	trID := resp.CreateElement("trID")
	if clTRID != "" {
		trID.CreateElement("clTRID").SetText(clTRID)
	}
	trID.CreateElement("svTRID").SetText(svTRID)
	return doc
}

func newEPPDocument() (*etree.Document, *etree.Element) {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8" standalone="no"`)
	epp := doc.CreateElement("epp")
	epp.CreateAttr("xmlns", nsEPP)
	return doc, epp
}

// checkData builds <domain:chkData> from check results, in request order.
func checkData(results []*queries.DomainCheckResult) *etree.Element {
	chk := etree.NewElement("domain:chkData")
	chk.CreateAttr("xmlns:domain", nsDomain)
	for _, r := range results {
		cd := chk.CreateElement("domain:cd")
		name := cd.CreateElement("domain:name")
		avail := "0"
		if r.Available {
			avail = "1"
		}
		name.CreateAttr("avail", avail)
		name.SetText(r.DomainName)
		if !r.Available && r.Reason != "" {
			cd.CreateElement("domain:reason").SetText(r.Reason)
		}
	}
	return chk
}

// eppTime formats a time as EPP's dateTime, in UTC.
func eppTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// infoData builds <domain:infData> (RFC 5731 §3.1.2). hosts is the
// <domain:info> hosts attribute: all, del (nameservers only), sub
// (subordinate hosts only) or none.
func infoData(info *queries.EPPDomainInfo, hosts string) *etree.Element {
	d := info.Domain
	inf := etree.NewElement("domain:infData")
	inf.CreateAttr("xmlns:domain", nsDomain)
	inf.CreateElement("domain:name").SetText(d.Name.String())
	inf.CreateElement("domain:roid").SetText(d.RoID.String())
	for _, st := range d.Status.StringSlice() {
		if st == entities.DomainStatusPendingRestore {
			continue // an RGP status (RFC 3915), not an RFC 5731 one
		}
		inf.CreateElement("domain:status").CreateAttr("s", st)
	}
	if d.RegistrantID != "" {
		inf.CreateElement("domain:registrant").SetText(d.RegistrantID.String())
	}
	for _, c := range []struct {
		kind string
		id   entities.ClIDType
	}{{"admin", d.AdminID}, {"billing", d.BillingID}, {"tech", d.TechID}} {
		if c.id != "" {
			el := inf.CreateElement("domain:contact")
			el.CreateAttr("type", c.kind)
			el.SetText(c.id.String())
		}
	}
	if (hosts == "all" || hosts == "del") && len(d.Hosts) > 0 {
		ns := inf.CreateElement("domain:ns")
		for _, h := range d.Hosts {
			ns.CreateElement("domain:hostObj").SetText(h.Name.String())
		}
	}
	if hosts == "all" || hosts == "sub" {
		for _, h := range info.SubordinateHosts {
			inf.CreateElement("domain:host").SetText(h)
		}
	}
	inf.CreateElement("domain:clID").SetText(d.ClID.String())
	if d.CrRr != "" {
		inf.CreateElement("domain:crID").SetText(d.CrRr.String())
	}
	inf.CreateElement("domain:crDate").SetText(eppTime(d.CreatedAt))
	if d.UpRr != "" {
		inf.CreateElement("domain:upID").SetText(d.UpRr.String())
		inf.CreateElement("domain:upDate").SetText(eppTime(d.UpdatedAt))
	}
	inf.CreateElement("domain:exDate").SetText(eppTime(d.ExpiryDate))
	// Info is answered only to the sponsor (ADR 0006), so the
	// authorization information is always theirs to see.
	inf.CreateElement("domain:authInfo").CreateElement("domain:pw").SetText(d.AuthInfo.String())
	return inf
}

// rgpInfoData builds <rgp:infData> (RFC 3915 §4.1), or nil when the domain
// is in no grace period.
func rgpInfoData(d *entities.Domain, now time.Time) *etree.Element {
	statuses := d.RGPStatuses(now)
	if len(statuses) == 0 {
		return nil
	}
	inf := etree.NewElement("rgp:infData")
	inf.CreateAttr("xmlns:rgp", nsRGP)
	for _, st := range statuses {
		inf.CreateElement("rgp:rgpStatus").CreateAttr("s", st)
	}
	return inf
}

// createData builds <domain:creData> (RFC 5731 §3.2.1).
func createData(d *entities.Domain) *etree.Element {
	cre := etree.NewElement("domain:creData")
	cre.CreateAttr("xmlns:domain", nsDomain)
	cre.CreateElement("domain:name").SetText(d.Name.String())
	cre.CreateElement("domain:crDate").SetText(eppTime(d.CreatedAt))
	cre.CreateElement("domain:exDate").SetText(eppTime(d.ExpiryDate))
	return cre
}

// secDNSInfoData builds <secDNS:infData> (RFC 5910 §5.1.2) with the
// domain's DS records, or nil when it has none.
func secDNSInfoData(d *entities.Domain) *etree.Element {
	if len(d.DSRecords) == 0 {
		return nil
	}
	inf := etree.NewElement("secDNS:infData")
	inf.CreateAttr("xmlns:secDNS", nsSecDNS)
	for _, ds := range d.DSRecords {
		el := inf.CreateElement("secDNS:dsData")
		el.CreateElement("secDNS:keyTag").SetText(strconv.Itoa(int(ds.KeyTag)))
		el.CreateElement("secDNS:alg").SetText(strconv.Itoa(int(ds.Algorithm)))
		el.CreateElement("secDNS:digestType").SetText(strconv.Itoa(int(ds.DigestType)))
		el.CreateElement("secDNS:digest").SetText(ds.Digest)
	}
	return inf
}
