package entities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sha256Digest = "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"

func TestNewDSRecord(t *testing.T) {
	ds, err := NewDSRecord(12345, 13, 2, " "+strings.ToLower(sha256Digest)+" ")
	require.NoError(t, err)
	assert.Equal(t, DSRecord{KeyTag: 12345, Algorithm: 13, DigestType: 2, Digest: sha256Digest}, ds, "the digest is normalised to upper-case hex")
	assert.Equal(t, "12345 13 2 "+sha256Digest, ds.String())

	for name, tc := range map[string]struct {
		alg, digestType uint8
		digest          string
	}{
		"reserved algorithm":           {0, 2, sha256Digest},
		"reserved digest type":         {13, 0, sha256Digest},
		"not hex":                      {13, 2, strings.Repeat("ZZ", 32)},
		"odd length":                   {13, 2, sha256Digest[:63]},
		"empty":                        {13, 2, ""},
		"SHA-256 digest of SHA-1 size": {13, 2, sha256Digest[:40]},
		"SHA-384 digest of SHA-256":    {13, 4, sha256Digest},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewDSRecord(1, tc.alg, tc.digestType, tc.digest)
			require.ErrorIs(t, err, ErrInvalidDSRecord)
		})
	}

	t.Run("an unassigned digest type is well formed if it is hex", func(t *testing.T) {
		// Whether to accept it is policy; a deposit may hold one.
		_, err := NewDSRecord(1, 13, 200, "ABCD")
		require.NoError(t, err)
	})
}

func TestDomainSetDSRecords(t *testing.T) {
	d := &Domain{}
	a, _ := NewDSRecord(1, 13, 2, sha256Digest)
	b, _ := NewDSRecord(2, 13, 2, sha256Digest)
	require.NoError(t, d.SetDSRecords([]DSRecord{a, b}))
	assert.Equal(t, []DSRecord{a, b}, d.DSRecords)

	lower := a
	lower.Digest = strings.ToLower(lower.Digest)
	require.ErrorIs(t, d.SetDSRecords([]DSRecord{a, lower}), ErrDuplicateDSRecord, "case does not make a digest different")
	assert.Equal(t, []DSRecord{a, b}, d.DSRecords, "a refused set leaves the records as they were")

	require.NoError(t, d.SetDSRecords(nil))
	assert.Nil(t, d.DSRecords)

	dom, err := NewDomain("1_DOM-APEX", "example.com", "registrar1", "Tr4nsf3r-C0de-Xq9#Lm2!")
	require.NoError(t, err)
	dom.DSRecords = []DSRecord{a, a}
	require.ErrorIs(t, dom.Validate(), ErrDuplicateDSRecord, "a domain holding duplicates is invalid however they got there")
	dom.DSRecords = []DSRecord{a, lower}
	require.ErrorIs(t, dom.Validate(), ErrDuplicateDSRecord, "digests that differ only in case are the same record")
}
