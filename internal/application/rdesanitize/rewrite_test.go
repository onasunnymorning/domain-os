package rdesanitize

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey is fixture material, not a credential: it never leaves the test
// binary and no derivative built with it is published.
var testKey = bytes.Repeat([]byte("eve-sanitize-test-key-material!!"), 2)

const (
	sourceTLD = "example"
	synthetic = "artful-dodger"
)

type sanitizeFixture struct {
	rw     *Rewriter
	tokens *Tokenizer
	suffix *SuffixRewriter
}

func newFixture(t *testing.T) sanitizeFixture {
	t.Helper()
	tok, err := NewTokenizer(testKey, "ryop1", PolicyVersion)
	require.NoError(t, err)
	sfx, err := NewSuffixRewriter(sourceTLD, synthetic)
	require.NoError(t, err)
	return sanitizeFixture{
		rw: &Rewriter{
			Profile: BaselineProfile(), Tokens: tok, Suffix: sfx,
			Limits: DefaultLimits(), Now: func() time.Time { return time.Unix(1757300000, 0).UTC() },
		},
		tokens: tok,
		suffix: sfx,
	}
}

func (f sanitizeFixture) run(t *testing.T, o rdetest.DepositOpts) (Result, string) {
	t.Helper()
	if o.TLD == "" {
		o.TLD = sourceTLD
	}
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), bytes.NewReader(rdetest.BuildXML(o)), &out)
	return res, out.String()
}

func (f sanitizeFixture) scan(t *testing.T, doc string) []Finding {
	t.Helper()
	return Scan(context.Background(), strings.NewReader(doc), ScanOptions{
		Profile: BaselineProfile(), Suffix: f.suffix, Limits: DefaultLimits(),
		Now: func() time.Time { return time.Unix(1757300000, 0).UTC() },
	})
}

func TestRewrite_FullDepositPasses(t *testing.T) {
	f := newFixture(t)
	res, doc := f.run(t, rdetest.DepositOpts{
		Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1,
		AuthInfo: true, SecDNS: true, Disclose: true, PrivacyProxy: true, IDNDomain: true, EppParams: true,
	})
	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)
	assert.Empty(t, res.Findings)
	assert.Empty(t, f.scan(t, doc), "the derivative must satisfy its own regression scan")

	// The derivative is still a well-formed, namespaced RDE deposit.
	assert.True(t, strings.HasPrefix(doc, `<?xml version="1.0" encoding="UTF-8"?>`))
	assert.Contains(t, doc, "<rde:deposit ")
	assert.Contains(t, doc, `<rdeDomain:status s="ok"/>`, "self-closing elements keep their form")

	// Credentials are gone entirely — element and value.
	assert.NotContains(t, doc, "authInfo")
	assert.NotContains(t, doc, "2fooBAR")

	// Direct identifiers are replaced or removed.
	assert.NotContains(t, doc, "contact1@example.com")
	assert.NotContains(t, doc, "Privacy Proxy Customer 4711", "an underlying proxy customer is as identifying as a registrant")
	assert.NotContains(t, doc, "Proxy Services Ltd")
	assert.NotContains(t, doc, "<contact:street>", "a registrant's street address is removed")
	assert.NotContains(t, doc, "<contact:org>", "a registrant's organisation is removed")
	assert.NotContains(t, doc, "<rdeContact:voice>", "an optional phone number is removed rather than replaced")
	assert.Contains(t, doc, "@example.invalid")
	assert.Contains(t, doc, "<contact:name>Contact ")

	// Registrar records are business data the profile deliberately retains.
	assert.Contains(t, doc, "<rdeRegistrar:name>Registrar 1</rdeRegistrar:name>")
	assert.Contains(t, doc, "<rdeRegistrar:status>ok</rdeRegistrar:status>")
	assert.Contains(t, doc, "<rdeRegistrar:street>1 Test Street</rdeRegistrar:street>")
	assert.Contains(t, doc, "<rdeRegistrar:voice>+1.5555550100</rdeRegistrar:voice>")

	// Analytical metadata is retained: it is what the derivative is for, and
	// it is why the output is pseudonymized rather than anonymous.
	assert.Contains(t, doc, "<contact:city>Testville</contact:city>")
	assert.Contains(t, doc, "<contact:cc>US</contact:cc>")
	assert.Contains(t, doc, "<contact:pc>12345</contact:pc>")

	// Public DNS material survives untouched.
	assert.Contains(t, doc, "<secDNS:pubKey>AQPJ////4Q==</secDNS:pubKey>")
	assert.Contains(t, doc, "<secDNS:digest>49FD46E6C4B45C55D4AC</secDNS:digest>")
	assert.Contains(t, doc, `<rdeHost:addr ip="v4">192.0.2.1</rdeHost:addr>`)

	// Disclosure preferences share their element names with the postal fields
	// and must not be mistaken for them.
	assert.Contains(t, doc, `<rdeContact:disclose flag="0">`)
	assert.Contains(t, doc, `<contact:org type="int"/>`)

	// Protocol policy is copied as a block.
	assert.Contains(t, doc, "<rdeEppParams:dcp>")
	assert.Contains(t, doc, "<epp:stated/>")

	assert.Positive(t, res.Counts.Kept)
	assert.Positive(t, res.Counts.Dropped)
	assert.Positive(t, res.Counts.Tokenized)
	assert.Positive(t, res.Counts.Synthesized)
	assert.Positive(t, res.Counts.Rewritten)
	assert.Equal(t, int64(4), res.Counts.ObjectsByType[entities.DOMAIN_URI], "3 domains plus the IDN one")
	assert.Equal(t, int64(2), res.Counts.ObjectsByType[entities.CONTACT_URI])
}

