package rdesynth

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ctxCheckEvery is how many domains are written between cancellation checks.
const ctxCheckEvery = 512

// Generate validates p and streams the gzip-compressed deposit to w.
//
// It stops with ctx's error if ctx is cancelled — a client that disconnects
// halfway through a quarter of a million domains should not keep the server
// busy writing them. A partial stream is never a valid deposit: the gzip
// trailer is only written on success.
func Generate(ctx context.Context, w io.Writer, p Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	// BestSpeed: at this volume compression time dominates, and the repetitive
	// XML compresses well even at the lowest level.
	gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("rdesynth: gzip writer: %w", err)
	}
	gz.Name = p.Filename()[:len(p.Filename())-len(".gz")]
	gz.ModTime = p.Watermark

	g := &generator{
		p: p,
		w: bufio.NewWriterSize(gz, 256<<10),
		// #nosec G404 -- synthetic test data must be reproducible from a seed; nothing here is a secret
		rng: rand.New(rand.NewPCG(p.Seed, p.Seed^0x9e3779b97f4a7c15)),
	}
	if err := g.run(ctx); err != nil {
		return err
	}
	if err := g.w.Flush(); err != nil {
		return fmt.Errorf("rdesynth: write deposit: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("rdesynth: close gzip stream: %w", err)
	}
	return nil
}

type generator struct {
	p   Params
	w   *bufio.Writer
	rng *rand.Rand

	nextHost int // ordinal of the last host written, for unique roids and addresses
}

// printf writes to the buffered writer. bufio.Writer keeps its first error and
// fails every later write, so errors are collected once per domain in run
// rather than at every call.
func (g *generator) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(g.w, format, args...)
}

func (g *generator) run(ctx context.Context) error {
	g.preamble()
	for i := 1; i <= registrarCount; i++ {
		g.registrar(i)
	}
	for i := 1; i <= g.p.Domains; i++ {
		if i%ctxCheckEvery == 0 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("rdesynth: generation stopped after %d domains: %w", i-1, err)
			}
			// Flush surfaces a write error (a closed connection) without
			// waiting for the buffer to fill.
			if err := g.w.Flush(); err != nil {
				return fmt.Errorf("rdesynth: write deposit: %w", err)
			}
		}
		g.domain(i)
	}
	for i := 1; i <= g.p.NNDNs; i++ {
		if i%(ctxCheckEvery*8) == 0 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("rdesynth: generation stopped at NNDN %d: %w", i, err)
			}
		}
		g.nndn(i)
	}
	g.printf("  </rde:contents>\n</rde:deposit>\n")
	return nil
}

