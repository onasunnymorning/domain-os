package rdevalidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below start from a deposit that independently validates against
// the pinned schemas (TestSchema_BaselineIsXSDValid proves it with xmllint
// itself) and break it in one way at a time.

func baselineXML() string {
	return string(rdetest.BuildXML(rdetest.DepositOpts{TLD: "example", Domains: 2, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1}))
}

func replaceOnce(t *testing.T, doc, old, new string) string {
	t.Helper()
	require.Contains(t, doc, old, "the fixture no longer contains %q; update the mutation", old)
	return strings.Replace(doc, old, new, 1)
}

func lineOf(t *testing.T, doc, marker string) int {
	t.Helper()
	i := strings.Index(doc, marker)
	require.GreaterOrEqual(t, i, 0, "marker %q not in document", marker)
	return strings.Count(doc[:i], "\n") + 1
}

var menuBlock = regexp.MustCompile(`(?s)  <rde:rdeMenu>.*?</rde:rdeMenu>\n`)

type schemaCase struct {
	name   string
	mutate func(t *testing.T, doc string) string
	// marker is the text on the line libxml2 must point at.
	marker string
	// rule is a fragment of the finding's Rule.
	rule string
	// message fragments the finding's Message must carry.
	message []string
	// object is what the finding's Object must be.
	object string
}

var schemaCases = []schemaCase{
	{
		name:   "required rdeMenu is missing",
		mutate: func(t *testing.T, d string) string { return menuBlock.ReplaceAllString(d, "") },
		marker: "<rde:contents>", rule: "element is not allowed at this position",
		message: []string{"rde:contents", "the schema allows: rde:rdeMenu"}, object: "rde:contents",
	},
	{
		name: "unexpected rde:unexpected before the watermark",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "  <rde:watermark>", "  <rde:unexpected>x</rde:unexpected>\n  <rde:watermark>")
		},
		marker: "<rde:unexpected>", rule: "element is not allowed at this position",
		message: []string{"the schema allows: rde:watermark"}, object: "rde:unexpected",
	},
	{
		name: "elements out of order: menu before watermark",
		mutate: func(t *testing.T, d string) string {
			menu := menuBlock.FindString(d)
			require.NotEmpty(t, menu)
			d = strings.Replace(d, menu, "", 1)
			return replaceOnce(t, d, "  <rde:watermark>", menu+"  <rde:watermark>")
		},
		marker: "<rde:rdeMenu>", rule: "element is not allowed at this position",
		message: []string{"rde:rdeMenu", "the schema allows: rde:watermark"}, object: "rde:rdeMenu",
	},
	{
		name: "required child of the menu is missing",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "    <rde:version>1.0</rde:version>\n", "")
		},
		marker: "<rde:objURI>", rule: "element is not allowed at this position",
		message: []string{"the schema allows: rde:version"}, object: "rde:objURI",
	},
	{
		name: "required attribute is missing",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, ` id="20260908001"`, "")
		},
		marker: "<rde:deposit", rule: "a required attribute is missing",
		message: []string{"rde:deposit", "required attribute id"}, object: "rde:deposit@id",
	},
	{
		name: "required attribute of a header count is missing",
		mutate: func(t *testing.T, d string) string {
			return regexp.MustCompile(`<rdeHeader:count uri="[^"]*">`).ReplaceAllString(d, "<rdeHeader:count>")
		},
		marker: "<rdeHeader:count>", rule: "a required attribute is missing",
		message: []string{"rdeHeader:count", "required attribute uri"}, object: "rdeHeader:count@uri",
	},
	{
		name: "value outside the enumeration",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "<rde:version>1.0</rde:version>", "<rde:version>9.9</rde:version>")
		},
		marker: "<rde:version>", rule: "value is not valid for its type",
		message: []string{"rde:version", "facet enumeration"}, object: "rde:version",
	},
	{
		name: "invalid schema type: a date the Go checks only warn about",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "<rdeDomain:crDate>2025-01-01T00:00:00Z</rdeDomain:crDate>", "<rdeDomain:crDate>last tuesday</rdeDomain:crDate>")
		},
		marker: "<rdeDomain:crDate>last", rule: "value is not valid for its type",
		message: []string{"rdeDomain:crDate", "xs:dateTime"}, object: "rdeDomain:crDate",
	},
	{
		name: "deposit type outside the enumeration",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, `type="FULL"`, `type="WEEKLY"`)
		},
		marker: "<rde:deposit", rule: "value is not valid for its type",
		message: []string{"attribute type of element rde:deposit"}, object: "rde:deposit@type",
	},
	{
		name: "unexpected element inside a domain",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "      <rdeDomain:clID>", "      <rdeDomain:bogus/>\n      <rdeDomain:clID>")
		},
		marker: "<rdeDomain:bogus/>", rule: "element is not allowed at this position",
		message: []string{"not allowed at this position"}, object: "rdeDomain:bogus",
	},
	{
		name: "domain children out of order",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d,
				"      <rdeDomain:clID>registrar1</rdeDomain:clID>\n      <rdeDomain:crRr>registrar1</rdeDomain:crRr>\n",
				"      <rdeDomain:crRr>registrar1</rdeDomain:crRr>\n      <rdeDomain:clID>registrar1</rdeDomain:clID>\n")
		},
		marker: "<rdeDomain:crRr>registrar1</rdeDomain:crRr>\n      <rdeDomain:clID>", rule: "element is not allowed at this position",
		message: []string{"rdeDomain:crRr"}, object: "rdeDomain:crRr",
	},
	{
		name: "element from an extension namespace the profile does not support",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "      <rdeDomain:clID>", "      <vnd:internalScore>0.93</vnd:internalScore>\n      <rdeDomain:clID>")
		},
		marker: "<vnd:internalScore>", rule: "element is not allowed at this position",
		object: "{?}internalScore",
	},
	{
		name: "unknown attribute on an element",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "<rdeDomain:domain>", `<rdeDomain:domain bogus="x">`)
		},
		marker: "<rdeDomain:domain bogus", rule: "attribute is not declared for this element",
		message: []string{"rdeDomain:domain"}, object: "rdeDomain:domain@bogus",
	},
	{
		name: "character content in an element-only element",
		mutate: func(t *testing.T, d string) string {
			return replaceOnce(t, d, "<rde:contents>", "<rde:contents>stray text")
		},
		// libxml2 reports character content where the text ends, which is the
		// line of the next tag.
		marker: "<rdeHeader:header>", rule: "character content is not allowed here",
		message: []string{"rde:contents", "only elements are allowed"}, object: "rde:contents",
	},
	{
		name: "root element no schema declares",
		mutate: func(t *testing.T, d string) string {
			d = replaceOnce(t, d, "<rde:deposit ", "<rde:bundle ")
			return replaceOnce(t, d, "</rde:deposit>", "</rde:bundle>")
		},
		marker: "<rde:bundle", rule: "not declared by any pinned schema",
		object: "rde:bundle",
	},
}

