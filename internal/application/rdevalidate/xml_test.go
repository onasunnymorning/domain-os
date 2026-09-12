package rdevalidate

import (
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// xmlFixtures are the committed synthetic deposits under testdata/. Regenerate
// with UPDATE_FIXTURES=1 go test ./internal/application/rdevalidate/ -run TestXMLFixtures.
var xmlFixtures = map[string]rdetest.DepositOpts{
	"deposit_valid.xml":           {TLD: "example", Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1},
	"deposit_count_mismatch.xml":  {TLD: "example", Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, HeaderCounts: map[string]int{entities.DOMAIN_URI: 5, entities.CONTACT_URI: 2, entities.HOST_URI: 2, entities.REGISTRAR_URI: 1}},
	"deposit_missing_objects.xml": {TLD: "example", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1, HeaderCounts: map[string]int{entities.DOMAIN_URI: 1, entities.CONTACT_URI: 1, entities.HOST_URI: 1, entities.REGISTRAR_URI: 1, entities.NNDN_URI: 4}},
	"deposit_bad_domain.xml":      {TLD: "example", Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1, BreakDomain: true},
	"deposit_wrong_tld.xml":       {TLD: "example", HeaderTLD: "other", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1},
	"deposit_malformed.xml":       {TLD: "example", Malformed: true},
	"deposit_no_header.xml":       {TLD: "example", OmitHeader: true},
	"deposit_no_deposit.xml":      {TLD: "example", OmitDeposit: true},
	"deposit_no_watermark.xml":    {TLD: "example", OmitWatermark: true},
	"deposit_diff_kind.xml":       {TLD: "example", Kind: "DIFF", PrevID: "20260907001", Resend: "2", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1},
	"deposit_bad_resend.xml":      {TLD: "example", Resend: "many", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1},
}

func TestXMLFixtures(t *testing.T) {
	if os.Getenv("UPDATE_FIXTURES") == "" {
		t.Skip("set UPDATE_FIXTURES=1 to regenerate testdata/*.xml")
	}
	for name, opts := range xmlFixtures {
		require.NoError(t, os.WriteFile(filepath.Join("testdata", name), rdetest.BuildXML(opts), 0o600))
	}
}

func validateFixture(t *testing.T, name string) (DepositSummary, []Finding) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "fixture missing; run with UPDATE_FIXTURES=1")
	v := &XMLValidator{BoundTLD: "example", Now: func() time.Time { return testNow }}
	return v.Validate(context.Background(), bytes.NewReader(raw))
}

func codesOf(fs []Finding) []Code {
	var r Result
	r.Findings = fs
	return r.Codes()
}

