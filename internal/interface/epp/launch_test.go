package epp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The launch phase extension (RFC 8334, ADR 0024) for first-come phases:
// <launch:check type="avail"> and <launch:create> in an open or landrush
// phase. Applications, sunrise and claims are refused until they land.

// launchPhaseXML is a <launch:phase>, with a name attribute when name is
// not empty.
func launchPhaseXML(value, name string) string {
	attr := ""
	if name != "" {
		attr = ` name="` + name + `"`
	}
	return `<launch:phase` + attr + `>` + value + `</launch:phase>`
}

// launchCreateXML is a <launch:create> extension: the phase, the type
// attribute when typ is not empty, and any further content.
func launchCreateXML(value, name, typ, more string) string {
	attr := ""
	if typ != "" {
		attr = ` type="` + typ + `"`
	}
	return `<launch:create xmlns:launch="` + nsLaunch + `"` + attr + `>` + launchPhaseXML(value, name) + more + `</launch:create>`
}

// launchCheckXML is a <domain:check> of names with a <launch:check> of the
// given form (type attribute; none when form is empty) and phase (none when
// value is empty).
func launchCheckXML(form, value, name string, names ...string) string {
	var b string
	for _, n := range names {
		b += "<domain:name>" + n + "</domain:name>"
	}
	attr, phase := "", ""
	if form != "" {
		attr = ` type="` + form + `"`
	}
	if value != "" {
		phase = launchPhaseXML(value, name)
	}
	return cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+b+`</domain:check></check>
<extension><launch:check xmlns:launch="`+nsLaunch+`"`+attr+`>`+phase+`</launch:check></extension>`, "LCK-1")
}

// launchRegistry seeds .test with launch phases beside its GA phase: an
// open landrush (first come, labels of five or more), an ended sunrise, a
// future custom phase, a landrush that collects applications, and two open
// "open" phases so that mode is ambiguous.
func launchRegistry(t *testing.T, creds *testCredentials) {
	t.Helper()
	reg := creds.domains.registry
	now := time.Now().UTC()
	past, future := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	firstCome := func(mode entities.LaunchMode) entities.LaunchSettings {
		return entities.LaunchSettings{Mode: mode, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true}
	}
	landrush := reg.AddLaunchPhase("test", "landrush", firstCome(entities.LaunchModeLandrush), past, &future)
	landrush.Policy.MinLabelLength = 5
	ended := past.Add(-time.Hour)
	reg.AddLaunchPhase("test", "sunrise", firstCome(entities.LaunchModeSunrise), past.Add(-48*time.Hour), &ended)
	reg.AddLaunchPhase("test", "sunrise-open", firstCome(entities.LaunchModeSunrise), past, nil)
	reg.AddLaunchPhase("test", "later", firstCome(entities.LaunchModeCustom), future, nil)
	reg.AddLaunchPhase("test", "auction", entities.LaunchSettings{Mode: entities.LaunchModeCustom, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true}, past, nil)
	reg.AddLaunchPhase("test", "open-a", firstCome(entities.LaunchModeOpen), past, nil)
	reg.AddLaunchPhase("test", "open-b", firstCome(entities.LaunchModeOpen), past, nil)
}

func TestLaunchCreateInFirstComePhase(t *testing.T) {
	creds := newTestCredentials(t)
	launchRegistry(t, creds)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsLaunch})), "1000")

	t.Run("by mode, when one phase of it is open", func(t *testing.T) {
		resp := c.roundTrip(createXML("rushed.test", createOpts{ext: launchCreateXML("landrush", "", "registration", "")}))
		requireCode(t, resp, "1000")
		assert.Equal(t, "rushed.test", text(resp, "//creData/name"))
		assert.Nil(t, resp.FindElement("//response/extension"), "a registration answers without <launch:creData> (RFC 8334 §3.3.1)")
		d := creds.domains.registry.Domain("rushed.test")
		require.NotNil(t, d)
		assert.Equal(t, entities.ClIDType("landrush"), d.RegistrationPhaseName)
	})
	t.Run("by name, resolving an ambiguous mode", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("named.test", createOpts{ext: launchCreateXML("open", "open-b", "", "")})), "1000")
		assert.Equal(t, entities.ClIDType("open-b"), creds.domains.registry.Domain("named.test").RegistrationPhaseName)
	})
	t.Run("the launch phase's label rules apply", func(t *testing.T) {
		resp := c.roundTrip(createXML("ab.test", createOpts{ext: launchCreateXML("landrush", "", "", "")}))
		requireCode(t, resp, "2306")
		assert.Contains(t, msg(resp), "phase")
		requireCode(t, c.roundTrip(createXML("ab.test", createOpts{})), "1000") // the GA phase allows short labels
		assert.Empty(t, creds.domains.registry.Domain("ab.test").RegistrationPhaseName, "a GA registration names no phase")
	})
}

