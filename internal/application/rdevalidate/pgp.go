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

// ParseTrustedKeyring parses the armored verification keys registered for the
// escrow source of a tenant/TLD. A key that fails to parse is reported as a
// WARNING finding (by index, never by content) and skipped, so one bad
// registration cannot break the others.
//
// Finding messages here and below are written in the words the escrow setup
// screens use — verification key, escrow source, receiving identity — because
// this is what an operator reads at the moment a deposit fails, and it is no
// help to be sent looking for a "trusted key" that is labelled something else
// everywhere they can act on it. The Code beside each message is the stable
// identifier; only the prose follows the interface.
func ParseTrustedKeyring(armored []string, now time.Time) (openpgp.EntityList, []Finding) {
	var ring openpgp.EntityList
	var findings []Finding
	for i, a := range armored {
		el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(a))
		if err != nil || len(el) == 0 {
			findings = append(findings, Finding{
				Code: CodeSigKeyUntrusted, Severity: SeverityWarning, Stage: StageSignature,
				ObjectType: "trusted-key", Locator: "trustedKey[" + itoa(i) + "]",
				Message: "a verification key registered for this escrow source could not be read, and was skipped", At: now,
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

	// Parsed before the keyring is judged, so a refusal can name the key that
	// signed. Which key that is, against what is registered, is the whole
	// answer to "why was this deposit not trusted" — and a key ID is public
	// metadata, never material.
	sigPacket, perr := parseSignaturePacket(trimmed)
	if perr != nil {
		return fail(CodeSigMalformed, "detached signature could not be parsed as an OpenPGP signature packet")
	}
	signedBy := "an unidentified key"
	if sigPacket.IssuerKeyId != nil {
		signedBy = "key " + keyIDHex(*sigPacket.IssuerKeyId)
	}

	if len(keyring) == 0 {
		return fail(CodeSigKeyUntrusted, "this top-level domain's escrow source has no active verification key, so nothing here can confirm who sent the deposit; it was signed by "+signedBy)
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
			return fail(CodeSigKeyUntrusted, "the deposit was signed by "+signedBy+
				", which is not among the verification keys active for this top-level domain's escrow source; active here: "+strings.Join(trustedKeyIDs(keyring), ", "))
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

// trustedKeyIDs lists what a keyring would accept a signature from — each
// entity's primary key and its signing subkeys — so a refusal can say which
// verification keys are active next to the key that signed. The list is
// capped: it goes into the notification the registry receives, and a long one
// helps nobody.
func trustedKeyIDs(ring openpgp.EntityList) []string {
	const max = 4
	var ids []string
	for _, e := range ring {
		if e.PrimaryKey != nil {
			ids = append(ids, keyIDHex(e.PrimaryKey.KeyId))
		}
		for _, sk := range e.Subkeys {
			if sk.PublicKey != nil && sk.PublicKey.CanSign() {
				ids = append(ids, keyIDHex(sk.PublicKey.KeyId))
			}
		}
	}
	if len(ids) > max {
		rest := len(ids) - max
		ids = append(ids[:max:max], "and "+itoa(rest)+" more")
	}
	return ids
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

// noDecryptionKeyMessage is shared with the pipeline, which reaches the same
// condition one step earlier: an operator should not read two different
// sentences for one missing key.
const noDecryptionKeyMessage = "the receiving identity for this top-level domain has no usable decryption key, so the deposit cannot be opened"

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
		return fail(CodeDecryptKeyUnavailable, noDecryptionKeyMessage)
	}
	md, err := openpgp.ReadMessage(ciphertext, keyring, nil, pgpConfig(now))
	if err != nil {
		if errors.Is(err, pgperrors.ErrKeyIncorrect) {
			return fail(CodeDecryptNotForServiceKey, "the deposit is not encrypted to any active decryption key of our receiving identity; the sender may still be using a public key we have retired")
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

// ErrPublicKeyHasNoUsableSubkeys is returned with the reduced entity when a
// key could only be read after dropping its subkeys.
var ErrPublicKeyHasNoUsableSubkeys = errors.New("the key's subkeys could not be read and were dropped")

// ErrPublicKeyReductionUnsafe is returned when a key could only be read by
// dropping material that might sign. Dropping it would leave a key that parses
// and then silently fails to verify the deposits it was registered for, so the
// key is refused instead.
var ErrPublicKeyReductionUnsafe = errors.New("the key can only be read by dropping a subkey that may be able to sign")

// ParseArmoredPublicKeyLenient parses a public key, and when OpenPGP refuses
// the block only because of an encryption subkey, parses the primary key alone.
//
// Registry signing keys from the 2000s reach us with an encryption subkey whose
// binding signature this library cannot read — it was stripped by an export, or
// it is signed with a hash the library does not implement — and OpenPGP then
// refuses the whole key ("subkey packet not followed by signature"). An
// encryption subkey has no part in verifying a deposit: the signature is made
// by the primary key, which is also what the fingerprint identifies. Dropping
// it keeps the key usable for verification and loses nothing verification needs.
//
// That reasoning holds only while everything dropped is incapable of signing,
// so it is checked rather than assumed: a key whose unreadable subkey could
// sign, or that hides material this library cannot identify, is refused with
// ErrPublicKeyReductionUnsafe. Capability is judged by algorithm, because the
// key flags live in the very binding signature that cannot be read.
//
// Nothing else is relaxed. The primary key's own self-signature is still
// verified, so this cannot turn an unauthenticated key into a trusted one. On
// success after a reduction the returned armored block is the reduced key —
// store that, so every later reader sees what was accepted here — and the error
// is ErrPublicKeyHasNoUsableSubkeys, which callers report rather than fail on.
func ParseArmoredPublicKeyLenient(armored string) (*openpgp.Entity, string, error) {
	entity, err := ParseArmoredPublicKey(armored)
	if err == nil {
		return entity, armored, nil
	}
	reduced, unsafe, rerr := primaryKeyOnly(armored)
	if rerr != nil {
		return nil, "", err // the original failure is the one worth reporting
	}
	if unsafe {
		return nil, "", ErrPublicKeyReductionUnsafe
	}
	entity, rerr = ParseArmoredPublicKey(reduced)
	if rerr != nil {
		return nil, "", err
	}
	// A primary key that cannot sign leaves nothing to verify deposits with,
	// which makes the reduction pointless: report the original failure.
	if !entity.PrimaryKey.CanSign() {
		return nil, "", err
	}
	return entity, reduced, ErrPublicKeyHasNoUsableSubkeys
}

// primaryKeyOnly re-serialises a public key block as its primary key, user IDs
// and the signatures over them, dropping the subkeys. It also reports whether
// dropping them is unsafe: a subkey whose algorithm can sign, or a packet this
// library cannot identify, could be what signs the registry's deposits.
//
// It reads with NextWithUnsupported because Next silently skips packets it
// cannot parse, and a signing subkey in an algorithm this library does not
// implement would then be dropped without anyone noticing.
func primaryKeyOnly(armored string) (reducedBlock string, unsafe bool, err error) {
	block, err := armor.Decode(strings.NewReader(armored))
	if err != nil {
		return "", false, err
	}
	reader := packet.NewReader(block.Body)
	var kept []packet.Packet
	for {
		p, err := reader.NextWithUnsupported()
		if err == io.EOF {
			break
		}
		if err != nil {
			// An unreadable packet before any subkey means the reduction would
			// change more than the subkeys; leave the key to the strict parser.
			if len(kept) == 0 {
				return "", false, err
			}
			// Past that point the rest of the block is unaccounted for, and it
			// may hold the key that signs.
			return "", true, nil
		}
		switch pkt := p.(type) {
		case *packet.UnsupportedPacket:
			// Something is here that this library cannot read. What it would
			// drop cannot be established, so the reduction is not safe.
			return "", true, nil
		case *packet.PublicKey:
			if pkt.IsSubkey {
				if pkt.CanSign() {
					return "", true, nil // this is what dropping would cost
				}
				continue // an encryption subkey is what we are dropping
			}
			if len(kept) > 0 {
				return "", false, errors.New("more than one primary key in the block")
			}
			kept = append(kept, pkt)
		case *packet.UserId, *packet.Signature:
			if len(kept) == 0 {
				return "", false, errors.New("the block does not start with a public key")
			}
			kept = append(kept, pkt)
		}
	}
	if len(kept) == 0 {
		return "", false, errors.New("no primary key found")
	}
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", false, err
	}
	for _, p := range kept {
		var serr error
		switch pkt := p.(type) {
		case *packet.PublicKey:
			serr = pkt.Serialize(w)
		case *packet.UserId:
			serr = pkt.Serialize(w)
		case *packet.Signature:
			serr = pkt.Serialize(w)
		}
		if serr != nil {
			return "", false, serr
		}
	}
	if err := w.Close(); err != nil {
		return "", false, err
	}
	return buf.String(), false, nil
}
