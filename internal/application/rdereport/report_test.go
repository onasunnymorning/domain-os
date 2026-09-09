package rdereport

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixedReceived = time.Date(2026, 9, 8, 3, 15, 0, 0, time.UTC)
	fixedValid    = time.Date(2026, 9, 8, 5, 15, 0, 0, time.UTC)
	fixedSigned   = time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
)

// runFixture executes the real pipeline on a synthetic pair so the report is
// built from a genuine Result, then pins the clock-derived fields so the
// goldens are stable across runs.
func runFixture(t *testing.T, opts rdetest.DepositOpts, breakSig bool) rdevalidate.Result {
	t.Helper()
	service := rdetest.NewKeyPair(t, "eve-service")
	registry := rdetest.NewKeyPair(t, "registry")
	pair := rdetest.BuildPair(t, opts, service, registry)
	sig := pair.Sig
	if breakSig {
		sig = rdetest.Sign(t, []byte("something else"), registry, false)
	}
	res := rdevalidate.Run(context.Background(), rdevalidate.Input{
		OpenArtifact:    func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(pair.Ryde)), nil },
		Sig:         sig,
		TrustedKeys: []string{registry.ArmoredPublic},
		ServiceKeys: openpgp.EntityList{service.Entity},
		BoundTLD:    "example",
		Limits:      rdevalidate.DefaultLimits(),
		Now:         func() time.Time { return time.Now().UTC().Add(time.Hour) },
	})
	// Pin non-deterministic fields.
	res.Signature.SignedAt = fixedSigned
	res.Signature.KeyFingerprint = "0000000000000000000000000000000000000001"
	res.Decryption.KeyFingerprint = "0000000000000000000000000000000000000002"
	res.Decryption.LiteralTime = time.Time{}
	res.Digests = rdevalidate.Digests{ArtifactSHA256: strings.Repeat("ab", 32), SignatureSHA256: strings.Repeat("cd", 32), PlaintextSHA256: strings.Repeat("ef", 32)}
	for i := range res.Findings {
		res.Findings[i].At = fixedValid
	}
	res.StartedAt, res.CompletedAt = fixedValid, fixedValid
	return res
}

func params(res rdevalidate.Result) Params {
	return Params{DEAName: "domain-os EVE (test)", Result: res, BoundTLD: "example", ReceivedAt: fixedReceived, ValidatedAt: fixedValid}
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden missing; run with UPDATE_GOLDEN=1")
	assert.Equal(t, string(want), string(got))
}

func validateXSD(t *testing.T, doc []byte) {
	t.Helper()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils); conformance cannot be skipped there")
		}
		t.Skip("xmllint not installed; install libxml2 to run schema conformance locally")
	}
	f := filepath.Join(t.TempDir(), "doc.xml")
	require.NoError(t, os.WriteFile(f, doc, 0o600))
	out, err := exec.Command(xmllint, "--noout", "--schema", rdeschema.Path(rdeschema.ReportSchemas), f).CombinedOutput() //nolint:gosec // binary path comes from exec.LookPath, arguments are fixed
	require.NoError(t, err, "xmllint: %s\n--- document ---\n%s", out, doc)
}

func TestBuildReport_Pass(t *testing.T) {
	res := runFixture(t, rdetest.DepositOpts{TLD: "example", Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1}, false)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)

	r, err := BuildReport(params(res))
	require.NoError(t, err)
	assert.Equal(t, "20260908001", r.ID)
	assert.Equal(t, 1, r.Version)
	assert.Equal(t, "RFC8909", r.RydeSpecEscrow)
	assert.Equal(t, "RFC9022", r.RydeSpecMapping)
	assert.Equal(t, 0, r.Resend)
	assert.Equal(t, "FULL", r.Kind)
	assert.Equal(t, "2026-09-08T00:00:00Z", r.Watermark)
	assert.Equal(t, "2026-09-08T00:15:00Z", r.CrDate, "crDate falls back to the signature time when the literal packet has none")
	assert.Equal(t, "example", r.Header.TLD)
	assert.Equal(t, Count{URI: entities.DOMAIN_URI, N: 3}, r.Header.Count[0])
	assert.Len(t, r.Header.Count, 6)

	doc, err := r.Marshal()
	require.NoError(t, err)
	checkGolden(t, "report_pass.xml", doc)
	validateXSD(t, doc)
}