func TestLaunchCreateRefusals(t *testing.T) {
	creds := newTestCredentials(t)
	launchRegistry(t, creds)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsLaunch})), "1000")

	notice := `<launch:notice><launch:noticeID validatorID="tmch">370d0b7c9223372036854775807</launch:noticeID><launch:notAfter>2026-11-22T09:30:47Z</launch:notAfter><launch:acceptedDate>2026-11-19T09:01:30Z</launch:acceptedDate></launch:notice>`
	smd := `<smd:encodedSignedMark xmlns:smd="urn:ietf:params:xml:ns:signedMark-1.0">QUJDRA==</smd:encodedSignedMark>`
	cases := []struct {
		name, ext, code, msgPart string
	}{
		{"an unknown phase name", launchCreateXML("custom", "nope", "", ""), "2306", "phase not found"},
		{"the GA phase by name", launchCreateXML("open", "GA-test", "", ""), "2306", "not a launch phase"},
		{"an ended phase", launchCreateXML("sunrise", "sunrise", "", ""), "2306", "not active"},
		{"a future phase", launchCreateXML("custom", "later", "", ""), "2306", "not active"},
		{"a mode with no phase open", launchCreateXML("claims", "", "", ""), "2306", "no claims phase is open"},
		{"a mode with two phases open", launchCreateXML("open", "", "", ""), "2306", "name the phase"},
		{"a name whose mode is not the value", launchCreateXML("open", "landrush", "", ""), "2306", "not of the mode"},
		{"type=registration in a phase that collects applications", launchCreateXML("custom", "auction", "registration", ""), "2306", "collects applications"},
		{"a sunrise phase", launchCreateXML("sunrise", "sunrise-open", "", ""), "2306", "not yet supported"},
		{"type=application in a first-come phase", launchCreateXML("landrush", "", "application", ""), "2306", "registers at once"},
		{"a claims notice", launchCreateXML("landrush", "", "registration", notice), "2306", "not yet verified"},
		{"an encoded signed mark", launchCreateXML("landrush", "", "", smd), "2306", "not yet verified"},
		{"a code mark", launchCreateXML("landrush", "", "", `<launch:codeMark><launch:code>49FD46E6</launch:code></launch:codeMark>`), "2306", "not yet verified"},
		{"a decoded signed mark is not modelled", launchCreateXML("landrush", "", "", `<smd:signedMark xmlns:smd="urn:ietf:params:xml:ns:signedMark-1.0" id="x"/>`), "2001", "signedMark"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(createXML("rushed.test", createOpts{ext: tc.ext}))
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.msgPart)
		})
	}
	assert.Empty(t, creds.domains.registry.Registered, "none of them registered anything")

	t.Run("info, update and delete with a launch extension are about an application, never the domain", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("mine.test", createOpts{})), "1000")
		id := `<launch:phase>landrush</launch:phase><launch:applicationID>1_APP-EPPTEST</launch:applicationID>`
		for _, tc := range []struct{ name, frame string }{
			{"info", cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name></domain:info></info>
<extension><launch:info xmlns:launch="`+nsLaunch+`">`+id+`</launch:info></extension>`, "LI-1")},
			{"update", cmdXML(`<update><domain:update xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name><domain:chg/></domain:update></update>
<extension><launch:update xmlns:launch="`+nsLaunch+`">`+id+`</launch:update></extension>`, "LU-1")},
			{"delete", cmdXML(`<delete><domain:delete xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name></domain:delete></delete>
<extension><launch:delete xmlns:launch="`+nsLaunch+`">`+id+`</launch:delete></extension>`, "LD-1")},
		} {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, "2303")
			assert.Contains(t, msg(resp), "application", tc.name)
		}
		d := creds.domains.registry.Domain("mine.test")
		assert.False(t, d.Status.PendingDelete, "the delete with <launch:delete> did nothing to the domain")
	})

	t.Run("the extension must be negotiated at login", func(t *testing.T) {
		c2 := dial(t, ts.addr)
		requireCode(t, c2.roundTrip(loginWithServices([]string{nsDomain}, nil)), "1000")
		resp := c2.roundTrip(createXML("neg.test", createOpts{ext: launchCreateXML("landrush", "", "", "")}))
		requireCode(t, resp, "2103")
	})
}

// A launch extension element is named for the domain command it extends;
// on any other command it is 2001, before the handler, so that it can
// neither be ignored (a <domain:delete> with <launch:update> deleting, a
// <domain:create> with <launch:check> registering in GA) nor act.
func TestLaunchExtensionMustMatchCommand(t *testing.T) {
	creds := newTestCredentials(t)
	launchRegistry(t, creds)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain, nsHost, nsContact}, []string{nsLaunch, nsSecDNS})), "1000")
	requireCode(t, c.roundTrip(createXML("mine.test", createOpts{})), "1000")

	domain := `xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"`
	ext := func(tag, inner string) string {
		return `<extension><launch:` + tag + ` xmlns:launch="` + nsLaunch + `">` + inner + `</launch:` + tag + `></extension>`
	}
	id := `<launch:phase>landrush</launch:phase><launch:applicationID>abc-1</launch:applicationID>`
	for _, tc := range []struct{ name, frame, want string }{
		{"<domain:delete> with <launch:update>", cmdXML(`<delete><domain:delete `+domain+`><domain:name>mine.test</domain:name></domain:delete></delete>`+ext("update", id), "MX-1"), "<launch:update> extends <domain:update>, not <domain:delete>"},
		{"<domain:create> with <launch:check>", createXML("other.test", createOpts{ext: `<launch:check xmlns:launch="` + nsLaunch + `" type="avail">` + launchPhaseXML("landrush", "") + `</launch:check>`}), "<launch:check> extends <domain:check>, not <domain:create>"},
		{"<domain:renew> with <launch:create>", cmdXML(`<renew><domain:renew `+domain+`><domain:name>mine.test</domain:name><domain:curExpDate>`+time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02")+`</domain:curExpDate></domain:renew></renew><extension>`+launchCreateXML("landrush", "", "", "")+`</extension>`, "MX-3"), "<launch:create> extends <domain:create>, not <domain:renew>"},
		{"<domain:update> with <launch:delete>", domainUpdateXML("mine.test", "<domain:chg/>", `<launch:delete xmlns:launch="`+nsLaunch+`">`+id+`</launch:delete>`), "<launch:delete> extends <domain:delete>, not <domain:update>"},
		{"<host:info> with <launch:info>", cmdXML(`<info><host:info xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns1.mine.test</host:name></host:info></info>`+ext("info", id), "MX-5"), "<launch:info> extends <domain:info>, not <host:info>"},
		{"<contact:check> with <launch:check>", cmdXML(`<check><contact:check xmlns:contact="urn:ietf:params:xml:ns:contact-1.0"><contact:id>registrar1-c1</contact:id></contact:check></check>`+ext("check", ""), "MX-6"), "<launch:check> extends <domain:check>, not <contact:check>"},
		{"<domain:delete> with <secDNS:update>", cmdXML(`<delete><domain:delete `+domain+`><domain:name>mine.test</domain:name></domain:delete></delete><extension>`+secDNSUpdateXML("", "<secDNS:rem><secDNS:all>true</secDNS:all></secDNS:rem>")+`</extension>`, "MX-7"), "<secDNS:update> extends <domain:update>, not <domain:delete>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, "2001")
			assert.Contains(t, msg(resp), tc.want)
		})
	}
	reg := creds.domains.registry
	assert.False(t, reg.Domain("mine.test").Status.PendingDelete, "the deletes did nothing")
	assert.Nil(t, reg.Domain("other.test"), "the create with <launch:check> registered nothing")
	assert.Len(t, reg.Registered, 1, "only mine.test was registered")
}

// A processing instruction or comment inside an element is not content:
// the schema accepts <launch:phase><?x?>landrush</launch:phase> as the value
// "landrush", and the handler must read the same value, not an empty one
// that would fall back to the GA phase (or, for <domain:name>, to no name).
func TestElementValuesSkipProcessingInstructions(t *testing.T) {
	creds := newTestCredentials(t)
	launchRegistry(t, creds)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsLaunch})), "1000")

	phase := `<launch:phase><?x?>land<!-- c -->rush</launch:phase>`
	ext := `<launch:create xmlns:launch="` + nsLaunch + `">` + phase + `</launch:create>`
	t.Run("create in the launch phase named", func(t *testing.T) {
		resp := c.roundTrip(createXML("ab.test", createOpts{ext: ext}))
		requireCode(t, resp, "2306") // the landrush minimum of five letters applies, so this was not GA
		assert.Contains(t, msg(resp), "phase")
		requireCode(t, c.roundTrip(createXML("rushed.test", createOpts{ext: ext})), "1000")
		assert.Equal(t, entities.ClIDType("landrush"), creds.domains.registry.Domain("rushed.test").RegistrationPhaseName)
	})
	t.Run("check in the launch phase named", func(t *testing.T) {
		resp := c.roundTrip(cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>ab.test</domain:name></domain:check></check>
<extension><launch:check xmlns:launch="`+nsLaunch+`" type="avail">`+phase+`</launch:check></extension>`, "LCK-PI"))
		requireCode(t, resp, "1000")
		assert.Equal(t, "0", resp.FindElement("//chkData/cd/name").SelectAttrValue("avail", ""), "two letters, under the landrush minimum")
	})
	t.Run("a domain name", func(t *testing.T) {
		resp := c.roundTrip(cmdXML(`<create><domain:create xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name><?x?>pi.test</domain:name><domain:registrant>registrar1-c1</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact><domain:authInfo><domain:pw><!-- c -->Tr4nsf3r-C0de-Xq9#Lm2!</domain:pw></domain:authInfo></domain:create></create>`, "CRE-PI"))
		requireCode(t, resp, "1000")
		assert.Equal(t, "pi.test", text(resp, "//creData/name"))
		require.NotNil(t, creds.domains.registry.Domain("pi.test"))
	})
}

