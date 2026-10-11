package rdesanitize

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// standardDiff is a DIFF deposit that uses the parts of RFC 8909 / RFC 9022 the
// synthetic fixture generator has no option for: every kind of delete record,
// contact transfer data, a registrar-escrow header with its count attributes,
// and the optional nameState flag. Every value in it that identifies the source
// is a marker the assertions below look for.
const standardDiff = `<?xml version="1.0" encoding="UTF-8"?>
<rde:deposit xmlns:rde="urn:ietf:params:xml:ns:rde-1.0" xmlns:rdeHeader="urn:ietf:params:xml:ns:rdeHeader-1.0" xmlns:rdeDomain="urn:ietf:params:xml:ns:rdeDomain-1.0" xmlns:rdeHost="urn:ietf:params:xml:ns:rdeHost-1.0" xmlns:rdeContact="urn:ietf:params:xml:ns:rdeContact-1.0" xmlns:rdeRegistrar="urn:ietf:params:xml:ns:rdeRegistrar-1.0" xmlns:rdeIDN="urn:ietf:params:xml:ns:rdeIDN-1.0" xmlns:rdeNNDN="urn:ietf:params:xml:ns:rdeNNDN-1.0" xmlns:contact="urn:ietf:params:xml:ns:contact-1.0" type="DIFF" id="20260909001" prevId="20260908001">
  <rde:watermark>2026-09-09T00:00:00Z</rde:watermark>
  <rde:rdeMenu>
    <rde:version>1.0</rde:version>
    <rde:objURI>urn:ietf:params:xml:ns:rdeHeader-1.0</rde:objURI>
    <rde:objURI>urn:ietf:params:xml:ns:rdeContact-1.0</rde:objURI>
    <rde:objURI>urn:ietf:params:xml:ns:rdeNNDN-1.0</rde:objURI>
  </rde:rdeMenu>
  <rde:deletes>
    <rdeDomain:delete><rdeDomain:name>gone-domain.example</rdeDomain:name></rdeDomain:delete>
    <rdeHost:delete><rdeHost:name>ns1.gone-host.example</rdeHost:name><rdeHost:roid>7_HOST-APEX</rdeHost:roid></rdeHost:delete>
    <rdeContact:delete><rdeContact:id>CONT1</rdeContact:id></rdeContact:delete>
    <rdeRegistrar:delete><rdeRegistrar:id>registrar9</rdeRegistrar:id></rdeRegistrar:delete>
    <rdeIDN:delete><rdeIDN:id>table-9</rdeIDN:id></rdeIDN:delete>
    <rdeNNDN:delete><rdeNNDN:aName>gone-nndn.example</rdeNNDN:aName></rdeNNDN:delete>
  </rde:deletes>
  <rde:contents>
    <rdeHeader:header>
      <rdeHeader:ppsp>Acme-Privacy-Services</rdeHeader:ppsp>
      <rdeHeader:count uri="urn:ietf:params:xml:ns:rdeContact-1.0" rcdn="example" registrarId="1234">1</rdeHeader:count>
      <rdeHeader:count uri="urn:ietf:params:xml:ns:rdeNNDN-1.0">1</rdeHeader:count>
      <rdeHeader:contentTag>operator-note-8841</rdeHeader:contentTag>
    </rdeHeader:header>
    <rdeContact:contact>
      <rdeContact:id>CONT1</rdeContact:id>
      <rdeContact:roid>1_CONT-APEX</rdeContact:roid>
      <rdeContact:status s="ok"/>
      <rdeContact:postalInfo type="int">
        <contact:name>Jane Registrant</contact:name>
        <contact:addr>
          <contact:street>1 Real Street</contact:street>
          <contact:city>Testville</contact:city>
          <contact:cc>US</contact:cc>
        </contact:addr>
      </rdeContact:postalInfo>
      <rdeContact:email>jane@real.example.org</rdeContact:email>
      <rdeContact:clID>registrar1</rdeContact:clID>
      <rdeContact:trDate>2025-06-01T00:00:00Z</rdeContact:trDate>
      <rdeContact:trnData>
        <rdeContact:trStatus>clientApproved</rdeContact:trStatus>
        <rdeContact:reRr client="1001">registrar2</rdeContact:reRr>
        <rdeContact:reDate>2025-05-25T00:00:00Z</rdeContact:reDate>
        <rdeContact:acRr client="1002">registrar1</rdeContact:acRr>
        <rdeContact:acDate>2025-06-01T00:00:00Z</rdeContact:acDate>
      </rdeContact:trnData>
    </rdeContact:contact>
    <rdeNNDN:NNDN>
      <rdeNNDN:aName>nndn-1.example</rdeNNDN:aName>
      <rdeNNDN:nameState mirroringNS="false">withheld</rdeNNDN:nameState>
    </rdeNNDN:NNDN>
  </rde:contents>
</rde:deposit>
`

