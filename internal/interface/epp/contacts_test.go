package epp

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func contactCmdXML(verb, inner string) string {
	return cmdXML(`<`+verb+`><contact:`+verb+` xmlns:contact="urn:ietf:params:xml:ns:contact-1.0">`+inner+`</contact:`+verb+`></`+verb+`>`, "CON-1")
}

func contactCheckXML(ids ...string) string {
	var b strings.Builder
	for _, id := range ids {
		b.WriteString("<contact:id>" + id + "</contact:id>")
	}
	return contactCmdXML("check", b.String())
}

func contactInfoXML(id string) string {
	return contactCmdXML("info", "<contact:id>"+id+"</contact:id>")
}

func contactDeleteXML(id string) string {
	return contactCmdXML("delete", "<contact:id>"+id+"</contact:id>")
}

// contactOpts are the parts of a <contact:create> a test varies; empty
// fields take valid defaults, and "-" leaves an optional element out.
type contactOpts struct {
	postalInfo string // the <contact:postalInfo> elements, whole
	voice      string // the <contact:voice> element, whole
	fax        string
	email      string
	pw         string
	disclose   string // the <contact:disclose> element, whole
}

func postalInfoXML(typ, name, org, city, cc string, streets ...string) string {
	var b strings.Builder
	b.WriteString(`<contact:postalInfo type="` + typ + `"><contact:name>` + name + `</contact:name>`)
	if org != "" {
		b.WriteString("<contact:org>" + org + "</contact:org>")
	}
	b.WriteString("<contact:addr>")
	for _, st := range streets {
		b.WriteString("<contact:street>" + st + "</contact:street>")
	}
	b.WriteString("<contact:city>" + city + "</contact:city><contact:sp>VA</contact:sp><contact:pc>20166</contact:pc><contact:cc>" + cc + "</contact:cc></contact:addr></contact:postalInfo>")
	return b.String()
}

func contactCreateXML(id string, o contactOpts) string {
	def := func(v, d string) string {
		switch v {
		case "":
			return d
		case "-":
			return ""
		}
		return v
	}
	o.postalInfo = def(o.postalInfo, postalInfoXML("int", "John Doe", "Example Inc.", "Dulles", "US", "123 Example Dr.", "Suite 100"))
	o.voice = def(o.voice, `<contact:voice>+1.7035555555</contact:voice>`)
	o.fax = def(o.fax, "")
	o.email = def(o.email, "jdoe@example.com")
	o.pw = def(o.pw, "Contact-Auth-1!")
	return contactCmdXML("create", "<contact:id>"+id+"</contact:id>"+o.postalInfo+o.voice+o.fax+
		"<contact:email>"+o.email+"</contact:email><contact:authInfo><contact:pw>"+o.pw+"</contact:pw></contact:authInfo>"+o.disclose)
}

func contactUpdateXML(id, changes string) string {
	return contactCmdXML("update", "<contact:id>"+id+"</contact:id>"+changes)
}

func contactStatusXML(s string) string { return `<contact:status s="` + s + `"/>` }

