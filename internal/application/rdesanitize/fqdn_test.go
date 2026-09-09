package rdesanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuffixRewriter(t *testing.T) {
	r, err := NewSuffixRewriter("Example.", "artful-dodger")
	require.NoError(t, err)

	cases := []struct {
		name    string
		in      string
		out     string
		changed bool
		ok      bool
	}{
		{"the bare TLD, as the header carries it", "example", "artful-dodger", true, true},
		{"a domain", "example-1.example", "example-1.artful-dodger", true, true},
		{"a host below a domain", "ns1.example-1.example", "ns1.example-1.artful-dodger", true, true},
		{"case is normalised on the suffix only", "Example-1.EXAMPLE", "Example-1.artful-dodger", true, true},
		{"a trailing dot survives", "example-1.example.", "example-1.artful-dodger.", true, true},
		{"an A-label domain keeps its punycode", "xn--nxasmm1c.example", "xn--nxasmm1c.artful-dodger", true, true},
		{"out of bailiwick is left alone", "ns1.outside.example.net", "ns1.outside.example.net", false, true},
		{"a similar-looking TLD is not ours", "example-1.examples", "example-1.examples", false, true},
		{"the suffix as a middle label is not a suffix", "example.co.uk", "example.co.uk", false, true},
		{"empty is not a name", "", "", false, true},
		{"a name with no label above the suffix", ".example", ".example", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, ok := r.Rewrite(tc.in)
			assert.Equal(t, tc.out, out)
			assert.Equal(t, tc.changed, changed)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

func TestSuffixRewriter_MatchesBothIDNForms(t *testing.T) {
	// An IDN TLD arrives as an A-label in `name` and a U-label in `uName`.
	// Rewriting one but not the other would desynchronise the pair.
	r, err := NewSuffixRewriter("xn--p1ai", "artful-dodger")
	require.NoError(t, err)

	out, changed, ok := r.Rewrite("shop.xn--p1ai")
	assert.True(t, ok)
	assert.True(t, changed)
	assert.Equal(t, "shop.artful-dodger", out)

	out, changed, ok = r.Rewrite("магазин.рф")
	assert.True(t, ok)
	assert.True(t, changed, "the U-label form of the same TLD must rewrite too")
	assert.Equal(t, "магазин.artful-dodger", out)
}

func TestSuffixRewriter_CarriesSuffix(t *testing.T) {
	r, err := NewSuffixRewriter("example", "artful-dodger")
	require.NoError(t, err)

	assert.True(t, r.CarriesSuffix("example"))
	assert.True(t, r.CarriesSuffix("example-1.example"))
	assert.True(t, r.CarriesSuffix("EXAMPLE-1.Example."))
	// A registrar URL under example.com and an out-of-bailiwick nameserver
	// under example.net are not names this registry delegated. Flagging them
	// would drown the real signal.
	assert.False(t, r.CarriesSuffix("https://registrar1.example.com"))
	assert.False(t, r.CarriesSuffix("ns1.outside.example.net"))
	assert.False(t, r.CarriesSuffix("artful-dodger"))
	assert.False(t, r.CarriesSuffix(""))
}

func TestNewSuffixRewriter_Rejects(t *testing.T) {
	for _, tc := range []struct{ name, from, to string }{
		{"no source", "", "artful-dodger"},
		{"no target", "example", ""},
		{"same suffix", "example", "Example."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSuffixRewriter(tc.from, tc.to)
			assert.ErrorIs(t, err, ErrInvalidSuffixRewrite)
		})
	}
}
