package rdesynth

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

var watermark = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

func generate(t *testing.T, p Params) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Generate(context.Background(), &buf, p); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return buf.Bytes()
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return out
}

// TestGenerateIsValid is the property the generator exists for: whatever the
// shape asked for, this repository's own validator finds nothing wrong with
// the result, and the deposit carries exactly the objects it declares.
func TestGenerateIsValid(t *testing.T) {
	cases := []Params{
		{TLD: "test"},
		{TLD: "test", Domains: 1},
		{TLD: "test", Domains: 1, ContactsPerDomain: 4, AvgHostsPerDomain: 13},
		{TLD: "test", Domains: 40, ContactsPerDomain: 0, AvgHostsPerDomain: 0},
		{TLD: "test", Domains: 40, ContactsPerDomain: 1, AvgHostsPerDomain: 0.5},
		{TLD: "test", Domains: 40, ContactsPerDomain: 2, AvgHostsPerDomain: 2.3, NNDNs: 7},
		{TLD: "test", Domains: 40, ContactsPerDomain: 3, AvgHostsPerDomain: 1},
		{TLD: "TEST", Domains: 600, ContactsPerDomain: 4, AvgHostsPerDomain: 2.5, NNDNs: 100},
		{TLD: "co.test", Domains: 25, ContactsPerDomain: 2, AvgHostsPerDomain: 2},
		{TLD: "xn--p1ai", Domains: 25, ContactsPerDomain: 2, AvgHostsPerDomain: 2},
		{TLD: "test", NNDNs: 50},
	}
	for _, p := range cases {
		name := fmt.Sprintf("%s/d%d/c%d/h%g/n%d", p.TLD, p.Domains, p.ContactsPerDomain, p.AvgHostsPerDomain, p.NNDNs)
		t.Run(name, func(t *testing.T) {
			p.Seed, p.Watermark = 42, watermark
			xml := gunzip(t, generate(t, p))

			want := p
			if err := want.Validate(); err != nil {
				t.Fatal(err)
			}
			v := &rdevalidate.XMLValidator{BoundTLD: want.TLD}
			summary, findings := v.Validate(context.Background(), bytes.NewReader(xml))
			for _, f := range findings {
				t.Errorf("finding %s %s %s %s: %s (%s)", f.Code, f.Severity, f.ObjectType, f.Object, f.Message, f.Rule)
			}
			if summary.Kind != "FULL" || !summary.Watermark.Equal(watermark) {
				t.Errorf("kind %q watermark %v", summary.Kind, summary.Watermark)
			}

			expected := want.ExpectedCounts()
			for _, uri := range []string{entities.DOMAIN_URI, entities.CONTACT_URI, entities.HOST_URI, entities.REGISTRAR_URI, entities.NNDN_URI} {
				if got := summary.Observed[uri]; got != expected[uri] {
					t.Errorf("%s: observed %d, expected %d", uri, got, expected[uri])
				}
			}
			if got, want := summary.Observed[entities.HOST_URI], int(float64(p.Domains)*p.AvgHostsPerDomain); got != want {
				t.Errorf("hosts: %d, want domains × avg = %d", got, want)
			}
		})
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	p := Params{TLD: "test", Domains: 50, ContactsPerDomain: 2, AvgHostsPerDomain: 1.5, NNDNs: 5, Seed: 7, Watermark: watermark}
	a, b := generate(t, p), generate(t, p)
	if !bytes.Equal(a, b) {
		t.Fatal("same params and seed produced different output")
	}
	p.Seed = 8
	if bytes.Equal(gunzip(t, a), gunzip(t, generate(t, p))) {
		t.Fatal("different seeds produced identical deposits")
	}
}

func TestGenerateGzipHeader(t *testing.T) {
	p := Params{TLD: "test", Domains: 1, Watermark: watermark}
	zr, err := gzip.NewReader(bytes.NewReader(generate(t, p)))
	if err != nil {
		t.Fatal(err)
	}
	if zr.Name != "test_2026-10-02_full_S1_R0.xml" {
		t.Errorf("gzip name %q", zr.Name)
	}
}

func TestGenerateStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Generate(ctx, io.Discard, Params{TLD: "test", Domains: MaxDomains, Watermark: watermark})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestParamsValidate(t *testing.T) {
	bad := []struct {
		name string
		p    Params
		msg  string
	}{
		{"empty tld", Params{}, "tld"},
		{"invalid tld", Params{TLD: "-bad-"}, "tld"},
		{"negative domains", Params{TLD: "test", Domains: -1}, "domains"},
		{"too many domains", Params{TLD: "test", Domains: MaxDomains + 1}, "domains"},
		{"too many contacts", Params{TLD: "test", ContactsPerDomain: 5}, "contactsPerDomain"},
		{"negative contacts", Params{TLD: "test", ContactsPerDomain: -1}, "contactsPerDomain"},
		{"too many hosts", Params{TLD: "test", AvgHostsPerDomain: 13.5}, "avgHostsPerDomain"},
		{"negative hosts", Params{TLD: "test", AvgHostsPerDomain: -0.1}, "avgHostsPerDomain"},
		{"too many nndns", Params{TLD: "test", NNDNs: MaxNNDNs + 1}, "nndns"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			err := c.p.Validate()
			if err == nil || !strings.Contains(err.Error(), c.msg) {
				t.Fatalf("want error mentioning %q, got %v", c.msg, err)
			}
		})
	}

	p := Params{TLD: " Example. ", Domains: MaxDomains, ContactsPerDomain: 4, AvgHostsPerDomain: 13, NNDNs: MaxNNDNs}
	if err := p.Validate(); err != nil {
		t.Fatalf("upper bounds rejected: %v", err)
	}
	if p.TLD != "example" || p.Watermark.IsZero() {
		t.Errorf("not normalised: tld %q watermark %v", p.TLD, p.Watermark)
	}
}

func TestHostsSpreadExactly(t *testing.T) {
	p := Params{Domains: 1000, AvgHostsPerDomain: 2.3}
	sum, lo, hi := 0, 99, 0
	for i := 1; i <= p.Domains; i++ {
		n := p.hostsFor(i)
		sum += n
		lo, hi = min(lo, n), max(hi, n)
	}
	if sum != p.TotalHosts() || sum != 2300 {
		t.Errorf("sum %d, TotalHosts %d", sum, p.TotalHosts())
	}
	if lo != 2 || hi != 3 {
		t.Errorf("per-domain hosts ranged %d..%d, want 2..3", lo, hi)
	}
}

// TestValidatorSeesDefects guards TestGenerateIsValid against passing
// vacuously: the same validator, given a generated deposit with one contact
// cut out, has to object to it.
func TestValidatorSeesDefects(t *testing.T) {
	p := Params{TLD: "test", Domains: 3, ContactsPerDomain: 1, AvgHostsPerDomain: 1, Seed: 1, Watermark: watermark}
	xml := string(gunzip(t, generate(t, p)))
	start := strings.Index(xml, "    <rdeContact:contact>")
	end := strings.Index(xml, "</rdeContact:contact>\n") + len("</rdeContact:contact>\n")
	if start < 0 || end <= start {
		t.Fatal("no contact in the generated deposit")
	}
	broken := xml[:start] + xml[end:]

	v := &rdevalidate.XMLValidator{BoundTLD: "test"}
	_, findings := v.Validate(context.Background(), strings.NewReader(broken))
	var codes []rdevalidate.Code
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	if len(codes) == 0 {
		t.Fatal("validator accepted a deposit with a contact removed")
	}
	t.Logf("findings on the broken deposit: %v", codes)
}
