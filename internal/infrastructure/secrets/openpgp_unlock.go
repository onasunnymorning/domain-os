package secrets

import (
	"bytes"
	"errors"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// errOpenPGPUnreadable is internal: callers map it to their own fixed-text
// sentinel so nothing from the input is ever echoed.
var errOpenPGPUnreadable = errors.New("openpgp material could not be read or unlocked")

// ParseAndUnlockArmoredPrivateKeys parses one or more concatenated
// ASCII-armored private key blocks and unlocks every private key with the
// passphrase. Public-only entities are skipped. The error is fixed text.
//
// It is the one place this repository turns armored private material into
// usable keys: the key-store loader and the import path both call it, so they
// cannot disagree about what "readable" means.
func ParseAndUnlockArmoredPrivateKeys(armored, passphrase string) (openpgp.EntityList, error) {
	if strings.TrimSpace(armored) == "" {
		return nil, errOpenPGPUnreadable
	}
	// ReadArmoredKeyRing stops after the first armored block, so a ring of
	// concatenated blocks (the rollover case) is split and parsed block by block.
	var ring openpgp.EntityList
	for _, block := range splitArmoredBlocks(armored) {
		el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(block))
		if err != nil || len(el) == 0 {
			return nil, errOpenPGPUnreadable
		}
		ring = append(ring, el...)
	}
	var unlocked openpgp.EntityList
	for _, e := range ring {
		if e.PrivateKey == nil {
			continue // a public key in the ring is harmless but useless here
		}
		if e.PrivateKey.Encrypted || anySubkeyEncrypted(e) {
			if passphrase == "" {
				return nil, errOpenPGPUnreadable
			}
			if err := e.DecryptPrivateKeys([]byte(passphrase)); err != nil {
				return nil, errOpenPGPUnreadable
			}
		}
		unlocked = append(unlocked, e)
	}
	if len(unlocked) == 0 {
		return nil, errOpenPGPUnreadable
	}
	return unlocked, nil
}

func anySubkeyEncrypted(e *openpgp.Entity) bool {
	for _, sk := range e.Subkeys {
		if sk.PrivateKey != nil && sk.PrivateKey.Encrypted {
			return true
		}
	}
	return false
}

const armorBegin = "-----BEGIN PGP "

// splitArmoredBlocks splits concatenated ASCII-armored blocks. Text outside a
// block is ignored; block boundaries are the BEGIN markers.
func splitArmoredBlocks(s string) []string {
	var blocks []string
	for {
		start := strings.Index(s, armorBegin)
		if start < 0 {
			return blocks
		}
		s = s[start:]
		next := strings.Index(s[len(armorBegin):], armorBegin)
		if next < 0 {
			blocks = append(blocks, strings.TrimSpace(s))
			return blocks
		}
		blocks = append(blocks, strings.TrimSpace(s[:next+len(armorBegin)]))
		s = s[next+len(armorBegin):]
	}
}

// ArmoredPublicKey serialises the public half of an entity as an ASCII-armored
// block, which is what the key registry stores and hands to registries.
func ArmoredPublicKey(e *openpgp.Entity) (string, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := e.Serialize(w); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// KeyExpiry returns when the entity's primary key expires, or nil when it does
// not carry an expiry.
func KeyExpiry(e *openpgp.Entity) *time.Time {
	id := e.PrimaryIdentity()
	if id == nil || id.SelfSignature == nil || id.SelfSignature.KeyLifetimeSecs == nil || *id.SelfSignature.KeyLifetimeSecs == 0 {
		return nil
	}
	t := e.PrimaryKey.CreationTime.Add(time.Duration(*id.SelfSignature.KeyLifetimeSecs) * time.Second).UTC()
	return &t
}

// CanEncrypt reports whether the entity has a usable encryption key now, which
// a decrypt-inbound version needs: registries encrypt deposits to it.
func CanEncrypt(e *openpgp.Entity, now time.Time) bool {
	_, ok := e.EncryptionKey(now)
	return ok
}
