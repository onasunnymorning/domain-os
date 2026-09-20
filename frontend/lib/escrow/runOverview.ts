/**
 * What the run-detail page's overview says at a glance: which pipeline steps
 * the deposit cleared, how many objects it held, and how to name it.
 *
 * Everything here is derived from what the run actually recorded. A step the
 * profile never runs is "not applicable", not a pass: an unsigned deposit has
 * no signature to have verified, and a tick there would claim something the
 * pipeline never checked.
 */

import type {
  EscrowFinding,
  EscrowFindingTally,
  EscrowSummaryCount,
  EscrowValidationRun,
} from '@/lib/api/escrow-runs';

// =============================================================================
// Pipeline steps
// =============================================================================

/**
 * The validator's stages in execution order (`Stage` in
 * internal/application/rdevalidate/codes.go). A run's `stageReached` is the
 * last one it entered.
 */
const STAGE_ORDER = ['intake', 'signature', 'decrypt', 'unpack', 'xml', 'rde'];

export type StepState =
  | 'passed'
  | 'failed'
  /** The service could not decide (outcome ERROR). About us, not the deposit. */
  | 'undecided'
  | 'running'
  | 'not-reached'
  | 'not-applicable';

export interface PipelineStep {
  key: 'intake' | 'signature' | 'decrypt' | 'structure' | 'validation';
  label: string;
  state: StepState;
  /** What the step checks, for a tooltip. */
  hint: string;
  errors: number;
  warnings: number;
}

interface StepSpec {
  key: PipelineStep['key'];
  label: string;
  stages: string[];
  hint: string;
  /** Only the signed profile runs it. */
  signedOnly?: boolean;
  /** Shown only when something went wrong there — a pass is the default. */
  hideWhenPassed?: boolean;
}

const STEPS: StepSpec[] = [
  {
    key: 'intake',
    label: 'Intake',
    stages: ['intake'],
    hint: 'The artifact was found, within size limits, and matches the digest recorded at intake.',
    hideWhenPassed: true,
  },
  {
    key: 'signature',
    label: 'Signature',
    stages: ['signature'],
    hint: 'The detached signature verifies against a trusted key.',
    signedOnly: true,
  },
  {
    key: 'decrypt',
    label: 'Decrypt',
    stages: ['decrypt'],
    hint: 'The deposit was encrypted to a key this service holds and decrypted intact.',
    signedOnly: true,
  },
  {
    key: 'structure',
    label: 'Structure',
    stages: ['unpack', 'xml'],
    hint: 'The archive unpacks safely and holds well-formed deposit XML.',
  },
  {
    key: 'validation',
    label: 'Validation',
    stages: ['rde'],
    hint: 'The deposit content is valid RDE: header, object counts, required objects and references.',
  },
];

type RunForSteps = Pick<
  EscrowValidationRun,
  'profile' | 'outcome' | 'stageReached' | 'findingTally' | 'findings'
>;

/** [errors, warnings] the run recorded in any of `stages`. */
function severityCounts(run: RunForSteps, stages: string[]): [number, number] {
  // The tally is exact; the retained findings are a sample. Runs recorded
  // before the tally existed have only the sample, which is the best there is.
  const rows: Array<Pick<EscrowFindingTally | EscrowFinding, 'severity' | 'stage'> & { count?: number }> =
    run.findingTally?.length ? run.findingTally : (run.findings ?? []);
  let errors = 0;
  let warnings = 0;
  for (const row of rows) {
    if (!stages.includes(row.stage)) continue;
    const n = row.count ?? 1;
    if (row.severity === 'ERROR') errors += n;
    else if (row.severity === 'WARNING') warnings += n;
  }
  return [errors, warnings];
}

export function pipelineSteps(run: RunForSteps): PipelineStep[] {
  const signed = run.profile === 'ryde+sig';

  // A run with no recorded stage that passed necessarily got to the end.
  const stageIdx = STAGE_ORDER.indexOf(run.stageReached ?? '');
  const reached = stageIdx >= 0 ? stageIdx : run.outcome === 'PASS' ? STAGE_ORDER.length - 1 : -1;

  return STEPS.flatMap((spec): PipelineStep[] => {
    const [errors, warnings] = severityCounts(run, spec.stages);
    const idx = spec.stages.map((s) => STAGE_ORDER.indexOf(s));
    const first = Math.min(...idx);
    const last = Math.max(...idx);

    const step = (state: StepState): PipelineStep[] => {
      // A step that only matters when it went wrong stays out of the way.
      if (spec.hideWhenPassed && (state === 'passed' || state === 'not-reached')) return [];
      return [{ key: spec.key, label: spec.label, hint: spec.hint, state, errors, warnings }];
    };

    if (spec.signedOnly && !signed) return step('not-applicable');

    // An ERROR outcome is "we could not decide", so its error findings are
    // about the service and must not read as the deposit failing a check.
    if (errors > 0) return step(run.outcome === 'ERROR' ? 'undecided' : 'failed');

    if (first > reached) return step('not-reached');
    if (reached > last) return step('passed');

    // The run stopped inside this step without recording an error against it.
    if (run.outcome === 'RUNNING') return step('running');
    if (run.outcome === 'ERROR') return step('undecided');
    return step(reached === last ? 'passed' : 'undecided');
  });
}