func TestLaunchCheckAvailability(t *testing.T) {
	creds := newTestCredentials(t)
	launchRegistry(t, creds)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsLaunch})), "1000")

	t.Run("the availability form answers in <domain:chkData> under the phase's rules", func(t *testing.T) {
		resp := c.roundTrip(launchCheckXML("avail", "landrush", "", "ab.test", "rushed.test", "registered.test"))
		requireCode(t, resp, "1000")
		cds := resp.FindElements("//chkData/cd")
		require.Len(t, cds, 3)
		assert.Equal(t, "0", cds[0].SelectElement("name").SelectAttrValue("avail", ""), "two letters, under the landrush minimum of five")
		assert.Equal(t, "Not allowed by registry policy", cds[0].SelectElement("reason").Text())
		assert.Equal(t, "1", cds[1].SelectElement("name").SelectAttrValue("avail", ""))
		assert.Equal(t, "0", cds[2].SelectElement("name").SelectAttrValue("avail", ""))
		assert.Equal(t, "In use", cds[2].SelectElement("reason").Text())
		assert.Nil(t, resp.FindElement("//response/extension"), "no <launch:chkData> in the availability form (RFC 8334 §3.1.1.2)")
	})
	t.Run("without a phase the GA phase applies", func(t *testing.T) {
		resp := c.roundTrip(launchCheckXML("avail", "", "", "ab.test"))
		requireCode(t, resp, "1000")
		assert.Equal(t, "1", resp.FindElement("//chkData/cd/name").SelectAttrValue("avail", ""))
	})
	t.Run("a phase that cannot be registered in refuses the command", func(t *testing.T) {
		for _, tc := range []struct{ value, name, msgPart string }{
			{"custom", "nope", "phase not found"},
			{"sunrise", "sunrise", "not active"},
			{"open", "", "name the phase"},
			{"open", "GA-test", "not a launch phase"},
		} {
			resp := c.roundTrip(launchCheckXML("avail", tc.value, tc.name, "rushed.test"))
			requireCode(t, resp, "2306")
			assert.Contains(t, msg(resp), tc.msgPart)
		}
	})
	t.Run("the claims form, the default, and the trademark form are not served", func(t *testing.T) {
		for _, form := range []string{"", "claims", "trademark"} {
			resp := c.roundTrip(launchCheckXML(form, "landrush", "", "rushed.test"))
			requireCode(t, resp, "2102")
			assert.Contains(t, msg(resp), "avail")
		}
	})
}