func TestContactCreateInfoDelete(t *testing.T) {
	_, _, c := objectServer(t)
	resp := c.roundTrip(contactCreateXML("jdoe-1", contactOpts{
		postalInfo: postalInfoXML("int", "John Doe", "Example Inc.", "Dulles", "US", "123 Example Dr.", "Suite 100") +
			postalInfoXML("loc", "Jöhn Döe", "", "Dülles", "us"),
		fax:      `<contact:fax>+1.7035555556</contact:fax>`,
		email:    "JDoe@Example.com",
		disclose: `<contact:disclose flag="1"><contact:name type="int"/><contact:email/></contact:disclose>`,
	}))
	requireCode(t, resp, "1000")
	assert.Equal(t, "jdoe-1", text(resp, "//resData/creData/id"))
	assert.NotEmpty(t, text(resp, "//resData/creData/crDate"))

	info := c.roundTrip(contactInfoXML("jdoe-1"))
	requireCode(t, info, "1000")
	requireSchemaValid(t, []byte(mustWrite(t, info)))
	assert.Regexp(t, `^\d+_CONT-EPPTEST$`, text(info, "//infData/roid"))
	assert.Equal(t, []string{"ok"}, statuses(info))
	intPI := info.FindElement(`//infData/postalInfo[@type='int']`)
	require.NotNil(t, intPI)
	assert.Equal(t, "John Doe", text(info, `//infData/postalInfo[@type='int']/name`))
	assert.Equal(t, "Example Inc.", text(info, `//infData/postalInfo[@type='int']/org`))
	assert.Equal(t, []string{"123 Example Dr.", "Suite 100"}, texts(intPI.FindElements("addr/street")))
	assert.Equal(t, "US", text(info, `//infData/postalInfo[@type='int']/addr/cc`))
	assert.Equal(t, "Jöhn Döe", text(info, `//infData/postalInfo[@type='loc']/name`))
	assert.Equal(t, "US", text(info, `//infData/postalInfo[@type='loc']/addr/cc`), "a country code as stored: upper case")
	assert.Equal(t, "+1.7035555555", text(info, "//infData/voice"))
	assert.Equal(t, "+1.7035555556", text(info, "//infData/fax"))
	assert.Equal(t, "jdoe@example.com", text(info, "//infData/email"))
	assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
	assert.Equal(t, testRegistrar, text(info, "//infData/crID"))
	assert.Equal(t, "Contact-Auth-1!", text(info, "//infData/authInfo/pw"), "the sponsor sees its authorization information")
	disclose := info.FindElement("//infData/disclose")
	require.NotNil(t, disclose)
	assert.Equal(t, "1", disclose.SelectAttrValue("flag", ""))
	assert.NotNil(t, disclose.FindElement(`name[@type='int']`))
	assert.NotNil(t, disclose.SelectElement("email"))
	assert.Nil(t, disclose.SelectElement("voice"))

	requireCode(t, c.roundTrip(contactDeleteXML("jdoe-1")), "1000")
	requireCode(t, c.roundTrip(contactInfoXML("jdoe-1")), "2303")
}

