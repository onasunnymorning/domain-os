package entities

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TMCH data (ADR 0024 §4, roadmap 9a): the three data sets the Trademark
// Clearinghouse publishes to registries, held platform-wide and refreshed on
// a schedule. None of them belongs to a TLD or a phase.
//
//   - The TMCH CA certificates: the certificate chain every signed mark
//     (SMD, RFC 7848) is verified against. Sunrise (9b) reads them.
//   - The SMD revocation list (SMDRL): the signed marks the Clearinghouse
//     has revoked, by SMD ID. A revoked mark is refused in sunrise (9b).
//   - The DNL list: the domain name labels that match a trademark record,
//     each with the lookup key a registrar needs to fetch the claims
//     notice. Claims (9c) answers <launch:check type="claims"> from it.
//
// The lists are replaced whole: the Clearinghouse publishes a complete list
// with its generation time, never a delta. The entities carry that time and
// the time this registry fetched the list, which is what freshness is
// measured on.

// TMCH data set names, as the status report, the metrics and the outbox
// events name them.
const (
	TMCHSetCertificates = "certificates"
	TMCHSetSMDRL        = "smdrl"
	TMCHSetDNL          = "dnl"
)

// Outbox events for TMCH data refreshes. Each carries the new set's
// TMCHSetStatus as its data and is written in the transaction that replaces
// the set (INV-01). A fetch that brings the generation already stored
// writes nothing and emits nothing.
const (
	EventTMCHCertificatesRefreshed = "tmch.certificates.refreshed"
	EventTMCHSMDRLRefreshed        = "tmch.smdrl.refreshed"
	EventTMCHDNLRefreshed          = "tmch.dnl.refreshed"
)

var (
	ErrTMCHNoCertificate       = errors.New("no certificate in the TMCH CA bundle")
	ErrTMCHCertificateInvalid  = errors.New("invalid certificate in the TMCH CA bundle")
	ErrTMCHCertificateNotCA    = errors.New("a certificate in the TMCH CA bundle is not a CA certificate")
	ErrTMCHCertificatesExpired = errors.New("every certificate in the TMCH CA bundle has expired")
	ErrTMCHListVersion         = errors.New("unsupported TMCH list version")
	ErrTMCHListGenerated       = errors.New("the TMCH list has no generation time")
	ErrTMCHListStale           = errors.New("the TMCH set does not supersede the one stored")
	ErrTMCHEntryInvalid        = errors.New("invalid TMCH list entry")
	ErrTMCHEntryDuplicate      = errors.New("duplicate TMCH list entry")
	ErrTMCHEntriesRejected     = errors.New("too many TMCH list entries rejected")
	ErrTMCHSetUnknown          = errors.New("unknown TMCH data set")
	ErrTMCHDataMissing         = errors.New("TMCH data has not been fetched yet")
	ErrTMCHNotConfigured       = errors.New("no TMCH feed is configured for this installation")
	// Refusals of what the Clearinghouse's feed returned (the parsers and the
	// fetcher in internal/infrastructure/tmch wrap these, INV-09).
	ErrTMCHFetch         = errors.New("TMCH fetch failed")
	ErrTMCHResponseLarge = errors.New("TMCH response larger than the configured limit")
	ErrTMCHListEmpty     = errors.New("empty TMCH list file")
	ErrTMCHListHeader    = errors.New("malformed TMCH list header")
	ErrTMCHListRecord    = errors.New("malformed TMCH list record")
)

// TMCHListVersion is the only file format version the Clearinghouse has
// published for the SMDRL and the DNL list (the first field of each file's
// first line).
const TMCHListVersion = 1

// TMCHCertificate is one certificate of the TMCH CA bundle, with the
// identity and validity read from it. PEM holds the certificate as fetched.
type TMCHCertificate struct {
	Subject      string
	SerialNumber string
	NotBefore    time.Time
	NotAfter     time.Time
	PEM          string
}

// TMCHCertificates is the TMCH CA bundle as fetched at FetchedAt from
// Source. Its "generation" is the latest NotBefore in the bundle: ICANN
// publishes a new bundle when it issues a new CA certificate.
type TMCHCertificates struct {
	Certificates []TMCHCertificate
	FetchedAt    time.Time
	Source       string
}