// Applications (RFC 8334 §3.3-3.5, ADR 0024 §3) in the "auction" phase of
// launchRegistry, a custom phase that collects applications.
func TestLaunchApplications(t *testing.T) {
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	launchRegistry(t, creds)
	reg := creds.domains.registry
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsLaunch, nsSecDNS})), "1000")

	launchRef := func(tag, phase, id string) string {
		return `<launch:` + tag + ` xmlns:launch="` + nsLaunch + `">` + phase + `<launch:applicationID>` + id + `</launch:applicationID></launch:` + tag + `>`
	}
	infoXML := func(name, ext string) string {
		return cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name></domain:info></info><extension>`+ext+`</extension>`, "AI-1")
	}
	deleteXML := func(name, ext string) string {
		return cmdXML(`<delete><domain:delete xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name></domain:delete></delete><extension>`+ext+`</extension>`, "AD-1")
	}

	var id string
	t.Run("create files an application: 1001 with <launch:creData>", func(t *testing.T) {
		resp := c.roundTrip(createXML("bid.test", createOpts{
			ns:  `<domain:ns><domain:hostObj>ns1.` + testRegistrar + `.net</domain:hostObj></domain:ns>`,
			ext: launchCreateXML("custom", "auction", "application", "") + secDNSCreate(dsDataXML(2371, 13, 2, "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4")),
		}))
		requireCode(t, resp, "1001")
		assert.Equal(t, "bid.test", text(resp, "//creData/name"))
		assert.NotEmpty(t, text(resp, "//creData/crDate"))
		assert.Nil(t, resp.FindElement("//creData/exDate"), "no expiry until allocation")
		id = text(resp, "//response/extension/creData/applicationID")
		require.NotEmpty(t, id)
		phase := resp.FindElement("//response/extension/creData/phase")
		require.NotNil(t, phase)
		assert.Equal(t, "custom", phase.Text())
		assert.Equal(t, "auction", phase.SelectAttrValue("name", ""))
		assert.Empty(t, reg.Registered, "nothing is registered")
		app := reg.Application(id)
		require.NotNil(t, app)
		assert.Equal(t, entities.ApplicationStatusPendingAllocation, app.Status)
		assert.Equal(t, entities.ClIDType(testRegistrar), app.ClID)
	})
	t.Run("the type may be left out; type=registration is a mismatch", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("second.test", createOpts{ext: launchCreateXML("custom", "auction", "", "")})), "1001")
		resp := c.roundTrip(createXML("third.test", createOpts{ext: launchCreateXML("custom", "auction", "registration", "")}))
		requireCode(t, resp, "2306")
		assert.Contains(t, msg(resp), "collects applications")
	})
	t.Run("one open application per registrar, name and phase", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("bid.test", createOpts{ext: launchCreateXML("custom", "auction", "application", "")})), "2302")
	})
	t.Run("info by ID", func(t *testing.T) {
		resp := c.roundTrip(infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "auction"), id)))
		requireCode(t, resp, "1000")
		assert.Equal(t, "bid.test", text(resp, "//infData/name"))
		assert.Equal(t, id, text(resp, "//infData/roid"), "the application ID stands as the ROID")
		assert.Equal(t, "pendingCreate", resp.FindElement("//resData/infData/status").SelectAttrValue("s", ""))
		assert.Equal(t, testRegistrar+"-c1", text(resp, "//infData/registrant"))
		assert.Equal(t, testRegistrar, text(resp, "//infData/clID"))
		assert.NotNil(t, resp.FindElement("//infData/authInfo/pw"), "a code is set")
		assert.Nil(t, resp.FindElement("//infData/exDate"))
		launch := resp.FindElement("//response/extension/infData")
		require.NotNil(t, launch)
		assert.Equal(t, id, launch.SelectElement("applicationID").Text())
		assert.Equal(t, "pendingAllocation", launch.SelectElement("status").SelectAttrValue("s", ""))
		assert.Equal(t, "custom", launch.SelectElement("phase").Text())
		assert.Equal(t, "ns1."+testRegistrar+".net", text(resp, "//resData/infData/ns/hostObj"))
		assert.Equal(t, "2371", text(resp, "//response/extension/infData/dsData/keyTag"), "the DS records, as the session logged in with secDNS")
	})
	t.Run("info with hosts=none leaves the nameservers out", func(t *testing.T) {
		resp := c.roundTrip(cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name hosts="none">bid.test</domain:name></domain:info></info><extension>`+launchRef("info", launchPhaseXML("custom", "auction"), id)+`</extension>`, "AI-2"))
		requireCode(t, resp, "1000")
		assert.Nil(t, resp.FindElement("//resData/infData/ns"))
	})
	t.Run("info refusals", func(t *testing.T) {
		for _, tc := range []struct{ name, frame, code, msgPart string }{
			{"no application named", infoXML("bid.test", `<launch:info xmlns:launch="`+nsLaunch+`">`+launchPhaseXML("custom", "auction")+`</launch:info>`), "2306", "applicationID"},
			{"an unknown application", infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "auction"), "9_APP-EPPTEST")), "2303", "application"},
			{"another name", infoXML("other.test", launchRef("info", launchPhaseXML("custom", "auction"), id)), "2306", "is for bid.test"},
			{"another phase, by mode", infoXML("bid.test", launchRef("info", launchPhaseXML("landrush", ""), id)), "2306", "a custom phase"},
			{"another phase, by name", infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "later"), id)), "2306", "is in auction"},
		} {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.msgPart, tc.name)
		}
	})
	t.Run("another registrar sees nothing", func(t *testing.T) {
		c2 := dial(t, ts.addr)
		requireCode(t, c2.roundTrip(cmdXML(`<login><clID>registrar2</clID><pw>secret-pw-02</pw>
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>`+nsDomain+`</objURI><svcExtension><extURI>`+nsLaunch+`</extURI></svcExtension></svcs></login>`, "R2-LOGIN")), "1000")
		requireCode(t, c2.roundTrip(infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "auction"), id))), "2201")
		requireCode(t, c2.roundTrip(deleteXML("bid.test", launchRef("delete", launchPhaseXML("custom", "auction"), id))), "2201")
		assert.NotNil(t, reg.Application(id))
	})
	t.Run("update changes the application", func(t *testing.T) {
		resp := c.roundTrip(domainUpdateXML("bid.test",
			`<domain:add><domain:ns><domain:hostObj>ns2.`+testRegistrar+`.net</domain:hostObj></domain:ns></domain:add><domain:chg><domain:registrant>`+testRegistrar+`-c2</domain:registrant></domain:chg>`,
			launchRef("update", launchPhaseXML("custom", "auction"), id)))
		requireCode(t, resp, "1000")
		app := reg.Application(id)
		assert.Equal(t, []string{"ns1." + testRegistrar + ".net", "ns2." + testRegistrar + ".net"}, app.HostNames)
		assert.Equal(t, entities.ClIDType(testRegistrar+"-c2"), app.RegistrantID)
		resp = c.roundTrip(domainUpdateXML("bid.test", `<domain:add><domain:status s="clientHold"/></domain:add>`, launchRef("update", launchPhaseXML("custom", "auction"), id)))
		requireCode(t, resp, "2306")
		assert.Contains(t, msg(resp), "statuses")
		resp = c.roundTrip(domainUpdateXML("bid.test", "", launchRef("update", launchPhaseXML("custom", "auction"), id)))
		requireCode(t, resp, "2003")
	})
	t.Run("a final application cannot change", func(t *testing.T) {
		reg.SetApplication(id, func(a *entities.LaunchApplication) { a.Status = entities.ApplicationStatusRejected })
		defer reg.SetApplication(id, func(a *entities.LaunchApplication) { a.Status = entities.ApplicationStatusPendingAllocation })
		requireCode(t, c.roundTrip(domainUpdateXML("bid.test", `<domain:chg><domain:registrant>`+testRegistrar+`-c1</domain:registrant></domain:chg>`, launchRef("update", launchPhaseXML("custom", "auction"), id))), "2304")
		requireCode(t, c.roundTrip(deleteXML("bid.test", launchRef("delete", launchPhaseXML("custom", "auction"), id))), "2304")
		resp := c.roundTrip(infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "auction"), id)))
		requireCode(t, resp, "1000")
		assert.Nil(t, resp.FindElement("//resData/infData/status"), "no pendingCreate once final")
		assert.Equal(t, "rejected", resp.FindElement("//response/extension/infData/status").SelectAttrValue("s", ""))
	})
	t.Run("delete withdraws it", func(t *testing.T) {
		requireCode(t, c.roundTrip(deleteXML("bid.test", launchRef("delete", launchPhaseXML("custom", "auction"), id))), "1000")
		assert.Nil(t, reg.Application(id))
		requireCode(t, c.roundTrip(infoXML("bid.test", launchRef("info", launchPhaseXML("custom", "auction"), id))), "2303")
		assert.Nil(t, reg.Domain("bid.test"), "there never was a domain")
		requireCode(t, c.roundTrip(createXML("bid.test", createOpts{ext: launchCreateXML("custom", "auction", "application", "")})), "1001")
	})
}