func TestBuildNotification_DVPN(t *testing.T) {
	res := runFixture(t, rdetest.DepositOpts{TLD: "example", Domains: 3, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1}, false)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome)
	p := params(res)
	lf := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	p.LastFullDate = &lf

	n, err := BuildNotification(p)
	require.NoError(t, err)
	assert.Equal(t, "DVPN", n.Status)
	assert.Equal(t, "2026-09-08", n.RepDate, "repDate is the watermark date")
	assert.Equal(t, "2026-09-08T03:15:00Z", n.ReDate)
	assert.Equal(t, "2026-09-08T05:15:00Z", n.VaDate)
	assert.Equal(t, "2026-09-01", n.LastFullDate)
	assert.Nil(t, n.Results, "DVPN carries no results")
	require.NotNil(t, n.Report)
	assert.Empty(t, n.XMLNSIIRDEA)

	doc, err := n.Marshal()
	require.NoError(t, err)
	checkGolden(t, "notification_dvpn.xml", doc)
	validateXSD(t, doc)

	// Round-trip through a namespace-aware decoder: the prefixed tags must
	// resolve to the right namespaces, not just look right.
	var probe struct {
		XMLName xml.Name
		Status  string `xml:"urn:ietf:params:xml:ns:rdeNotification-1.0 status"`
		Report  struct {
			ID string `xml:"urn:ietf:params:xml:ns:rdeReport-1.0 id"`
		} `xml:"urn:ietf:params:xml:ns:rdeReport-1.0 report"`
	}
	require.NoError(t, xml.Unmarshal(doc, &probe))
	assert.Equal(t, xml.Name{Space: NSNotification, Local: "notification"}, probe.XMLName)
	assert.Equal(t, "DVPN", probe.Status)
	assert.Equal(t, "20260908001", probe.Report.ID)
}

func TestBuildNotification_DVFN_ContentFailure(t *testing.T) {
	// Count mismatch + broken domains: the deposit was opened, so the report
	// carries its real id/kind/watermark and the observed counts.
	res := runFixture(t, rdetest.DepositOpts{TLD: "example", Domains: 2, Contacts: 1, Hosts: 1, Registrars: 1, BreakDomain: true,
		HeaderCounts: map[string]int{entities.DOMAIN_URI: 4, entities.CONTACT_URI: 1, entities.HOST_URI: 1, entities.REGISTRAR_URI: 1}}, false)
	require.Equal(t, rdevalidate.OutcomeFail, res.Outcome)

	n, err := BuildNotification(params(res))
	require.NoError(t, err)
	assert.Equal(t, "DVFN", n.Status)
	require.NotNil(t, n.Results)
	assert.Equal(t, NSIIRDEA, n.XMLNSIIRDEA)
	codes := map[int]bool{}
	for _, r := range n.Results.Result {
		codes[r.Code] = true
		assert.NotEmpty(t, r.Msg)
	}
	assert.True(t, codes[4504], "RDE_OBJECT_INVALID -> 4504")
	assert.True(t, codes[4502], "RDE_COUNT_MISMATCH -> 4502")
	assert.Equal(t, Count{URI: entities.DOMAIN_URI, N: 2}, n.Report.Header.Count[0], "header reports what was observed, not what the deposit claimed")

	doc, err := n.Marshal()
	require.NoError(t, err)
	checkGolden(t, "notification_dvfn_content.xml", doc)
	validateXSD(t, doc)
}

func TestBuildNotification_DVFN_SignatureFailure(t *testing.T) {
	// The deposit was never opened: id/kind/watermark come from the file-name
	// hints, and the header counts are all zero.
	res := runFixture(t, rdetest.DepositOpts{TLD: "example"}, true)
	require.Equal(t, rdevalidate.OutcomeFail, res.Outcome)
	require.Equal(t, rdevalidate.StageSignature, res.StageReached)

	p := params(res)
	hints, ok := ParseRydeFileName("uploads/2026/example_2026-09-07_full_S1_R0.ryde")
	require.True(t, ok)
	p.Hints = hints

	n, err := BuildNotification(p)
	require.NoError(t, err)
	assert.Equal(t, "DVFN", n.Status)
	assert.Equal(t, "2026-09-07", n.RepDate)
	assert.Equal(t, "20260907001", n.Report.ID)
	assert.Equal(t, "FULL", n.Report.Kind)
	assert.Equal(t, "2026-09-07T00:00:00Z", n.Report.Watermark)
	require.Len(t, n.Results.Result, 1)
	assert.Equal(t, 4103, n.Results.Result[0].Code)
	for _, c := range n.Report.Header.Count {
		assert.Equal(t, 0, c.N)
	}

	doc, err := n.Marshal()
	require.NoError(t, err)
	checkGolden(t, "notification_dvfn_signature.xml", doc)
	validateXSD(t, doc)

	t.Run("without hints the digest identifies the artifact", func(t *testing.T) {
		p.Hints = Hints{}
		n, err := BuildNotification(p)
		require.NoError(t, err)
		assert.Equal(t, strings.ToUpper(strings.Repeat("ab", 32))[:13], n.Report.ID)
		assert.Equal(t, "2026-09-08", n.RepDate, "watermark falls back to the receipt time")
		doc, err := n.Marshal()
		require.NoError(t, err)
		validateXSD(t, doc)
	})
}