func TestContactCreateRefusals(t *testing.T) {
	_, _, c := objectServer(t)
	long := strings.Repeat("x", 256)
	for _, tc := range []struct {
		name, frame, code, says string
	}{
		{"an ID longer than 16", contactCreateXML(strings.Repeat("a", 17), contactOpts{}), "2001", ""},
		{"an empty ID", contactCreateXML("", contactOpts{}), "2001", ""},
		{"a postal info type that is not int or loc", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("tni", "John", "", "Dulles", "US")}), "2001", ""},
		{"an empty name", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "", "", "Dulles", "US")}), "2001", ""},
		{"an org longer than 255", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", long, "Dulles", "US")}), "2001", ""},
		{"a street longer than 255", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "Dulles", "US", long)}), "2001", ""},
		{"an empty city", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "", "US")}), "2001", ""},
		{"a city longer than 255", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", long, "US")}), "2001", ""},
		{"a country code that is not ISO 3166", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "Dulles", "XX")}), "2005", "XX"},
		{"a one-letter country code", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "Dulles", "U")}), "2001", ""},
		{"a three-letter country code", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "Dulles", "USA")}), "2001", ""},
		{"a name outside ASCII in an int address", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "Jöhn", "", "Dulles", "US")}), "2005", "ASCII"},
		{"two int addresses", contactCreateXML("ref-1", contactOpts{postalInfo: postalInfoXML("int", "John", "", "Dulles", "US") + postalInfoXML("int", "Jane", "", "Dulles", "US")}), "2306", "one postal address of each type"},
		{"an empty voice number", contactCreateXML("ref-1", contactOpts{voice: `<contact:voice/>`}), "2306", "leave the element out"},
		{"an empty fax number", contactCreateXML("ref-1", contactOpts{fax: `<contact:fax></contact:fax>`}), "2306", "leave the element out"},
		{"a voice extension", contactCreateXML("ref-1", contactOpts{voice: `<contact:voice x="1234">+1.7035555555</contact:voice>`}), "2102", "extensions are not supported"},
		{"a fax extension", contactCreateXML("ref-1", contactOpts{fax: `<contact:fax x="1">+1.7035555556</contact:fax>`}), "2102", "extensions are not supported"},
		{"an email without @", contactCreateXML("ref-1", contactOpts{email: "example"}), "2005", "invalid email"},
		{"an email without a local part", contactCreateXML("ref-1", contactOpts{email: "@example.com"}), "2005", "invalid email"},
		{"an email of only @", contactCreateXML("ref-1", contactOpts{email: "@"}), "2005", "invalid email"},
		{"an email whose domain has one label", contactCreateXML("ref-1", contactOpts{email: "jdoe@localhost"}), "2005", "invalid email"},
		{"an email with a local part longer than 64", contactCreateXML("ref-1", contactOpts{email: strings.Repeat("a", 65) + "@example.com"}), "2005", "64"},
		{"an email with a display name", contactCreateXML("ref-1", contactOpts{email: "John Doe &lt;jdoe@example.com&gt;"}), "2005", "invalid email"},
		{"authorization information that breaks the registry's rules", contactCreateXML("ref-1", contactOpts{pw: "weakpass"}), "2306", ""},
		{"an ID in use", contactCreateXML("registrar2-c1", contactOpts{}), "2302", "already exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.says)
		})
	}
	for _, v := range []string{"+12345", "+1111.1", "55", "+1.", "+1.123456789012345", "+1.a", "+a.1"} {
		t.Run("an invalid voice number "+v, func(t *testing.T) {
			requireCode(t, c.roundTrip(contactCreateXML("ref-1", contactOpts{voice: `<contact:voice>` + v + `</contact:voice>`})), "2001")
		})
	}
	requireCode(t, c.roundTrip(contactInfoXML("ref-1")), "2303")
}

func TestContactCheck(t *testing.T) {
	_, _, c := objectServer(t)
	resp := c.roundTrip(contactCheckXML("registrar1-c1", "registrar2-c1", "free-1"))
	requireCode(t, resp, "1000")
	requireSchemaValid(t, []byte(mustWrite(t, resp)))
	type answer struct{ avail, reason string }
	got := map[string]answer{}
	for _, cd := range resp.FindElements("//resData/chkData/cd") {
		id := cd.SelectElement("id")
		reason := ""
		if r := cd.SelectElement("reason"); r != nil {
			reason = r.Text()
		}
		got[id.Text()] = answer{id.SelectAttrValue("avail", ""), reason}
	}
	assert.Equal(t, map[string]answer{
		"registrar1-c1": {"0", "In use"},
		"registrar2-c1": {"0", "In use"}, // an ID is unique in the registry
		"free-1":        {"1", ""},
	}, got)

	t.Run("more IDs than EPP_MAX_CHECK_NAMES", func(t *testing.T) {
		ids := make([]string, testConfig().Commands.MaxCheckNames+1)
		for i := range ids {
			ids[i] = "free-1"
		}
		requireCode(t, c.roundTrip(contactCheckXML(ids...)), "2306")
	})
}

func TestContactSponsorship(t *testing.T) {
	_, _, c := objectServer(t)
	for _, tc := range []struct{ name, frame, code string }{
		{"info on another registrar's contact", contactInfoXML("registrar2-c1"), "2201"},
		{"info with its authorization information", contactCmdXML("info", `<contact:id>registrar2-c1</contact:id><contact:authInfo><contact:pw>Contact-Auth-1!</contact:pw></contact:authInfo>`), "2201"},
		{"update of another registrar's contact", contactUpdateXML("registrar2-c1", "<contact:chg><contact:email>x@example.com</contact:email></contact:chg>"), "2201"},
		{"delete of another registrar's contact", contactDeleteXML("registrar2-c1"), "2201"},
		{"info on a contact nobody has", contactInfoXML("nobody-1"), "2303"},
		{"update of a contact nobody has", contactUpdateXML("nobody-1", "<contact:chg><contact:email>x@example.com</contact:email></contact:chg>"), "2303"},
		{"delete of a contact nobody has", contactDeleteXML("nobody-1"), "2303"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.NotContains(t, msg(resp), "registrar2,", "the sponsor is not disclosed")
		})
	}
}

