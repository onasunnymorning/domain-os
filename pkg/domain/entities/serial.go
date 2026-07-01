package entities

// ---------------------------------------------------------------------------
// RFC 1982 — Serial Number Arithmetic
// https://www.rfc-editor.org/rfc/rfc1982
//
// DNS SOA serial numbers are unsigned 32-bit integers. Comparing them
// as plain integers is WRONG because serials wrap around 2^32. This
// file implements the correct comparison per RFC 1982 §3.2.
// ---------------------------------------------------------------------------

const serialBits = 32

// SerialLessThan returns true iff s1 < s2 per RFC 1982 §3.2.
//
//	s1 < s2  ⟺  (s1 ≠ s2) ∧ ((s2 - s1) mod 2^32) ∈ (0, 2^31)
//
// The undefined case (exactly 2^31 apart) returns false.
func SerialLessThan(s1, s2 uint32) bool {
	if s1 == s2 {
		return false
	}
	diff := s2 - s1 // wraps naturally in uint32
	return diff > 0 && diff < (1<<(serialBits-1))
}

// SerialEqual returns true iff s1 == s2 (trivial, but named for symmetry).
func SerialEqual(s1, s2 uint32) bool {
	return s1 == s2
}

// SerialCompare returns:
//
//	-1  if s1 < s2  (per RFC 1982)
//	 0  if s1 == s2, or the relationship is undefined (exactly 2^31 apart)
//	+1  if s1 > s2  (per RFC 1982)
func SerialCompare(s1, s2 uint32) int {
	if s1 == s2 {
		return 0
	}
	diff := s2 - s1
	switch {
	case diff > 0 && diff < (1 << (serialBits - 1)):
		return -1 // s1 < s2
	case diff > (1 << (serialBits - 1)):
		return 1 // s1 > s2
	default:
		// diff == 2^31: undefined per RFC 1982
		return 0
	}
}
