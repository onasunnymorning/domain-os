'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { launchWorkflow, type WorkflowMeta } from '@/lib/api/workflows';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';

interface WorkflowLaunchFormProps {
  workflow: WorkflowMeta | null;
  onClose: () => void;
  onLaunched: (run: WorkflowRun) => void;
}

// =============================================================================
// Per-workflow form bodies
// =============================================================================

function EscrowStagingForm({
  params,
  onChange,
}: {
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="tld">TLD</Label>
        <Input
          id="tld"
          placeholder="e.g. com"
          value={params.tld ?? ''}
          onChange={(e) => onChange({ ...params, tld: e.target.value })}
        />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="objectKey">Object Key</Label>
        <Input
          id="objectKey"
          placeholder="e.g. escrow/2024/full/com.xml.gz"
          value={params.objectKey ?? ''}
          onChange={(e) => onChange({ ...params, objectKey: e.target.value })}
        />
      </div>
    </div>
  );
}

function EscrowIngestionForm({
  params,
  onChange,
}: {
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="tld">TLD</Label>
        <Input
          id="tld"
          placeholder="e.g. com"
          value={params.tld ?? ''}
          onChange={(e) => onChange({ ...params, tld: e.target.value })}
        />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="stagedDbKey">Staged DB Key</Label>
        <Input
          id="stagedDbKey"
          placeholder="e.g. staged/com/2024-01-15"
          value={params.stagedDbKey ?? ''}
          onChange={(e) => onChange({ ...params, stagedDbKey: e.target.value })}
        />
      </div>
    </div>
  );
}

function TldCleanupForm({
  params,
  onChange,
}: {
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  return (
    <div className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="tldName">TLD Name</Label>
        <Input
          id="tldName"
          placeholder="e.g. com"
          value={params.tldName ?? ''}
          onChange={(e) => onChange({ ...params, tldName: e.target.value })}
        />
      </div>
      <div className="flex items-center gap-3">
        <Switch
          id="keepTLDAndPhases"
          checked={params.keepTLDAndPhases ?? false}
          onCheckedChange={(checked) =>
            onChange({ ...params, keepTLDAndPhases: checked })
          }
        />
        <Label htmlFor="keepTLDAndPhases">Keep TLD and Phases</Label>
      </div>
    </div>
  );
}

function SyncRegistrarsForm({
  params,
  onChange,
}: {
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor="batchSize">Batch Size</Label>
      <Input
        id="batchSize"
        type="number"
        min={1}
        max={10000}
        placeholder="100"
        value={params.batchSize ?? 100}
        onChange={(e) =>
          onChange({ ...params, batchSize: parseInt(e.target.value, 10) || 100 })
        }
      />
    </div>
  );
}

function ZeroParamConfirmation({ name }: { name: string }) {
  return (
    <p className="text-muted-foreground text-sm">
      Run <span className="text-foreground font-medium">{name}</span> now?
    </p>
  );
}

// =============================================================================
// Form body router
// =============================================================================

function FormBody({
  workflowKey,
  workflowName,
  params,
  onChange,
}: {
  workflowKey: string;
  workflowName: string;
  params: Record<string, any>;
  onChange: (p: Record<string, any>) => void;
}) {
  switch (workflowKey) {
    case 'escrow-staging':
      return <EscrowStagingForm params={params} onChange={onChange} />;
    case 'escrow-ingestion':
      return <EscrowIngestionForm params={params} onChange={onChange} />;
    case 'tld-cleanup':
      return <TldCleanupForm params={params} onChange={onChange} />;
    case 'sync-registrars':
      return <SyncRegistrarsForm params={params} onChange={onChange} />;
    default:
      return <ZeroParamConfirmation name={workflowName} />;
  }
}

// =============================================================================
// Main Dialog Component
// =============================================================================

export function WorkflowLaunchForm({
  workflow,
  onClose,
  onLaunched,
}: WorkflowLaunchFormProps) {
  const [params, setParams] = useState<Record<string, any>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleClose = () => {
    setParams({});
    setIsSubmitting(false);
    onClose();
  };

  const handleSubmit = async () => {
    if (!workflow) return;

    setIsSubmitting(true);
    try {
      const result = await launchWorkflow(workflow.key, params);

      const run: WorkflowRun = {
        workflowId: result.workflowId,
        runId: result.runId,
        type: workflow.key,
        displayName: workflow.name,
        status: 'RUNNING',
        temporalUrl: result.url,
        startedAt: new Date().toISOString(),
        steps: result.steps,
        params,
      };

      onLaunched(run);
      toast.success(`Workflow "${workflow.name}" launched successfully`, {
        description: `ID: ${result.workflowId}`,
      });
      handleClose();
    } catch (error: any) {
      const message =
        error?.response?.data?.message || error?.message || 'Failed to launch workflow';
      toast.error('Launch failed', { description: message });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={workflow !== null} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{workflow?.name ?? 'Launch Workflow'}</DialogTitle>
          <DialogDescription>
            {workflow?.description ?? 'Configure and launch a workflow'}
          </DialogDescription>
        </DialogHeader>

        {workflow && (
          <FormBody
            workflowKey={workflow.key}
            workflowName={workflow.name}
            params={params}
            onChange={setParams}
          />
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting && <Loader2 className="size-4 animate-spin" />}
            Launch Workflow
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