/** Whether the run got far enough to have read the deposit's objects. */
export function reachedObjects(run: Pick<EscrowValidationRun, 'stageReached' | 'outcome'>): boolean {
  const idx = STAGE_ORDER.indexOf(run.stageReached ?? '');
  return idx >= STAGE_ORDER.indexOf('xml') || (idx < 0 && run.outcome === 'PASS');
}

// =============================================================================
// Object counts
// =============================================================================

export type ObjectGroup = 'domains' | 'contacts' | 'hosts' | 'registrars' | 'other';

export interface ObjectCount {
  key: ObjectGroup;
  label: string;
  /** What the validator actually found in the deposit. */
  observed: number;
  /** What the deposit's own header claimed; undefined when it declared nothing. */
  declared?: number;
  /** The header and the deposit disagree — the RDE_COUNT_MISMATCH fact. */
  mismatch: boolean;
}

const GROUPS: Array<{ key: ObjectGroup; label: string; match: RegExp }> = [
  { key: 'domains', label: 'Domains', match: /rdeDomain-/ },
  { key: 'contacts', label: 'Contacts', match: /rdeContact-/ },
  { key: 'hosts', label: 'Hosts', match: /rdeHost-/ },
  { key: 'registrars', label: 'Registrars', match: /rdeRegistrar-/ },
];

/**
 * Fold a summary's per-namespace counts into the five headline tiles.
 *
 * Namespaces the validator does not count (`not-checked`: rdeEppParams,
 * rdePolicy, ...) are dropped rather than shown as zero — "0 policies" would be
 * wrong, the deposit carries one that nobody walks. IDN table references, NNDN
 * and anything else countable land in "other".
 *
 * All five tiles are always returned. Zero registrars is a fact about a
 * deposit, not an absent column.
 */
export function objectCounts(counts: EscrowSummaryCount[]): ObjectCount[] {
  const tiles = new Map<ObjectGroup, ObjectCount>(
    [...GROUPS.map((g) => ({ key: g.key, label: g.label })), { key: 'other' as const, label: 'Other' }].map(
      (g) => [g.key, { ...g, observed: 0, mismatch: false }]
    )
  );

  for (const row of counts) {
    if (row.status === 'not-checked') continue;
    const key = GROUPS.find((g) => g.match.test(row.uri))?.key ?? 'other';
    const tile = tiles.get(key)!;
    tile.observed += row.observed;
    if (row.declared !== undefined && row.declared !== null) {
      tile.declared = (tile.declared ?? 0) + row.declared;
    }
    if (row.status === 'mismatch') tile.mismatch = true;
  }
  return [...tiles.values()];
}

// =============================================================================
// Naming and time
// =============================================================================

const pad = (n: number) => String(n).padStart(2, '0');

/**
 * Every timestamp on the run page is UTC. A deposit's watermark is a UTC
 * instant — usually midnight — so rendering it in a viewer's local zone shows
 * the previous evening's date to anyone west of Greenwich.
 */
export function formatUtc(iso?: string, withSeconds = true): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const date = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
  const time = `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}${withSeconds ? `:${pad(d.getUTCSeconds())}` : ''}`;
  return `${date} ${time} UTC`;
}

export function formatUtcDate(iso?: string): string | undefined {
  if (!iso) return undefined;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return undefined;
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

/** RFC 8909 deposit types, in the words an operator uses. */
export function depositKindLabel(kind?: string): string {
  switch (kind?.toUpperCase()) {
    case 'FULL':
      return 'Full deposit';
    case 'INCR':
      return 'Incremental deposit';
    case 'DIFF':
      return 'Differential deposit';
    case undefined:
    case '':
      return 'Deposit';
    default:
      return `${kind} deposit`;
  }
}

/**
 * "Deposit 2026-07-16 / TI8QO0": the deposit's own date and its own id, as the
 * depositor wrote them into the header. A run that stopped before the header
 * was read has neither, so it falls back to the id we assigned at intake.
 */
export function depositHeading(
  run: Pick<EscrowValidationRun, 'rdeWatermark' | 'rdeDepositId' | 'depositId'>
): string {
  const parts = [formatUtcDate(run.rdeWatermark), run.rdeDepositId].filter(Boolean);
  return `Deposit ${parts.length ? parts.join(' / ') : run.depositId.slice(0, 8)}`;
}

export function resendLabel(resend: number): string {
  return resend > 0 ? `#${resend}` : 'No';
}
