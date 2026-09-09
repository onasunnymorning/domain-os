// Package rdetest builds synthetic escrow fixtures for tests: throwaway
// OpenPGP key pairs, RFC 8909/9022-shaped deposit XML, tar/gzip payload
// layouts, encryption to a recipient and detached signatures. Everything is
// generated at test time so no key material or binary blobs live in git, and
// every failure path is an option tweak on a valid fixture.
//
// It is a normal (non-_test) package so several test packages can share it;
// it must only ever be imported by tests.
package rdetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// KeyPair is a generated OpenPGP identity with its armored forms.
type KeyPair struct {
	Entity         *openpgp.Entity
	ArmoredPublic  string
	ArmoredPrivate string // unencrypted
	Fingerprint    string // upper-case hex, primary key
}

// NewKeyPair generates a fast Ed25519/Curve25519 key pair.
func NewKeyPair(t testing.TB, name string) KeyPair {
	t.Helper()
	cfg := &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA}
	e, err := openpgp.NewEntity(name, "synthetic test key", name+"@example.test", cfg)
	if err != nil {
		t.Fatalf("rdetest: NewEntity: %v", err)
	}
	return KeyPair{
		Entity:         e,
		ArmoredPublic:  armorPublic(t, e),
		ArmoredPrivate: armorPrivate(t, e),
		Fingerprint:    strings.ToUpper(fmt.Sprintf("%x", e.PrimaryKey.Fingerprint)),
	}
}

// EncryptedArmoredPrivate returns the private key armored and protected by
// passphrase, leaving kp.Entity itself unlocked.
func EncryptedArmoredPrivate(t testing.TB, kp KeyPair, passphrase string) string {
	t.Helper()
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(kp.ArmoredPrivate))
	if err != nil || len(el) != 1 {
		t.Fatalf("rdetest: reparse private key: %v", err)
	}
	if err := el[0].EncryptPrivateKeys([]byte(passphrase), nil); err != nil {
		t.Fatalf("rdetest: EncryptPrivateKeys: %v", err)
	}
	// The keys are locked now, so serialise without re-signing.
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("rdetest: armor: %v", err)
	}
	if err := el[0].SerializePrivateWithoutSigning(w, nil); err != nil {
		t.Fatalf("rdetest: serialize locked private: %v", err)
	}
	_ = w.Close()
	return buf.String()
}

func armorPublic(t testing.TB, e *openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("rdetest: armor: %v", err)
	}
	if err := e.Serialize(w); err != nil {
		t.Fatalf("rdetest: serialize public: %v", err)
	}
	_ = w.Close()
	return buf.String()
}

func armorPrivate(t testing.TB, e *openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("rdetest: armor: %v", err)
	}
	if err := e.SerializePrivate(w, nil); err != nil {
		t.Fatalf("rdetest: serialize private: %v", err)
	}
	_ = w.Close()
	return buf.String()
}

// Layout mirrors rdevalidate.Layout without importing it (rdevalidate's tests
// import this package).
type Layout string

const (
	LayoutXML  Layout = "xml"
	LayoutGzip Layout = "gzip"
	LayoutTar  Layout = "tar"
)

// TarEntry is an extra archive member injected before or after the payload.
type TarEntry struct {
	Name string
	Type byte // tar.TypeReg, tar.TypeDir, tar.TypeSymlink…
	Body []byte
	Link string
}