// NewTMCHCertificates parses a PEM bundle into the certificates it holds.
// Every block must be a CA certificate (the bundle is what signed marks are
// verified against) and at least one must be valid at now; an expired
// certificate beside a valid one is kept, since a mark signed under it may
// still have to be checked for revocation.
func NewTMCHCertificates(bundle []byte, source string, now time.Time) (*TMCHCertificates, error) {
	bundleCerts := &TMCHCertificates{FetchedAt: now.UTC(), Source: source}
	rest := bundle
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTMCHCertificateInvalid, err)
		}
		if !cert.IsCA {
			return nil, fmt.Errorf("%w: %s", ErrTMCHCertificateNotCA, cert.Subject.String())
		}
		bundleCerts.Certificates = append(bundleCerts.Certificates, TMCHCertificate{
			Subject:      cert.Subject.String(),
			SerialNumber: cert.SerialNumber.Text(16),
			NotBefore:    cert.NotBefore.UTC(),
			NotAfter:     cert.NotAfter.UTC(),
			PEM:          string(pem.EncodeToMemory(block)),
		})
	}
	if len(bundleCerts.Certificates) == 0 {
		return nil, ErrTMCHNoCertificate
	}
	if !bundleCerts.ValidAt(now) {
		return nil, ErrTMCHCertificatesExpired
	}
	return bundleCerts, nil
}

// ValidAt reports whether at least one certificate of the bundle is within
// its validity at now.
func (c *TMCHCertificates) ValidAt(now time.Time) bool {
	for _, cert := range c.Certificates {
		if !now.Before(cert.NotBefore) && now.Before(cert.NotAfter) {
			return true
		}
	}
	return false
}

// GeneratedAt is the bundle's generation time: the latest NotBefore in it.
func (c *TMCHCertificates) GeneratedAt() time.Time {
	var latest time.Time
	for _, cert := range c.Certificates {
		if cert.NotBefore.After(latest) {
			latest = cert.NotBefore
		}
	}
	return latest
}

// ExpiresAt is when the last certificate of the bundle expires.
func (c *TMCHCertificates) ExpiresAt() time.Time {
	var latest time.Time
	for _, cert := range c.Certificates {
		if cert.NotAfter.After(latest) {
			latest = cert.NotAfter
		}
	}
	return latest
}

// Fingerprint identifies the bundle's contents: the hash of its
// certificates' serial numbers, in order. A republished bundle that only
// drops a certificate has no later NotBefore, so the fingerprint is what
// tells it apart from the one stored.
func (c *TMCHCertificates) Fingerprint() string {
	h := sha256.New()
	for _, cert := range c.Certificates {
		h.Write([]byte(cert.SerialNumber))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Status describes the bundle for the freshness report.
func (c *TMCHCertificates) Status() TMCHSetStatus {
	expires := c.ExpiresAt()
	return TMCHSetStatus{Set: TMCHSetCertificates, GeneratedAt: c.GeneratedAt(), FetchedAt: c.FetchedAt, Source: c.Source, Entries: len(c.Certificates), ExpiresAt: &expires, Fingerprint: c.Fingerprint()}
}

// SMDRevocation is one revoked signed mark: the SMD ID (the <smd:id> of the
// signed mark data) and when the Clearinghouse revoked it.
type SMDRevocation struct {
	SMDID     string
	RevokedAt time.Time
}

// SMDRevocationList is the SMD revocation list as generated by the
// Clearinghouse at GeneratedAt and fetched at FetchedAt from Source.
// Rejected counts the rows of the file that were not entries (no SMD ID, no
// time, or an ID already listed) and were left out.
type SMDRevocationList struct {
	Version     int
	GeneratedAt time.Time
	Entries     []SMDRevocation
	Rejected    int
	FetchedAt   time.Time
	Source      string
}

// NewSMDRevocationList builds a list from its header fields and entries,
// checking the version and the generation time. A row that is not an entry
// is left out and counted, never fatal: one bad row must not keep a day's
// revocations from being stored. A file where more rows are rejected than
// kept is refused whole (ErrTMCHEntriesRejected): that is not a list.
// Entries are kept in SMD ID order.
func NewSMDRevocationList(version int, generatedAt time.Time, entries []SMDRevocation, source string, now time.Time) (*SMDRevocationList, error) {
	if err := validateTMCHListHeader(version, generatedAt); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(entries))
	sorted := make([]SMDRevocation, 0, len(entries))
	rejected := 0
	for _, e := range entries {
		e.SMDID = strings.TrimSpace(e.SMDID)
		if e.SMDID == "" || e.RevokedAt.IsZero() {
			rejected++
			continue
		}
		if _, dup := seen[e.SMDID]; dup {
			rejected++
			continue
		}
		seen[e.SMDID] = struct{}{}
		e.RevokedAt = e.RevokedAt.UTC()
		sorted = append(sorted, e)
	}
	if err := tooManyRejected(rejected, len(sorted)); err != nil {
		return nil, err
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].SMDID < sorted[j].SMDID })
	return &SMDRevocationList{Version: version, GeneratedAt: generatedAt.UTC(), Entries: sorted, Rejected: rejected, FetchedAt: now.UTC(), Source: source}, nil
}

