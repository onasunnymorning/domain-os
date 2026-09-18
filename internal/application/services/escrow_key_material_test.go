package services

import (
	"bytes"
	"crypto"
	"crypto/dsa" //nolint:staticcheck // reproducing a 2001-era GnuPG key on purpose
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/elgamal"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The registry key registry (ADR-0009) accepts keys from registries that have
// been signing escrow deposits for twenty years, and when it cannot, it says
// which check failed rather than "could not be read".

// legacyKey builds a DSA-1024 primary key with an ElGamal encryption subkey and
// SHA-1 self-signatures — the shape GnuPG 1.4 produced.
func legacyKey(t *testing.T) *openpgp.Entity {
	t.Helper()
	var params dsa.Parameters
	if err := dsa.GenerateParameters(&params, rand.Reader, dsa.L1024N160); err != nil {
		t.Fatal(err)
	}
	priv := &dsa.PrivateKey{PublicKey: dsa.PublicKey{Parameters: params}}
	if err := dsa.GenerateKey(priv, rand.Reader); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2001, 8, 5, 0, 0, 0, 0, time.UTC)
	no := false
	cfg := &packet.Config{NonDeterministicSignaturesViaNotation: &no}

	egPriv := &elgamal.PrivateKey{}
	egPriv.P = params.P
	egPriv.G = params.G
	egPriv.X = priv.X
	egPriv.Y = priv.Y

	e := &openpgp.Entity{
		PrimaryKey: packet.NewDSAPublicKey(created, &priv.PublicKey),
		PrivateKey: packet.NewDSAPrivateKey(created, priv),
		Identities: map[string]*openpgp.Identity{},
	}
	uid := packet.NewUserId("Legacy Registry", "", "legacy@registry.test")
	isPrimary := true
	sig := &packet.Signature{
		Version: 4, SigType: packet.SigTypePositiveCert, PubKeyAlgo: packet.PubKeyAlgoDSA,
		Hash: crypto.SHA1, CreationTime: created, IssuerKeyId: &e.PrimaryKey.KeyId,
		IsPrimaryId: &isPrimary, FlagsValid: true, FlagSign: true, FlagCertify: true,
	}
	if err := sig.SignUserId(uid.Id, e.PrimaryKey, e.PrivateKey, cfg); err != nil {
		t.Fatal(err)
	}
	e.Identities[uid.Id] = &openpgp.Identity{Name: uid.Id, UserId: uid, SelfSignature: sig, Signatures: []*packet.Signature{sig}}

	subPub := packet.NewElGamalPublicKey(created, &egPriv.PublicKey)
	subPub.IsSubkey = true
	subSig := &packet.Signature{
		Version: 4, SigType: packet.SigTypeSubkeyBinding, PubKeyAlgo: packet.PubKeyAlgoDSA,
		Hash: crypto.SHA1, CreationTime: created, IssuerKeyId: &e.PrimaryKey.KeyId,
		FlagsValid: true, FlagEncryptCommunications: true, FlagEncryptStorage: true,
	}
	if err := subSig.SignKey(subPub, e.PrivateKey, cfg); err != nil {
		t.Fatal(err)
	}
	subPriv := packet.NewElGamalPrivateKey(created, egPriv)
	subPriv.IsSubkey = true
	e.Subkeys = []openpgp.Subkey{{PublicKey: subPub, PrivateKey: subPriv, Sig: subSig}}
	return e
}

func armorPub(t *testing.T, e *openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Serialize(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestAddPublicKey_AcceptsA2001DSAKey pins that age alone is no reason for a
// refusal: a DSA-1024 primary with an ElGamal subkey and SHA-1 self-signatures,
// the shape GnuPG 1.4 produced, is read and fingerprinted.
func TestAddPublicKey_AcceptsA2001DSAKey(t *testing.T) {
	armored := armorPub(t, legacyKey(t))

	entity, err := rdevalidate.ParseArmoredPublicKey(armored)
	require.NoError(t, err, "a 2001-era DSA/ElGamal key must still be readable")
	assert.Len(t, rdevalidate.FingerprintHex(entity), 40)
}

// TestPublicKeyRejection_SaysWhichCheckFailed: the person pasting a key gets a
// reason they can act on, and it never quotes the material.
// armoredBlock builds an armor block from its kind, so this file holds no
// literal private-key header for the secret scanner to read as a leak.
func armoredBlock(kind string) string {
	return fmt.Sprintf("-----BEGIN PGP %s KEY BLOCK-----\nbody\n-----END PGP %s KEY BLOCK-----", kind, kind)
}

func TestPublicKeyRejection_SaysWhichCheckFailed(t *testing.T) {
	parseErr := errors.New("openpgp: invalid data: user ID self-signature invalid")

	for _, tc := range []struct {
		name, armored, want string
	}{
		{"a private key in the public field", armoredBlock("PRIVATE"), "that is a private key"},
		{"not a key at all", "hello", "ASCII-armored OpenPGP public key"},
		{"several keys at once", armoredBlock("PUBLIC"), "more than one key"},
		{"a key OpenPGP cannot read", armoredBlock("PUBLIC"), "user ID self-signature invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := parseErr
			if tc.name == "several keys at once" {
				err = errors.New("expected exactly one key in the armored block")
			}
			got := publicKeyRejection(tc.armored, err)

			require.ErrorIs(t, got, entities.ErrEscrowKeyMaterialUnreadable, "existing handling still recognises it")
			var rejected *entities.EscrowKeyMaterialRejection
			require.ErrorAs(t, got, &rejected)
			assert.Contains(t, rejected.Reason, tc.want)
			assert.NotContains(t, rejected.Reason, "body", "the reason never quotes the submitted block")
		})
	}
}