func TestBuildNotification_ErrorOutcomeEmitsNothing(t *testing.T) {
	res := runFixture(t, rdetest.DepositOpts{TLD: "example"}, false)
	res.Outcome = rdevalidate.OutcomeError
	_, err := BuildNotification(params(res))
	require.ErrorIs(t, err, ErrNoNotificationForOutcome)
}

func TestBuildNotification_Validation(t *testing.T) {
	res := runFixture(t, rdetest.DepositOpts{TLD: "example"}, false)
	p := params(res)
	p.DEAName = " "
	_, err := BuildNotification(p)
	assert.Error(t, err)
	p = params(res)
	p.ValidatedAt = time.Time{}
	_, err = BuildNotification(p)
	assert.Error(t, err)
	p = params(res)
	p.ReceivedAt = time.Time{}
	_, err = BuildReport(p)
	assert.Error(t, err)
	p = params(res)
	p.BoundTLD = "not a tld!"
	_, err = BuildReport(p)
	assert.Error(t, err)
}

func TestParseRydeFileName(t *testing.T) {
	h, ok := ParseRydeFileName("EXAMPLE_2026-09-07_FULL_S12_R3.ryde")
	require.True(t, ok)
	assert.Equal(t, Hints{DepositID: "20260907012", Kind: "FULL", Watermark: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, h)
	h, ok = ParseRydeFileName("/bucket/path/xn--abc_2026-01-31_diff_S1_R0.ryde")
	require.True(t, ok)
	assert.Equal(t, "DIFF", h.Kind)
	_, ok = ParseRydeFileName("deposit.ryde")
	assert.False(t, ok)
	_, ok = ParseRydeFileName("example_2026-13-40_full_S1_R0.ryde")
	assert.False(t, ok)
}

func TestResultCodes_CoverEveryFailureCode(t *testing.T) {
	// Every code that can carry ERROR severity and produce a DVFN must map to
	// a documented 4xxx result; error-class and informational codes need not.
	all := []rdevalidate.Code{
		rdevalidate.CodeIntakeArtifactMissing, rdevalidate.CodeIntakeLimitCompressedSize,
		rdevalidate.CodeSigMalformed, rdevalidate.CodeSigKeyUntrusted, rdevalidate.CodeSigInvalid,
		rdevalidate.CodeDecryptNotForServiceKey, rdevalidate.CodeDecryptFailed,
		rdevalidate.CodeArchiveUnsupportedLayout, rdevalidate.CodeArchiveUnsafeEntry, rdevalidate.CodeArchiveLimitFileCount,
		rdevalidate.CodeArchiveLimitUnpackedSize, rdevalidate.CodeArchiveLimitNesting, rdevalidate.CodeArchiveNoXML,
		rdevalidate.CodeXMLMalformed, rdevalidate.CodeXMLNoDeposit, rdevalidate.CodeXMLNoHeader,
		rdevalidate.CodeRDEHeaderTLDMismatch, rdevalidate.CodeRDECountMismatch, rdevalidate.CodeRDEObjectDecodeError,
		rdevalidate.CodeRDEObjectInvalid, rdevalidate.CodeRDERequiredObjectMissing,
	}
	seen := map[int]rdevalidate.Code{}
	for _, c := range all {
		code, msg := ResultCodeFor(c)
		assert.NotEqual(t, unknownResultCode.Code, code, "%s has no result code", c)
		assert.NotEmpty(t, msg)
		assert.True(t, code >= 4000 && code <= 4998, "%s: %d outside the 4xxx range", c, code)
		if prev, dup := seen[code]; dup {
			t.Errorf("result code %d used by both %s and %s", code, prev, c)
		}
		seen[code] = c
	}
	code, _ := ResultCodeFor("SOMETHING_NEW")
	assert.Equal(t, 4999, code)
}
