/**
 * Readiness — whether an escrow relationship works today, in one sentence.
 *
 * The registry's own vocabulary answers "what is true of this key": a
 * lifecycle state, a probe result, a purpose. An operator is asking something
 * else — can we receive deposits from this source, and if not, what do I do
 * next. Deriving that here, once, is what keeps every page from making the
 * reader combine a state column with a probe column in their head.
 *
 * Nothing here decides anything: the server remains the authority on what a
 * key may be used for. This only reads what it reported.
 */

import {
  PURPOSE_LABELS,
  type EscrowKeyPurpose,
  type EscrowKeyVersion,
  type EscrowParty,
} from '@/lib/api/escrow-keys';

export type ReadinessStatus =
  | 'ready'
  | 'rotating'
  | 'ready-to-activate'
  | 'testing'
  | 'setup-incomplete'
  | 'action-required'
  | 'revoked'
  | 'unknown';

export interface Readiness {
  status: ReadinessStatus;
  /** The status as a person would say it. Empty for `unknown`, which renders nothing. */
  label: string;
  /** What is true, and what to do about it. Always set when something is missing. */
  detail?: string;
}

/**
 * How alarming each status is, for picking one to show for a party that has
 * several keys. A rotation ranks just above plain readiness: it is worth
 * seeing, but it is a healthy state, not a problem.
 */
const SEVERITY: Record<ReadinessStatus, number> = {
  unknown: -1,
  ready: 0,
  rotating: 1,
  'ready-to-activate': 2,
  testing: 3,
  'setup-incomplete': 4,
  'action-required': 5,
  revoked: 6,
};

/**
 * Purposes that are a capability rather than a requirement. A receiving
 * identity can take deposits all day without a pseudonymisation key — that one
 * is only needed to produce sanitized copies — so its absence must not report
 * the identity as half-configured on every installation that never sanitizes.
 */
const OPTIONAL_PURPOSES: EscrowKeyPurpose[] = ['pseudonymise'];

/** The purposes a party must hold an active key for before deposits can flow. */
export function requiredPurposes(party: Pick<EscrowParty, 'purposes'>): EscrowKeyPurpose[] {
  return party.purposes.filter((p) => !OPTIONAL_PURPOSES.includes(p));
}

const needsProbe = (v: EscrowKeyVersion) => v.material !== 'openpgp-public';

/**
 * Readiness of one key: every version the party holds for a single purpose.
 *
 * A version that failed its test outranks everything, even while an older
 * version keeps deposits flowing — somebody has to act on it, and the detail
 * says whether deposits are affected meanwhile.
 */
export function keyReadiness(purpose: EscrowKeyPurpose, versions: EscrowKeyVersion[]): Readiness {
  const noun = PURPOSE_LABELS[purpose].noun;
  const live = versions.filter((v) => v.state !== 'DESTROYED');
  const active = live.filter((v) => v.state === 'ACTIVE');
  const staged = live.filter((v) => v.state === 'STAGED');
  const failed = staged.find((v) => v.lastProbeAt && !v.lastProbeOk);
  const untested = staged.filter((v) => needsProbe(v) && !v.lastProbeAt);
  const activatable = staged.filter((v) => !needsProbe(v) || v.lastProbeOk);

  if (failed) {
    return {
      status: 'action-required',
      label: 'Action required',
      detail:
        `Version ${failed.version} could not be used by a worker. Check the key and its passphrase, then test it again.` +
        (active.length > 0 ? ' Deposits are unaffected meanwhile: an older version is still active.' : ''),
    };
  }

  if (active.length > 1) {
    return {
      status: 'rotating',
      label: 'Rotation in progress',
      detail: `${active.length} versions are active at once. That is what a rotation looks like — retire the old one once the other side has switched over.`,
    };
  }

  if (active.length === 1) {
    const pending = untested[0]
      ? `Version ${untested[0].version} is being tested.`
      : activatable[0]
        ? `Version ${activatable[0].version} is ready to activate.`
        : undefined;
    return { status: 'ready', label: 'Ready', detail: pending };
  }

  if (untested.length > 0) {
    return {
      status: 'testing',
      label: 'Testing key',
      detail: `We are checking that a worker can read and use version ${untested[0].version} before it is activated.`,
    };
  }

  if (activatable.length > 0) {
    return {
      status: 'ready-to-activate',
      label: 'Ready to activate',
      detail: `Version ${activatable[0].version} is waiting. Nothing uses this ${noun} until it is activated.`,
    };
  }

  if (live.some((v) => v.state === 'REVOKED')) {
    return {
      status: 'revoked',
      label: 'Revoked',
      detail: `The ${noun} was revoked and nothing has replaced it. Add a new one.`,
    };
  }

  if (live.some((v) => v.state === 'HISTORICAL')) {
    return {
      status: 'setup-incomplete',
      label: 'Setup incomplete',
      detail: `The ${noun} was retired and nothing has replaced it. Older deposits still open; new ones will not.`,
    };
  }

  return { status: 'setup-incomplete', label: 'Setup incomplete', detail: `No ${noun} yet.` };
}

/**
 * Readiness of a party from its full version list — the party page, where
 * every version is known. Optional capabilities are left out: see
 * `requiredPurposes`.
 */
export function partyReadiness(
  party: Pick<EscrowParty, 'purposes'>,
  versions: EscrowKeyVersion[]
): Readiness {
  const required = requiredPurposes(party);
  if (required.length === 0) return { status: 'unknown', label: '' };
  const each = required.map((purpose) =>
    keyReadiness(
      purpose,
      versions.filter((v) => v.purpose === purpose)
    )
  );
  return each.reduce((worst, r) => (SEVERITY[r.status] > SEVERITY[worst.status] ? r : worst));
}

/**
 * Readiness from a list row, where only the purposes holding an active key are
 * known. It can tell "ready" from "something is missing" and name what — not a
 * rotation, a pending test or a revocation, which need the versions. A server
 * that did not report active purposes yields `unknown`, and pages show nothing
 * rather than guessing.
 */
export function partyReadinessFromList(party: EscrowParty): Readiness {
  if (!party.activePurposes) return { status: 'unknown', label: '' };
  const required = requiredPurposes(party);
  if (required.length === 0) return { status: 'unknown', label: '' };
  const missing = required.filter((p) => !party.activePurposes!.includes(p));
  if (missing.length === 0) return { status: 'ready', label: 'Ready' };
  return {
    status: 'setup-incomplete',
    label: 'Setup incomplete',
    detail: `No active ${missing.map((p) => PURPOSE_LABELS[p].noun).join(' and no active ')}.`,
  };
}
