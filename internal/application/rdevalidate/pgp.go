package rdevalidate

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const pgpArmorPrefix = "-----BEGIN PGP "

func pgpConfig(now time.Time) *packet.Config {
	return &packet.Config{Time: func() time.Time { return now }}
}

// FingerprintHex returns the upper-case hex primary-key fingerprint of an entity.
func FingerprintHex(e *openpgp.Entity) string {
	if e == nil || e.PrimaryKey == nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(e.PrimaryKey.Fingerprint))
}

func keyIDHex(id uint64) string {
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = byte(id)
		id >>= 8
	}
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

// ParseArmoredPublicKey parses one ASCII-armored public key block and returns
// its entity. Used by the admin surface to compute a fingerprint at
// registration time.
func ParseArmoredPublicKey(armored string) (*openpgp.Entity, error) {
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
	if err != nil {
		return nil, err
	}
	if len(el) != 1 {
		return nil, errors.New("expected exactly one key in the armored block")
	}
	if el[0].PrivateKey != nil {
		return nil, errors.New("a private key was supplied where a public key was expected")
	}
	return el[0], nil
}

// ParseTrustedKeyring parses the armored trusted public keys for a tenant/TLD.
// A key that fails to parse is reported as a WARNING finding (by index, never
// by content) and skipped, so one bad registration cannot break the others.
func ParseTrustedKeyring(armored []string, now time.Time) (openpgp.EntityList, []Finding) {
	var ring openpgp.EntityList
	var findings []Finding
	for i, a := range armored {
		el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(a))
		if err != nil || len(el) == 0 {
			findings = append(findings, Finding{
				Code: CodeSigKeyUntrusted, Severity: SeverityWarning, Stage: StageSignature,
				ObjectType: "trusted-key", Locator: "trustedKey[" + itoa(i) + "]",
				Message: "trusted key could not be parsed and was skipped", At: now,
			})
			continue
		}
		ring = append(ring, el...)
	}
	return ring, findings
}

// VerifyDetached checks a detached signature (binary or armored) over data
// against the trusted keyring. It returns exactly one of (info, nil) or
// (zero, finding). Data is read to EOF in every case so the caller's counters
// and hashers see the whole artifact.
func VerifyDetached(keyring openpgp.EntityList, data io.Reader, sig []byte, now time.Time) (SignatureInfo, *Finding) {
	fail := func(code Code, msg string) (SignatureInfo, *Finding) {
		_, _ = io.Copy(io.Discard, data)
		return SignatureInfo{}, &Finding{Code: code, Severity: SeverityError, Stage: StageSignature, ObjectType: "signature", Message: msg, At: now}
	}
	if len(keyring) == 0 {
		return fail(CodeSigKeyUntrusted, "no active trusted signing key is registered for this tenant/TLD")
	}
	// Only an armored signature may be trimmed: a binary packet can legitimately
	// start or end with a byte that happens to be a whitespace value.
	armored := bytes.HasPrefix(bytes.TrimLeft(sig, " \t\r\n"), []byte(pgpArmorPrefix))
	trimmed := sig
	if armored {
		trimmed = bytes.TrimSpace(sig)
	}
	if len(trimmed) == 0 {
		return fail(CodeSigMalformed, "detached signature is empty")
	}

	sigPacket, perr := parseSignaturePacket(trimmed)
	if perr != nil {
		return fail(CodeSigMalformed, "detached signature could not be parsed as an OpenPGP signature packet")
	}

	cfg := pgpConfig(now)
	var signer *openpgp.Entity
	var err error
	if armored {
		signer, err = openpgp.CheckArmoredDetachedSignature(keyring, data, bytes.NewReader(trimmed), cfg)
	} else {
		signer, err = openpgp.CheckDetachedSignature(keyring, data, bytes.NewReader(trimmed), cfg)
	}
	if err != nil {
		var sigErr pgperrors.SignatureError
		switch {
		case errors.Is(err, pgperrors.ErrUnknownIssuer):
			return fail(CodeSigKeyUntrusted, "signature was made by a key that is not trusted for this tenant/TLD")
		case errors.As(err, &sigErr):
			return fail(CodeSigInvalid, "signature does not verify over the deposit: the artifact or the signature was altered")
		default:
			return fail(CodeSigMalformed, "detached signature could not be processed")
		}
	}
	// Drain whatever the verifier did not consume (it reads to EOF, but be explicit).
	_, _ = io.Copy(io.Discard, data)

	info := SignatureInfo{KeyFingerprint: FingerprintHex(signer), SignedAt: sigPacket.CreationTime}
	if sigPacket.IssuerKeyId != nil {
		info.KeyID = keyIDHex(*sigPacket.IssuerKeyId)
	}
	return info, nil
}

func parseSignaturePacket(sig []byte) (*packet.Signature, error) {
	var r io.Reader = bytes.NewReader(sig)
	if bytes.HasPrefix(sig, []byte(pgpArmorPrefix)) {
		block, err := armor.Decode(r)
		if err != nil {
			return nil, err
		}
		if block.Type != openpgp.SignatureType {
			return nil, errors.New("armored block is not a signature")
		}
		r = block.Body
	}
	p, err := packet.Read(r)
	if err != nil {
		return nil, err
	}
	s, ok := p.(*packet.Signature)
	if !ok {
		return nil, errors.New("first packet is not a signature")
	}
	return s, nil
}

// Decrypt opens an OpenPGP message with the service keyring. go-crypto tries
// every private key in the list, so key rollover needs no extra logic here;
// which key succeeded is reported for audit. The returned MessageDetails'
// UnverifiedBody must be read to EOF by the caller: the integrity check (MDC)
// is only performed then and surfaces as a SignatureError from Read.
func Decrypt(keyring openpgp.EntityList, ciphertext io.Reader, now time.Time) (*openpgp.MessageDetails, DecryptionInfo, *Finding) {
	fail := func(code Code, msg string) (*openpgp.MessageDetails, DecryptionInfo, *Finding) {
		return nil, DecryptionInfo{}, &Finding{Code: code, Severity: SeverityError, Stage: StageDecrypt, ObjectType: "deposit", Message: msg, At: now}
	}
	if len(keyring) == 0 {
		return fail(CodeDecryptKeyUnavailable, "no service decryption key is available")
	}
	md, err := openpgp.ReadMessage(ciphertext, keyring, nil, pgpConfig(now))
	if err != nil {
		if errors.Is(err, pgperrors.ErrKeyIncorrect) {
			return fail(CodeDecryptNotForServiceKey, "deposit is not encrypted to any current service key")
		}
		return fail(CodeDecryptFailed, "deposit could not be decrypted: ciphertext is malformed or corrupt")
	}
	if !md.IsEncrypted {
		return fail(CodeDecryptFailed, "deposit is not an encrypted OpenPGP message")
	}
	info := DecryptionInfo{
		KeyFingerprint: FingerprintHex(md.DecryptedWith.Entity),
		InnerSigned:    md.IsSigned,
	}
	if md.LiteralData != nil && md.LiteralData.Time != 0 {
		info.LiteralTime = time.Unix(int64(md.LiteralData.Time), 0).UTC()
	}
	for _, id := range md.EncryptedToKeyIds {
		info.EncryptedToKeyIDs = append(info.EncryptedToKeyIDs, keyIDHex(id))
	}
	return md, info, nil
}

// IsIntegrityError reports whether an error from reading a decrypted body is
// an OpenPGP integrity (MDC/AEAD) failure.
func IsIntegrityError(err error) bool {
	var sigErr pgperrors.SignatureError
	return errors.As(err, &sigErr)
}