func TestContactUpdate(t *testing.T) {
	creds, _, c := objectServer(t)
	reg := creds.domains.registry
	requireCode(t, c.roundTrip(contactCreateXML("upd-1", contactOpts{})), "1000")
	update := func(changes string) *etree.Document { return c.roundTrip(contactUpdateXML("upd-1", changes)) }
	info := func() *etree.Document { return c.roundTrip(contactInfoXML("upd-1")) }
	infData := func() string {
		doc := etree.NewDocument()
		doc.SetRoot(info().FindElement("//resData/infData").Copy())
		return mustWrite(t, doc)
	}
	add := func(s string) string { return "<contact:add>" + contactStatusXML(s) + "</contact:add>" }
	rem := func(s string) string { return "<contact:rem>" + contactStatusXML(s) + "</contact:rem>" }
	chg := func(inner string) string { return "<contact:chg>" + inner + "</contact:chg>" }

	t.Run("a change of only part of a postal address keeps the rest", func(t *testing.T) {
		requireCode(t, update(chg(`<contact:postalInfo type="int"><contact:name>Jane Doe</contact:name></contact:postalInfo>`)), "1000")
		i := info()
		assert.Equal(t, "Jane Doe", text(i, `//infData/postalInfo[@type='int']/name`))
		assert.Equal(t, "Example Inc.", text(i, `//infData/postalInfo[@type='int']/org`))
		assert.Equal(t, "Dulles", text(i, `//infData/postalInfo[@type='int']/addr/city`))
		assert.Equal(t, testRegistrar, text(i, "//infData/upID"))
	})
	t.Run("an empty org removes it", func(t *testing.T) {
		requireCode(t, update(chg(`<contact:postalInfo type="int"><contact:org/></contact:postalInfo>`)), "1000")
		assert.Nil(t, info().FindElement(`//infData/postalInfo[@type='int']/org`))
	})
	t.Run("a new postal address type must be given whole", func(t *testing.T) {
		requireCode(t, update(chg(`<contact:postalInfo type="loc"><contact:name>Jäne</contact:name></contact:postalInfo>`)), "2003")
		requireCode(t, update(chg(postalInfoXML("loc", "Jäne Döe", "", "Dülles", "US"))), "1000")
		assert.Equal(t, "Jäne Döe", text(info(), `//infData/postalInfo[@type='loc']/name`))
	})
	t.Run("phone numbers, email and authorization information change; an empty number removes it", func(t *testing.T) {
		requireCode(t, update(chg(`<contact:voice/><contact:fax>+1.7035555557</contact:fax><contact:email>jane@example.net</contact:email><contact:authInfo><contact:pw>New-Auth-2?</contact:pw></contact:authInfo>`)), "1000")
		i := info()
		assert.Nil(t, i.FindElement("//infData/voice"))
		assert.Equal(t, "+1.7035555557", text(i, "//infData/fax"))
		assert.Equal(t, "jane@example.net", text(i, "//infData/email"))
		assert.Equal(t, "New-Auth-2?", text(i, "//infData/authInfo/pw"))
	})
	t.Run("disclosure is set and withdrawn", func(t *testing.T) {
		requireCode(t, update(chg(`<contact:disclose flag="1"><contact:voice/><contact:email/></contact:disclose>`)), "1000")
		assert.NotNil(t, info().FindElement("//infData/disclose/email"))
		requireCode(t, update(chg(`<contact:disclose flag="0"><contact:voice/><contact:email/></contact:disclose>`)), "1000")
		assert.Nil(t, info().FindElement("//infData/disclose"))
	})
	for _, tc := range []struct{ name, changes, code string }{
		{"no add, rem or chg", "", "2003"},
		{"two int addresses", chg(`<contact:postalInfo type="int"><contact:name>A</contact:name></contact:postalInfo><contact:postalInfo type="int"><contact:name>B</contact:name></contact:postalInfo>`), "2306"},
		{"a country code that is not ISO 3166", chg(postalInfoXML("int", "Jane", "", "Dulles", "XX")), "2005"},
		{"an invalid email", chg("<contact:email>example.example</contact:email>"), "2005"},
		{"a voice extension", chg(`<contact:voice x="9">+1.7035555555</contact:voice>`), "2102"},
		{"adding a status the server sets", add("serverUpdateProhibited"), "2306"},
		{"adding linked", add("linked"), "2306"},
		{"adding ok", add("ok"), "2306"},
		{"adding a pending status", add("pendingDelete"), "2306"},
		{"removing a status the server sets", rem("serverDeleteProhibited"), "2306"},
		{"removing a status the contact does not have", rem("clientDeleteProhibited"), "2306"},
		{"a status the schema does not have", add("clientHold"), "2001"},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			before := infData()
			requireCode(t, update(tc.changes), tc.code)
			assert.Equal(t, before, infData(), "nothing changed")
		})
	}
	t.Run("client statuses are added and removed, and added once", func(t *testing.T) {
		for _, st := range []string{"clientDeleteProhibited", "clientTransferProhibited"} {
			requireCode(t, update(add(st)), "1000")
			assert.Contains(t, statuses(info()), st)
			requireCode(t, update(add(st)), "2306")
			requireCode(t, update(rem(st)), "1000")
		}
		assert.Equal(t, []string{"ok"}, statuses(info()))
	})
	t.Run("clientUpdateProhibited refuses every update but one removing it", func(t *testing.T) {
		requireCode(t, update(add("clientUpdateProhibited")), "1000")
		requireCode(t, update(add("clientUpdateProhibited")), "2304")
		requireCode(t, update(chg("<contact:email>held@example.com</contact:email>")), "2304")
		requireCode(t, update(rem("clientUpdateProhibited")+chg("<contact:email>freed@example.com</contact:email>")), "1000")
		assert.Equal(t, "freed@example.com", text(info(), "//infData/email"))
	})
	t.Run("serverUpdateProhibited refuses every update", func(t *testing.T) {
		reg.SetContactStatus("upd-1", func(s *entities.ContactStatus) { s.ServerUpdateProhibited, s.OK = true, false })
		defer reg.SetContactStatus("upd-1", func(s *entities.ContactStatus) { s.ServerUpdateProhibited, s.OK = false, true })
		requireCode(t, update(add("clientDeleteProhibited")), "2304")
	})
}

func TestContactLinked(t *testing.T) {
	_, _, c := objectServer(t)
	requireCode(t, c.roundTrip(createXML("contacts.test", createOpts{})), "1000") // registrar1-c1 registrant, registrar1-c2 tech
	t.Run("info shows linked", func(t *testing.T) {
		assert.Equal(t, []string{"ok", "linked"}, statuses(c.roundTrip(contactInfoXML("registrar1-c1"))))
	})
	t.Run("a contact a domain uses is not deleted", func(t *testing.T) {
		requireCode(t, c.roundTrip(contactDeleteXML("registrar1-c2")), "2305")
		requireCode(t, c.roundTrip(contactInfoXML("registrar1-c2")), "1000")
	})
	t.Run("clientDeleteProhibited refuses a delete", func(t *testing.T) {
		requireCode(t, c.roundTrip(contactCreateXML("del-1", contactOpts{})), "1000")
		requireCode(t, c.roundTrip(contactUpdateXML("del-1", "<contact:add>"+contactStatusXML("clientDeleteProhibited")+"</contact:add>")), "1000")
		requireCode(t, c.roundTrip(contactDeleteXML("del-1")), "2304")
	})
}
