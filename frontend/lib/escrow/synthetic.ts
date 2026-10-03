/**
 * The shape of a synthetic escrow deposit (POST /escrow/synthetic), and the
 * arithmetic the form shows before anything is generated.
 *
 * The limits mirror internal/application/rdesynth/params.go, which is what
 * actually enforces them; these only keep the form from offering a request the
 * server will refuse.
 */

export const SYNTHETIC_LIMITS = {
  domains: 250_000,
  contactsPerDomain: 4,
  avgHostsPerDomain: 13,
  nndns: 1_000_000,
} as const;

/** Registrars sponsoring the generated domains; fixed server-side. */
export const SYNTHETIC_REGISTRARS = 5;

export interface SyntheticDepositParams {
  tld: string;
  domains: number;
  contactsPerDomain: number;
  avgHostsPerDomain: number;
  nndns: number;
  /** Omit to let the server pick one; it is returned with the download. */
  seed?: number;
}

export interface SyntheticCounts {
  domains: number;
  contacts: number;
  hosts: number;
  registrars: number;
  nndns: number;
}

/**
 * The objects the deposit will carry. Hosts are floor(domains × average):
 * the server spreads the fraction so the total is exact.
 */
export function syntheticCounts(p: Pick<SyntheticDepositParams, 'domains' | 'contactsPerDomain' | 'avgHostsPerDomain' | 'nndns'>): SyntheticCounts {
  return {
    domains: p.domains,
    contacts: p.domains * p.contactsPerDomain,
    hosts: Math.floor(p.domains * p.avgHostsPerDomain),
    registrars: SYNTHETIC_REGISTRARS,
    nndns: p.nndns,
  };
}

// Compressed bytes per object, measured on generated output. Hosts vary the
// most (sequential nameserver names compress better the more a domain has),
// so the estimate is a rough guide, not a promise.
const GZ_BYTES = { base: 1_300, domain: 52, contact: 100, host: 55, nndn: 6 };

/** Approximate size of the .xml.gz download, in bytes. */
export function estimateSyntheticBytes(c: SyntheticCounts): number {
  return GZ_BYTES.base + c.domains * GZ_BYTES.domain + c.contacts * GZ_BYTES.contact + c.hosts * GZ_BYTES.host + c.nndns * GZ_BYTES.nndn;
}

/** A human-readable size: "840 KB", "12.4 MB". */
export function formatBytes(n: number): string {
  if (n < 1_000) return `${n} B`;
  if (n < 1_000_000) return `${Math.round(n / 1_000)} KB`;
  if (n < 1_000_000_000) return `${(n / 1_000_000).toFixed(1)} MB`;
  return `${(n / 1_000_000_000).toFixed(2)} GB`;
}

/**
 * The problem with params, or null. Mirrors the server's bounds so the form
 * can say what is wrong before a round trip; the server stays the authority.
 */
export function syntheticParamsProblem(p: SyntheticDepositParams): string | null {
  const tld = p.tld.trim().replace(/^\.+|\.+$/g, '');
  if (!tld) return 'Enter a TLD.';
  if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/i.test(tld)) {
    return 'The TLD may only contain letters, digits and hyphens (use the xn-- form for an IDN TLD).';
  }
  const int = (v: number, max: number, what: string) =>
    !Number.isInteger(v) || v < 0 || v > max ? `${what} must be a whole number from 0 to ${max.toLocaleString('en-US')}.` : null;
  return (
    int(p.domains, SYNTHETIC_LIMITS.domains, 'Domains') ??
    int(p.contactsPerDomain, SYNTHETIC_LIMITS.contactsPerDomain, 'Contacts per domain') ??
    (!Number.isFinite(p.avgHostsPerDomain) || p.avgHostsPerDomain < 0 || p.avgHostsPerDomain > SYNTHETIC_LIMITS.avgHostsPerDomain
      ? `Average hosts per domain must be between 0 and ${SYNTHETIC_LIMITS.avgHostsPerDomain}.`
      : null) ??
    int(p.nndns, SYNTHETIC_LIMITS.nndns, 'NNDNs') ??
    (p.seed !== undefined && (!Number.isSafeInteger(p.seed) || p.seed < 0) ? 'The seed must be a non-negative whole number.' : null)
  );
}

/** The ICANN-style name the server would give the file; used if the header is not readable. */
export function fallbackSyntheticFilename(tld: string, now: Date = new Date()): string {
  const clean = tld.trim().replace(/^\.+|\.+$/g, '').toLowerCase() || 'synthetic';
  return `${clean}_${now.toISOString().slice(0, 10)}_full_S1_R0.xml.gz`;
}

/** The filename from a Content-Disposition header, or null. */
export function filenameFromContentDisposition(header: string | null | undefined): string | null {
  if (!header) return null;
  const star = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header);
  if (star) {
    try {
      return decodeURIComponent(star[1].trim());
    } catch {
      // fall through to the plain form
    }
  }
  const plain = /filename\s*=\s*"([^"]+)"|filename\s*=\s*([^;]+)/i.exec(header);
  const name = plain?.[1] ?? plain?.[2]?.trim();
  return name || null;
}
