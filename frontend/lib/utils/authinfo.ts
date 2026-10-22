// Transfer codes (a domain's AuthInfo) as RFC 9154 has them.
//
// The registry keeps only a salted hash of a code and never returns it, so a
// code generated here is shown once, to be handed to the registrant; after
// saving, the domain says only whether a code is set. A code must carry at
// least 128 bits of entropy, estimated as the registry estimates it
// (pkg/domain/entities/authInfoHash.go, ValidateSecureAuthInfo): its length
// times log2 of the character classes it uses (26 lower-case, 26 upper-case,
// 10 digits, 33 other printable ASCII), with at least 8 distinct characters.
// No code at all is the normal state: one is set only when the registrant
// wants to transfer the domain.

const UPPER = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
const LOWER = 'abcdefghijklmnopqrstuvwxyz';
const DIGITS = '0123456789';
// Punctuation that needs no escaping in JSON, and none of XML's <, >, & or
// quotes, so a code can be pasted into an EPP frame as it is.
const SPECIAL = '!#$%*+-:=?@^_~';
const ALL = UPPER + LOWER + DIGITS + SPECIAL; // 76 characters: 6.25 bits each

/** The fewest bits of entropy a code must carry (RFC 9154 §4.1). */
export const SECURE_AUTHINFO_BITS = 128;
/** The longest code the registry accepts. */
export const SECURE_AUTHINFO_MAX_LENGTH = 128;
/** The length generated: 24 characters of ALL carry about 150 bits. */
export const GENERATED_AUTHINFO_LENGTH = 24;

function randomInt(max: number): number {
  // Rejection sampling over crypto.getRandomValues: no modulo bias.
  const limit = Math.floor(0x100000000 / max) * max;
  const buf = new Uint32Array(1);
  for (;;) {
    crypto.getRandomValues(buf);
    if (buf[0] < limit) return buf[0] % max;
  }
}

/**
 * Generates a random transfer code that meets RFC 9154 as the registry
 * checks it: one character of each class, the rest from all of them,
 * shuffled.
 */
export function generateAuthInfo(length = GENERATED_AUTHINFO_LENGTH): string {
  const L = Math.min(SECURE_AUTHINFO_MAX_LENGTH, Math.max(GENERATED_AUTHINFO_LENGTH, length));
  const chars = [UPPER, LOWER, DIGITS, SPECIAL].map((set) => set[randomInt(set.length)]);
  while (chars.length < L) chars.push(ALL[randomInt(ALL.length)]);
  for (let i = chars.length - 1; i > 0; i--) {
    const j = randomInt(i + 1);
    [chars[i], chars[j]] = [chars[j], chars[i]];
  }
  return chars.join('');
}

/** The entropy, in bits, the registry estimates a code to carry. */
export function estimatedAuthInfoBits(s: string): number {
  let alphabet = 0;
  if (/[a-z]/.test(s)) alphabet += 26;
  if (/[A-Z]/.test(s)) alphabet += 26;
  if (/[0-9]/.test(s)) alphabet += 10;
  if (/[^A-Za-z0-9]/.test(s)) alphabet += 33;
  return alphabet === 0 ? 0 : s.length * Math.log2(alphabet);
}

/**
 * Whether the registry will accept s as a transfer code (RFC 9154): the same
 * rule EPP and the admin API apply. The server's check is authoritative.
 */
export function isLikelyCompliantAuthInfo(s: string): boolean {
  if (s.length === 0 || s.length > SECURE_AUTHINFO_MAX_LENGTH) return false;
  if (!/^[\x20-\x7e]+$/.test(s)) return false;
  if (new Set(s).size < 8) return false;
  return estimatedAuthInfoBits(s) >= SECURE_AUTHINFO_BITS;
}