func (g *generator) preamble() {
	p := g.p
	g.printf(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	g.printf(`<rde:deposit xmlns:rde="%s" xmlns:rdeHeader="%s" xmlns:rdeDomain="%s" xmlns:rdeHost="%s" xmlns:rdeContact="%s" xmlns:rdeRegistrar="%s" xmlns:rdeNNDN="%s" xmlns:domain="urn:ietf:params:xml:ns:domain-1.0" xmlns:contact="urn:ietf:params:xml:ns:contact-1.0" type="FULL" id="%s001">`+"\n",
		entities.RDE_URI, entities.RDE_HEADER_URI, entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.NNDN_URI,
		p.Watermark.Format("20060102"))
	g.printf("  <rde:watermark>%s</rde:watermark>\n", p.Watermark.Format(time.RFC3339))
	g.printf("  <rde:rdeMenu>\n    <rde:version>1.0</rde:version>\n")
	for _, uri := range []string{entities.RDE_HEADER_URI, entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.NNDN_URI} {
		g.printf("    <rde:objURI>%s</rde:objURI>\n", uri)
	}
	g.printf("  </rde:rdeMenu>\n  <rde:contents>\n")

	counts := p.ExpectedCounts()
	g.printf("    <rdeHeader:header>\n      <rdeHeader:tld>%s</rdeHeader:tld>\n", p.TLD)
	for _, uri := range []string{entities.DOMAIN_URI, entities.HOST_URI, entities.CONTACT_URI, entities.REGISTRAR_URI, entities.NNDN_URI} {
		if n, ok := counts[uri]; ok {
			g.printf("      <rdeHeader:count uri=\"%s\">%d</rdeHeader:count>\n", uri, n)
		}
	}
	g.printf("    </rdeHeader:header>\n")
}

func registrarID(i int) string { return fmt.Sprintf("synreg%02d", i) }

func (g *generator) registrar(i int) {
	pl := places[(i-1)%len(places)]
	name := fmt.Sprintf("%s %s Registrar %s", title(adjectives[(i*7)%len(adjectives)]), title(nouns[(i*11)%len(nouns)]), orgSuffixes[i%len(orgSuffixes)])
	g.printf(`    <rdeRegistrar:registrar>
      <rdeRegistrar:id>%[1]s</rdeRegistrar:id>
      <rdeRegistrar:name>%[2]s</rdeRegistrar:name>
      <rdeRegistrar:gurid>%[3]d</rdeRegistrar:gurid>
      <rdeRegistrar:status>ok</rdeRegistrar:status>
      <rdeRegistrar:postalInfo type="int">
        <rdeRegistrar:addr>
          <rdeRegistrar:street>%[4]d %[5]s</rdeRegistrar:street>
          <rdeRegistrar:city>%[6]s</rdeRegistrar:city>
          <rdeRegistrar:pc>%[7]s</rdeRegistrar:pc>
          <rdeRegistrar:cc>%[8]s</rdeRegistrar:cc>
        </rdeRegistrar:addr>
      </rdeRegistrar:postalInfo>
      <rdeRegistrar:voice>+%[9]s.5550%06[3]d</rdeRegistrar:voice>
      <rdeRegistrar:email>support@%[1]s.example.net</rdeRegistrar:email>
      <rdeRegistrar:url>https://www.%[1]s.example.net</rdeRegistrar:url>
      <rdeRegistrar:whoisInfo>
        <rdeRegistrar:name>whois.%[1]s.example.net</rdeRegistrar:name>
        <rdeRegistrar:url>https://whois.%[1]s.example.net</rdeRegistrar:url>
      </rdeRegistrar:whoisInfo>
      <rdeRegistrar:crDate>%[10]s</rdeRegistrar:crDate>
    </rdeRegistrar:registrar>
`, registrarID(i), name, 90000+i, 10+i, streets[i%len(streets)], pl.city, pl.pc, pl.cc, pl.dial,
		g.p.Watermark.AddDate(-12, 0, 0).Format(time.RFC3339))
}

// contactRoles are the links a domain makes, in the order they are filled.
var contactRoles = []string{"registrant", "admin", "tech", "billing"}

func (g *generator) domain(i int) {
	p := g.p
	name := fmt.Sprintf("%s%s-%d.%s", pick(g.rng, adjectives), pick(g.rng, nouns), i, p.TLD)
	clID := registrarID((i-1)%registrarCount + 1)

	// Created at some point in the eight years before the watermark and
	// renewed on its anniversary to the next one or two after it: an active
	// registration.
	crDate := p.Watermark.Add(-time.Duration(g.rng.Int64N(int64(8 * 365 * 24 * time.Hour)))).Truncate(time.Second)
	exDate := crDate.AddDate(1, 0, 0)
	for !exDate.After(p.Watermark) {
		exDate = exDate.AddDate(1, 0, 0)
	}
	exDate = exDate.AddDate(g.rng.IntN(2), 0, 0)
	upDate := crDate.Add(time.Duration(g.rng.Int64N(int64(p.Watermark.Sub(crDate)) + 1))).Truncate(time.Second)

	nHosts := p.hostsFor(i)
	hosts := make([]string, nHosts)
	for j := range hosts {
		hosts[j] = fmt.Sprintf("ns%d.%s", j+1, name)
	}

	contactIDs := make([]string, p.ContactsPerDomain)
	for r := range contactIDs {
		contactIDs[r] = fmt.Sprintf("C%09d", (i-1)*p.ContactsPerDomain+r+1)
	}

	g.printf("    <rdeDomain:domain>\n      <rdeDomain:name>%s</rdeDomain:name>\n      <rdeDomain:roid>%d_DOM-SYN</rdeDomain:roid>\n", name, i)
	for _, s := range domainStatuses(g.rng, nHosts > 0) {
		g.printf("      <rdeDomain:status s=\"%s\"/>\n", s)
	}
	for r, id := range contactIDs {
		if r == 0 {
			g.printf("      <rdeDomain:registrant>%s</rdeDomain:registrant>\n", id)
			continue
		}
		g.printf("      <rdeDomain:contact type=\"%s\">%s</rdeDomain:contact>\n", contactRoles[r], id)
	}
	if nHosts > 0 {
		g.printf("      <rdeDomain:ns>\n")
		for _, h := range hosts {
			g.printf("        <domain:hostObj>%s</domain:hostObj>\n", h)
		}
		g.printf("      </rdeDomain:ns>\n")
	}
	g.printf(`      <rdeDomain:clID>%[1]s</rdeDomain:clID>
      <rdeDomain:crRr>%[1]s</rdeDomain:crRr>
      <rdeDomain:crDate>%[2]s</rdeDomain:crDate>
      <rdeDomain:exDate>%[3]s</rdeDomain:exDate>
      <rdeDomain:upRr>%[1]s</rdeDomain:upRr>
      <rdeDomain:upDate>%[4]s</rdeDomain:upDate>
    </rdeDomain:domain>
`, clID, crDate.Format(time.RFC3339), exDate.Format(time.RFC3339), upDate.Format(time.RFC3339))

	// A domain's contacts and hosts follow it. RFC 9022 does not order the
	// contents, and writing them here is what keeps generation single-pass.
	for _, id := range contactIDs {
		g.contact(id, clID, crDate)
	}
	for _, h := range hosts {
		g.host(h, clID, crDate)
	}
}

// domainStatuses is a realistic spread: most domains plain ok, some locked
// against transfer, a few fully client-locked. A domain with no nameservers is
// inactive instead of ok (RFC 5731 §2.3); ok never combines with another
// status.
func domainStatuses(rng *rand.Rand, delegated bool) []string {
	var st []string
	switch n := rng.IntN(100); {
	case n < 5:
		st = []string{entities.DomainStatusClientDeleteProhibited, entities.DomainStatusClientTransferProhibited, entities.DomainStatusClientUpdateProhibited}
	case n < 20:
		st = []string{entities.DomainStatusClientTransferProhibited}
	}
	if !delegated {
		return append(st, entities.DomainStatusInactive)
	}
	if len(st) == 0 {
		return []string{entities.DomainStatusOK}
	}
	return st
}

func (g *generator) contact(id, clID string, crDate time.Time) {
	first, last := pick(g.rng, firstNames), pick(g.rng, lastNames)
	pl := places[g.rng.IntN(len(places))]
	org := ""
	if g.rng.IntN(3) == 0 {
		org = fmt.Sprintf("        <contact:org>%s %s %s</contact:org>\n", title(pick(g.rng, adjectives)), title(pick(g.rng, nouns)), pick(g.rng, orgSuffixes))
	}
	g.printf(`    <rdeContact:contact>
      <rdeContact:id>%[1]s</rdeContact:id>
      <rdeContact:roid>%[2]s_CONT-SYN</rdeContact:roid>
      <rdeContact:status s="ok"/>
      <rdeContact:postalInfo type="int">
        <contact:name>%[3]s %[4]s</contact:name>
%[5]s        <contact:addr>
          <contact:street>%[6]d %[7]s</contact:street>
          <contact:city>%[8]s</contact:city>
          <contact:pc>%[9]s</contact:pc>
          <contact:cc>%[10]s</contact:cc>
        </contact:addr>
      </rdeContact:postalInfo>
      <rdeContact:voice>+%[11]s.555%07[12]d</rdeContact:voice>
      <rdeContact:email>%[13]s.%[14]s.%[2]s@example.net</rdeContact:email>
      <rdeContact:clID>%[15]s</rdeContact:clID>
      <rdeContact:crRr>%[15]s</rdeContact:crRr>
      <rdeContact:crDate>%[16]s</rdeContact:crDate>
    </rdeContact:contact>
`, id, id[1:], first, last, org, 1+g.rng.IntN(250), pick(g.rng, streets), pl.city, pl.pc, pl.cc, pl.dial, g.rng.IntN(10_000_000),
		lower(first), lower(last), clID, crDate.Format(time.RFC3339))
}

func (g *generator) host(name, clID string, crDate time.Time) {
	g.nextHost++
	n := g.nextHost
	// IPv4 glue cycles through the three RFC 5737 documentation networks;
	// IPv6 glue is unique per host inside 2001:db8::/32 (RFC 3849).
	nets := [...]string{"192.0.2", "198.51.100", "203.0.113"}
	v4 := fmt.Sprintf("%s.%d", nets[(n-1)%3], 1+((n-1)/3)%254)
	v6 := fmt.Sprintf("2001:db8:%x:%x::53", n>>16, n&0xffff)
	g.printf(`    <rdeHost:host>
      <rdeHost:name>%s</rdeHost:name>
      <rdeHost:roid>%d_HOST-SYN</rdeHost:roid>
      <rdeHost:status s="ok"/>
      <rdeHost:status s="linked"/>
      <rdeHost:addr ip="v4">%s</rdeHost:addr>
      <rdeHost:addr ip="v6">%s</rdeHost:addr>
      <rdeHost:clID>%s</rdeHost:clID>
      <rdeHost:crRr>%s</rdeHost:crRr>
      <rdeHost:crDate>%s</rdeHost:crDate>
    </rdeHost:host>
`, name, n, v4, v6, clID, clID, crDate.Format(time.RFC3339))
}

// nndnStates are RFC 9022 §9's two name states.
var nndnStates = [...]string{"blocked", "withheld"}

func (g *generator) nndn(i int) {
	g.printf(`    <rdeNNDN:NNDN>
      <rdeNNDN:aName>%s-reserved-%d.%s</rdeNNDN:aName>
      <rdeNNDN:nameState>%s</rdeNNDN:nameState>
      <rdeNNDN:crDate>%s</rdeNNDN:crDate>
    </rdeNNDN:NNDN>
`, pick(g.rng, nouns), i, g.p.TLD, nndnStates[i%2], g.p.Watermark.AddDate(-5, 0, 0).Format(time.RFC3339))
}

func pick(rng *rand.Rand, from []string) string { return from[rng.IntN(len(from))] }

// title and lower handle the ASCII word lists only.
func title(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