func TestSchema_BaselineIsXSDValid(t *testing.T) {
	eng := rdetest.SchemaEngine(t)
	s, err := eng.Start(context.Background())
	require.NoError(t, err)
	_, _ = s.Write([]byte(baselineXML()))
	vd := s.Finish()
	require.Equal(t, rdeschema.ConclusionValid, vd.Conclusion, "violations: %+v", vd.Violations)
}

// The reason this work exists: the Go checks alone give each of these a clean
// bill of health. The test pins that, so the claim "the schema check is what
// rejects them" stays checkable, and it fails the day a Go check learns to
// catch one (at which point the case belongs in a different test).
func TestSchema_GoChecksAloneMissTheOriginalDefects(t *testing.T) {
	for _, name := range []string{"required rdeMenu is missing", "unexpected rde:unexpected before the watermark"} {
		for _, c := range schemaCases {
			if c.name != name {
				continue
			}
			doc := c.mutate(t, baselineXML())
			v := &XMLValidator{BoundTLD: "example"}
			_, fs := v.Validate(context.Background(), strings.NewReader(doc))
			assert.Empty(t, fs, "%s: the Go checks were expected to see nothing", name)
			assert.Equal(t, OutcomePass, Decide(fs))
		}
	}
}

func TestSchema_NegativeFixtures_Unsigned(t *testing.T) {
	for _, c := range schemaCases {
		t.Run(c.name, func(t *testing.T) {
			doc := c.mutate(t, baselineXML())
			res := Run(context.Background(), plaintextInput(t, []byte(doc)))

			require.Equal(t, OutcomeFail, res.Outcome, "findings: %+v", res.Findings)
			assert.False(t, res.Verified())
			f := findSchemaFinding(t, res, c.rule)
			assert.Equal(t, CodeXMLSchemaInvalid, f.Code)
			assert.Equal(t, SeverityError, f.Severity)
			assert.Equal(t, StageXML, f.Stage)
			assert.Equal(t, c.object, f.Object)
			assert.Equal(t, "line="+itoa(lineOf(t, doc, c.marker)), f.Locator, "the finding must point at the offending line")
			for _, frag := range c.message {
				assert.Contains(t, f.Message, frag)
			}
		})
	}
}

