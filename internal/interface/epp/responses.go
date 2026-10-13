package epp

import (
	"strconv"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// serverID is the <svID> in the greeting.
const serverID = "domain-os EPP"

// supportedObjects are the object services the greeting offers and a
// <login> may ask for. Contact and host commands are not bound yet and are
// answered 2101.
var supportedObjects = []string{nsDomain, nsContact, nsHost}

// supportedExtensions are the extensions the greeting offers and a <login>
// may ask for. None is implemented yet.
var supportedExtensions []string

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
// standard text for code. resData, when given, is placed in <resData>.
// clTRID is echoed only when the client sent one.
func responseDocument(code int, msg, clTRID, svTRID string, resData *etree.Element) *etree.Document {
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
