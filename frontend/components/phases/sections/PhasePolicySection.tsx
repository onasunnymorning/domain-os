'use client';

import { Phase, ContactDataPolicyType } from '@/lib/types/phase';
import { useState } from 'react';
import { useUpdatePhasePolicy } from '@/lib/hooks/usePhases';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Settings } from 'lucide-react';
import { cn } from '@/lib/utils';

interface PhasePolicySectionProps {
  phase: Phase;
  tldName: string;
  onRefetch: () => void;
}

// Contact data policy badge color
function contactPolicyBadge(value: ContactDataPolicyType | undefined) {
  if (!value) return null;
  const config = {
    mandatory: { variant: 'default' as const, className: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400 border-green-200 dark:border-green-800/50' },
    optional: { variant: 'outline' as const, className: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-amber-200 dark:border-amber-700/50' },
    prohibited: { variant: 'outline' as const, className: 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-700/50' },
  };
  const c = config[value] || config.optional;
  return (
    <Badge variant={c.variant} className={cn('text-[10px] capitalize', c.className)}>
      {value}
    </Badge>
  );
}

export function PhasePolicySection({ phase, tldName, onRefetch }: PhasePolicySectionProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editedPolicy, setEditedPolicy] = useState<Phase['policy'] | null>(null);

  const { mutate: updatePolicy, isPending: isSaving } = useUpdatePhasePolicy(tldName, phase.name);

  const policy = phase.policy;

  const handleEdit = () => {
    setEditedPolicy({ ...policy });
    setIsEditing(true);
  };

  const handleCancel = () => {
    setEditedPolicy(null);
    setIsEditing(false);
  };

  const handleSave = () => {
    if (!editedPolicy) return;
    updatePolicy(editedPolicy, {
      onSuccess: () => {
        setIsEditing(false);
        setEditedPolicy(null);
        onRefetch();
      },
    });
  };

  const handleChange = (field: keyof Phase['policy'], value: number | boolean | string | undefined) => {
    if (!editedPolicy) return;
    setEditedPolicy({ ...editedPolicy, [field]: value });
  };

  const labelsInvalid =
    isEditing && !!editedPolicy && (editedPolicy.minLabelLength ?? 1) > (editedPolicy.maxLabelLength ?? 63);

  return (
    <div className="space-y-4">
      {/* Section Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Phase settings</span>
        </div>
        {!isEditing ? (
          <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={handleEdit}>
            Edit
          </Button>
        ) : (
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7 text-xs" onClick={handleSave} disabled={isSaving || labelsInvalid}>
              {isSaving ? 'Saving...' : 'Save'}
            </Button>
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={handleCancel} disabled={isSaving}>
              Cancel
            </Button>
          </div>
        )}
      </div>

      {/* Domain Label Length */}
      <div className="rounded-lg bg-muted/30 border border-border/40 p-3">
        <div className="text-[10px] text-muted-foreground uppercase tracking-wide mb-1">Domain Label Length</div>
        {!isEditing ? (
          <div className="font-semibold">
            {policy.minLabelLength || 1}–{policy.maxLabelLength || 63} characters
          </div>
        ) : (
          <div className="flex items-center gap-2">
            <input
              type="number" min="1" max="63"
              value={editedPolicy?.minLabelLength || 1}
              onChange={(e) => handleChange('minLabelLength', parseInt(e.target.value))}
              className="w-16 px-2 py-1 border rounded text-sm bg-background text-center"
            />
            <span className="text-muted-foreground">to</span>
            <input
              type="number" min="1" max="63"
              value={editedPolicy?.maxLabelLength || 63}
              onChange={(e) => handleChange('maxLabelLength', parseInt(e.target.value))}
              className="w-16 px-2 py-1 border rounded text-sm bg-background text-center"
            />
            <span className="text-xs text-muted-foreground">chars</span>
          </div>
        )}
        {labelsInvalid && (
          <p className="text-[11px] text-red-700 dark:text-red-400 mt-1">The minimum cannot be above the maximum.</p>
        )}
      </div>

      {/* Rules (toggles) */}
      <div className="grid grid-cols-2 gap-3">
        {/* Requires Validation */}
        <ToggleField
          label="Requires Validation"
          value={isEditing ? editedPolicy?.requiresValidation : policy.requiresValidation}
          isEditing={isEditing}
          onChange={(v) => handleChange('requiresValidation', v)}
        />

        {/* Base Currency */}
        <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-1">
          <div className="text-[10px] text-muted-foreground uppercase tracking-wide">Base Currency</div>
          {!isEditing ? (
            <div className="font-semibold font-mono">{policy.baseCurrency || 'USD'}</div>
          ) : (
            <input
              type="text" maxLength={3}
              value={editedPolicy?.baseCurrency || ''}
              onChange={(e) => handleChange('baseCurrency', e.target.value.toUpperCase())}
              placeholder="USD"
              className="w-16 px-2 py-1 border rounded text-sm bg-background text-center uppercase"
            />
          )}
        </div>
      </div>

      {/* Contact Data Policy */}
      <div className="space-y-2">
        <div className="text-xs font-medium text-orange-700 dark:text-orange-400">Contact Data Policy</div>
        <div className="grid grid-cols-2 gap-2">
          {([
            { key: 'registrantContactDataPolicy' as const, label: 'Registrant' },
            { key: 'techContactDataPolicy' as const, label: 'Tech' },
            { key: 'adminContactDataPolicy' as const, label: 'Admin' },
            { key: 'billingContactDataPolicy' as const, label: 'Billing' },
          ]).map(({ key, label }) => (
            <div key={key} className="rounded-lg bg-muted/30 border border-border/40 p-2.5 space-y-1">
              <div className="text-[10px] text-muted-foreground uppercase tracking-wide">{label}</div>
              {!isEditing ? (
                contactPolicyBadge(policy[key] as ContactDataPolicyType)
              ) : (
                <select
                  value={(editedPolicy?.[key] as string) || 'mandatory'}
                  onChange={(e) => handleChange(key, e.target.value as ContactDataPolicyType)}
                  className="w-full px-2 py-1 border rounded text-sm bg-background"
                >
                  <option value="mandatory">Mandatory</option>
                  <option value="optional">Optional</option>
                  <option value="prohibited">Prohibited</option>
                </select>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ── Toggle Field ───────────────────────────────────────────────────────────

function ToggleField({
  label,
  value,
  isEditing,
  onChange,
}: {
  label: string;
  value: boolean | undefined;
  isEditing: boolean;
  onChange: (v: boolean) => void;
}) {
  const isOn = !!value;
  return (
    <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-2">
      <div className="text-[10px] text-muted-foreground uppercase tracking-wide">{label}</div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={!isEditing}
          onClick={() => isEditing && onChange(!isOn)}
          className={cn(
            'inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors',
            isOn ? 'bg-primary' : 'bg-input',
            isEditing ? 'cursor-pointer' : 'cursor-default opacity-80',
          )}
        >
          <div className={cn(
            'h-4 w-4 rounded-full bg-background shadow-lg transition-transform',
            isOn ? 'translate-x-4' : 'translate-x-0',
          )} />
        </button>
        <span className="text-sm font-medium">
          {isOn ? 'Enabled' : 'Disabled'}
        </span>
      </div>
    </div>
  );
}
