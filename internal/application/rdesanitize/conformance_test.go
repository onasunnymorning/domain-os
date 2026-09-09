package rdesanitize

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// richDeposit is the fixture the conformance and golden checks run against: it
// carries something from every category in the ticket's field-policy table.
func richDeposit() rdetest.DepositOpts {
	return rdetest.DepositOpts{
		TLD: sourceTLD, Domains: 2, Contacts: 2, Hosts: 2, Registrars: 1, IDNs: 1, NNDNs: 1,
		AuthInfo: true, SecDNS: true, Disclose: true, PrivacyProxy: true, SharedContact: true,
		IDNDomain: true, EppParams: true,
	}
}

func xmllintAgainstDepositSchemas(t *testing.T, doc []byte) (string, bool) {
	t.Helper()
	bin, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils); conformance cannot be skipped there")
		}
		t.Skip("xmllint not installed; install libxml2 to run schema conformance locally")
	}
	f := filepath.Join(t.TempDir(), "doc.xml")
	require.NoError(t, os.WriteFile(f, doc, 0o600))
	out, err := exec.Command(bin, "--noout", "--schema", rdeschema.Path(rdeschema.DepositSchemas), f).CombinedOutput() //nolint:gosec // binary path comes from exec.LookPath, arguments are fixed
	return string(out), err == nil
}

func TestDerivative_IsSchemaValidRDE(t *testing.T) {
	f := newFixture(t)
	opts := richDeposit()
	src := rdetest.BuildXML(opts)

	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), bytes.NewReader(src), &out)
	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)

	// The derivative validates against the published RFC 8909 / RFC 9022 and
	// EPP schemas — not against our own structs. A consumer that only has the
	// standard schemas would accept it.
	msg, ok := xmllintAgainstDepositSchemas(t, out.Bytes())
	assert.True(t, ok, "derivative failed schema validation:\n%s", msg)

	// The source does not validate, and the only reason is the credential
	// blocks the profile removes. Sanitising a deposit therefore makes it
	// *more* conformant, never less: RDE has no element for an authInfo.
	msg, ok = xmllintAgainstDepositSchemas(t, src)
	assert.False(t, ok, "the fixture is expected to carry authInfo, which RDE does not define")
	assert.Contains(t, msg, "authInfo")
	assert.NotContains(t, msg, "postalInfo")
}

func TestDerivative_Golden(t *testing.T) {
	f := newFixture(t)
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), bytes.NewReader(rdetest.BuildXML(richDeposit())), &out)
	require.Equal(t, OutcomePass, res.Outcome, "findings: %v", res.Findings)
	checkGolden(t, "derivative.xml", out.Bytes())

	run, err := entities.NewEscrowSanitizationRun(
		mustScope(t), sourceTLD,
		uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PolicyVersion, "escrow-sanitize/1", synthetic, "escrow-sanitize-example-20260909-000000", "run-1",
		time.Unix(1757300000, 0).UTC())
	require.NoError(t, err)
	run.ID = uuid.MustParse("33333333-3333-4333-8333-333333333333")

	m := NewManifest(run, res, entities.EscrowProfileRydeSig, f.tokens.KeyFingerprint(),
		"escrow-validation/ryop1/example/22222222-2222-4222-8222-222222222222/33333333-3333-4333-8333-333333333333/sanitized/deposit-"+PolicyVersion+".xml.gz",
		"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210", 4096)
	raw, err := m.Marshal()
	require.NoError(t, err)
	checkGolden(t, "manifest.json", append(raw, '\n'))

	// The manifest is the only description of the derivative anyone downstream
	// reads, so it must carry counts and identifiers and nothing else.
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, entities.EscrowDerivativeLabel, decoded["label"])
	for _, forbidden := range []string{"Testville", "Contact ", "@example", "registrar1@", "Privacy Proxy", "CONT1", "example-1"} {
		assert.NotContains(t, string(raw), forbidden, "the manifest must carry no values from the deposit")
	}
	assert.NotContains(t, string(raw), string(testKey), "the manifest must never carry key material")
}

func mustScope(t *testing.T) entities.OperatorID {
	t.Helper()
	s, err := entities.NewOperatorID("ryop1")
	require.NoError(t, err)
	return s
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
