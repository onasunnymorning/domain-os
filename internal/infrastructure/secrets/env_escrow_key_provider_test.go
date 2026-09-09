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

// TestEnvEscrowKeyProviderOptional covers the distinction the worker relies on:
// an absent keyring is a deployment that does not accept signed deposits and
// must not take the escrow activities off the worker, while unreadable material
// is a misconfiguration that should still stop startup.
func TestEnvEscrowKeyProviderOptional(t *testing.T) {
	t.Run("absent yields a provider that holds no keys", func(t *testing.T) {
		t.Setenv(EnvEscrowPrivateKeys, "")
		t.Setenv(EnvEscrowPrivateKeyPassphrase, "")

		p, err := NewEnvEscrowKeyProviderOptionalFromEnv()
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Empty(t, p.Fingerprints())

		// A signed deposit still cannot be decrypted — the pipeline turns this
		// into DECRYPT_KEY_UNAVAILABLE against a recorded run.
		ring, err := p.DecryptionKeyring(context.Background())
		assert.ErrorIs(t, err, ErrNoEscrowPrivateKeys)
		assert.Nil(t, ring)
	})

	t.Run("unreadable material is still fatal", func(t *testing.T) {
		// A real keyring with the wrong passphrase: the misconfiguration an
		// operator actually hits during a rollover. Tolerating it would hide a
		// broken deployment behind a worker that silently accepts no signed
		// deposit at all.
		k := rdetest.NewKeyPair(t, "eve-2026")
		t.Setenv(EnvEscrowPrivateKeys, rdetest.EncryptedArmoredPrivate(t, k, "correct horse battery staple"))
		t.Setenv(EnvEscrowPrivateKeyPassphrase, "nope")

		_, err := NewEnvEscrowKeyProviderOptionalFromEnv()
		require.ErrorIs(t, err, ErrEscrowPrivateKeysUnreadable)
		assert.NotContains(t, err.Error(), "BEGIN PGP")
	})

	t.Run("a configured keyring is loaded as usual", func(t *testing.T) {
		k := rdetest.NewKeyPair(t, "eve-2026")
		const pass = "correct horse battery staple"
		t.Setenv(EnvEscrowPrivateKeys, rdetest.EncryptedArmoredPrivate(t, k, pass))
		t.Setenv(EnvEscrowPrivateKeyPassphrase, pass)

		p, err := NewEnvEscrowKeyProviderOptionalFromEnv()
		require.NoError(t, err)
		assert.Equal(t, []string{k.Fingerprint}, p.Fingerprints())
	})
}
