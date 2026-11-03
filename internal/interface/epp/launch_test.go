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
		{"a phase that collects applications", launchCreateXML("custom", "auction", "", ""), "2306", "applications"},
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

	t.Run("applications do not exist yet: info, update and delete", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("mine.test", createOpts{})), "1000")
		id := `<launch:phase>landrush</launch:phase><launch:applicationID>abc-1</launch:applicationID>`
		for _, tc := range []struct{ name, frame string }{
			{"info", cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name></domain:info></info>
<extension><launch:info xmlns:launch="`+nsLaunch+`">`+id+`</launch:info></extension>`, "LI-1")},
			{"update", cmdXML(`<update><domain:update xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name><domain:chg/></domain:update></update>
<extension><launch:update xmlns:launch="`+nsLaunch+`">`+id+`</launch:update></extension>`, "LU-1")},
			{"delete", cmdXML(`<delete><domain:delete xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name></domain:delete></delete>
<extension><launch:delete xmlns:launch="`+nsLaunch+`">`+id+`</launch:delete></extension>`, "LD-1")},
		} {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, "2306")
			assert.Contains(t, msg(resp), "applications", tc.name)
		}
		d := creds.domains.registry.Domain("mine.test")
		assert.False(t, d.Status.PendingDelete, "the delete with <launch:delete> did nothing")
	})

	t.Run("the extension must be negotiated at login", func(t *testing.T) {
		c2 := dial(t, ts.addr)
		requireCode(t, c2.roundTrip(loginWithServices([]string{nsDomain}, nil)), "1000")
		resp := c2.roundTrip(createXML("neg.test", createOpts{ext: launchCreateXML("landrush", "", "", "")}))
		requireCode(t, resp, "2103")
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