func TestRewrite_SuffixIsRewrittenEverywhere(t *testing.T) {
	f := newFixture(t)
	_, doc := f.run(t, rdetest.DepositOpts{Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1, NNDNs: 1, IDNDomain: true})

	assert.Empty(t, f.scan(t, doc))
	assert.Contains(t, doc, "<rdeHeader:tld>artful-dodger</rdeHeader:tld>")
	assert.Contains(t, doc, "<rdeDomain:name>example-1.artful-dodger</rdeDomain:name>")
	assert.Contains(t, doc, "<rdeHost:name>ns1.example-1.artful-dodger</rdeHost:name>")
	assert.Contains(t, doc, "<domain:hostObj>ns1.example-1.artful-dodger</domain:hostObj>")
	assert.Contains(t, doc, "<rdeNNDN:aName>nndn-1.artful-dodger</rdeNNDN:aName>")

	// The A-label and U-label of one IDN stay in step.
	assert.Contains(t, doc, "<rdeDomain:name>xn--nxasmm1c.artful-dodger</rdeDomain:name>")
	assert.Contains(t, doc, "<rdeDomain:uName>βόλος.artful-dodger</rdeDomain:uName>")

	// An out-of-bailiwick nameserver is left alone: rewriting it would invent a
	// delegation the registry never made.
	assert.Contains(t, doc, "<domain:hostObj>ns1.outside.example.net</domain:hostObj>")
}

func TestRewrite_RelationshipsSurvive(t *testing.T) {
	f := newFixture(t)
	_, doc := f.run(t, rdetest.DepositOpts{Domains: 3, Contacts: 1, Hosts: 1, Registrars: 1, SharedContact: true})

	// One contact referenced by three domains must still be one contact.
	contactID := f.tokens.Value(ValContactID, "CONT1")
	assert.Equal(t, 3, strings.Count(doc, "<rdeDomain:registrant>"+contactID+"</rdeDomain:registrant>"))
	assert.Contains(t, doc, "<rdeContact:id>"+contactID+"</rdeContact:id>")
	assert.NotContains(t, doc, "CONT1")

	// A tokenised contact id still fits eppcom:clIDType.
	assert.Len(t, contactID, 16)
}

func TestRewrite_FailsClosed(t *testing.T) {
	cases := []struct {
		name string
		opts rdetest.DepositOpts
		want Code
	}{
		{"unclassified vendor extension", rdetest.DepositOpts{Domains: 1, VendorExtension: true}, CodePolicyUnknownNamespace},
		{"document type declaration", rdetest.DepositOpts{Domains: 1, DTD: true}, CodeXMLDTDPresent},
		{"non-UTF-8 encoding", rdetest.DepositOpts{Domains: 1, Encoding: "ISO-8859-1"}, CodeXMLUnsupportedEncoding},
		{"malformed source", rdetest.DepositOpts{Domains: 2, Malformed: true}, CodeXMLMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			res, _ := f.run(t, tc.opts)
			require.Equal(t, OutcomeQuarantined, res.Outcome, "findings: %v", res.Findings)
			assert.True(t, res.Has(tc.want), "codes: %v", res.Codes())
		})
	}
}

