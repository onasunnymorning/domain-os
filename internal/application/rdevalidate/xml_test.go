package rdevalidate

import (
	"bytes"
	"context"
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
