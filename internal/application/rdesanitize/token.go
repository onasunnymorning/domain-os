package rdesanitize

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"strings"
)

// ErrNoTokenKey is returned when no HMAC key is available. It is fixed text and
// never echoes any input.
var ErrNoTokenKey = errors.New("no sanitization token key is available")

// MinTokenKeyBytes is the shortest master key accepted. Below this the tokens
// stop being a meaningful barrier to re-identification.
const MinTokenKeyBytes = 32

// TokenPurpose names what a set of tokens is for. Tokens are scoped by tenant
// *and* purpose so that a linkage which is legitimate inside one analysis
// cannot be carried into another.
const TokenPurpose = "rde-sanitize"

// b32 is unpadded, upper-case and alphanumeric, which keeps a token inside
// eppcom:clIDType and eppcom:roidType without escaping.
var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Tokenizer produces the deterministic pseudonyms in a derivative.
//
// Determinism is the point: the same value inside one tenant and purpose always
// yields the same token, so a contact shared by a thousand domains stays one
// contact and longitudinal analysis across deposits still works. It is also the
// residual risk, which is why the output is labelled pseudonymized and not
// anonymous.
//
// No mapping table is written anywhere. Re-identification requires the master
// key, which lives in the secrets manager and is never logged, persisted or
// placed in a manifest — only the key's fingerprint is, so that a rotation is
// visible as a break in joinability instead of a silent one.
type Tokenizer struct {
	key         []byte
	fingerprint string
}

// NewTokenizer derives the per-tenant, per-purpose subkey from the master key.
// Deriving rather than storing a key per tenant means rotation is one value and
// cross-tenant tokens still differ, which is what the ticket requires.
func NewTokenizer(master []byte, tenant, policyVersion string) (*Tokenizer, error) {
	if len(master) < MinTokenKeyBytes {
		return nil, ErrNoTokenKey
	}
	if strings.TrimSpace(tenant) == "" || strings.TrimSpace(policyVersion) == "" {
		return nil, errors.New("tenant and policy version are required to derive a token key")
	}
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(tenant))
	mac.Write([]byte{0})
	mac.Write([]byte(TokenPurpose))
	mac.Write([]byte{0})
	mac.Write([]byte(policyVersion))
	sub := mac.Sum(nil)

	// The fingerprint identifies the master key without revealing it: it is a
	// MAC over a fixed public string, so it cannot be inverted.
	fp := hmac.New(sha256.New, master)
	fp.Write([]byte("domain-os/sanitize-key-fingerprint"))
	return &Tokenizer{key: sub, fingerprint: b32.EncodeToString(fp.Sum(nil))[:16]}, nil
}

// KeyFingerprint identifies the master key in a manifest or a run record.
func (t *Tokenizer) KeyFingerprint() string { return t.fingerprint }

// raw returns the MAC of one (kind, value) pair.
func (t *Tokenizer) raw(kind ValueKind, value string) []byte {
	mac := hmac.New(sha256.New, t.key)
	mac.Write([]byte(kind))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

// Value returns the replacement for one classified field. Every shape is chosen
// to stay inside the schema type the original had, so the derivative validates
// as an RDE deposit rather than merely parsing.
func (t *Tokenizer) Value(kind ValueKind, original string) string {
	d := b32.EncodeToString(t.raw(kind, original))
	switch kind {
	case ValContactID:
		// eppcom:clIDType: 3-16 characters.
		return "C" + d[:15]
	case ValContactROID:
		// eppcom:roidType: (\w|_){1,80}-\w{1,8}
		return d[:12] + "-RDE"
	case ValPersonName:
		return "Contact " + d[:12]
	case ValEmail:
		// RFC 2606 reserves .invalid, so a synthetic address can never route.
		return strings.ToLower(d[:16]) + "@example.invalid"
	default:
		return d[:16]
	}
}