// DepositOpts shapes a synthetic deposit. The zero value with a TLD set
// produces a small valid FULL deposit.
type DepositOpts struct {
	TLD       string
	ID        string
	PrevID    string
	Kind      string // FULL (default) | DIFF | INCR
	Resend    string // attribute text; "" omits the attribute
	Watermark time.Time

	Domains, Contacts, Hosts, Registrars, IDNs, NNDNs int

	// HeaderCounts, when non-nil, replaces the generated header counts (uri -> n).
	HeaderCounts  map[string]int
	OmitHeader    bool
	OmitDeposit   bool // wrap the contents in a non-rde root element
	OmitWatermark bool
	HeaderTLD     string // overrides the header tld when set
	Malformed     bool   // truncate the document mid-element
	// Canary is planted in a contact org name so redaction tests can look for it.
	Canary string
	// BreakDomain drops the roid from every domain (RDE-required element missing).
	BreakDomain bool

	// The options below exist for the sanitisation work (issue #415): they
	// produce the material a derivative has to remove, replace, retain or
	// refuse. None of them are needed to validate a deposit.

	// AuthInfo adds <rdeDomain:authInfo>/<rdeContact:authInfo> with a password
	// that must never survive into a derivative.
	AuthInfo bool
	// SecDNS adds both dsData and keyData to every domain. Both are published
	// in DNS and must be retained unchanged apart from the suffix rewrite.
	SecDNS bool
	// Disclose adds a contact disclosure block, whose child elements share
	// their names with the postal fields but are preferences, not values.
	Disclose bool
	// PrivacyProxy makes the first contact a privacy/proxy record whose
	// underlying customer data is as identifying as a registrant's.
	PrivacyProxy bool
	// SharedContact points every domain at the same contact handle, so a
	// derivative can be checked for preserving the relationship.
	SharedContact bool
	// IDNDomain adds an IDN domain carrying both the A-label and the U-label.
	IDNDomain bool
	// VendorExtension adds an element in a namespace the profile does not
	// classify.
	VendorExtension bool
	// EppParams adds the rdeEppParams block including its DCP policy statement.
	EppParams bool
	// DTD prepends a document type declaration.
	DTD bool
	// Encoding overrides the XML declaration's encoding (e.g. "ISO-8859-1").
	Encoding string

	Layout    Layout
	EntryGzip bool       // gzip the XML inside the tar entry
	GzipWraps int        // extra gzip layers around the whole payload
	Before    []TarEntry // entries injected before the payload
	After     []TarEntry // entries injected after the payload
	EntryName string     // payload entry name (default deposit.xml)
	EntryType byte       // payload entry type (default tar.TypeReg)
}

func (o DepositOpts) withDefaults() DepositOpts {
	if o.TLD == "" {
		o.TLD = "example"
	}
	if o.ID == "" {
		o.ID = "20260908001"
	}
	if o.Kind == "" {
		o.Kind = "FULL"
	}
	if o.Watermark.IsZero() {
		o.Watermark = time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	}
	if o.Domains == 0 && o.Contacts == 0 && o.Hosts == 0 && o.Registrars == 0 {
		o.Domains, o.Contacts, o.Hosts, o.Registrars = 2, 2, 2, 1
	}
	if o.Layout == "" {
		o.Layout = LayoutTar
	}
	if o.EntryName == "" {
		o.EntryName = "deposit.xml"
	}
	if o.EntryType == 0 {
		o.EntryType = tar.TypeReg
	}
	return o
}

