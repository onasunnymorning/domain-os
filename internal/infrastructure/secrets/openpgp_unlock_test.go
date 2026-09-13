package secrets

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAndUnlockArmoredPrivateKeys covers the one place armored private
// material becomes usable keys — for the key store loader and the import path.
func TestParseAndUnlockArmoredPrivateKeys(t *testing.T) {
	k1 := rdetest.NewKeyPair(t, "eve-2026")
	k2 := rdetest.NewKeyPair(t, "eve-2025")
	const pass = "correct horse battery staple"
	ring := rdetest.EncryptedArmoredPrivate(t, k1, pass) + "\n" + rdetest.EncryptedArmoredPrivate(t, k2, pass)

	t.Run("empty input", func(t *testing.T) {
		_, err := ParseAndUnlockArmoredPrivateKeys("", "")
		assert.Error(t, err)
	})
	t.Run("garbage never echoes the input", func(t *testing.T) {
		_, err := ParseAndUnlockArmoredPrivateKeys("-----BEGIN PGP PRIVATE KEY BLOCK-----\nSECRETMATERIAL\n-----END PGP PRIVATE KEY BLOCK-----", "")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "SECRETMATERIAL")
	})
	t.Run("wrong or missing passphrase", func(t *testing.T) {
		_, err := ParseAndUnlockArmoredPrivateKeys(ring, "nope")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "BEGIN PGP")
		_, err = ParseAndUnlockArmoredPrivateKeys(ring, "")
		assert.Error(t, err)
	})
	t.Run("a concatenated ring decrypts a message addressed to either key", func(t *testing.T) {
		keys, err := ParseAndUnlockArmoredPrivateKeys(ring, pass)
		require.NoError(t, err)
		require.Len(t, keys, 2)
		for _, recipient := range []rdetest.KeyPair{k1, k2} {
			ct := rdetest.Encrypt(t, []byte("payload"), recipient)
			md, info, f := rdevalidate.Decrypt(keys, bytes.NewReader(ct), rdevalidateNow())
			require.Nil(t, f)
			body, err := io.ReadAll(md.UnverifiedBody)
			require.NoError(t, err)
			assert.Equal(t, "payload", string(body))
			assert.Equal(t, recipient.Fingerprint, info.KeyFingerprint)
		}
	})
	t.Run("an unencrypted key needs no passphrase", func(t *testing.T) {
		keys, err := ParseAndUnlockArmoredPrivateKeys(k1.ArmoredPrivate, "")
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	})
	t.Run("public-only material is rejected", func(t *testing.T) {
		_, err := ParseAndUnlockArmoredPrivateKeys(k1.ArmoredPublic, "")
		assert.Error(t, err)
	})
}

func TestOpenPGPHelpers(t *testing.T) {
	k := rdetest.NewKeyPair(t, "eve-helpers")
	pub, err := ArmoredPublicKey(k.Entity)
	require.NoError(t, err)
	parsed, err := rdevalidate.ParseArmoredPublicKey(pub)
	require.NoError(t, err)
	assert.Equal(t, k.Fingerprint, rdevalidate.FingerprintHex(parsed), "the exported public half is the same key")
	assert.True(t, CanEncrypt(k.Entity, time.Now().Add(time.Hour)))
	assert.Nil(t, KeyExpiry(k.Entity), "generated test keys do not expire")
}