// The signature, the key and the encryption are all correct, so the rejection
// can only be the XML's — and a rejection of the signature cannot be what hides
// the schema check from the test.
func TestSchema_NegativeFixtures_SignedAndEncrypted(t *testing.T) {
	f := newPipelineFixture(t)
	opts := rdetest.DepositOpts{TLD: "example", Domains: 2, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1}
	for _, c := range schemaCases {
		t.Run(c.name, func(t *testing.T) {
			doc := c.mutate(t, baselineXML())
			pair := rdetest.BuildPairFromXML(t, opts, []byte(doc), f.service, f.registry)
			res := Run(context.Background(), f.input(pair.Ryde, pair.Sig))

			require.Equal(t, OutcomeFail, res.Outcome, "findings: %+v", res.Findings)
			assert.False(t, res.Verified(), "a schema-invalid deposit is not a verified pass")
			assert.Equal(t, f.registry.Fingerprint, res.Signature.KeyFingerprint, "the signature verified; only the XML is at fault")
			assert.Equal(t, f.service.Fingerprint, res.Decryption.KeyFingerprint)
			assert.False(t, res.Has(CodeSigInvalid) || res.Has(CodeDecryptFailed))
			ff := findSchemaFinding(t, res, c.rule)
			assert.Equal(t, "line="+itoa(lineOf(t, doc, c.marker)), ff.Locator)
		})
	}
}

func TestSchema_GzippedPlaintextIsChecked(t *testing.T) {
	opts := rdetest.DepositOpts{TLD: "example", Layout: rdetest.LayoutGzip}
	doc := menuBlock.ReplaceAllString(baselineXML(), "")
	art := rdetest.BuildPayload(t, opts, []byte(doc))
	res := Run(context.Background(), plaintextInput(t, art))
	require.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLSchemaInvalid))
}

func findSchemaFinding(t *testing.T, res Result, ruleFragment string) Finding {
	t.Helper()
	for _, f := range res.Findings {
		if f.Code == CodeXMLSchemaInvalid && strings.Contains(f.Rule, ruleFragment) {
			return f
		}
	}
	t.Fatalf("no %s finding with rule containing %q in %+v", CodeXMLSchemaInvalid, ruleFragment, res.Findings)
	return Finding{}
}

// Legitimate optional content and the supported extensions stay accepted.
func TestSchema_AcceptsOptionalContentAndSupportedExtensions(t *testing.T) {
	cases := map[string]rdetest.DepositOpts{
		"secDNS dsData and keyData, IDN domain, disclosure, EPP params": {
			TLD: "example", Domains: 2, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1,
			SecDNS: true, Disclose: true, IDNDomain: true, EppParams: true,
		},
		"only registrars": {TLD: "example", Registrars: 2, Domains: -0 + 0, Contacts: 0, Hosts: 0},
		"DIFF deposit with prevId": {
			TLD: "example", Kind: "DIFF", PrevID: "20260907001", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1,
		},
		"resend attribute": {TLD: "example", Resend: "2"},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			res := Run(context.Background(), plaintextInput(t, rdetest.BuildXML(o)))
			require.Equal(t, OutcomePass, res.Outcome, "findings: %+v", res.Findings)
			assert.Empty(t, res.Findings)
		})
	}
}

