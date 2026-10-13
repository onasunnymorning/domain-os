package epp

import (
	"strconv"
	"time"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"
)

// serverID is the <svID> in the greeting.
const serverID = "domain-os EPP"

// greetingDocument builds the RFC 5730 §2.4 greeting.
func greetingDocument(now time.Time) *etree.Document {
	doc, epp := newEPPDocument()
	g := epp.CreateElement("greeting")
	g.CreateElement("svID").SetText(serverID)
	g.CreateElement("svDate").SetText(now.UTC().Format(time.RFC3339))
	menu := g.CreateElement("svcMenu")
	menu.CreateElement("version").SetText("1.0")
	menu.CreateElement("lang").SetText("en")
	for _, uri := range []string{nsDomain, nsContact, nsHost} {
		menu.CreateElement("objURI").SetText(uri)
	}
	return doc
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

// placeholderCheckData is the fixed <domain:chkData> the stub domain:check
// returns. It does not reflect the names asked about; the real handler is
// Phase 4 of docs/epp/ARCHITECTURE.md.
func placeholderCheckData() *etree.Element {
	chk := etree.NewElement("domain:chkData")
	chk.CreateAttr("xmlns:domain", nsDomain)
	name := chk.CreateElement("domain:cd").CreateElement("domain:name")
	name.CreateAttr("avail", "1")
	name.SetText("geoff.smoketestcnic")
	return chk
}
