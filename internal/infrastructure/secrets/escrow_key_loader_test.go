package secrets

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storeOpenPGP(t *testing.T, store *MemorySecretStore, kp rdetest.KeyPair, pass string) entities.EscrowKeyVersionRef {
	t.Helper()
	value, err := EncodeOpenPGPSecret(rdetest.EncryptedArmoredPrivate(t, kp, pass), pass)
	require.NoError(t, err)
	ref, err := store.Put(context.Background(), "escrow-keys/platform/"+uuid.NewString(), nil, value)
	require.NoError(t, err)
	return entities.EscrowKeyVersionRef{ID: uuid.New(), Purpose: entities.EscrowKeyPurposeDecryptInbound, Fingerprint: kp.Fingerprint, SecretRef: &ref}
}

func TestEscrowKeyLoader_OpenPGP(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySecretStore()
	kp := rdetest.NewKeyPair(t, "eve-v1")
	ref := storeOpenPGP(t, store, kp, "pass phrase")
	loader := NewEscrowKeyLoader(store, time.Minute)

	e, err := loader.OpenPGPPrivateKey(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, kp.Fingerprint, rdevalidate.FingerprintHex(e))

	ct := rdetest.Encrypt(t, []byte("deposit"), kp)
	md, info, f := rdevalidate.Decrypt(openpgp.EntityList{e}, bytes.NewReader(ct), time.Now())
	require.Nil(t, f)
	body, err := io.ReadAll(md.UnverifiedBody)
	require.NoError(t, err)
	assert.Equal(t, "deposit", string(body))
	assert.Equal(t, kp.Fingerprint, info.KeyFingerprint)

	t.Run("a swapped backend value fails closed", func(t *testing.T) {
		other := rdetest.NewKeyPair(t, "attacker")
		swapped, err := EncodeOpenPGPSecret(other.ArmoredPrivate, "")
		require.NoError(t, err)
		newRef := store.Replace(*ref.SecretRef, swapped)
		// The reference still pins the original backend version, so it keeps working...
		_, err = NewEscrowKeyLoader(store, time.Minute).OpenPGPPrivateKey(ctx, ref)
		require.NoError(t, err)
		// ...and a reference to the swapped version is refused by fingerprint.
		moved := ref
		moved.SecretRef = &newRef
		_, err = NewEscrowKeyLoader(store, time.Minute).OpenPGPPrivateKey(ctx, moved)
		assert.ErrorIs(t, err, entities.ErrEscrowKeyFingerprintMismatch)
	})

	t.Run("errors never echo material", func(t *testing.T) {
		bad := ref
		badRef, err := store.Put(ctx, "garbage", nil, []byte(`{"format":"openpgp-private/v1","armoredPrivateKey":"-----BEGIN PGP PRIVATE KEY BLOCK-----\nSECRETMATERIAL\n-----END PGP PRIVATE KEY BLOCK-----","passphrase":"hunter2"}`))
		require.NoError(t, err)
		bad.ID, bad.SecretRef = uuid.New(), &badRef
		_, err = loader.OpenPGPPrivateKey(ctx, bad)
		require.ErrorIs(t, err, entities.ErrEscrowKeyMaterialUnreadable)
		assert.NotContains(t, err.Error(), "SECRETMATERIAL")
		assert.NotContains(t, err.Error(), "hunter2")
	})

	t.Run("store outage is transient; no store is not", func(t *testing.T) {
		down := NewMemorySecretStore()
		down.FailWith = entities.ErrEscrowKeyStoreUnavailable
		_, err := NewEscrowKeyLoader(down, time.Minute).OpenPGPPrivateKey(ctx, ref)
		assert.True(t, IsEscrowKeyStoreTransient(err))
		_, err = NewEscrowKeyLoader(nil, time.Minute).OpenPGPPrivateKey(ctx, ref)
		assert.ErrorIs(t, err, entities.ErrEscrowKeyStoreNotConfigured)
		assert.False(t, IsEscrowKeyStoreTransient(err))
	})

	t.Run("cache serves until forgotten or expired", func(t *testing.T) {
		s := NewMemorySecretStore()
		r := storeOpenPGP(t, s, kp, "p")
		l := NewEscrowKeyLoader(s, time.Minute)
		clock := time.Now()
		l.now = func() time.Time { return clock }
		_, err := l.OpenPGPPrivateKey(ctx, r)
		require.NoError(t, err)
		s.FailWith = entities.ErrEscrowKeyStoreUnavailable
		_, err = l.OpenPGPPrivateKey(ctx, r)
		require.NoError(t, err, "served from cache during an outage")
		l.Forget(r.ID)
		_, err = l.OpenPGPPrivateKey(ctx, r)
		assert.Error(t, err)
		s.FailWith = nil
		_, err = l.OpenPGPPrivateKey(ctx, r)
		require.NoError(t, err)
		s.FailWith = entities.ErrEscrowKeyStoreUnavailable
		clock = clock.Add(2 * time.Minute)
		_, err = l.OpenPGPPrivateKey(ctx, r)
		assert.Error(t, err, "expired entries are refetched")
	})
}

func TestEscrowKeyLoader_Symmetric(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySecretStore()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	fp, err := rdesanitize.MasterKeyFingerprint(key)
	require.NoError(t, err)
	value, err := EncodeSymmetricSecret(key)
	require.NoError(t, err)
	ref, err := store.Put(ctx, "sym", nil, value)
	require.NoError(t, err)
	vref := entities.EscrowKeyVersionRef{ID: uuid.New(), Purpose: entities.EscrowKeyPurposePseudonymise, Fingerprint: fp, SecretRef: &ref}

	got, err := NewEscrowKeyLoader(store, time.Minute).SymmetricKey(ctx, vref)
	require.NoError(t, err)
	assert.Equal(t, key, got)

	tok, err := rdesanitize.NewTokenizer(got, "ryop1", rdesanitize.PolicyVersion)
	require.NoError(t, err)
	assert.Equal(t, fp, tok.KeyFingerprint(), "the registry fingerprint is the one derivatives already record")

	wrong := vref
	wrong.Fingerprint = "AAAAAAAAAAAAAAAA"
	_, err = NewEscrowKeyLoader(store, time.Minute).SymmetricKey(ctx, wrong)
	assert.ErrorIs(t, err, entities.ErrEscrowKeyFingerprintMismatch)

	_, err = NewEscrowKeyLoader(store, time.Minute).OpenPGPPrivateKey(ctx, vref)
	assert.ErrorIs(t, err, entities.ErrEscrowKeyMaterialUnreadable, "a symmetric secret is not an OpenPGP secret")
}