// Nothing the deposit says about itself may reach Message, Rule or Locator, and
// nothing at all reaches the structured log's inputs (Code/Stage only).
func TestSchema_FindingsDoNotExposeDepositContent(t *testing.T) {
	const canary = "CANARY-7f3a91"
	doc := baselineXML()
	doc = replaceOnce(t, doc, "<rde:version>1.0</rde:version>", "<rde:version>"+canary+"</rde:version>")
	doc = replaceOnce(t, doc, "<rdeDomain:crDate>2025-01-01T00:00:00Z</rdeDomain:crDate>", "<rdeDomain:crDate>"+canary+"</rdeDomain:crDate>")
	doc = replaceOnce(t, doc, `<rdeDomain:domain>`, `<rdeDomain:domain `+"attr"+canary+`="`+canary+`">`)
	doc = replaceOnce(t, doc, "  <rde:watermark>", "  <rde:elem"+canary+">"+canary+"</rde:elem"+canary+">\n  <rde:watermark>")

	res := Run(context.Background(), plaintextInput(t, []byte(doc)))
	require.Equal(t, OutcomeFail, res.Outcome)
	var withObject int
	for _, f := range res.Findings {
		if f.Code != CodeXMLSchemaInvalid {
			continue
		}
		assert.NotContains(t, f.Message, canary)
		assert.NotContains(t, f.Rule, canary)
		assert.NotContains(t, f.Locator, canary)
		assert.NotContains(t, f.ObjectType, canary)
		if strings.Contains(f.Object, canary) {
			withObject++
		}
		assert.LessOrEqual(t, len(f.Object), 2*maxObjectLen)
	}
	// The element and attribute names the deposit chose are carried, cleaned,
	// in the one field reserved for deposit-sourced text.
	assert.Positive(t, withObject)
	for _, tl := range res.Tally {
		assert.NotContains(t, tl.Rule, canary)
	}
}

const maxObjectLen = 128

func TestSchema_NoPlaintextKeyOrValueInResultJSON(t *testing.T) {
	f := newPipelineFixture(t)
	const canary = "CANARY-5d0c11"
	opts := rdetest.DepositOpts{TLD: "example", Canary: canary}
	doc := menuBlock.ReplaceAllString(string(rdetest.BuildXML(opts)), "")
	pair := rdetest.BuildPairFromXML(t, opts, []byte(doc), f.service, f.registry)
	res := Run(context.Background(), f.input(pair.Ryde, pair.Sig))
	require.Equal(t, OutcomeFail, res.Outcome)
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), canary, "no value from the deposit may be in the run record")
	assert.NotContains(t, string(raw), "PRIVATE KEY")
}

// ---------------------------------------------------------------------------
// The check must not be skippable, and a broken engine is never a PASS.
// ---------------------------------------------------------------------------

type fakeEngine struct {
	startErr error
	verdict  rdeschema.Verdict
	sess     *fakeSession
}

type fakeSession struct {
	buf     bytes.Buffer
	verdict rdeschema.Verdict
	aborted bool
	done    bool
}

func (s *fakeSession) Write(p []byte) (int, error) { return s.buf.Write(p) }
func (s *fakeSession) Finish() rdeschema.Verdict   { s.done = true; return s.verdict }
func (s *fakeSession) Abort() rdeschema.Verdict {
	s.aborted = true
	return rdeschema.Verdict{Conclusion: rdeschema.ConclusionUnavailable}
}

func (e *fakeEngine) Start(context.Context) (rdeschema.Session, error) {
	if e.startErr != nil {
		return nil, e.startErr
	}
	e.sess = &fakeSession{verdict: e.verdict}
	return e.sess, nil
}

func TestSchema_UnavailableEngineIsErrorNeverPass(t *testing.T) {
	good := rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"})
	cases := map[string]func(*Input){
		"no engine configured": func(in *Input) { in.Schema = nil },
		"engine fails to start": func(in *Input) {
			in.Schema = &fakeEngine{startErr: errors.Join(rdeschema.ErrUnavailable, errors.New("xmllint not found"))}
		},
		"engine could not decide": func(in *Input) {
			in.Schema = &fakeEngine{verdict: rdeschema.Verdict{Conclusion: rdeschema.ConclusionUnavailable, Detail: "schema engine exited with status 9"}}
		},
		"engine returned a verdict it does not define": func(in *Input) {
			in.Schema = &fakeEngine{verdict: rdeschema.Verdict{Conclusion: "banana"}}
		},
		"engine returned an empty verdict": func(in *Input) {
			in.Schema = &fakeEngine{verdict: rdeschema.Verdict{}}
		},
		"engine not installed": func(in *Input) {
			in.Schema = rdeschema.NewXMLLint(rdeschema.Config{Binary: "/nonexistent/xmllint"})
		},
		"binary that accepts everything fails its self-test": func(in *Input) {
			in.Schema = rdeschema.NewXMLLint(rdeschema.Config{Binary: "true"})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := plaintextInput(t, good)
			mutate(&in)
			res := Run(context.Background(), in)
			assert.Equal(t, OutcomeError, res.Outcome, "findings: %+v", res.Findings)
			assert.True(t, res.Has(CodeSchemaEngineUnavailable))
			assert.NotEqual(t, OutcomePass, res.Outcome)
			assert.Equal(t, "", string(NotificationStatus(res.Outcome)), "an undecided run claims nothing")
		})
	}
}

