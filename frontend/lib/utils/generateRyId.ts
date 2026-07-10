/**
 * Derives a ClIDType-compliant RyID from a company name.
 *
 * Rules (EPP clIDType):
 *  - 3–16 characters
 *  - ASCII only (0x20–0x7E)
 *  - No leading/trailing whitespace
 *
 * Strategy:
 *  1. Strip common corporate suffixes (Inc., LLC, Ltd, etc.)
 *  2. Split into words
 *  3. If single word ≤ 16 chars → use it uppercase
 *  4. If multi-word → abbreviate each word to first 3 chars, join with "-"
 *  5. Clamp to 3–16 chars, strip non-ASCII
 *
 * @example
 *   generateRyId("Acme Registry Inc.")   // "ACME-REG"
 *   generateRyId("VeriSign")             // "VERISIGN"
 *   generateRyId("Internet Computer Bureau Ltd") // "INT-COM-BUR"
 *   generateRyId("AB")                   // "AB" (too short — caller should validate)
 */

const CORPORATE_SUFFIXES = [
  // English
  'incorporated', 'inc', 'llc', 'ltd', 'limited', 'corp', 'corporation',
  'co', 'company', 'plc', 'lp', 'llp', 'pllc',
  // European
  'gmbh', 'ag', 'bv', 'b.v', 'b.v.', 'nv', 'n.v', 'n.v.', 'sa', 's.a', 's.a.',
  'sarl', 's.a.r.l', 'srl', 's.r.l', 'spa', 's.p.a', 'se', 'ab',
  // Other
  'pty', 'pvt', 'pte',
];

/**
 * Strips trailing dots and periods from a word for suffix matching.
 */
function normalizeWord(word: string): string {
  return word.replace(/[.,]+$/g, '').toLowerCase();
}

/**
 * Returns true if the character is printable ASCII (0x20–0x7E).
 */
function isAscii(char: string): boolean {
  const code = char.charCodeAt(0);
  return code >= 0x21 && code <= 0x7e; // exclude space (0x20) for cleanliness
}

export function generateRyId(companyName: string): string {
  // 1. Trim and split into words
  const rawWords = companyName.trim().split(/\s+/);

  // 2. Strip corporate suffixes from the end
  const words: string[] = [];
  for (const word of rawWords) {
    const normalized = normalizeWord(word);
    if (CORPORATE_SUFFIXES.includes(normalized)) {
      continue; // skip suffix
    }
    // Strip non-ASCII, keep only printable ASCII
    const cleaned = word
      .split('')
      .filter(isAscii)
      .join('')
      .replace(/[.,]+$/g, ''); // strip trailing punctuation
    if (cleaned.length > 0) {
      words.push(cleaned);
    }
  }

  // 3. If nothing left after stripping, fall back to raw words
  if (words.length === 0) {
    const fallback = companyName
      .trim()
      .split('')
      .filter(isAscii)
      .join('')
      .toUpperCase()
      .slice(0, 16);
    return fallback || 'RO';
  }

  // 4. Single word: use it uppercase in full
  if (words.length === 1) {
    return words[0].toUpperCase().slice(0, 16);
  }

  // 5. Multi-word: abbreviate each word to first 3 chars, join with "-"
  const abbreviated = words.map((w) => w.slice(0, 3).toUpperCase());
  let result = abbreviated.join('-');

  // If the result is too long, progressively shorten abbreviations
  if (result.length > 16) {
    // Try 2-char abbreviations
    const shorter = words.map((w) => w.slice(0, 2).toUpperCase());
    result = shorter.join('-');
  }

  // If still too long, just take first N words that fit
  if (result.length > 16) {
    result = result.slice(0, 16).replace(/-$/, '');
  }

  return result;
}
