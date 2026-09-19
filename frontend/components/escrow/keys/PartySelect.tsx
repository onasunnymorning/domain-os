'use client';

import Link from 'next/link';
import { AlertTriangle } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { lacksActiveKey, type EscrowKeyPurpose, type EscrowParty } from '@/lib/api/escrow-keys';

const INHERIT = '__inherit__';

/**
 * Chooses the source, or our identity, for one side — or leaves the side to
 * follow the default above it.
 *
 * With `needsPurpose` it also marks the ones holding no active key for that
 * side. Choosing one stays allowed: a source is routinely added before the key
 * it will sign with arrives. But it is the state in which every deposit fails,
 * and nothing else on the page would say so until a validation run did.
 */
export function PartySelect({
  parties,
  value,
  onChange,
  inheritLabel,
  disabled,
  needsPurpose,
}: {
  parties: EscrowParty[];
  value?: string;
  onChange: (partyId: string | undefined) => void;
  inheritLabel: string;
  disabled?: boolean;
  needsPurpose?: EscrowKeyPurpose;
}) {
  const unready = (p: EscrowParty) => needsPurpose !== undefined && lacksActiveKey(p, needsPurpose);

  return (
    <Select value={value ?? INHERIT} onValueChange={(v) => onChange(v === INHERIT ? undefined : v)} disabled={disabled}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={INHERIT}>{inheritLabel}</SelectItem>
        {parties.map((p) => (
          <SelectItem key={p.id} value={p.id}>
            <span className="flex items-center gap-1.5">
              {p.name}
              {p.owner.kind === 'platform' ? ' (platform)' : ''}
              {unready(p) && (
                <span className="flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                  <AlertTriangle className="h-3 w-3" />
                  no active key
                </span>
              )}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * The warning under a chosen party that holds no ACTIVE key for the side it
 * fills. It names the consequence, because "no active key" alone does not say
 * that deposits will fail.
 */
export function MissingKeyNotice({
  party,
  purpose,
  side,
  href,
}: {
  party?: EscrowParty;
  purpose: EscrowKeyPurpose;
  side: 'depositor' | 'receiver';
  href: string;
}) {
  if (!lacksActiveKey(party, purpose)) return null;
  const consequence =
    side === 'depositor'
      ? 'every deposit it signs fails verification until its public verification key is added and activated'
      : 'no deposit encrypted to it can be opened until its decryption key is imported and activated';
  return (
    <p className="flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-400">
      <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>
        <Link href={href} className="font-medium underline underline-offset-2">
          {party?.name}
        </Link>{' '}
        has no active key for this side: {consequence}.
      </span>
    </p>
  );
}