func TestSchema_ErrorOutranksFailWhenTheCheckCouldNotRun(t *testing.T) {
	in := plaintextInput(t, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example", HeaderTLD: "other"}))
	in.Schema = nil
	res := Run(context.Background(), in)
	assert.True(t, res.Has(CodeRDEHeaderTLDMismatch))
	assert.Equal(t, OutcomeError, res.Outcome)
}

// A document that ends mid-element is not well-formed. The Go checks and
// libxml2 both see that; the run reports it once, as XML_MALFORMED, and does
// not dress it up as a schema violation.
func TestSchema_TruncatedDocumentIsReportedOnceAsMalformed(t *testing.T) {
	res := Run(context.Background(), plaintextInput(t, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example", Malformed: true})))
	assert.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLMalformed))
	assert.False(t, res.Has(CodeXMLSchemaInvalid), "findings: %+v", res.Findings)
	n := 0
	for _, f := range res.Findings {
		if f.Code == CodeXMLMalformed {
			n++
		}
	}
	assert.Equal(t, 1, n)
}

// When the Go checks give up part-way, the schema session has seen a prefix and
// is abandoned rather than asked to judge it.
func TestSchema_AbandonedWhenTheGoChecksStopEarly(t *testing.T) {
	doc := replaceOnce(t, baselineXML(), "</rdeHost:name>", "</rdeHost:nope>")
	eng := &fakeEngine{verdict: rdeschema.Verdict{Conclusion: rdeschema.ConclusionValid}}
	in := plaintextInput(t, []byte(doc))
	in.Schema = eng
	res := Run(context.Background(), in)
	assert.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLMalformed))
	require.NotNil(t, eng.sess)
	assert.True(t, eng.sess.aborted)
	assert.False(t, eng.sess.done)
}

func TestSchema_SessionSeesExactlyWhatTheGoChecksRead(t *testing.T) {
	xml := rdetest.BuildXML(rdetest.DepositOpts{TLD: "example", Domains: 3, Contacts: 2, Hosts: 1, Registrars: 1})
	eng := &fakeEngine{verdict: rdeschema.Verdict{Conclusion: rdeschema.ConclusionValid}}
	in := plaintextInput(t, xml)
	in.Schema = eng
	res := Run(context.Background(), in)
	require.Equal(t, OutcomePass, res.Outcome, "findings: %+v", res.Findings)
	require.NotNil(t, eng.sess)
	assert.True(t, eng.sess.done)
	assert.Equal(t, string(xml), eng.sess.buf.String(), "the schema session must receive the whole document, byte for byte")
}

