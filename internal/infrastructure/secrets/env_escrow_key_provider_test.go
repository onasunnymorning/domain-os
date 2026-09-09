package secrets

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvEscrowKeyProvider(t *testing.T) {
	k1 := rdetest.NewKeyPair(t, "eve-2026")
	k2 := rdetest.NewKeyPair(t, "eve-2025")
	const pass = "correct horse battery staple"
	ring := rdetest.EncryptedArmoredPrivate(t, k1, pass) + "\n" + rdetest.EncryptedArmoredPrivate(t, k2, pass)

	t.Run("unset is a distinct error", func(t *testing.T) {
		_, err := NewEnvEscrowKeyProvider("", "")
		assert.ErrorIs(t, err, ErrNoEscrowPrivateKeys)
	})
	t.Run("garbage never echoes the input", func(t *testing.T) {
		_, err := NewEnvEscrowKeyProvider("-----BEGIN PGP PRIVATE KEY BLOCK-----\nSECRETMATERIAL\n-----END PGP PRIVATE KEY BLOCK-----", "")
		require.ErrorIs(t, err, ErrEscrowPrivateKeysUnreadable)
		assert.NotContains(t, err.Error(), "SECRETMATERIAL")
	})
	t.Run("wrong passphrase", func(t *testing.T) {
		_, err := NewEnvEscrowKeyProvider(ring, "nope")
		require.ErrorIs(t, err, ErrEscrowPrivateKeysUnreadable)
		assert.NotContains(t, err.Error(), "BEGIN PGP")
	})
	t.Run("missing passphrase for an encrypted key", func(t *testing.T) {
		_, err := NewEnvEscrowKeyProvider(ring, "")
		require.ErrorIs(t, err, ErrEscrowPrivateKeysUnreadable)
	})
	t.Run("two-key ring decrypts a message addressed to either", func(t *testing.T) {
		p, err := NewEnvEscrowKeyProvider(ring, pass)
		require.NoError(t, err)
		assert.Equal(t, []string{k1.Fingerprint, k2.Fingerprint}, p.Fingerprints())
		assert.NotContains(t, p.String(), "BEGIN")

		keys, err := p.DecryptionKeyring(context.Background())
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
	t.Run("unencrypted key needs no passphrase", func(t *testing.T) {
		p, err := NewEnvEscrowKeyProvider(k1.ArmoredPrivate, "")
		require.NoError(t, err)
		assert.Len(t, p.Fingerprints(), 1)
	})
	t.Run("public-only material is rejected", func(t *testing.T) {
		_, err := NewEnvEscrowKeyProvider(k1.ArmoredPublic, "")
		require.ErrorIs(t, err, ErrEscrowPrivateKeysUnreadable)
	})
	t.Run("nil provider is safe", func(t *testing.T) {
		var p *EnvEscrowKeyProvider
		_, err := p.DecryptionKeyring(context.Background())
		assert.ErrorIs(t, err, ErrNoEscrowPrivateKeys)
	})
	_ = strings.TrimSpace
}