func TestXMLValidator(t *testing.T) {
	t.Run("valid deposit has no findings and a complete summary", func(t *testing.T) {
		s, fs := validateFixture(t, "deposit_valid.xml")
		assert.Empty(t, fs, "findings: %v", fs)
		assert.Equal(t, "20260908001", s.ID)
		assert.Equal(t, "FULL", s.Kind)
		assert.Equal(t, 0, s.Resend)
		assert.False(t, s.Watermark.IsZero())
		assert.True(t, s.HeaderFound)
		assert.Equal(t, "example", s.Header.TLD)
		assert.Equal(t, 3, s.Observed[entities.DOMAIN_URI])
		assert.Equal(t, 2, s.Observed[entities.CONTACT_URI])
		assert.Equal(t, 2, s.Observed[entities.HOST_URI])
		assert.Equal(t, 1, s.Observed[entities.REGISTRAR_URI])
		assert.Equal(t, 1, s.Observed[entities.IDN_URI])
		assert.Equal(t, 1, s.Observed[entities.NNDN_URI])
	})

	cases := []struct {
		fixture string
		want    []Code
	}{
		{"deposit_count_mismatch.xml", []Code{CodeRDECountMismatch}},
		{"deposit_missing_objects.xml", []Code{CodeRDERequiredObjectMissing}},
		{"deposit_bad_domain.xml", []Code{CodeRDEObjectInvalid}},
		{"deposit_wrong_tld.xml", []Code{CodeRDEHeaderTLDMismatch}},
		{"deposit_malformed.xml", []Code{CodeXMLMalformed}},
		{"deposit_no_header.xml", []Code{CodeXMLNoHeader}},
		{"deposit_no_deposit.xml", []Code{CodeXMLNoDeposit}},
		{"deposit_no_watermark.xml", []Code{CodeRDEObjectInvalid}},
		{"deposit_bad_resend.xml", []Code{CodeRDEObjectInvalid}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			_, fs := validateFixture(t, c.fixture)
			assert.Equal(t, c.want, codesOf(fs), "findings: %v", fs)
			for _, f := range fs {
				assert.Equal(t, SeverityError, f.Severity)
			}
		})
	}

	t.Run("DIFF deposit with prevId and resend is accepted and summarised", func(t *testing.T) {
		s, fs := validateFixture(t, "deposit_diff_kind.xml")
		assert.Empty(t, fs)
		assert.Equal(t, "DIFF", s.Kind)
		assert.Equal(t, "20260907001", s.PrevID)
		assert.Equal(t, 2, s.Resend, "resend is parsed exactly, not clamped to 1")
	})

	t.Run("bad domain findings locate by index and offset only", func(t *testing.T) {
		_, fs := validateFixture(t, "deposit_bad_domain.xml")
		require.Len(t, fs, 2)
		assert.Equal(t, "domain", fs[0].ObjectType)
		assert.Contains(t, fs[0].Locator, "domain#1 offset=")
		assert.NotContains(t, fs[0].Message, "example-1", "object names never leak")
	})

	// Object is the one field allowed to carry deposit content, and the point
	// of it: an ordinal and a byte offset identify an object exactly and are
	// no use to an operator who does not have the deposit open.
	t.Run("a finding names the object it is about", func(t *testing.T) {
		_, fs := validateFixture(t, "deposit_bad_domain.xml")
		require.NotEmpty(t, fs)
		f := fs[0]
		require.NotEmpty(t, f.Object, "a finding about an object says which one")
		assert.Contains(t, f.Object, "example-1")
		// The other three fields keep the constant-template rule they had
		// before Object existed: these are what the DVFN and the logs read.
		assert.NotContains(t, f.Message, f.Object)
		assert.NotContains(t, f.Locator, f.Object)
		assert.NotContains(t, f.Rule, f.Object)
	})

	t.Run("every kind of object names itself", func(t *testing.T) {
		// One of each object type, each broken the same way: strip the roid,
		// which RFC 9022 requires of a domain, contact and host alike.
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1}
		raw := rdetest.BuildXML(opts)
		for _, ns := range []string{"rdeDomain", "rdeContact", "rdeHost"} {
			raw = dropElements(raw, ns+":roid")
		}
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))

		named := map[string]string{}
		for _, f := range fs {
			if f.Code == CodeRDEObjectInvalid && f.ObjectType != "" {
				named[f.ObjectType] = f.Object
			}
		}
		assert.Equal(t, "example-1.example", named["domain"])
		assert.Equal(t, "CONT1", named["contact"])
		assert.Equal(t, "ns1.example-1.example", named["host"])
	})

	t.Run("entity rejection is a warning, not a failure", func(t *testing.T) {
		// A registrar URL under the reserved .test domain is valid RDE but is
		// rejected by the domain-os URL rule; a registry-specific rule must not
		// fail another registry's deposit.
		raw := bytes.ReplaceAll(rdetest.BuildXML(rdetest.DepositOpts{Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1}), []byte("https://registrar1.example.com"), []byte("https://registrar1.example.test"))
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))
		require.Len(t, fs, 1)
		assert.Equal(t, CodeRDEObjectEntityRejected, fs[0].Code)
		assert.Equal(t, SeverityWarning, fs[0].Severity)
		assert.Equal(t, OutcomePass, Decide(fs))
	})

	// A rejection that does not say which rule rejected the object is a
	// rejection an operator cannot act on.
	t.Run("a rejection names the rule that refused the object", func(t *testing.T) {
		raw := bytes.ReplaceAll(
			rdetest.BuildXML(rdetest.DepositOpts{Domains: 2, Contacts: 1, Hosts: 1}),
			[]byte("_DOM-APEX"), []byte(" DOM APEX"))
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))
		require.NotEmpty(t, fs)

		var rejected []Finding
		for _, f := range fs {
			if f.Code == CodeRDEObjectEntityRejected {
				rejected = append(rejected, f)
			}
		}
		require.Len(t, rejected, 2, "both domains are refused")
		for _, f := range rejected {
			assert.Equal(t, `roid: does not match the EPP roidType pattern (\w|_){1,80}-\w{1,8}`, f.Rule)
			assert.Contains(t, f.Message, f.Rule, "the message carries the rule, so a reader of the findings list alone still learns it")
			assert.NotContains(t, f.Message, "example-", "the offending value never appears")
			assert.NotContains(t, f.Rule, "example-")
		}
	})

	// RFC 9022 makes most of an object's elements optional. Requiring them
	// rejected conformant deposits: a registrar with no postalInfo (§7.1 makes
	// it minOccurs="0") was the single ERROR that failed a real .radio deposit,
	// and every one of its 56,926 warnings came from rules like it.
	t.Run("a deposit is judged by RFC 9022, not by this registry's conventions", func(t *testing.T) {
		raw := rdetest.BuildXML(rdetest.DepositOpts{Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1, NNDNs: 1})
		// Strip the elements RFC 9022 marks optional but this validator used to
		// insist on, and give every object a roid in another registry's shape.
		for _, el := range []string{"crRr", "crDate", "upRr", "upDate", "exDate"} {
			raw = dropElements(raw, el)
		}
		raw = dropElements(raw, "rdeRegistrar:postalInfo")
		raw = regexp.MustCompile(`<(\w+):roid>[^<]*</`).ReplaceAll(raw, []byte("<${1}:roid>Dztys40879-RADIO</"))

		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))
		for _, f := range fs {
			assert.NotEqual(t, CodeRDEObjectInvalid, f.Code,
				"an element RFC 9022 makes optional is not a missing required element: %s / %s", f.ObjectType, f.Rule)
		}
		assert.NotEqual(t, OutcomeFail, Decide(fs))
	})

	// A contact or host that no domain uses is dead weight a successor registry
	// would inherit, and for a contact it is personal data with nothing left to
	// justify it. RFC 9022 does not forbid it, so it is a warning, not a failure.
	t.Run("a contact or host no domain references is reported", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 3, Hosts: 2}
		raw := dropElements(rdetest.BuildXML(opts), "rdeDomain:ns")
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))

		byType := map[string]int{}
		var orphaned []string
		for _, f := range fs {
			if f.Code == CodeRDEObjectNotReferenced {
				assert.Equal(t, SeverityWarning, f.Severity)
				assert.Contains(t, f.Locator, f.ObjectType+"#")
				assert.NotEmpty(t, f.Object, "an orphan an operator cannot name is not actionable")
				byType[f.ObjectType]++
				orphaned = append(orphaned, f.Object)
			}
		}
		assert.ElementsMatch(t,
			[]string{"CONT2", "CONT3", "ns1.example-1.example", "ns2.example-1.example"},
			orphaned, "each orphan is named, not just counted")
		// The one domain uses CONT1 for all three of its roles, so CONT2 and
		// CONT3 are orphans; stripping the ns block orphans both hosts.
		assert.Equal(t, 2, byType["contact"])
		assert.Equal(t, 2, byType["host"])
		assert.NotEqual(t, OutcomeFail, Decide(fs), "an orphan never fails a deposit")
	})

	// A broken reference is not the mirror of an orphan. An orphan is data
	// nobody asked for; a domain pointing at a contact that is not in the
	// deposit means the deposit is not integral and that domain cannot be
	// imported, so it fails the run.
	t.Run("a reference the deposit does not carry fails the deposit", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 1, Hosts: 1}
		raw := bytes.Replace(rdetest.BuildXML(opts),
			[]byte("<rdeDomain:registrant>CONT1</rdeDomain:registrant>"),
			[]byte("<rdeDomain:registrant>CONT-ELSEWHERE</rdeDomain:registrant>"), 1)
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))

		var dangling []Finding
		for _, f := range fs {
			if f.Code == CodeRDEReferenceNotInDeposit {
				dangling = append(dangling, f)
			}
		}
		require.Len(t, dangling, 1)
		assert.Equal(t, SeverityError, dangling[0].Severity)
		assert.Equal(t, "contact", dangling[0].ObjectType)
		assert.Equal(t, "domain#1", dangling[0].Locator, "located at the domain that made the reference")
		assert.NotContains(t, dangling[0].Message, "CONT-ELSEWHERE", "the identifier never appears in the template fields")
		assert.NotContains(t, dangling[0].Rule, "CONT-ELSEWHERE")
		assert.NotContains(t, dangling[0].Locator, "CONT-ELSEWHERE")
		// It does appear as the Object: the unresolved identifier is the one
		// thing the operator has to go and find, and the locator already says
		// which domain wanted it.
		assert.Equal(t, "CONT-ELSEWHERE", dangling[0].Object)
		assert.Equal(t, OutcomeFail, Decide(fs), "a deposit that is not integral is not a pass")
	})

	// The same domain, with the same missing contact, delegating to a
	// nameserver under another TLD: that host is somebody else's object and
	// its absence is not a defect, so only the contact fails the deposit.
	t.Run("an orphan alone never fails the deposit", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 3, Hosts: 2}
		raw := dropElements(rdetest.BuildXML(opts), "rdeDomain:ns")
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))

		var orphans int
		for _, f := range fs {
			if f.Code == CodeRDEObjectNotReferenced {
				orphans++
			}
		}
		require.Positive(t, orphans)
		assert.Equal(t, OutcomePass, Decide(fs))
	})

	// A nameserver under another TLD is somebody else's host object and RFC
	// 9022 does not ask this deposit to carry it. The generated deposits all
	// delegate to ns1.outside.example.net, so if this were wrong every fixture
	// above would carry a dangling-host warning.
	t.Run("an out-of-bailiwick nameserver is not a dangling reference", func(t *testing.T) {
		_, fs := validateFixture(t, "deposit_valid.xml")
		for _, f := range fs {
			assert.NotEqual(t, CodeRDEReferenceNotInDeposit, f.Code)
		}
	})

	// A registry running the host-object model declares an object for every
	// nameserver its domains use, and most of them are under other TLDs. Those
	// are not orphans: filtering host references by bailiwick before asking
	// whether a host is used called 2,168 of them orphans in a real deposit its
	// own domains delegate to.
	t.Run("a declared out-of-bailiwick host its domains use is not an orphan", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 1, Hosts: 1}
		raw := bytes.ReplaceAll(rdetest.BuildXML(opts),
			[]byte("ns1.example-1.example"), []byte("ns1.elsewhere.net"))
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))
		for _, f := range fs {
			assert.NotEqual(t, CodeRDEObjectNotReferenced, f.Code, "%s %s", f.ObjectType, f.Locator)
			assert.NotEqual(t, CodeRDEReferenceNotInDeposit, f.Code, "%s %s", f.ObjectType, f.Locator)
		}
	})

	// Only a FULL deposit is self-contained. In a DIFF the domain that uses a
	// contact may have been deposited weeks ago.
	t.Run("a DIFF deposit is not cross-referenced", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Kind: "DIFF", PrevID: "20260907001", Domains: 1, Contacts: 3, Hosts: 2}
		raw := dropElements(rdetest.BuildXML(opts), "rdeDomain:ns")
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(context.Background(), bytes.NewReader(raw))
		for _, f := range fs {
			assert.NotEqual(t, CodeRDEObjectNotReferenced, f.Code)
			assert.NotEqual(t, CodeRDEReferenceNotInDeposit, f.Code)
		}
	})

	// A registrar that writes its website without a scheme — "www.example.com/es"
	// — used to panic inside URL.Validate, and a panic anywhere in a decoder
	// unwinds the whole scan: Run recovers at the top of the pipeline, so a real
	// 813 MB .co deposit spent five minutes streaming 12 million objects and came
	// back with a single INTERNAL_ERROR finding that named nothing. It is a
	// rejected registrar, one warning among the rest.
	t.Run("a registrar URL with no scheme is a warning, not the end of the run", func(t *testing.T) {
		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 1, Hosts: 1, Registrars: 1, NNDNs: 1}
		raw := bytes.ReplaceAll(rdetest.BuildXML(opts),
			[]byte("https://registrar1.example.com<"), []byte("www.registrar1.example.com/es<"))
		v := &XMLValidator{BoundTLD: "example"}
		sum, fs := v.Validate(context.Background(), bytes.NewReader(raw))

		for _, f := range fs {
			assert.NotEqual(t, CodeInternal, f.Code, "%s %s: %s", f.ObjectType, f.Locator, f.Message)
		}
		var rejected []Finding
		for _, f := range fs {
			if f.Code == CodeRDEObjectEntityRejected && f.ObjectType == "registrar" {
				rejected = append(rejected, f)
			}
		}
		require.Len(t, rejected, 1)
		assert.Equal(t, SeverityWarning, rejected[0].Severity)
		assert.Equal(t, "registrar1", rejected[0].Object)
		assert.NotEqual(t, OutcomeError, Decide(fs))
		// The scan reached the end of the deposit rather than stopping at it.
		assert.Equal(t, 1, sum.Observed[entities.NNDN_URI])
	})

	// The containment behind that: a constructor that panics is a defect in
	// this service and the run cannot pass, but it costs one object and the
	// rest of the deposit is still validated — with the object, its offset and
	// Go's own words for what went wrong.
	t.Run("a panicking constructor costs one object, not the deposit", func(t *testing.T) {
		restore := objectSpecs
		t.Cleanup(func() { objectSpecs = restore })
		objectSpecs = append([]objectSpec{}, restore...)
		for i := range objectSpecs {
			if objectSpecs[i].name != "contact" {
				continue
			}
			inner := objectSpecs[i].decode
			objectSpecs[i].decode = func(dec *xml.Decoder, se *xml.StartElement) objectResult {
				out := inner(dec, se)
				if out.Name == "CONT2" {
					// exactly how the real decoders call their constructor
					out.Panicked, out.Rejected = guard(func() error {
						var boom []string
						_ = boom[2] // #nosec G602 -- panicking on purpose is the test
						return nil
					})
				}
				return out
			}
		}

		opts := rdetest.DepositOpts{TLD: "example", Domains: 1, Contacts: 3, Hosts: 1, Registrars: 1}
		v := &XMLValidator{BoundTLD: "example"}
		sum, fs := v.Validate(context.Background(), bytes.NewReader(rdetest.BuildXML(opts)))

		var internal []Finding
		for _, f := range fs {
			if f.Code == CodeInternal {
				internal = append(internal, f)
			}
		}
		require.Len(t, internal, 1)
		assert.Equal(t, SeverityError, internal[0].Severity)
		assert.Equal(t, "contact", internal[0].ObjectType)
		assert.Equal(t, "CONT2", internal[0].Object, "the object is named, which is the whole point")
		assert.Contains(t, internal[0].Locator, "contact#2 offset=")
		assert.Contains(t, internal[0].Message, "index out of range", "Go's own words for what went wrong")
		assert.Equal(t, OutcomeError, Decide(fs), "a defect in this service is not a verdict on the deposit")
		assert.Equal(t, 3, sum.Observed[entities.CONTACT_URI], "the scan carried on past it")
		assert.Equal(t, 1, sum.Observed[entities.REGISTRAR_URI])
	})

	t.Run("cancelled context yields a timeout finding", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		v := &XMLValidator{BoundTLD: "example"}
		_, fs := v.Validate(ctx, bytes.NewReader(rdetest.BuildXML(rdetest.DepositOpts{})))
		require.NotEmpty(t, fs)
		assert.Equal(t, CodeValidationTimeout, fs[0].Code)
	})
}

// dropElements removes every <ns:name>…</ns:name> and every <ns:name … /> from
// a deposit, so a fixture can be reduced to what RFC 9022 actually requires.
func dropElements(deposit []byte, name string) []byte {
	if !strings.Contains(name, ":") {
		name = `\w+:` + name
	}
	paired := regexp.MustCompile(`(?s)<` + name + `\b[^>]*>.*?</` + name + `>`)
	empty := regexp.MustCompile(`<` + name + `\b[^>]*/>`)
	return empty.ReplaceAll(paired.ReplaceAll(deposit, nil), nil)
}
