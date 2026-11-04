'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { Rocket } from 'lucide-react';
import { Phase, LaunchSettings, LaunchMode, RegistrationModel, LAUNCH_MODES, REGISTRATION_MODELS } from '@/lib/types/phase';
import { useSetLaunchSettings } from '@/lib/hooks/usePhases';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface PhaseLaunchSectionProps {
  phase: Phase;
  tldName: string;
  onRefetch: () => void;
}

const defaults: LaunchSettings = { mode: 'open', model: 'first-come', acceptSignedMarks: true, acceptCodeMarks: false, retainMarks: false };

/**
 * A launch phase's launch settings (ADR 0024): what kind of launch phase it
 * is (mode), how it registers (first come, or applications the operator
 * allocates) and which trademark evidence a sunrise accepts. The mode is
 * fixed once the phase has started; the model once it has started or has
 * applications filed; nothing changes on an ended phase. Sunrise and claims
 * phases exist and can be priced, but are not yet served over EPP.
 */
export function PhaseLaunchSection({ phase, tldName, onRefetch }: PhaseLaunchSectionProps) {
  const settings: LaunchSettings = { ...defaults, ...(phase.launch ?? {}) };
  const [isEditing, setIsEditing] = useState(false);
  const [edited, setEdited] = useState<LaunchSettings>(settings);
  const { mutate: save, isPending: isSaving } = useSetLaunchSettings(tldName, phase.name);

  const now = new Date();
  const started = new Date(phase.starts) <= now;
  const ended = !!phase.ends && new Date(phase.ends) <= now;
  const mode = LAUNCH_MODES.find((m) => m.value === settings.mode);
  const model = REGISTRATION_MODELS.find((m) => m.value === settings.model);

  const handleSave = () => {
    save(edited, {
      onSuccess: () => {
        setIsEditing(false);
        toast.success(`${phase.name}: launch settings saved.`);
        onRefetch();
      },
      onError: (err: unknown) => {
        const e = err as { response?: { data?: { error?: string } } };
        toast.error(e.response?.data?.error ?? 'Could not change the launch settings');
      },
    });
  };

  const sunriseWithoutMarks = isEditing && edited.mode === 'sunrise' && !edited.acceptSignedMarks && !edited.acceptCodeMarks;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Rocket className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Launch settings</span>
        </div>
        {!isEditing ? (
          !ended && (
            <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => { setEdited(settings); setIsEditing(true); }}>
              Edit
            </Button>
          )
        ) : (
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7 text-xs" onClick={handleSave} disabled={isSaving || sunriseWithoutMarks}>
              {isSaving ? 'Saving...' : 'Save'}
            </Button>
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setIsEditing(false)} disabled={isSaving}>
              Cancel
            </Button>
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-1">
          <div className="text-[10px] text-muted-foreground uppercase tracking-wide">Mode</div>
          {!isEditing ? (
            <div className="flex items-center gap-2">
              <span className="font-semibold">{mode?.label ?? settings.mode}</span>
              {mode && !mode.servedOverEPP && (
                <Badge variant="outline" className="text-[10px] bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-amber-200 dark:border-amber-700/50">
                  not yet over EPP
                </Badge>
              )}
            </div>
          ) : (
            <>
              <select
                aria-label="Mode"
                value={edited.mode}
                disabled={started}
                onChange={(e) => setEdited({ ...edited, mode: e.target.value as LaunchMode })}
                className="w-full px-2 py-1 border rounded text-sm bg-background disabled:opacity-60"
              >
                {LAUNCH_MODES.map((m) => (
                  <option key={m.value} value={m.value}>{m.label}{m.servedOverEPP ? '' : ' (not yet over EPP)'}</option>
                ))}
              </select>
              {started && <p className="text-[11px] text-muted-foreground">Fixed once the phase has started.</p>}
            </>
          )}
          {!isEditing && mode && <p className="text-[11px] text-muted-foreground">{mode.hint}</p>}
        </div>

        <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-1">
          <div className="text-[10px] text-muted-foreground uppercase tracking-wide">Registration model</div>
          {!isEditing ? (
            <div className="font-semibold">{model?.label ?? settings.model}</div>
          ) : (
            <>
              <select
                aria-label="Registration model"
                value={edited.model}
                disabled={started}
                onChange={(e) => setEdited({ ...edited, model: e.target.value as RegistrationModel })}
                className="w-full px-2 py-1 border rounded text-sm bg-background disabled:opacity-60"
              >
                {REGISTRATION_MODELS.map((m) => (
                  <option key={m.value} value={m.value}>{m.label}</option>
                ))}
              </select>
              {started && <p className="text-[11px] text-muted-foreground">Fixed once the phase has started or has applications.</p>}
            </>
          )}
          {!isEditing && model && <p className="text-[11px] text-muted-foreground">{model.hint}</p>}
        </div>
      </div>

      <div className="space-y-2">
        <div className="text-xs font-medium text-orange-700 dark:text-orange-400">Trademark evidence (sunrise)</div>
        <div className="grid grid-cols-3 gap-2">
          {([
            { key: 'acceptSignedMarks' as const, label: 'Signed marks (SMD)' },
            { key: 'acceptCodeMarks' as const, label: 'Code marks' },
            { key: 'retainMarks' as const, label: 'Retain marks' },
          ]).map(({ key, label }) => {
            const on = isEditing ? edited[key] : settings[key];
            return (
              <div key={key} className="rounded-lg bg-muted/30 border border-border/40 p-2.5 space-y-1.5">
                <div className="text-[10px] text-muted-foreground uppercase tracking-wide">{label}</div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={on}
                  aria-label={label}
                  disabled={!isEditing}
                  onClick={() => isEditing && setEdited({ ...edited, [key]: !on })}
                  className={cn(
                    'inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 transition-colors',
                    on ? 'bg-primary' : 'bg-input',
                    isEditing ? 'cursor-pointer' : 'cursor-default opacity-80',
                  )}
                >
                  <div className={cn('h-4 w-4 rounded-full bg-background shadow-lg transition-transform', on ? 'translate-x-4' : 'translate-x-0')} />
                </button>
              </div>
            );
          })}
        </div>
        {sunriseWithoutMarks && (
          <p className="text-[11px] text-red-700 dark:text-red-400">A sunrise must accept signed marks or code marks.</p>
        )}
        <p className="text-[11px] text-muted-foreground">
          Sunrise and claims phases are not yet served over EPP: registrars are answered 2306 until trademark verification lands.
        </p>
      </div>
    </div>
  );
}
