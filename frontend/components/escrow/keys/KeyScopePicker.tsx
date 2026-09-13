'use client';

import { Building2, Globe2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { OperatorScopeSelect } from '@/components/workflows/OperatorScopeSelect';

interface KeyScopePickerProps {
  /** '' with platform=false means nothing chosen yet. */
  tenantId: string;
  platform: boolean;
  onChange: (next: { tenantId: string; platform: boolean }) => void;
}

/**
 * Chooses whose key registry to look at: one operator's (which includes the
 * platform's parties, read-only) or the platform's own, which only staff with
 * the platform permission can open.
 */
export function KeyScopePicker({ tenantId, platform, onChange }: KeyScopePickerProps) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <div className="sm:w-80">
        <OperatorScopeSelect value={platform ? '' : tenantId} onChange={(ryid) => onChange({ tenantId: ryid, platform: false })} />
      </div>
      <span className="text-xs text-muted-foreground">or</span>
      <Button
        variant={platform ? 'default' : 'outline'}
        size="sm"
        onClick={() => onChange({ tenantId: '', platform: true })}
        className="gap-1.5"
      >
        {platform ? <Globe2 className="h-4 w-4" /> : <Building2 className="h-4 w-4" />}
        Platform registry
      </Button>
    </div>
  );
}