func TestSchema_VerdictsMapToFindings(t *testing.T) {
	run := func(vd rdeschema.Verdict) Result {
		in := plaintextInput(t, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"}))
		in.Schema = &fakeEngine{verdict: vd}
		return Run(context.Background(), in)
	}
	t.Run("not well-formed per libxml2", func(t *testing.T) {
		res := run(rdeschema.Verdict{Conclusion: rdeschema.ConclusionNotWellFormed, ParseLine: 12})
		assert.Equal(t, OutcomeFail, res.Outcome)
		require.True(t, res.Has(CodeXMLMalformed))
		assert.Equal(t, "line=12", res.Findings[0].Locator)
	})
	t.Run("document type declaration", func(t *testing.T) {
		res := run(rdeschema.Verdict{Conclusion: rdeschema.ConclusionRejected, Reason: rdeschema.RejectDocType})
		assert.Equal(t, OutcomeFail, res.Outcome)
		assert.True(t, res.Has(CodeXMLDTDNotSupported))
	})
	t.Run("prolog", func(t *testing.T) {
		res := run(rdeschema.Verdict{Conclusion: rdeschema.ConclusionRejected, Reason: rdeschema.RejectProlog})
		assert.Equal(t, OutcomeFail, res.Outcome)
		assert.True(t, res.Has(CodeXMLPrologNotSupported))
	})
	t.Run("truncated list is said so", func(t *testing.T) {
		res := run(rdeschema.Verdict{Conclusion: rdeschema.ConclusionInvalid, Total: 5000,
			Violations: []rdeschema.Violation{{Class: rdeschema.ClassOther, Line: 1}}})
		assert.Equal(t, OutcomeFail, res.Outcome)
		assert.True(t, res.Has(CodeXMLSchemaCheckTruncated))
	})
	t.Run("a verdict of invalid with no violations still fails", func(t *testing.T) {
		res := run(rdeschema.Verdict{Conclusion: rdeschema.ConclusionInvalid})
		assert.Equal(t, OutcomeFail, res.Outcome, "findings: %+v", res.Findings)
	})
}

// ---------------------------------------------------------------------------
// Entities, DTDs and bounds, against the real engine.
// ---------------------------------------------------------------------------

func TestSchema_DoctypeIsRefusedAndNothingIsResolved(t *testing.T) {
	const canary = "CANARY-FILE-CONTENT-8841"
	dir := t.TempDir()
	secret := dir + "/secret.txt"
	require.NoError(t, os.WriteFile(secret, []byte(canary), 0o600))

	doc := baselineXML()
	doc = replaceOnce(t, doc, "<rde:deposit ", `<!DOCTYPE rde:deposit [ <!ENTITY xxe SYSTEM "file://`+secret+`"> ]>`+"\n<rde:deposit ")
	doc = replaceOnce(t, doc, "<rde:version>1.0</rde:version>", "<rde:version>&xxe;</rde:version>")

	res := Run(context.Background(), plaintextInput(t, []byte(doc)))
	require.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLDTDNotSupported))
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), canary, "the entity must never have been resolved")
}

func TestSchema_NonUTF8DeclarationIsRefused(t *testing.T) {
	doc := strings.Replace(baselineXML(), `encoding="UTF-8"`, `encoding="UTF-16"`, 1)
	res := Run(context.Background(), plaintextInput(t, []byte(doc)))
	assert.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLPrologNotSupported) || res.Has(CodeXMLMalformed))
}

func TestSchema_SchemaLocationHintsAreIgnored(t *testing.T) {
	doc := replaceOnce(t, baselineXML(), "<rde:deposit ",
		`<rde:deposit xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:x file:///dev/zero" `)
	// xsi:schemaLocation is a schema-declared attribute only if the schema
	// declares it; libxml2 treats the xsi namespace as built in. What matters
	// here is that the hint is not followed and the run is decided.
	res := Run(context.Background(), plaintextInput(t, []byte(doc)))
	assert.Contains(t, []Outcome{OutcomePass, OutcomeFail}, res.Outcome)
	assert.NotEqual(t, OutcomeError, res.Outcome)
}

func TestSchema_ViolationsAreBoundedAndTheDepositStillFails(t *testing.T) {
	eng := rdeschema.NewXMLLint(rdeschema.Config{MaxRetained: 20, StopAfter: 50})
	require.NoError(t, eng.Err())
	t.Cleanup(func() { _ = eng.Close() })

	opts := rdetest.DepositOpts{TLD: "example", Domains: 400, Contacts: 0, Hosts: 0, Registrars: 1}
	doc := regexp.MustCompile(`<rdeDomain:crDate>[^<]*</rdeDomain:crDate>`).ReplaceAllString(string(rdetest.BuildXML(opts)), "<rdeDomain:crDate>nope</rdeDomain:crDate>")
	in := plaintextInput(t, []byte(doc))
	in.Schema = eng
	res := Run(context.Background(), in)

	require.Equal(t, OutcomeFail, res.Outcome)
	assert.True(t, res.Has(CodeXMLSchemaCheckTruncated), "the cut-off must be stated")
	n := 0
	for _, f := range res.Findings {
		if f.Code == CodeXMLSchemaInvalid {
			n++
		}
	}
	assert.LessOrEqual(t, n, 20)
	assert.Equal(t, 400, res.Deposit.Observed["urn:ietf:params:xml:ns:rdeDomain-1.0"], "the Go checks still read the whole deposit")
}

func TestSchema_CancellationStopsTheCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Run(ctx, plaintextInput(t, rdetest.BuildXML(rdetest.DepositOpts{TLD: "example"})))
	assert.NotEqual(t, OutcomePass, res.Outcome)
}

var _ io.Reader