// A deposit that conforms to the standard is sanitised, not quarantined, and
// what comes out still conforms: the classification decisions for the parts of
// the standard that were once missing are made here and pinned.
func TestRewrite_StandardConformantDepositUsingEveryOptionalPartPasses(t *testing.T) {
	// The source is itself schema-valid — otherwise this would be a test of
	// how the sanitiser treats a broken deposit.
	if msg, ok := xmllintAgainstDepositSchemas(t, []byte(standardDiff)); !ok {
		t.Fatalf("the fixture is meant to be standard-conformant:\n%s", msg)
	}

	f := newFixture(t)
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), strings.NewReader(standardDiff), &out)
	require.Equal(t, OutcomePass, res.Outcome, "findings: %+v", res.Findings)
	doc := out.String()
	assert.Empty(t, f.scan(t, doc), "the derivative must satisfy its own regression scan")

	msg, ok := xmllintAgainstDepositSchemas(t, out.Bytes())
	assert.True(t, ok, "the derivative must conform to the standard too:\n%s", msg)

	// Delete records are classified by the entries of the objects they name.
	assert.NotContains(t, doc, "gone-domain.example")
	assert.Contains(t, doc, "<rdeDomain:name>gone-domain.artful-dodger</rdeDomain:name>")
	assert.Contains(t, doc, "<rdeNNDN:aName>gone-nndn.artful-dodger</rdeNNDN:aName>")
	assert.Contains(t, doc, "<rdeHost:name>ns1.gone-host.artful-dodger</rdeHost:name>")
	assert.Contains(t, doc, "<rdeHost:roid>7_HOST-APEX</rdeHost:roid>")
	assert.Contains(t, doc, "<rdeRegistrar:id>registrar9</rdeRegistrar:id>")
	assert.Contains(t, doc, "<rdeIDN:id>table-9</rdeIDN:id>")

	// A deleted contact is the same pseudonym as the contact it deletes, or a
	// derivative could not be used to follow a record through a DIFF.
	token := f.tokens.Value(ValContactID, "CONT1")
	assert.Equal(t, 2, strings.Count(doc, token), "the contact and its delete record carry one token")
	assert.NotContains(t, doc, "CONT1")

	// Header: the privacy/proxy provider is tokenised, the optional free text is
	// gone, and the count's TLD label — the source registry's name — is gone
	// with it while the attributes that identify nothing are kept.
	assert.NotContains(t, doc, "Acme-Privacy-Services")
	assert.Contains(t, doc, "<rdeHeader:ppsp>"+f.tokens.Value(ValContactID, "Acme-Privacy-Services")+"</rdeHeader:ppsp>")
	assert.NotContains(t, doc, "operator-note-8841")
	assert.NotContains(t, doc, "contentTag")
	assert.NotContains(t, doc, "rcdn")
	assert.Contains(t, doc, `registrarId="1234"`)
	assert.NotContains(t, doc, `"example"`, "no attribute may carry the source TLD")

	// Contact transfer data mirrors the domain's: retained.
	assert.Contains(t, doc, "<rdeContact:trStatus>clientApproved</rdeContact:trStatus>")
	assert.Contains(t, doc, `<rdeContact:reRr client="1001">registrar2</rdeContact:reRr>`)
	assert.Contains(t, doc, "<rdeContact:acDate>2025-06-01T00:00:00Z</rdeContact:acDate>")
	assert.Contains(t, doc, `<rdeNNDN:nameState mirroringNS="false">withheld</rdeNNDN:nameState>`)

	// And the direct identifiers are still replaced.
	assert.NotContains(t, doc, "Jane Registrant")
	assert.NotContains(t, doc, "jane@real.example.org")
	assert.NotContains(t, doc, "1 Real Street")
}

// The attribute removal is not special-cased to the one document above: a
// dropped attribute is counted, and an attribute the profile neither allows nor
// drops still quarantines.
func TestRewrite_DroppedAttributeIsCountedAndUnknownOnesStillQuarantine(t *testing.T) {
	f := newFixture(t)
	var out bytes.Buffer
	res := f.rw.Rewrite(context.Background(), strings.NewReader(standardDiff), &out)
	require.Equal(t, OutcomePass, res.Outcome)
	assert.GreaterOrEqual(t, res.Counts.Dropped, int64(2), "contentTag and rcdn were both removed")

	bad := strings.Replace(standardDiff, `registrarId="1234"`, `registrarId="1234" unlisted="x"`, 1)
	out.Reset()
	res = f.rw.Rewrite(context.Background(), strings.NewReader(bad), &out)
	assert.Equal(t, OutcomeQuarantined, res.Outcome)
	assert.True(t, res.Has(CodePolicyUnknownAttribute))
}
