package entities

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// DNSSEC delegation signer records (RFC 4034 §5) on a domain, as RFC 5910's
// dsData carries them.

var (
	// ErrInvalidDSRecord is a DS record that is not well formed.
	ErrInvalidDSRecord = errors.New("invalid DS record")
	// ErrDuplicateDSRecord is the same DS record twice on one domain.
	ErrDuplicateDSRecord = errors.New("duplicate DS record")
)

// dsDigestLengths are the digest lengths, in octets, of the DS digest types
// IANA has assigned (RFC 4034, 4509, 5933, 6605).
var dsDigestLengths = map[uint8]int{
	1: 20, // SHA-1
	2: 32, // SHA-256
	3: 32, // GOST R 34.11-94
	4: 48, // SHA-384
}

// DSRecord is one delegation signer record. Digest is upper-case hex.
type DSRecord struct {
	KeyTag     uint16 `json:"KeyTag"`
	Algorithm  uint8  `json:"Algorithm"`
	DigestType uint8  `json:"DigestType"`
	Digest     string `json:"Digest"`
}

// NewDSRecord returns a validated DS record, with the digest normalised to
// upper-case hex.
func NewDSRecord(keyTag uint16, algorithm, digestType uint8, digest string) (DSRecord, error) {
	ds := DSRecord{KeyTag: keyTag, Algorithm: algorithm, DigestType: digestType, Digest: strings.ToUpper(strings.TrimSpace(digest))}
	if err := ds.Validate(); err != nil {
		return DSRecord{}, err
	}
	return ds, nil
}

// Validate checks that the record is well formed: an algorithm and digest
// type other than the reserved 0, and a hex digest whose length matches its
// digest type where that type is assigned. Which algorithms and digest
// types a registry accepts from registrars is policy, not validity: a
// deposit from another registry keeps what it holds.
func (d DSRecord) Validate() error {
	if d.Algorithm == 0 {
		return fmt.Errorf("%w: algorithm 0 is reserved", ErrInvalidDSRecord)
	}
	if d.DigestType == 0 {
		return fmt.Errorf("%w: digest type 0 is reserved", ErrInvalidDSRecord)
	}
	raw, err := hex.DecodeString(d.Digest)
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("%w: the digest is not hex", ErrInvalidDSRecord)
	}
	if want, ok := dsDigestLengths[d.DigestType]; ok && len(raw) != want {
		return fmt.Errorf("%w: a digest of type %d is %d octets, not %d", ErrInvalidDSRecord, d.DigestType, want, len(raw))
	}
	return nil
}

// String is the record's DNS presentation (RDATA) form.
func (d DSRecord) String() string {
	return fmt.Sprintf("%d %d %d %s", d.KeyTag, d.Algorithm, d.DigestType, d.Digest)
}

// SetDSRecords replaces the domain's DS records, after validating each and
// refusing duplicates. Nil or empty removes them all.
func (d *Domain) SetDSRecords(records []DSRecord) error {
	seen := map[DSRecord]bool{}
	out := make([]DSRecord, 0, len(records))
	for _, r := range records {
		r.Digest = strings.ToUpper(r.Digest)
		if err := r.Validate(); err != nil {
			return err
		}
		if seen[r] {
			return fmt.Errorf("%w: %s", ErrDuplicateDSRecord, r)
		}
		seen[r] = true
		out = append(out, r)
	}
	if len(out) == 0 {
		out = nil
	}
	d.DSRecords = out
	return nil
}
