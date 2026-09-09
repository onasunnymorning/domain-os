package rdevalidate

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testNow is pinned after key generation: OpenPGP treats a verification time
// earlier than a key's creation as "key expired" (fail closed), so tests must
// not use a fixed date in the past relative to the generated keys.
var testNow = time.Now().UTC().Truncate(time.Second).Add(time.Hour)

func TestVerifyDetached(t *testing.T) {
	registry := rdetest.NewKeyPair(t, "registry")
	stranger := rdetest.NewKeyPair(t, "stranger")
	data := []byte("the deposit bytes")
	ring, fs := ParseTrustedKeyring([]string{registry.ArmoredPublic}, testNow)
	require.Empty(t, fs)
	require.Len(t, ring, 1)

	t.Run("binary signature verifies", func(t *testing.T) {
		info, f := VerifyDetached(ring, bytes.NewReader(data), rdetest.Sign(t, data, registry, false), testNow)
		require.Nil(t, f)
		assert.Equal(t, registry.Fingerprint, info.KeyFingerprint)
		assert.NotEmpty(t, info.KeyID)
		assert.False(t, info.SignedAt.IsZero())
	})
	t.Run("armored signature verifies", func(t *testing.T) {
		_, f := VerifyDetached(ring, bytes.NewReader(data), rdetest.Sign(t, data, registry, true), testNow)
		require.Nil(t, f)
	})
	t.Run("tampered data is SIG_INVALID", func(t *testing.T) {
		_, f := VerifyDetached(ring, bytes.NewReader(rdetest.Tamper(data, 3)), rdetest.Sign(t, data, registry, false), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeSigInvalid, f.Code)
	})
	t.Run("unknown signer is SIG_KEY_UNTRUSTED", func(t *testing.T) {
		_, f := VerifyDetached(ring, bytes.NewReader(data), rdetest.Sign(t, data, stranger, false), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeSigKeyUntrusted, f.Code)
	})
	t.Run("empty keyring is SIG_KEY_UNTRUSTED", func(t *testing.T) {
		_, f := VerifyDetached(nil, bytes.NewReader(data), rdetest.Sign(t, data, registry, false), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeSigKeyUntrusted, f.Code)
	})
	t.Run("garbage signature is SIG_MALFORMED", func(t *testing.T) {
		_, f := VerifyDetached(ring, bytes.NewReader(data), []byte("not a signature"), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeSigMalformed, f.Code)
		_, f = VerifyDetached(ring, bytes.NewReader(data), nil, testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeSigMalformed, f.Code)
	})
	t.Run("data is drained on failure", func(t *testing.T) {
		r := &countingReader{r: bytes.NewReader(data)}
		VerifyDetached(ring, r, []byte("junk"), testNow)
		assert.Equal(t, int64(len(data)), r.n)
	})
	t.Run("unparsable trusted key is skipped with a warning", func(t *testing.T) {
		ring2, fs := ParseTrustedKeyring([]string{"garbage", registry.ArmoredPublic}, testNow)
		require.Len(t, fs, 1)
		assert.Equal(t, SeverityWarning, fs[0].Severity)
		assert.Equal(t, "trustedKey[0]", fs[0].Locator)
		require.Len(t, ring2, 1)
	})
}

func TestDecrypt(t *testing.T) {
	oldKey := rdetest.NewKeyPair(t, "service-old")
	newKey := rdetest.NewKeyPair(t, "service-new")
	other := rdetest.NewKeyPair(t, "someone-else")
	plain := []byte("<xml/>")
	ring := openpgp.EntityList{newKey.Entity, oldKey.Entity}

	readAll := func(md *openpgp.MessageDetails) ([]byte, error) {
		return io.ReadAll(md.UnverifiedBody)
	}

	t.Run("decrypts with the current key", func(t *testing.T) {
		md, info, f := Decrypt(ring, bytes.NewReader(rdetest.Encrypt(t, plain, newKey)), testNow)
		require.Nil(t, f)
		got, err := readAll(md)
		require.NoError(t, err)
		assert.Equal(t, plain, got)
		assert.Equal(t, newKey.Fingerprint, info.KeyFingerprint)
		assert.Len(t, info.EncryptedToKeyIDs, 1)
		assert.False(t, info.InnerSigned)
	})
	t.Run("rollover: still decrypts with the previous key", func(t *testing.T) {
		md, info, f := Decrypt(ring, bytes.NewReader(rdetest.Encrypt(t, plain, oldKey)), testNow)
		require.Nil(t, f)
		_, err := readAll(md)
		require.NoError(t, err)
		assert.Equal(t, oldKey.Fingerprint, info.KeyFingerprint)
	})
	t.Run("encrypted to a stranger", func(t *testing.T) {
		_, _, f := Decrypt(ring, bytes.NewReader(rdetest.Encrypt(t, plain, other)), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeDecryptNotForServiceKey, f.Code)
	})
	t.Run("garbage ciphertext", func(t *testing.T) {
		_, _, f := Decrypt(ring, bytes.NewReader([]byte("definitely not pgp")), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeDecryptFailed, f.Code)
	})
	t.Run("empty keyring is key-unavailable", func(t *testing.T) {
		_, _, f := Decrypt(nil, bytes.NewReader(rdetest.Encrypt(t, plain, newKey)), testNow)
		require.NotNil(t, f)
		assert.Equal(t, CodeDecryptKeyUnavailable, f.Code)
		assert.True(t, f.Code.IsErrorClass())
	})
	t.Run("corrupted ciphertext body fails integrity on read", func(t *testing.T) {
		ct := rdetest.Encrypt(t, bytes.Repeat(plain, 200), newKey)
		md, _, f := Decrypt(ring, bytes.NewReader(rdetest.Tamper(ct, len(ct)-40)), testNow)
		if f != nil {
			assert.Equal(t, CodeDecryptFailed, f.Code)
			return
		}
		_, err := readAll(md)
		require.Error(t, err)
		assert.True(t, IsIntegrityError(err), "expected integrity error, got %v", err)
	})
}

func TestParseArmoredPublicKey(t *testing.T) {
	kp := rdetest.NewKeyPair(t, "k")
	e, err := ParseArmoredPublicKey(kp.ArmoredPublic)
	require.NoError(t, err)
	assert.Equal(t, kp.Fingerprint, FingerprintHex(e))
	_, err = ParseArmoredPublicKey(kp.ArmoredPrivate)
	assert.Error(t, err, "private key rejected")
	_, err = ParseArmoredPublicKey("nope")
	assert.Error(t, err)
}