func TestRewrite_UnknownAttributeQuarantines(t *testing.T) {
	f := newFixture(t)
	doc := strings.Replace(string(rdetest.BuildXML(rdetest.DepositOpts{TLD: sourceTLD, Domains: 1})),
		`<rdeDomain:status s="ok"/>`, `<rdeDomain:status s="ok" vendorFlag="7"/>`, 1)
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), strings.NewReader(doc), &out)
	require.Equal(t, OutcomeQuarantined, res.Outcome)
	assert.True(t, res.Has(CodePolicyUnknownAttribute), "codes: %v", res.Codes())
}

// A refusal that does not say what it refused cannot be acted on: the whole
// remedy for a POLICY_UNKNOWN_* finding is to add that name to the profile.
func TestRewrite_NamesWhatTheProfileDoesNotClassify(t *testing.T) {
	t.Run("attribute", func(t *testing.T) {
		f := newFixture(t)
		doc := strings.Replace(string(rdetest.BuildXML(rdetest.DepositOpts{TLD: sourceTLD, Domains: 1})),
			`<rdeDomain:status s="ok"/>`, `<rdeDomain:status s="ok" vendorFlag="7"/>`, 1)
		var out bytes.Buffer
		res := f.rw.Rewrite(context.Background(), strings.NewReader(doc), &out)

		require.Equal(t, OutcomeQuarantined, res.Outcome)
		got := findingWithCode(t, res, CodePolicyUnknownAttribute)
		assert.Equal(t, "rdeDomain:status@vendorFlag", got.Object,
			"the object is the profile entry that is missing")
	})

	t.Run("namespace", func(t *testing.T) {
		f := newFixture(t)
		res, _ := f.run(t, rdetest.DepositOpts{Domains: 1, VendorExtension: true})

		require.Equal(t, OutcomeQuarantined, res.Outcome)
		got := findingWithCode(t, res, CodePolicyUnknownNamespace)
		assert.NotEmpty(t, got.Object)
		assert.Contains(t, got.Object, ":", "a namespace is reported as its URI, which Alias maps")
	})
}

// The refusal used to stop the walk at the first unclassified name, so
// extending the profile meant re-running the whole deposit once per gap. It
// now surveys to the end and reports them together.
func TestRewrite_SurveysTheWholeSourceOnRefusal(t *testing.T) {
	f := newFixture(t)
	doc := string(rdetest.BuildXML(rdetest.DepositOpts{TLD: sourceTLD, Domains: 1, Contacts: 1}))
	doc = strings.Replace(doc, `<rdeDomain:status s="ok"/>`, `<rdeDomain:status s="ok" vendorFlag="7"/>`, 1)
	doc = strings.Replace(doc, `<rdeContact:status s="ok"/>`, `<rdeContact:status s="ok" legacyId="3"/>`, 1)
	require.Contains(t, doc, "vendorFlag")
	require.Contains(t, doc, "legacyId")

	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), strings.NewReader(doc), &out)
	require.Equal(t, OutcomeQuarantined, res.Outcome)

	var objects []string
	for _, x := range res.Findings {
		if x.Code == CodePolicyUnknownAttribute {
			objects = append(objects, x.Object)
		}
	}
	assert.ElementsMatch(t, []string{"rdeDomain:status@vendorFlag", "rdeContact:status@legacyId"}, objects,
		"one run reports every gap, not just the first")

	// Surveying is not producing: the moment the profile refuses, the
	// derivative stops being written, so nothing downstream can mistake what
	// was staged for a complete document.
	assert.NotContains(t, out.String(), "</rde:deposit>")
}

