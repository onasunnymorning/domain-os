package rdesanitize

import (
	"errors"
	"strings"

	"golang.org/x/net/idna"
)

// ErrInvalidSuffixRewrite is returned when the configured pair cannot define a
// rewrite.
var ErrInvalidSuffixRewrite = errors.New("invalid suffix rewrite")

// SuffixRewriter replaces the registry suffix in a fully-qualified name while
// preserving every label above it, so `example-1.suffix` becomes
// `example-1.artful-dodger` and the relationships between domains, hosts,
// nameserver references and glue stay internally consistent.
//
// Out-of-bailiwick names — a nameserver under someone else's TLD — are left
// alone. Rewriting them would invent a name the registry never delegated and
// would break the very relationships this is meant to preserve.
//
// Both the A-label and U-label forms of the source suffix are matched, because
// a deposit carries a domain twice: `name` in punycode and `uName` in Unicode.
// Rewriting only one of them would desynchronise the pair.
type SuffixRewriter struct {
	fromA, fromU string
	to           string
}

// NewSuffixRewriter builds a rewriter from the source TLD and the configured
// synthetic suffix.
func NewSuffixRewriter(sourceTLD, synthetic string) (*SuffixRewriter, error) {
	from := normalizeName(sourceTLD)
	to := normalizeName(synthetic)
	if from == "" || to == "" {
		return nil, errors.Join(ErrInvalidSuffixRewrite, errors.New("both the source TLD and the synthetic suffix are required"))
	}
	if from == to {
		return nil, errors.Join(ErrInvalidSuffixRewrite, errors.New("the synthetic suffix must differ from the source TLD"))
	}
	s := &SuffixRewriter{fromA: from, fromU: from, to: to}
	// idna conversions are best-effort here: an ASCII TLD converts to itself,
	// and a suffix we cannot convert simply has one fewer form to match.
	if u, err := idna.ToUnicode(from); err == nil && u != "" {
		s.fromU = u
	}
	if a, err := idna.ToASCII(from); err == nil && a != "" {
		s.fromA = a
	}
	return s, nil
}

// To returns the synthetic suffix.
func (s *SuffixRewriter) To() string { return s.to }

// Rewrite returns the rewritten name and whether it changed. ok is false when
// the value is in-bailiwick but cannot be rewritten into a well-formed name,
// which is a fail-closed condition rather than something to paper over.
func (s *SuffixRewriter) Rewrite(value string) (out string, changed, ok bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value, false, true
	}
	trailingDot := strings.HasSuffix(trimmed, ".")
	name := strings.TrimSuffix(trimmed, ".")
	lower := strings.ToLower(name)

	for _, from := range []string{s.fromA, s.fromU} {
		if from == "" {
			continue
		}
		if lower == from {
			return s.withDot(s.to, trailingDot), true, true
		}
		if strings.HasSuffix(lower, "."+from) {
			prefix := name[:len(name)-len(from)-1]
			if prefix == "" || strings.HasSuffix(prefix, ".") {
				// ".suffix" or "a..suffix": not a name we can rewrite honestly.
				return value, false, false
			}
			return s.withDot(prefix+"."+s.to, trailingDot), true, true
		}
	}
	return value, false, true
}

func (s *SuffixRewriter) withDot(name string, trailingDot bool) string {
	if trailingDot {
		return name + "."
	}
	return name
}

// CarriesSuffix reports whether a value is still a name inside the source
// registry: it equals the source suffix or ends in a label boundary before it.
// This is the regression check the derivative scan uses.
//
// It deliberately does not look for the suffix as a bare substring or as any
// label. A registry called "example" coexists with registrar URLs under
// example.com and out-of-bailiwick nameservers under example.net, and neither
// is a name the registry delegated; flagging them would make the check useless
// noise rather than a signal.
func (s *SuffixRewriter) CarriesSuffix(value string) bool {
	v := normalizeName(value)
	if v == "" {
		return false
	}
	for _, from := range []string{s.fromA, s.fromU} {
		if from == "" {
			continue
		}
		if v == from || strings.HasSuffix(v, "."+from) {
			return true
		}
	}
	return false
}

func normalizeName(s string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(s), "."))
}
