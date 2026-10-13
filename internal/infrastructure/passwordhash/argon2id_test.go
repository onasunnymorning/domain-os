package passwordhash

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// fast keeps the tests quick; the format and comparison are what is tested.
var fast = Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestHashVerifies(t *testing.T) {
	h := New(fast)
	enc, err := h.Hash("s3cret-pw")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(enc, "$argon2id$v=19$m=64,t=1,p=1$"), enc)

	ok, err := h.Verify(enc, "s3cret-pw")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = h.Verify(enc, "s3cret-pX")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestHashIsSalted(t *testing.T) {
	h := New(fast)
	a, err := h.Hash("same-password")
	require.NoError(t, err)
	b, err := h.Hash("same-password")
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}

func TestVerifyReadsParametersFromTheHash(t *testing.T) {
	enc, err := New(fast).Hash("pw-123456")
	require.NoError(t, err)
	ok, err := New(DefaultParams).Verify(enc, "pw-123456")
	require.NoError(t, err)
	require.True(t, ok, "a hash made with older parameters must still verify")
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := New(fast)
	for _, enc := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=16$m=64,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$!!$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5",
	} {
		_, err := h.Verify(enc, "whatever")
		require.ErrorIs(t, err, ErrMalformedHash, enc)
	}
}