// Revoked reports whether the signed mark with this ID is on the list.
func (l *SMDRevocationList) Revoked(smdID string) bool {
	smdID = strings.TrimSpace(smdID)
	i := sort.Search(len(l.Entries), func(i int) bool { return l.Entries[i].SMDID >= smdID })
	return i < len(l.Entries) && l.Entries[i].SMDID == smdID
}

// Status describes the list for the freshness report.
func (l *SMDRevocationList) Status() TMCHSetStatus {
	return TMCHSetStatus{Set: TMCHSetSMDRL, GeneratedAt: l.GeneratedAt, FetchedAt: l.FetchedAt, Source: l.Source, Entries: len(l.Entries), Rejected: l.Rejected}
}

// DNLEntry is one label of the DNL list: a domain name label that matches
// at least one trademark record, the lookup key a registrar presents to the
// Clearinghouse for the claims notice, and when the label was listed.
type DNLEntry struct {
	Label      string
	LookupKey  string
	InsertedAt time.Time
}

// DNLList is the DNL (domain name label) list as generated by the
// Clearinghouse at GeneratedAt and fetched at FetchedAt from Source.
// Rejected counts the rows that were not entries (a label that is not a
// valid domain label, no key, no time, or a label already listed) and were
// left out: a label this registry could never register loses nothing by
// being left off the list.
type DNLList struct {
	Version     int
	GeneratedAt time.Time
	Entries     []DNLEntry
	Rejected    int
	FetchedAt   time.Time
	Source      string
}

// NewDNLList builds a list from its header fields and entries, checking the
// version and the generation time. Labels are lower-cased; a row that is
// not an entry is left out and counted, and a file where more rows are
// rejected than kept is refused whole (ErrTMCHEntriesRejected). Entries
// are kept in label order.
func NewDNLList(version int, generatedAt time.Time, entries []DNLEntry, source string, now time.Time) (*DNLList, error) {
	if err := validateTMCHListHeader(version, generatedAt); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(entries))
	sorted := make([]DNLEntry, 0, len(entries))
	rejected := 0
	for _, e := range entries {
		label := Label(strings.ToLower(strings.TrimSpace(e.Label)))
		if err := label.Validate(); err != nil || e.LookupKey == "" || e.InsertedAt.IsZero() {
			rejected++
			continue
		}
		e.Label = label.String()
		if _, dup := seen[e.Label]; dup {
			rejected++
			continue
		}
		seen[e.Label] = struct{}{}
		e.InsertedAt = e.InsertedAt.UTC()
		sorted = append(sorted, e)
	}
	if err := tooManyRejected(rejected, len(sorted)); err != nil {
		return nil, err
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Label < sorted[j].Label })
	return &DNLList{Version: version, GeneratedAt: generatedAt.UTC(), Entries: sorted, Rejected: rejected, FetchedAt: now.UTC(), Source: source}, nil
}

// Lookup returns the list's entry for a label, nil if the label is not
// listed. The label is compared lower-cased, as the Clearinghouse lists it.
func (l *DNLList) Lookup(label string) *DNLEntry {
	label = strings.ToLower(strings.TrimSpace(label))
	i := sort.Search(len(l.Entries), func(i int) bool { return l.Entries[i].Label >= label })
	if i < len(l.Entries) && l.Entries[i].Label == label {
		return &l.Entries[i]
	}
	return nil
}

// Status describes the list for the freshness report.
func (l *DNLList) Status() TMCHSetStatus {
	return TMCHSetStatus{Set: TMCHSetDNL, GeneratedAt: l.GeneratedAt, FetchedAt: l.FetchedAt, Source: l.Source, Entries: len(l.Entries), Rejected: l.Rejected}
}

// tooManyRejected refuses a list that lost more rows than it kept.
func tooManyRejected(rejected, kept int) error {
	if rejected > 0 && rejected > kept {
		return fmt.Errorf("%w: %d rejected, %d kept", ErrTMCHEntriesRejected, rejected, kept)
	}
	return nil
}