// BuildXML renders the deposit document.
func BuildXML(o DepositOpts) []byte {
	o = o.withDefaults()
	var b strings.Builder
	enc := o.Encoding
	if enc == "" {
		enc = "UTF-8"
	}
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="%s"?>`+"\n", enc)
	if o.DTD {
		b.WriteString("<!DOCTYPE rde:deposit [ <!ENTITY dummy \"x\"> ]>\n")
	}
	root := "rde:deposit"
	if o.OmitDeposit {
		root = "rde:bundle"
	}
	fmt.Fprintf(&b, `<%s xmlns:rde="%s" xmlns:rdeHeader="%s" xmlns:rdeDomain="%s" xmlns:rdeHost="%s" xmlns:rdeContact="%s" xmlns:rdeRegistrar="%s" xmlns:rdeIDN="%s" xmlns:rdeNNDN="%s" xmlns:domain="urn:ietf:params:xml:ns:domain-1.0" xmlns:secDNS="urn:ietf:params:xml:ns:secDNS-1.1" xmlns:contact="urn:ietf:params:xml:ns:contact-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0" xmlns:rdeEppParams="%s" xmlns:vnd="urn:example:vendor-1.0" type="%s" id="%s"`,
		root, entities.RDE_URI, entities.RDE_HEADER_URI, entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.IDN_URI, entities.NNDN_URI, entities.EPP_PARAMS_URI, o.Kind, o.ID)
	if o.PrevID != "" {
		fmt.Fprintf(&b, ` prevId="%s"`, o.PrevID)
	}
	if o.Resend != "" {
		fmt.Fprintf(&b, ` resend="%s"`, o.Resend)
	}
	b.WriteString(">\n")
	if !o.OmitWatermark {
		fmt.Fprintf(&b, "  <rde:watermark>%s</rde:watermark>\n", o.Watermark.UTC().Format(time.RFC3339))
	}
	b.WriteString("  <rde:rdeMenu>\n    <rde:version>1.0</rde:version>\n")
	for _, uri := range []string{entities.RDE_HEADER_URI, entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.IDN_URI, entities.NNDN_URI} {
		fmt.Fprintf(&b, "    <rde:objURI>%s</rde:objURI>\n", uri)
	}
	b.WriteString("  </rde:rdeMenu>\n  <rde:contents>\n")

	if !o.OmitHeader {
		counts := o.HeaderCounts
		if counts == nil {
			counts = map[string]int{}
			domains := o.Domains
			if o.IDNDomain {
				domains++ // the IDN domain is a domain like any other and is counted
			}
			for uri, n := range map[string]int{entities.DOMAIN_URI: domains, entities.HOST_URI: o.Hosts, entities.CONTACT_URI: o.Contacts, entities.REGISTRAR_URI: o.Registrars, entities.IDN_URI: o.IDNs, entities.NNDN_URI: o.NNDNs} {
				if n > 0 {
					counts[uri] = n
				}
			}
		}
		htld := o.TLD
		if o.HeaderTLD != "" {
			htld = o.HeaderTLD
		}
		fmt.Fprintf(&b, "    <rdeHeader:header>\n      <rdeHeader:tld>%s</rdeHeader:tld>\n", htld)
		for _, uri := range []string{entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.IDN_URI, entities.NNDN_URI} {
			if n, ok := counts[uri]; ok {
				fmt.Fprintf(&b, `      <rdeHeader:count uri="%s">%d</rdeHeader:count>`+"\n", uri, n)
			}
		}
		b.WriteString("    </rdeHeader:header>\n")
	}

	for i := 1; i <= o.Registrars; i++ {
		fmt.Fprintf(&b, `    <rdeRegistrar:registrar>
      <rdeRegistrar:id>registrar%d</rdeRegistrar:id>
      <rdeRegistrar:name>Registrar %d</rdeRegistrar:name>
      <rdeRegistrar:gurid>%d</rdeRegistrar:gurid>
      <rdeRegistrar:status>ok</rdeRegistrar:status>
      <rdeRegistrar:postalInfo type="int">
        <rdeRegistrar:addr>
          <rdeRegistrar:street>1 Test Street</rdeRegistrar:street>
          <rdeRegistrar:city>Testville</rdeRegistrar:city>
          <rdeRegistrar:cc>US</rdeRegistrar:cc>
        </rdeRegistrar:addr>
      </rdeRegistrar:postalInfo>
      <rdeRegistrar:voice>+1.5555550100</rdeRegistrar:voice>
      <rdeRegistrar:email>registrar%d@example.com</rdeRegistrar:email>
      <rdeRegistrar:url>https://registrar%d.example.com</rdeRegistrar:url>
      <rdeRegistrar:whoisInfo>
        <rdeRegistrar:name>whois.registrar%d.example.com</rdeRegistrar:name>
        <rdeRegistrar:url>https://whois.registrar%d.example.com</rdeRegistrar:url>
      </rdeRegistrar:whoisInfo>
      <rdeRegistrar:crDate>2025-01-01T00:00:00Z</rdeRegistrar:crDate>
    </rdeRegistrar:registrar>
`, i, i, 9000+i, i, i, i, i)
	}
	for i := 1; i <= o.Contacts; i++ {
		org := "Org " + fmt.Sprint(i)
		if o.Canary != "" {
			org = o.Canary
		}
		name := fmt.Sprintf("Contact %d", i)
		if o.PrivacyProxy && i == 1 {
			// A privacy/proxy record: the visible contact is the proxy service,
			// the underlying customer is in the same fields and is exactly as
			// identifying as a registrant.
			name = "Privacy Proxy Customer 4711"
			org = "Proxy Services Ltd"
		}
		disclose := ""
		if o.Disclose {
			disclose = `      <rdeContact:disclose flag="0">
        <contact:name type="int"/>
        <contact:org type="int"/>
        <contact:addr type="int"/>
        <contact:voice/>
        <contact:email/>
      </rdeContact:disclose>
`
		}
		authInfo := ""
		if o.AuthInfo {
			authInfo = `      <rdeContact:authInfo>
        <contact:pw>2fooBAR-contact</contact:pw>
      </rdeContact:authInfo>
`
		}
		fmt.Fprintf(&b, `    <rdeContact:contact>
      <rdeContact:id>CONT%d</rdeContact:id>
      <rdeContact:roid>%d_CONT-APEX</rdeContact:roid>
      <rdeContact:status s="ok"/>
      <rdeContact:postalInfo type="int">
        <contact:name>%s</contact:name>
        <contact:org>%s</contact:org>
        <contact:addr>
          <contact:street>%d Test Street</contact:street>
          <contact:city>Testville</contact:city>
          <contact:pc>12345</contact:pc>
          <contact:cc>US</contact:cc>
        </contact:addr>
      </rdeContact:postalInfo>
      <rdeContact:voice>+1.555555%04d</rdeContact:voice>
      <rdeContact:email>contact%d@example.com</rdeContact:email>
      <rdeContact:clID>registrar1</rdeContact:clID>
      <rdeContact:crRr>registrar1</rdeContact:crRr>
      <rdeContact:crDate>2025-01-01T00:00:00Z</rdeContact:crDate>
%s%s    </rdeContact:contact>
`, i, i, name, org, i, i, i, disclose, authInfo)
	}
	for i := 1; i <= o.Hosts; i++ {
		fmt.Fprintf(&b, `    <rdeHost:host>
      <rdeHost:name>ns%d.example-1.%s</rdeHost:name>
      <rdeHost:roid>%d_HOST-APEX</rdeHost:roid>
      <rdeHost:status s="ok"/>
      <rdeHost:addr ip="v4">192.0.2.%d</rdeHost:addr>
      <rdeHost:clID>registrar1</rdeHost:clID>
      <rdeHost:crRr>registrar1</rdeHost:crRr>
      <rdeHost:crDate>2025-01-01T00:00:00Z</rdeHost:crDate>
    </rdeHost:host>
`, i, o.TLD, i, i)
	}
	for i := 1; i <= o.Domains; i++ {
		roid := fmt.Sprintf("      <rdeDomain:roid>%d_DOM-APEX</rdeDomain:roid>\n", i)
		if o.BreakDomain {
			roid = ""
		}
		contactID := "CONT1"
		if !o.SharedContact && o.Contacts > 1 {
			contactID = fmt.Sprintf("CONT%d", (i-1)%o.Contacts+1)
		}
		extra := ""
		if o.SecDNS {
			// dsData and keyData are both published in DNS and must survive.
			extra += `      <rdeDomain:secDNS>
        <secDNS:dsData>
          <secDNS:keyTag>12345</secDNS:keyTag>
          <secDNS:alg>13</secDNS:alg>
          <secDNS:digestType>2</secDNS:digestType>
          <secDNS:digest>49FD46E6C4B45C55D4AC</secDNS:digest>
          <secDNS:keyData>
            <secDNS:flags>257</secDNS:flags>
            <secDNS:protocol>3</secDNS:protocol>
            <secDNS:alg>13</secDNS:alg>
            <secDNS:pubKey>AQPJ////4Q==</secDNS:pubKey>
          </secDNS:keyData>
        </secDNS:dsData>
      </rdeDomain:secDNS>
`
		}
		if o.AuthInfo {
			extra += `      <rdeDomain:authInfo>
        <domain:pw>2fooBAR-domain</domain:pw>
      </rdeDomain:authInfo>
`
		}
		if o.VendorExtension {
			extra += "      <vnd:internalScore>0.93</vnd:internalScore>\n"
		}
		fmt.Fprintf(&b, `    <rdeDomain:domain>
      <rdeDomain:name>example-%d.%s</rdeDomain:name>
%s      <rdeDomain:status s="ok"/>
      <rdeDomain:registrant>%s</rdeDomain:registrant>
      <rdeDomain:contact type="admin">%s</rdeDomain:contact>
      <rdeDomain:contact type="tech">%s</rdeDomain:contact>
      <rdeDomain:ns>
        <domain:hostObj>ns1.example-1.%s</domain:hostObj>
        <domain:hostObj>ns1.outside.example.net</domain:hostObj>
      </rdeDomain:ns>
      <rdeDomain:clID>registrar1</rdeDomain:clID>
      <rdeDomain:crRr>registrar1</rdeDomain:crRr>
      <rdeDomain:crDate>2025-01-01T00:00:00Z</rdeDomain:crDate>
      <rdeDomain:exDate>2027-01-01T00:00:00Z</rdeDomain:exDate>
%s    </rdeDomain:domain>
`, i, o.TLD, roid, contactID, contactID, contactID, o.TLD, extra)
	}
	if o.IDNDomain {
		// The A-label and the U-label describe the same name; a suffix rewrite
		// that touched only one of them would desynchronise the pair.
		fmt.Fprintf(&b, `    <rdeDomain:domain>
      <rdeDomain:name>xn--nxasmm1c.%s</rdeDomain:name>
      <rdeDomain:roid>IDN_DOM-APEX</rdeDomain:roid>
      <rdeDomain:uName>βόλος.%s</rdeDomain:uName>
      <rdeDomain:idnTableId>Greek</rdeDomain:idnTableId>
      <rdeDomain:status s="ok"/>
      <rdeDomain:registrant>CONT1</rdeDomain:registrant>
      <rdeDomain:clID>registrar1</rdeDomain:clID>
      <rdeDomain:crRr>registrar1</rdeDomain:crRr>
      <rdeDomain:crDate>2025-01-01T00:00:00Z</rdeDomain:crDate>
    </rdeDomain:domain>
`, o.TLD, o.TLD)
	}
	for i := 1; i <= o.IDNs; i++ {
		fmt.Fprintf(&b, `    <rdeIDN:idnTableRef id="table-%d">
      <rdeIDN:url>https://idn.example.com/table-%d.txt</rdeIDN:url>
      <rdeIDN:urlPolicy>https://idn.example.com/policy-%d.html</rdeIDN:urlPolicy>
    </rdeIDN:idnTableRef>
`, i, i, i)
	}
	for i := 1; i <= o.NNDNs; i++ {
		fmt.Fprintf(&b, `    <rdeNNDN:NNDN>
      <rdeNNDN:aName>nndn-%d.%s</rdeNNDN:aName>
      <rdeNNDN:nameState>withheld</rdeNNDN:nameState>
      <rdeNNDN:crDate>2025-01-01T00:00:00Z</rdeNNDN:crDate>
    </rdeNNDN:NNDN>
`, i, o.TLD)
	}
	if o.EppParams {
		b.WriteString(`    <rdeEppParams:eppParams>
      <rdeEppParams:version>1.0</rdeEppParams:version>
      <rdeEppParams:lang>en</rdeEppParams:lang>
      <rdeEppParams:objURI>urn:ietf:params:xml:ns:domain-1.0</rdeEppParams:objURI>
      <rdeEppParams:svcExtension>
        <epp:extURI>urn:ietf:params:xml:ns:secDNS-1.1</epp:extURI>
      </rdeEppParams:svcExtension>
      <rdeEppParams:dcp>
        <epp:access><epp:all/></epp:access>
        <epp:statement>
          <epp:purpose><epp:admin/><epp:prov/></epp:purpose>
          <epp:recipient><epp:ours/></epp:recipient>
          <epp:retention><epp:stated/></epp:retention>
        </epp:statement>
      </rdeEppParams:dcp>
    </rdeEppParams:eppParams>
`)
	}
	b.WriteString("  </rde:contents>\n")
	fmt.Fprintf(&b, "</%s>\n", root)
	out := b.String()
	if o.Malformed {
		out = out[:len(out)*2/3]
	}
	return []byte(out)
}

// BuildPayload wraps xml in the requested layout (the decrypted plaintext).
func BuildPayload(t testing.TB, o DepositOpts, xml []byte) []byte {
	t.Helper()
	o = o.withDefaults()
	var payload []byte
	switch o.Layout {
	case LayoutXML:
		payload = xml
	case LayoutGzip:
		payload = Gzip(t, xml)
	case LayoutTar:
		body := xml
		if o.EntryGzip {
			body = Gzip(t, xml)
		}
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		write := func(e TarEntry) {
			body := e.Body
			if e.Type != tar.TypeReg {
				body = nil // only regular files carry content
			}
			hdr := &tar.Header{Name: e.Name, Typeflag: e.Type, Mode: 0o644, Size: int64(len(body)), Linkname: e.Link, ModTime: time.Unix(0, 0)}
			if e.Type == tar.TypeDir {
				hdr.Mode = 0o755
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatalf("rdetest: tar header: %v", err)
			}
			if _, err := tw.Write(body); err != nil {
				t.Fatalf("rdetest: tar body: %v", err)
			}
		}
		for _, e := range o.Before {
			write(e)
		}
		write(TarEntry{Name: o.EntryName, Type: o.EntryType, Body: body})
		for _, e := range o.After {
			write(e)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("rdetest: tar close: %v", err)
		}
		payload = buf.Bytes()
	default:
		t.Fatalf("rdetest: unknown layout %q", o.Layout)
	}
	for i := 0; i < o.GzipWraps; i++ {
		payload = Gzip(t, payload)
	}
	return payload
}

// Gzip compresses b.
func Gzip(t testing.TB, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		t.Fatalf("rdetest: gzip: %v", err)
	}
	_ = gz.Close()
	return buf.Bytes()
}

// Encrypt produces an OpenPGP message for the recipient (binary, unsigned).
func Encrypt(t testing.TB, plaintext []byte, recipients ...KeyPair) []byte {
	t.Helper()
	var to []*openpgp.Entity
	for _, r := range recipients {
		to = append(to, r.Entity)
	}
	var buf bytes.Buffer
	w, err := openpgp.Encrypt(&buf, to, nil, &openpgp.FileHints{IsBinary: true}, nil)
	if err != nil {
		t.Fatalf("rdetest: Encrypt: %v", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		t.Fatalf("rdetest: Encrypt write: %v", err)
	}
	_ = w.Close()
	return buf.Bytes()
}

// Sign produces a detached signature over data, binary or armored.
func Sign(t testing.TB, data []byte, signer KeyPair, armored bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	if armored {
		err = openpgp.ArmoredDetachSign(&buf, signer.Entity, bytes.NewReader(data), nil)
	} else {
		err = openpgp.DetachSign(&buf, signer.Entity, bytes.NewReader(data), nil)
	}
	if err != nil {
		t.Fatalf("rdetest: DetachSign: %v", err)
	}
	return buf.Bytes()
}

// Pair is a complete .ryde/.sig fixture with the material that produced it.
type Pair struct {
	Ryde      []byte
	Sig       []byte
	XML       []byte
	Plaintext []byte
}

// BuildPair renders, wraps, encrypts to recipient and signs with signer.
func BuildPair(t testing.TB, o DepositOpts, recipient, signer KeyPair) Pair {
	t.Helper()
	xml := BuildXML(o)
	plain := BuildPayload(t, o, xml)
	ryde := Encrypt(t, plain, recipient)
	return Pair{Ryde: ryde, Sig: Sign(t, ryde, signer, false), XML: xml, Plaintext: plain}
}

// Tamper flips one bit at position at (modulo len) in a copy of b.
func Tamper(b []byte, at int) []byte {
	out := append([]byte(nil), b...)
	if len(out) == 0 {
		return out
	}
	i := at % len(out)
	if i < 0 {
		i += len(out)
	}
	out[i] ^= 0x01
	return out
}