// One unclassified name used a hundred times is one gap in the profile, not a
// hundred findings to read past.
func TestRewrite_ReportsEachGapOnce(t *testing.T) {
	f := newFixture(t)
	doc := strings.ReplaceAll(string(rdetest.BuildXML(rdetest.DepositOpts{TLD: sourceTLD, Domains: 20})),
		`<rdeDomain:status s="ok"/>`, `<rdeDomain:status s="ok" vendorFlag="7"/>`)
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), strings.NewReader(doc), &out)
	require.Equal(t, OutcomeQuarantined, res.Outcome)

	listed, counted := 0, 0
	for _, x := range res.Findings {
		if x.Code == CodePolicyUnknownAttribute {
			listed++
		}
	}
	for _, e := range res.Tally {
		if e.Code == CodePolicyUnknownAttribute {
			counted += e.Count
		}
	}
	assert.Equal(t, 1, listed, "the findings list carries one example of the gap")
	assert.Greater(t, counted, 1, "the tally still counts every occurrence")
}

func findingWithCode(t *testing.T, res Result, c Code) Finding {
	t.Helper()
	for _, f := range res.Findings {
		if f.Code == c {
			return f
		}
	}
	require.FailNowf(t, "no finding with code", "%s; codes: %v", string(c), res.Codes())
	return Finding{}
}

func TestRewrite_LimitsAreEnforced(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		f := newFixture(t)
		f.rw.Limits.MaxXMLDepth = 3
		res, _ := f.run(t, rdetest.DepositOpts{Domains: 1})
		require.Equal(t, OutcomeQuarantined, res.Outcome)
		assert.True(t, res.Has(CodeLimitXMLDepth), "codes: %v", res.Codes())
	})
	t.Run("element count", func(t *testing.T) {
		f := newFixture(t)
		f.rw.Limits.MaxElements = 5
		res, _ := f.run(t, rdetest.DepositOpts{Domains: 5})
		require.Equal(t, OutcomeQuarantined, res.Outcome)
		assert.True(t, res.Has(CodeLimitElementCount), "codes: %v", res.Codes())
	})
	t.Run("field length", func(t *testing.T) {
		f := newFixture(t)
		f.rw.Limits.MaxFieldBytes = 4
		res, _ := f.run(t, rdetest.DepositOpts{Domains: 1, Contacts: 1})
		require.Equal(t, OutcomeQuarantined, res.Outcome)
		assert.True(t, res.Has(CodeLimitFieldBytes), "codes: %v", res.Codes())
	})
	t.Run("misconfigured limits are an error, not a quarantine", func(t *testing.T) {
		f := newFixture(t)
		f.rw.Limits.MaxElements = 0
		res, _ := f.run(t, rdetest.DepositOpts{Domains: 1})
		require.Equal(t, OutcomeError, res.Outcome, "a service misconfiguration must not be blamed on the deposit")
	})
}

func TestRewrite_PreservesDocumentStructure(t *testing.T) {
	// The derivative must be the same document with different values: same
	// elements, in the same order, in the same namespaces. Comparing resolved
	// token streams checks that without depending on byte-level formatting.
	f := newFixture(t)
	opts := rdetest.DepositOpts{TLD: sourceTLD, Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1, NNDNs: 1, IDNs: 1, SecDNS: true, EppParams: true}
	src := rdetest.BuildXML(opts)
	res, doc := f.run(t, opts)
	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)

	want := elementPaths(t, bytes.NewReader(src))
	got := elementPaths(t, strings.NewReader(doc))

	// The only structural difference is the set of elements the profile
	// removes outright. Listing them here means a future profile change that
	// silently drops something else fails this test.
	removed := map[string]bool{
		"{urn:ietf:params:xml:ns:contact-1.0}org":      true,
		"{urn:ietf:params:xml:ns:contact-1.0}street":   true,
		"{urn:ietf:params:xml:ns:rdeContact-1.0}voice": true,
	}
	wantKept := make([]string, 0, len(want))
	for _, p := range want {
		if !removed[p] {
			wantKept = append(wantKept, p)
		}
	}
	assert.Equal(t, wantKept, got, "the derivative is the same document with different values")
}

// elementPaths returns every element as "{namespace}local", in document order,
// with namespace prefixes resolved.
func elementPaths(t *testing.T, r io.Reader) []string {
	t.Helper()
	dec := xml.NewDecoder(r)
	var out []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if se, ok := tok.(xml.StartElement); ok {
			out = append(out, "{"+se.Name.Space+"}"+se.Name.Local)
		}
	}
	return out
}