func validateTMCHListHeader(version int, generatedAt time.Time) error {
	if version != TMCHListVersion {
		return fmt.Errorf("%w: %d", ErrTMCHListVersion, version)
	}
	if generatedAt.IsZero() {
		return ErrTMCHListGenerated
	}
	return nil
}

// TMCHSetStatus is one data set's freshness: when the Clearinghouse
// generated what is stored, when this registry last fetched the set, from
// where, and how many entries it holds. ExpiresAt is set for the
// certificate bundle only. It is the admin API's and the metrics' view, and the data of the
// refresh events.
type TMCHSetStatus struct {
	Set         string    `json:"set"`
	GeneratedAt time.Time `json:"generatedAt"`
	// FetchedAt is when this registry last fetched the set from the
	// Clearinghouse, whether or not that fetch brought a new generation.
	FetchedAt time.Time `json:"fetchedAt"`
	Source    string    `json:"source"`
	Entries   int       `json:"entries"`
	// Rejected counts the rows of the fetched file left out of the stored
	// set (see DNLList and SMDRevocationList).
	Rejected  int        `json:"rejected,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// Fingerprint identifies the contents of a set that has no monotonic
	// generation (the certificate bundle); empty for the lists.
	Fingerprint string `json:"fingerprint,omitempty"`
}

// Supersedes reports whether a fetched set should replace the stored one:
// there is none, the Clearinghouse generated the fetched one later, or
// (for a set with a fingerprint) its contents differ at the same
// generation. The same generation fetched again does not supersede, and an
// older one (a mirror behind the one last read) never replaces a newer one.
func (s TMCHSetStatus) Supersedes(stored *TMCHSetStatus) bool {
	if stored == nil {
		return true
	}
	if s.GeneratedAt.After(stored.GeneratedAt) {
		return true
	}
	return s.GeneratedAt.Equal(stored.GeneratedAt) && s.Fingerprint != "" && s.Fingerprint != stored.Fingerprint
}

// Age is how old the stored data is at now, measured from its generation:
// a list fetched a minute ago that the Clearinghouse generated two days ago
// is two days old.
func (s TMCHSetStatus) Age(now time.Time) time.Duration {
	return now.Sub(s.GeneratedAt)
}

// SinceFetch is how long ago this registry last fetched the set.
func (s TMCHSetStatus) SinceFetch(now time.Time) time.Duration {
	return now.Sub(s.FetchedAt)
}

// TMCHStatus is the freshness of the three sets. A set never fetched is
// nil.
type TMCHStatus struct {
	Certificates *TMCHSetStatus `json:"certificates"`
	SMDRL        *TMCHSetStatus `json:"smdrl"`
	DNL          *TMCHSetStatus `json:"dnl"`
}

// Set returns the status of the named set, ErrTMCHSetUnknown for any other
// name and ErrTMCHDataMissing when the set was never fetched.
func (s TMCHStatus) Set(name string) (*TMCHSetStatus, error) {
	var set *TMCHSetStatus
	switch name {
	case TMCHSetCertificates:
		set = s.Certificates
	case TMCHSetSMDRL:
		set = s.SMDRL
	case TMCHSetDNL:
		set = s.DNL
	default:
		return nil, fmt.Errorf("%w: %s", ErrTMCHSetUnknown, name)
	}
	if set == nil {
		return nil, fmt.Errorf("%w: %s", ErrTMCHDataMissing, name)
	}
	return set, nil
}

// NewTMCHRefreshEvent is the outbox event for a set replaced by a fresher
// generation; source names the process (see DomainEvent.Source).
func NewTMCHRefreshEvent(source string, status TMCHSetStatus) (DomainEvent, error) {
	var eventType string
	switch status.Set {
	case TMCHSetCertificates:
		eventType = EventTMCHCertificatesRefreshed
	case TMCHSetSMDRL:
		eventType = EventTMCHSMDRLRefreshed
	case TMCHSetDNL:
		eventType = EventTMCHDNLRefreshed
	default:
		return DomainEvent{}, fmt.Errorf("%w: %s", ErrTMCHSetUnknown, status.Set)
	}
	description := fmt.Sprintf("TMCH %s refreshed: %d entries, generated %s", status.Set, status.Entries, status.GeneratedAt.Format(time.RFC3339))
	return NewDomainEvent(source, eventType, status.Set, description, status), nil
}
