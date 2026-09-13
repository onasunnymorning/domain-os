'use client';

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { EscrowParty } from '@/lib/api/escrow-keys';

const INHERIT = '__inherit__';

/** Chooses a party for one side of an arrangement, or "inherit". */
export function PartySelect({
  parties,
  value,
  onChange,
  inheritLabel,
  disabled,
}: {
  parties: EscrowParty[];
  value?: string;
  onChange: (partyId: string | undefined) => void;
  inheritLabel: string;
  disabled?: boolean;
}) {
  return (
    <Select value={value ?? INHERIT} onValueChange={(v) => onChange(v === INHERIT ? undefined : v)} disabled={disabled}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={INHERIT}>{inheritLabel}</SelectItem>
        {parties.map((p) => (
          <SelectItem key={p.id} value={p.id}>
            {p.name}
            {p.owner.kind === 'platform' ? ' (platform)' : ''}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
