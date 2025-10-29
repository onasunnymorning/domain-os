// Utility to generate a backend-compliant AuthInfo string
// Backend constraints (see internal/domain/entities/authInfoType.go):
// - Length between 8 and 22
// - Must include at least 1 uppercase, 1 lowercase, and 1 special character
// - Error message mentions a number as well; include at least 1 digit for forward-compat

function getRandomInt(max: number) {
  const array = new Uint32Array(1);
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(array);
    return array[0] % max;
  }
  // Fallback (non-crypto); not ideal but acceptable in non-browser tests
  return Math.floor(Math.random() * max);
}

function pick(chars: string): string {
  return chars[getRandomInt(chars.length)];
}

function shuffle<T>(arr: T[]): T[] {
  // Fisher–Yates shuffle using crypto if available
  for (let i = arr.length - 1; i > 0; i--) {
    const j = getRandomInt(i + 1);
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
}

export function generateAuthInfo(length = 14): string {
  const min = 8;
  const max = 22;
  const L = Math.min(max, Math.max(min, length));

  const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const lower = 'abcdefghijklmnopqrstuvwxyz';
  const digits = '0123456789';
  // Avoid whitespace and newlines; stick to common ASCII punctuation used server-side
  const special = `!"#$%&'()*+,-./:;<=>?@[\\]^_{|}~`;

  const all = upper + lower + digits + special;

  const chars: string[] = [];
  // Ensure at least one of each category
  chars.push(pick(upper));
  chars.push(pick(lower));
  chars.push(pick(digits));
  chars.push(pick(special));

  // Fill remaining
  while (chars.length < L) {
    chars.push(pick(all));
  }

  // Shuffle to avoid predictable positions
  return shuffle(chars).join('');
}

// Simple predicate used by tests and optional client-side validation aids
export function isLikelyCompliantAuthInfo(s: string): boolean {
  if (s.length < 8 || s.length > 22) return false;
  if (!/[A-Z]/.test(s)) return false;
  if (!/[a-z]/.test(s)) return false;
  if (!/\d/.test(s)) return false; // include digit for forward compat
  if (!/[^A-Za-z0-9]/.test(s)) return false; // has special
  return true;
}
