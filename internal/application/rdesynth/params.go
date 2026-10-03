package rdesynth

import (
	"fmt"
	"math"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The bounds a request is held to. They are a judgement about what one
// synchronous request can reasonably produce, not a property of the RDE
// format.
//
// MaxDomains is the binding one. The validator's cross-reference check holds
// rdevalidate.DefaultMaxCrossReferenceObjects (20M) identifiers and every
// contact and host costs two, so at the largest per-domain settings (4
// contacts, 13 hosts) a deposit past about 590k domains could no longer be
// fully checked, and "valid" would quietly mean "partly validated". The UI also
// holds the whole download in memory before saving it. 250k keeps the worst
// case near 2.7 GB of XML, a few hundred MB compressed.
const (
	MaxDomains           = 250_000
	MaxContactsPerDomain = 4  // registrant, admin, tech, billing
	MaxAvgHostsPerDomain = 13 // the most nameservers any TLD's zone sensibly carries
	MaxNNDNs             = 1_000_000
)

// registrarCount is how many registrars sponsor the generated domains,
// round-robin.
const registrarCount = 5

// Params shapes one synthetic deposit.
type Params struct {
	// TLD the deposit is for. Validate normalises it to lower case.
	TLD string
	// Domains is the number of domain objects.
	Domains int
	// ContactsPerDomain is how many contacts each domain links, each one a
	// distinct contact object: registrant, then admin, tech and billing.
	ContactsPerDomain int
	// AvgHostsPerDomain is the mean number of nameservers a domain delegates
	// to. Each is a subordinate host object with glue. Fractions are spread
	// over the domains so the total is exact: 2.5 gives alternate domains two
	// and three.
	AvgHostsPerDomain float64
	// NNDNs is the number of NNDN (blocked or withheld name) objects.
	NNDNs int
	// Seed makes the output reproducible.
	Seed uint64
	// Watermark is the deposit's point in time. The zero value means now.
	Watermark time.Time
}

// Validate normalises p and rejects values outside the bounds above. The TLD
// is checked by the domain-name constructor rather than here.
func (p *Params) Validate() error {
	tld, err := entities.NewDomainName(p.TLD)
	if err != nil {
		return fmt.Errorf("tld %q is not a valid domain name: %w", p.TLD, err)
	}
	p.TLD = tld.String()
	if p.Domains < 0 || p.Domains > MaxDomains {
		return fmt.Errorf("domains must be between 0 and %d", MaxDomains)
	}
	if p.ContactsPerDomain < 0 || p.ContactsPerDomain > MaxContactsPerDomain {
		return fmt.Errorf("contactsPerDomain must be between 0 and %d", MaxContactsPerDomain)
	}
	if math.IsNaN(p.AvgHostsPerDomain) || p.AvgHostsPerDomain < 0 || p.AvgHostsPerDomain > MaxAvgHostsPerDomain {
		return fmt.Errorf("avgHostsPerDomain must be between 0 and %d", MaxAvgHostsPerDomain)
	}
	if p.NNDNs < 0 || p.NNDNs > MaxNNDNs {
		return fmt.Errorf("nndns must be between 0 and %d", MaxNNDNs)
	}
	if p.Watermark.IsZero() {
		p.Watermark = time.Now()
	}
	p.Watermark = p.Watermark.UTC().Truncate(time.Second)
	return nil
}

// hostsFor is the number of nameservers domain i (1-based) delegates to. The
// difference of two floors spreads the fractional part evenly, and the sum
// over 1..n telescopes to floor(n·avg), which is what TotalHosts reports.
func (p Params) hostsFor(i int) int {
	return int(math.Floor(float64(i)*p.AvgHostsPerDomain)) - int(math.Floor(float64(i-1)*p.AvgHostsPerDomain))
}

// TotalHosts is the number of host objects the deposit carries.
func (p Params) TotalHosts() int {
	return int(math.Floor(float64(p.Domains) * p.AvgHostsPerDomain))
}

// ExpectedCounts is the object count per RDE namespace the deposit will
// declare in its header and carry. A namespace with no objects is absent.
func (p Params) ExpectedCounts() map[string]int {
	counts := map[string]int{}
	for uri, n := range map[string]int{
		entities.DOMAIN_URI:    p.Domains,
		entities.CONTACT_URI:   p.Domains * p.ContactsPerDomain,
		entities.HOST_URI:      p.TotalHosts(),
		entities.REGISTRAR_URI: registrarCount,
		entities.NNDN_URI:      p.NNDNs,
	} {
		if n > 0 {
			counts[uri] = n
		}
	}
	return counts
}

// Filename follows ICANN's deposit naming ({tld}_{date}_full_S1_R0) with the
// extension of what is actually produced: XML, gzipped, neither encrypted nor
// signed — so not .ryde.
func (p Params) Filename() string {
	return fmt.Sprintf("%s_%s_full_S1_R0.xml.gz", p.TLD, p.Watermark.UTC().Format("2006-01-02"))
}
