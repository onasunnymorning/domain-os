package epp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanFrameClassifies(t *testing.T) {
	cases := []struct {
		name   string
		xml    string
		label  string
		clTRID string
	}{
		{"hello", `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`, "hello", ""},
		{"login", loginXML("ABC-1"), "login", "ABC-1"},
		{"logout", cmdXML("<logout/>", "  OUT-1  "), "logout", "OUT-1"},
		{"domain check", domainCheckXML("CHK-1"), "domain:check", "CHK-1"},
		{"host info", cmdXML(`<info><host:info xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns1.example.test</host:name></host:info></info>`, ""), "host:info", ""},
		{"contact create", cmdXML(`<create><contact:create xmlns:contact="urn:ietf:params:xml:ns:contact-1.0"/></create>`, ""), "contact:create", ""},
		{"unknown object", cmdXML(`<check><x:check xmlns:x="urn:example:x"/></check>`, ""), "other:check", ""},
		{"poll has no object", cmdXML(`<poll op="req"/>`, "POLL-1"), "poll", "POLL-1"},
		{"extension before clTRID is ignored", cmdXML(`<logout/><extension><x xmlns="urn:example:x"/></extension>`, "EXT-1"), "logout", "EXT-1"},
		{"greeting from a client is not a command", `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><greeting/></epp>`, "unknown", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := scanFrame([]byte(tc.xml), 32, 2000)
			require.NoError(t, err)
			assert.Equal(t, tc.label, f.label())
			assert.Equal(t, tc.clTRID, f.clTRID)
		})
	}
}

func TestScanFrameRejects(t *testing.T) {
	cases := []struct {
		name, xml, reason string
	}{
		{"empty", ``, rejectSyntax},
		{"unclosed", `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command>`, rejectSyntax},
		{"wrong namespace", `<epp xmlns="urn:example:not-epp"><hello/></epp>`, rejectSyntax},
		{"undeclared entity", `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">&boom;</epp>`, rejectSyntax},
		{"non-UTF-8 encoding", `<?xml version="1.0" encoding="ISO-8859-1"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`, rejectSyntax},
		{"DOCTYPE", `<!DOCTYPE epp><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`, rejectDoctype},
		{"clTRID too short", cmdXML("<logout/>", "ab"), rejectClTRID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := scanFrame([]byte(tc.xml), 32, 2000)
			var fe *frameError
			require.True(t, errors.As(err, &fe), "got %v", err)
			assert.Equal(t, tc.reason, fe.reason)
		})
	}
}

func TestScanFrameCaps(t *testing.T) {
	nested := `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><logout/></command></epp>` // 3 levels, 3 elements

	_, err := scanFrame([]byte(nested), 3, 3)
	require.NoError(t, err, "exactly at both caps is allowed")

	_, err = scanFrame([]byte(nested), 2, 100)
	var fe *frameError
	require.True(t, errors.As(err, &fe))
	assert.Equal(t, rejectDepth, fe.reason)

	_, err = scanFrame([]byte(nested), 100, 2)
	require.True(t, errors.As(err, &fe))
	assert.Equal(t, rejectElements, fe.reason)
}

func TestResponseDocument(t *testing.T) {
	doc := responseDocument(2002, "", "CL-1", "SV-1", nil)
	result := doc.FindElement("/epp/response/result")
	require.NotNil(t, result)
	assert.Equal(t, "2002", result.SelectAttrValue("code", ""))
	assert.Equal(t, "Command use error", result.FindElement("msg").Text(), "the standard text when no message is given")
	assert.Equal(t, "CL-1", text(doc, "/epp/response/trID/clTRID"))
	assert.Equal(t, "SV-1", text(doc, "/epp/response/trID/svTRID"))
	assert.Equal(t, nsEPP, doc.Root().SelectAttrValue("xmlns", ""))

	escaped := responseDocument(1000, `a <b> & "c"`, "", "SV-2", nil)
	assert.Equal(t, `a <b> & "c"`, text(escaped, "//msg"), "text is escaped, not spliced into the XML")
}
